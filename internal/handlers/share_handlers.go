package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
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

	customAlias := strings.TrimSpace(r.FormValue("custom_alias"))
	var token string
	if customAlias != "" {
		// Validate custom alias length (3 - 64 chars) and allowed characters
		if len(customAlias) < 3 || len(customAlias) > 64 {
			http.Redirect(w, r, "/?err=Custom+alias+must+be+between+3+and+64+characters", http.StatusSeeOther)
			return
		}
		for _, ch := range customAlias {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
				http.Redirect(w, r, "/?err=Custom+alias+can+only+contain+alphanumeric+characters,+hyphens,+and+underscores", http.StatusSeeOther)
				return
			}
		}

		// Disallow reserved paths
		reserved := map[string]bool{
			"download": true, "audio": true, "static": true, "login": true, "register": true,
			"logout": true, "search": true, "files": true, "snippets": true, "categories": true, "items": true,
		}
		if reserved[strings.ToLower(customAlias)] {
			http.Redirect(w, r, "/?err=This+custom+alias+is+reserved.+Please+choose+another.", http.StatusSeeOther)
			return
		}
		token = customAlias
	} else {
		var err error
		token, err = auth.GenerateShareToken()
		if err != nil {
			http.Redirect(w, r, "/?err=Failed+to+generate+share+token", http.StatusSeeOther)
			return
		}
	}

	password := strings.TrimSpace(r.FormValue("password"))
	var passwordHash string
	if password != "" {
		hash, err := auth.HashSharePassword(password)
		if err != nil {
			http.Redirect(w, r, "/?err=Failed+to+hash+share+password", http.StatusSeeOther)
			return
		}
		passwordHash = hash
	}

	maxViews, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("max_views")))
	if maxViews < 0 {
		maxViews = 0
	}

	link := &models.ShareLink{
		Token:        token,
		UserID:       user.ID,
		ItemID:       itemID,
		ItemType:     itemType,
		PasswordHash: passwordHash,
		MaxViews:     maxViews,
		ExpiresAt:    expiresAt,
	}

	if err := h.Store.CreateShareLink(link); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "already exists") {
			http.Redirect(w, r, "/?err=Custom+link+alias+is+already+taken.+Please+choose+another.", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, safeRedirectURL(r.Header.Get("Referer"), r.Host), http.StatusSeeOther)
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

	http.Redirect(w, r, safeRedirectURL(r.Header.Get("Referer"), r.Host), http.StatusSeeOther)
}

func safeRedirectURL(raw, host string) string {
	if raw == "" {
		return "/"
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	if u, err := url.Parse(raw); err == nil {
		if u.Host == "" || u.Host == host {
			if u.Path != "" {
				return u.RequestURI()
			}
		}
	}
	return "/"
}

func isShareUnlocked(r *http.Request, token string) bool {
	cookie, err := r.Cookie("unlock_" + token)
	return err == nil && cookie.Value == "1"
}

// ViewSharedItem provides unauthenticated public access to a shared file or snippet.
func (h *ShareHandler) ViewSharedItem(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	shareLink, err := h.Store.GetShareLink(token)
	if err != nil {
		if errors.Is(err, store.ErrShareExpired) {
			h.Render(w, r, "share_view.html", "Link Expired - NRQ", map[string]interface{}{
				"Expired": true,
			}, "This share link has expired.", "")
			return
		}
		http.NotFound(w, r)
		return
	}

	if shareLink.IsExhausted() {
		h.Render(w, r, "share_view.html", "Link Expired - NRQ", map[string]interface{}{
			"Exhausted": true,
		}, "This share link has reached its maximum view limit.", "")
		return
	}

	if shareLink.HasPassword() && !isShareUnlocked(r, token) {
		h.Render(w, r, "share_view.html", "Protected Link - NRQ", map[string]interface{}{
			"PasswordRequired": true,
			"Token":            token,
		}, "", "")
		return
	}

	// Increment view count
	_ = h.Store.IncrementShareView(token)

	h.Render(w, r, "share_view.html", "Shared Item - NRQ", map[string]interface{}{
		"Share":   shareLink,
		"Expired": false,
	}, "", "")
}

// UnlockShareLink verifies password for a protected link and sets an access cookie.
func (h *ShareHandler) UnlockShareLink(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	shareLink, err := h.Store.GetShareLink(token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if shareLink.IsExhausted() {
		h.Render(w, r, "share_view.html", "Link Expired - NRQ", map[string]interface{}{
			"Exhausted": true,
		}, "This share link has reached its maximum view limit.", "")
		return
	}

	password := r.FormValue("password")
	if !shareLink.HasPassword() || auth.CheckPassword(password, shareLink.PasswordHash) {
		http.SetCookie(w, &http.Cookie{
			Name:     "unlock_" + token,
			Value:    "1",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/share/"+token, http.StatusSeeOther)
		return
	}

	h.Render(w, r, "share_view.html", "Protected Link - NRQ", map[string]interface{}{
		"PasswordRequired": true,
		"Token":            token,
	}, "Incorrect password. Please try again.", "")
}

// DownloadSharedFile allows direct public download of a shared file.
func (h *ShareHandler) DownloadSharedFile(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	shareLink, err := h.Store.GetShareLink(token)
	if err != nil || shareLink.ItemType != models.ItemTypeFile || shareLink.File == nil {
		http.NotFound(w, r)
		return
	}

	if shareLink.IsExhausted() {
		http.Error(w, "Share link has reached its view limit", http.StatusGone)
		return
	}

	if shareLink.HasPassword() && !isShareUnlocked(r, token) {
		http.Redirect(w, r, "/share/"+token, http.StatusSeeOther)
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

	if shareLink.IsExhausted() {
		http.Error(w, "Share link has reached its view limit", http.StatusGone)
		return
	}

	if shareLink.HasPassword() && !isShareUnlocked(r, token) {
		http.Redirect(w, r, "/share/"+token, http.StatusSeeOther)
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

// ViewSharedMediaInline streams shared images, PDFs, and videos inline with byte-range support.
func (h *ShareHandler) ViewSharedMediaInline(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	shareLink, err := h.Store.GetShareLink(token)
	if err != nil || shareLink.ItemType != models.ItemTypeFile || shareLink.File == nil {
		http.NotFound(w, r)
		return
	}

	if shareLink.IsExhausted() {
		http.Error(w, "Share link has reached its view limit", http.StatusGone)
		return
	}

	if shareLink.HasPassword() && !isShareUnlocked(r, token) {
		http.Redirect(w, r, "/share/"+token, http.StatusSeeOther)
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

	w.Header().Set("Content-Type", shareLink.File.MimeType)
	w.Header().Set("Accept-Ranges", "bytes")
	SetContentDisposition(w, "inline", shareLink.File.OriginalFilename)
	http.ServeContent(w, r, shareLink.File.OriginalFilename, stat.ModTime(), f)
}
