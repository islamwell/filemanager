package handlers

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"filemanager/internal/auth"
	"filemanager/internal/middleware"
	"filemanager/internal/models"
	"filemanager/internal/store"
)

type ShareHandler struct {
	*BaseHandler
}

func NewShareHandler(base *BaseHandler) *ShareHandler {
	return &ShareHandler{BaseHandler: base}
}

// CreateShareLink creates a public share link with an optional expiration duration.
func (h *ShareHandler) CreateShareLink(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	itemID := strings.TrimSpace(r.FormValue("item_id"))
	itemType := models.ItemType(strings.TrimSpace(r.FormValue("item_type")))
	expiryOption := strings.TrimSpace(r.FormValue("expiry"))

	if itemID == "" || (itemType != models.ItemTypeFile && itemType != models.ItemTypeSnippet) {
		http.Redirect(w, r, "/?err=Invalid+share+request", http.StatusSeeOther)
		return
	}

	// Verify item ownership
	if itemType == models.ItemTypeFile {
		if _, err := h.Store.GetFileByID(itemID, user.ID); err != nil {
			http.Redirect(w, r, "/?err=File+not+found", http.StatusSeeOther)
			return
		}
	} else {
		if _, err := h.Store.GetSnippetByID(itemID, user.ID); err != nil {
			http.Redirect(w, r, "/?err=Snippet+not+found", http.StatusSeeOther)
			return
		}
	}

	var expiresAt *time.Time
	now := time.Now().UTC()
	switch expiryOption {
	case "1h":
		exp := now.Add(1 * time.Hour)
		expiresAt = &exp
	case "24h":
		exp := now.Add(24 * time.Hour)
		expiresAt = &exp
	case "7d":
		exp := now.Add(7 * 24 * time.Hour)
		expiresAt = &exp
	case "30d":
		exp := now.Add(30 * 24 * time.Hour)
		expiresAt = &exp
	default:
		// "never" or empty leaves expiresAt as nil
	}

	token, err := auth.GenerateShareToken()
	if err != nil {
		http.Redirect(w, r, "/?err=Failed+to+generate+share+token", http.StatusSeeOther)
		return
	}

	link := &models.ShareLink{
		Token:     token,
		UserID:    user.ID,
		ItemID:    itemID,
		ItemType:  itemType,
		ExpiresAt: expiresAt,
	}

	if err := h.Store.CreateShareLink(link); err != nil {
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// DeleteShareLink revokes a share token immediately.
func (h *ShareHandler) DeleteShareLink(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	token := r.PathValue("token")
	_ = h.Store.DeleteShareLink(token, user.ID)

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// ViewSharedItem provides unauthenticated public access to a shared file or snippet.
func (h *ShareHandler) ViewSharedItem(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	shareLink, err := h.Store.GetShareLink(token)
	if err != nil {
		if errors.Is(err, store.ErrShareExpired) {
			h.Render(w, r, "share_view.html", "Link Expired - File Manager", map[string]interface{}{
				"Expired": true,
			}, "This share link has expired.", "")
			return
		}
		http.NotFound(w, r)
		return
	}

	// Increment view count asynchronously/safely
	_ = h.Store.IncrementShareView(token)

	h.Render(w, r, "share_view.html", "Shared Item - File Manager", map[string]interface{}{
		"Share":   shareLink,
		"Expired": false,
	}, "", "")
}

// DownloadSharedFile allows direct public download of a shared file.
func (h *ShareHandler) DownloadSharedFile(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	shareLink, err := h.Store.GetShareLink(token)
	if err != nil || shareLink.ItemType != models.ItemTypeFile || shareLink.File == nil {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(shareLink.File.StoredPath)
	if err != nil {
		http.Error(w, "File not available on disk", http.StatusGone)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Failed to stat file", http.StatusInternalServerError)
		return
	}

	_ = h.Store.IncrementShareView(token)
	SetContentDisposition(w, "attachment", shareLink.File.OriginalFilename)
	w.Header().Set("Content-Type", shareLink.File.MimeType)
	http.ServeContent(w, r, shareLink.File.OriginalFilename, stat.ModTime(), f)
}

// AudioSharedStream streams MP3 audio content for public share links with byte ranges.
func (h *ShareHandler) AudioSharedStream(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	shareLink, err := h.Store.GetShareLink(token)
	if err != nil || shareLink.ItemType != models.ItemTypeFile || shareLink.File == nil {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(shareLink.File.StoredPath)
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
	SetContentDisposition(w, "inline", shareLink.File.OriginalFilename)
	http.ServeContent(w, r, shareLink.File.OriginalFilename, stat.ModTime(), f)
}
