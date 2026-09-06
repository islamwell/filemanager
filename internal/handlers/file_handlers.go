package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
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
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, "/?err=No+file+selected", http.StatusSeeOther)
		return
	}
	defer file.Close()

	if header.Size > h.Config.MaxFileSize {
		http.Redirect(w, r, "/?err=File+exceeds+maximum+allowed+size", http.StatusSeeOther)
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
		http.Redirect(w, r, "/?err=Internal+storage+error", http.StatusSeeOther)
		return
	}

	diskPath := filepath.Join(subDir, fileUUID+".dat")
	out, err := os.OpenFile(diskPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		log.Printf("[UPLOAD ERROR] Failed to create file on disk: %v", err)
		http.Redirect(w, r, "/?err=Failed+to+save+file+on+disk", http.StatusSeeOther)
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
			http.Redirect(w, r, "/?err=Write+error", http.StatusSeeOther)
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
		http.Redirect(w, r, "/?err=Failed+during+file+transfer", http.StatusSeeOther)
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
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?msg=File+uploaded+successfully", http.StatusSeeOther)
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
