package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"filemanager/internal/config"
	"filemanager/internal/middleware"
	"filemanager/internal/models"
	"filemanager/internal/services"
	"filemanager/internal/store"
)

// BaseHandler provides shared state, template rendering, and helper methods.
type BaseHandler struct {
	Store     store.Store
	Config    *config.Config
	Templates map[string]*template.Template
}

// PageData contains the standard template view model passed to all server-rendered pages.
type PageData struct {
	Title         string
	User          *models.User
	CSRFToken     string
	FooterVersion string
	FlashError    string
	FlashSuccess  string
	SearchQuery   string
	Data          interface{}
}

// TemplateFuncs provides custom template helper functions.
var TemplateFuncs = template.FuncMap{
	"formatDate": func(t time.Time) string {
		if t.IsZero() {
			return "-"
		}
		return t.Format("2006-01-02 15:04")
	},
	"formatDateShort": func(t time.Time) string {
		if t.IsZero() {
			return "-"
		}
		return t.Format("Jan 02, 2006")
	},
	"formatSize": func(size int64) string {
		f := &models.FileItem{FileSize: size}
		return f.FormattedSize()
	},
	"joinTags": func(tags []string) string {
		return strings.Join(tags, ", ")
	},
	"safeHTML": func(s string) template.HTML {
		return template.HTML(s)
	},
	"sub": func(a, b int) int {
		return a - b
	},
}

// NewBaseHandler constructs the base handler and parses templates from the provided filesystem per page.
func NewBaseHandler(st store.Store, cfg *config.Config, templateFS fs.FS) (*BaseHandler, error) {
	pageTemplates := []string{
		"login.html",
		"register.html",
		"dashboard.html",
		"snippet_new.html",
		"snippet_edit.html",
		"snippet_view.html",
		"share_view.html",
		"search_results.html",
		"admin_users.html",
		"offline.html",
	}

	tmpls := make(map[string]*template.Template)
	for _, page := range pageTemplates {
		t, err := template.New("base.html").Funcs(TemplateFuncs).ParseFS(templateFS, "base.html", page)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", page, err)
		}
		tmpls[page] = t
	}

	return &BaseHandler{
		Store:     st,
		Config:    cfg,
		Templates: tmpls,
	}, nil
}

// OfflineView renders the offline fallback page.
func (h *BaseHandler) OfflineView(w http.ResponseWriter, r *http.Request) {
	h.Render(w, r, "offline.html", "Offline Mode - NRQ", nil, "", "")
}

// Render renders an HTML template within the base layout.
func (h *BaseHandler) Render(w http.ResponseWriter, r *http.Request, tmplName string, title string, data interface{}, flashErr, flashSuccess string) {
	user := middleware.GetUserFromContext(r.Context())
	csrfToken := middleware.GetCSRFTokenFromContext(r.Context())

	pageData := PageData{
		Title:         title,
		User:          user,
		CSRFToken:     csrfToken,
		FooterVersion: services.GetFooterVersion(),
		FlashError:    flashErr,
		FlashSuccess:  flashSuccess,
		SearchQuery:   r.URL.Query().Get("q"),
		Data:          data,
	}

	tmpl, ok := h.Templates[tmplName]
	if !ok {
		log.Printf("[ERROR] Template not found: %s", tmplName)
		http.Error(w, "Page not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", pageData); err != nil {
		log.Printf("[ERROR] Render template %s: %v", tmplName, err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

// JSON sends a JSON response with status code.
func (h *BaseHandler) JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[ERROR] JSON encode: %v", err)
	}
}

// SanitizeFilename cleans an untrusted filename to prevent path traversal, control chars, and weird UTF-8 exploits.
func SanitizeFilename(raw string) string {
	// Normalize backslashes to forward slashes first to handle Windows paths across any OS
	normalized := strings.ReplaceAll(raw, "\\", "/")
	name := filepath.Base(filepath.Clean(normalized))
	name = strings.TrimSpace(name)

	// Replace risky characters
	reg := regexp.MustCompile(`[^\w\.\-\s\(\)\[\]]`)
	cleaned := reg.ReplaceAllString(name, "_")

	// Strip leading dots or traversal artifacts
	cleaned = strings.TrimLeft(cleaned, "._ ")
	if cleaned == "" {
		cleaned = fmt.Sprintf("file_%d", time.Now().Unix())
	}
	return cleaned
}

// SetContentDisposition sets RFC 5987 UTF-8 compliant Content-Disposition header.
func SetContentDisposition(w http.ResponseWriter, disposition, filename string) {
	safeFilename := SanitizeFilename(filename)
	encoded := url.PathEscape(filename)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, safeFilename, encoded))
}
