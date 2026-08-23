package handlers

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"filemanager/internal/middleware"
	"filemanager/internal/models"
	"filemanager/internal/store"
)

type SnippetHandler struct {
	*BaseHandler
}

func NewSnippetHandler(base *BaseHandler) *SnippetHandler {
	return &SnippetHandler{BaseHandler: base}
}

// NewSnippetView renders the form to create a new text snippet.
func (h *SnippetHandler) NewSnippetView(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	categories, _ := h.Store.ListCategories(user.ID)
	h.Render(w, r, "snippet_new.html", "New Text Snippet - File Manager", map[string]interface{}{
		"Categories": categories,
	}, "", "")
}

// CreateSnippet validates and stores a new text snippet (max 10,000 characters).
func (h *SnippetHandler) CreateSnippet(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	content := r.FormValue("content")
	language := strings.TrimSpace(r.FormValue("language"))
	categoryIDStr := strings.TrimSpace(r.FormValue("category_id"))
	tagsRaw := r.FormValue("tags")

	if title == "" {
		title = "Untitled Snippet"
	}
	if strings.TrimSpace(content) == "" {
		categories, _ := h.Store.ListCategories(user.ID)
		h.Render(w, r, "snippet_new.html", "New Text Snippet", map[string]interface{}{
			"Title":      title,
			"Content":    content,
			"Language":   language,
			"Categories": categories,
		}, "Snippet content cannot be empty.", "")
		return
	}

	runeCount := utf8.RuneCountInString(content)
	if runeCount > 10000 {
		categories, _ := h.Store.ListCategories(user.ID)
		h.Render(w, r, "snippet_new.html", "New Text Snippet", map[string]interface{}{
			"Title":      title,
			"Content":    content,
			"Language":   language,
			"Categories": categories,
		}, "Snippet exceeds the maximum 10,000 character limit.", "")
		return
	}

	var categoryID *string
	if categoryIDStr != "" {
		categoryID = &categoryIDStr
	}

	var tags []string
	if tagsRaw != "" {
		for _, t := range strings.Split(tagsRaw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, t)
			}
		}
	}

	snippet := &models.Snippet{
		UserID:     user.ID,
		CategoryID: categoryID,
		Title:      title,
		Content:    content,
		Language:   language,
		Tags:       tags,
	}

	if err := h.Store.CreateSnippet(snippet); err != nil {
		categories, _ := h.Store.ListCategories(user.ID)
		h.Render(w, r, "snippet_new.html", "New Text Snippet", map[string]interface{}{
			"Title":      title,
			"Content":    content,
			"Language":   language,
			"Categories": categories,
		}, "Failed to save snippet: "+err.Error(), "")
		return
	}

	http.Redirect(w, r, "/snippets/"+snippet.ID+"?msg=Snippet+created+successfully", http.StatusSeeOther)
}

// ViewSnippet displays the full snippet content.
func (h *SnippetHandler) ViewSnippet(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	snippet, err := h.Store.GetSnippetByID(id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	flashMsg := r.URL.Query().Get("msg")
	flashErr := r.URL.Query().Get("err")

	h.Render(w, r, "snippet_view.html", snippet.Title+" - Snippet", snippet, flashErr, flashMsg)
}

// DeleteSnippet deletes a snippet.
func (h *SnippetHandler) DeleteSnippet(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	if err := h.Store.DeleteSnippet(id, user.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Redirect(w, r, "/?err=Snippet+not+found", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?msg=Snippet+deleted+successfully", http.StatusSeeOther)
}
