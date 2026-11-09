package handlers

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"filemanager/internal/middleware"
	"filemanager/internal/models"
	"filemanager/internal/store"
	"github.com/google/uuid"
)

type FileHandler struct {
	*BaseHandler
}

func NewFileHandler(base *BaseHandler) *FileHandler {
	return &FileHandler{BaseHandler: base}
}

func (h *FileHandler) sendUploadResponse(w http.ResponseWriter, r *http.Request, errMsg string, fileItem *models.FileItem) {
	isXHR := r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json")
	if isXHR {
		w.Header().Set("Content-Type", "application/json")
		if errMsg != "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   errMsg,
			})
		} else {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":  true,
				"message":  "File uploaded successfully",
				"file_id":  fileItem.ID,
				"filename": fileItem.OriginalFilename,
			})
		}
		return
	}

	if errMsg != "" {
		http.Redirect(w, r, "/?err="+url.QueryEscape(errMsg), http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/?msg=File+uploaded+successfully", http.StatusSeeOther)
	}
}

// UploadFile handles streaming multipart file uploads directly to disk.
func (h *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Limit request body size to max configured file size + 1MB overhead for form boundaries
	r.Body = http.MaxBytesReader(w, r.Body, h.Config.MaxFileSize+1024*1024)
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max in-memory parse buffer
		log.Printf("[UPLOAD ERROR] ParseMultipartForm: %v", err)
		h.sendUploadResponse(w, r, err.Error(), nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.sendUploadResponse(w, r, "No file selected", nil)
		return
	}
	defer file.Close()

	if header.Size > h.Config.MaxFileSize {
		h.sendUploadResponse(w, r, "File exceeds maximum allowed size", nil)
		return
	}

	rawFilename := header.Filename
	if customName := strings.TrimSpace(r.FormValue("filename")); customName != "" {
		rawFilename = customName
	}
	sanitizedName := SanitizeFilename(rawFilename)
	categoryIDStr := strings.TrimSpace(r.FormValue("category_id"))
	var categoryID *string
	if categoryIDStr != "" {
		categoryID = &categoryIDStr
	}

	tagsRaw := r.FormValue("tags")
	var tags []string
	if tagsRaw != "" {
		for _, t := range strings.Split(tagsRaw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, t)
			}
		}
	}

	// Generate storage location with subfolder shard to avoid filesystem directory file count limits
	fileUUID := uuid.New().String()
	subDir := filepath.Join(h.Config.StorageDir, fileUUID[:2])
	if err := os.MkdirAll(subDir, 0750); err != nil {
		log.Printf("[UPLOAD ERROR] Failed to create storage dir: %v", err)
		h.sendUploadResponse(w, r, "Internal storage error", nil)
		return
	}

	diskPath := filepath.Join(subDir, fileUUID+".dat")
	out, err := os.OpenFile(diskPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		log.Printf("[UPLOAD ERROR] Failed to create file on disk: %v", err)
		h.sendUploadResponse(w, r, "Failed to save file on disk", nil)
		return
	}
	defer out.Close()

	// Sniff MIME type from the first 512 bytes while streaming
	buffer := make([]byte, 512)
	n, _ := io.ReadFull(file, buffer)
	mimeType := http.DetectContentType(buffer[:n])

	// If extension is .mp3, ensure audio/mpeg is set
	ext := strings.ToLower(filepath.Ext(sanitizedName))
	if ext == ".mp3" {
		mimeType = "audio/mpeg"
	}

	hasher := sha256.New()
	// Write initial sniffed bytes to disk and hasher
	if n > 0 {
		if _, err := out.Write(buffer[:n]); err != nil {
			_ = os.Remove(diskPath)
			h.sendUploadResponse(w, r, "Write error", nil)
			return
		}
		hasher.Write(buffer[:n])
	}

	// Stream remainder of the file in 32KB chunks directly to disk & compute SHA-256
	multiWriter := io.MultiWriter(out, hasher)
	written, err := io.Copy(multiWriter, file)
	if err != nil {
		_ = os.Remove(diskPath)
		log.Printf("[UPLOAD ERROR] Streaming copy failed: %v", err)
		h.sendUploadResponse(w, r, "Failed during file transfer", nil)
		return
	}

	totalSize := int64(n) + written
	checksum := hex.EncodeToString(hasher.Sum(nil))

	fileItem := &models.FileItem{
		ID:               fileUUID,
		UserID:           user.ID,
		CategoryID:       categoryID,
		OriginalFilename: sanitizedName,
		StoredPath:       diskPath,
		FileSize:         totalSize,
		MimeType:         mimeType,
		Sha256Checksum:   checksum,
		Tags:             tags,
	}

	if err := h.Store.CreateFile(fileItem); err != nil {
		_ = os.Remove(diskPath)
		log.Printf("[UPLOAD ERROR] Store.CreateFile: %v", err)
		h.sendUploadResponse(w, r, err.Error(), nil)
		return
	}

	h.sendUploadResponse(w, r, "", fileItem)
}

// DownloadFile serves the file with proper Content-Disposition attachment headers.
func (h *FileHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	fileItem, err := h.Store.GetFileByID(id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(fileItem.StoredPath)
	if err != nil {
		log.Printf("[DOWNLOAD ERROR] Disk file missing: %s (%v)", fileItem.StoredPath, err)
		http.Error(w, "File content is missing from storage", http.StatusGone)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Failed to stat file", http.StatusInternalServerError)
		return
	}

	SetContentDisposition(w, "attachment", fileItem.OriginalFilename)
	w.Header().Set("Content-Type", fileItem.MimeType)
	http.ServeContent(w, r, fileItem.OriginalFilename, stat.ModTime(), f)
}

// AudioStream serves MP3 files supporting HTTP byte-range requests (206 Partial Content).
func (h *FileHandler) AudioStream(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	fileItem, err := h.Store.GetFileByID(id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(fileItem.StoredPath)
	if err != nil {
		http.Error(w, "Audio file missing", http.StatusGone)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Failed to stat audio file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Accept-Ranges", "bytes")
	SetContentDisposition(w, "inline", fileItem.OriginalFilename)
	http.ServeContent(w, r, fileItem.OriginalFilename, stat.ModTime(), f)
}

// DeleteFile removes a file record and purges it from disk.
func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	fileItem, err := h.Store.DeleteFile(id, user.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Redirect(w, r, "/?err=File+not+found", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	// Purge from physical disk
	if fileItem != nil && fileItem.StoredPath != "" {
		_ = os.Remove(fileItem.StoredPath)
	}

	http.Redirect(w, r, "/?msg=File+deleted+successfully", http.StatusSeeOther)
}

// ViewFileInline serves media files (images, PDFs, videos, text) with inline Content-Disposition and byte-range streaming.
func (h *FileHandler) ViewFileInline(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	fileItem, err := h.Store.GetFileByID(id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(fileItem.StoredPath)
	if err != nil {
		log.Printf("[VIEW ERROR] Disk file missing: %s (%v)", fileItem.StoredPath, err)
		http.Error(w, "File content is missing from storage", http.StatusGone)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Failed to stat file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", fileItem.MimeType)
	w.Header().Set("Accept-Ranges", "bytes")
	SetContentDisposition(w, "inline", fileItem.OriginalFilename)
	http.ServeContent(w, r, fileItem.OriginalFilename, stat.ModTime(), f)
}

// MoveFile updates the category/folder of a file.
func (h *FileHandler) MoveFile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	categoryIDStr := strings.TrimSpace(r.FormValue("category_id"))
	var newCatID *string
	if categoryIDStr != "" {
		newCatID = &categoryIDStr
	}

	if err := h.Store.MoveFile(id, user.ID, newCatID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Redirect(w, r, "/?err=File+not+found", http.StatusSeeOther)
			return
		}
		if errors.Is(err, store.ErrCategoryNotFound) {
			http.Redirect(w, r, "/?err=Target+folder+not+found", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?msg=File+moved+successfully", http.StatusSeeOther)
}

// RenameFile renames an existing file.
func (h *FileHandler) RenameFile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	newName := strings.TrimSpace(r.FormValue("filename"))
	if newName == "" {
		http.Redirect(w, r, "/?err=Filename+cannot+be+empty", http.StatusSeeOther)
		return
	}
	newName = SanitizeFilename(newName)

	if err := h.Store.RenameFile(id, user.ID, newName); err != nil {
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
		return
	}

	http.Redirect(w, r, "/?msg=File+renamed+successfully", http.StatusSeeOther)
}

// BatchZipDownload streams selected files as a single zip archive.
func (h *FileHandler) BatchZipDownload(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?err=Invalid+form+data", http.StatusSeeOther)
		return
	}

	ids := r.Form["file_ids"]
	if len(ids) == 0 {
		if raw := r.FormValue("file_ids"); raw != "" {
			ids = strings.Split(raw, ",")
		}
	}

	if len(ids) == 0 {
		http.Redirect(w, r, "/?err=No+files+selected+for+download", http.StatusSeeOther)
		return
	}

	files, err := h.Store.GetFilesByIDs(ids, user.ID)
	if err != nil || len(files) == 0 {
		http.Redirect(w, r, "/?err=No+valid+files+found", http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"files.zip\"")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	seenNames := make(map[string]int)

	for _, f := range files {
		diskFile, err := os.Open(f.StoredPath)
		if err != nil {
			continue
		}

		entryName := f.OriginalFilename
		if count, ok := seenNames[entryName]; ok {
			seenNames[entryName]++
			ext := filepath.Ext(entryName)
			base := strings.TrimSuffix(entryName, ext)
			entryName = fmt.Sprintf("%s (%d)%s", base, count+1, ext)
		} else {
			seenNames[entryName] = 1
		}

		header := &zip.FileHeader{
			Name:     entryName,
			Method:   zip.Deflate,
			Modified: f.UpdatedAt,
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			diskFile.Close()
			continue
		}

		_, _ = io.Copy(writer, diskFile)
		diskFile.Close()
	}
}

// BatchDeleteFiles removes multiple files and deletes them from disk.
func (h *FileHandler) BatchDeleteFiles(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?err=Invalid+request", http.StatusSeeOther)
		return
	}

	ids := r.Form["file_ids"]
	if len(ids) == 0 {
		if raw := r.FormValue("file_ids"); raw != "" {
			ids = strings.Split(raw, ",")
		}
	}

	if len(ids) == 0 {
		http.Redirect(w, r, "/?err=No+files+selected", http.StatusSeeOther)
		return
	}

	deletedFiles, err := h.Store.DeleteFiles(ids, user.ID)
	if err != nil {
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	for _, f := range deletedFiles {
		if f != nil && f.StoredPath != "" {
			_ = os.Remove(f.StoredPath)
		}
	}

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"success":true,"count":%d}`, len(deletedFiles))))
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/?msg=%d+files+deleted+successfully", len(deletedFiles)), http.StatusSeeOther)
}

// BatchMoveFiles moves multiple files to a folder/category.
func (h *FileHandler) BatchMoveFiles(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?err=Invalid+request", http.StatusSeeOther)
		return
	}

	ids := r.Form["file_ids"]
	if len(ids) == 0 {
		if raw := r.FormValue("file_ids"); raw != "" {
			ids = strings.Split(raw, ",")
		}
	}

	if len(ids) == 0 {
		http.Redirect(w, r, "/?err=No+files+selected", http.StatusSeeOther)
		return
	}

	catIDStr := strings.TrimSpace(r.FormValue("category_id"))
	var newCatID *string
	if catIDStr != "" {
		newCatID = &catIDStr
	}

	if err := h.Store.MoveFiles(ids, user.ID, newCatID); err != nil {
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/?msg=%d+files+moved+successfully", len(ids)), http.StatusSeeOther)
}

