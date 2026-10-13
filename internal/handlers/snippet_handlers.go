package handlers

import (
	"errors"
	"fmt"
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

	categories, _ := h.Store.ListCategories(user.ID)
	data := struct {
		*models.Snippet
		Categories []*models.Category
	}{
		Snippet:    snippet,
		Categories: categories,
	}

	h.Render(w, r, "snippet_view.html", snippet.Title+" - Snippet", data, flashErr, flashMsg)
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

// EditSnippetView renders the form to edit an existing snippet.
func (h *SnippetHandler) EditSnippetView(w http.ResponseWriter, r *http.Request) {
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

	categories, _ := h.Store.ListCategories(user.ID)
	h.Render(w, r, "snippet_edit.html", "Edit Snippet - "+snippet.Title, map[string]interface{}{
		"Snippet":    snippet,
		"Categories": categories,
		"TagsString": strings.Join(snippet.Tags, ", "),
	}, "", "")
}

// UpdateSnippet validates and updates an existing text snippet.
func (h *SnippetHandler) UpdateSnippet(w http.ResponseWriter, r *http.Request) {
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
		h.Render(w, r, "snippet_edit.html", "Edit Snippet - "+snippet.Title, map[string]interface{}{
			"Snippet":    snippet,
			"Categories": categories,
			"TagsString": tagsRaw,
		}, "Snippet content cannot be empty.", "")
		return
	}

	runeCount := utf8.RuneCountInString(content)
	if runeCount > 10000 {
		categories, _ := h.Store.ListCategories(user.ID)
		h.Render(w, r, "snippet_edit.html", "Edit Snippet - "+snippet.Title, map[string]interface{}{
			"Snippet":    snippet,
			"Categories": categories,
			"TagsString": tagsRaw,
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

	snippet.Title = title
	snippet.Content = content
	snippet.Language = language
	snippet.CategoryID = categoryID
	snippet.Tags = tags

	if err := h.Store.UpdateSnippet(snippet); err != nil {
		categories, _ := h.Store.ListCategories(user.ID)
		h.Render(w, r, "snippet_edit.html", "Edit Snippet - "+snippet.Title, map[string]interface{}{
			"Snippet":    snippet,
			"Categories": categories,
			"TagsString": tagsRaw,
		}, "Failed to update snippet: "+err.Error(), "")
		return
	}

	http.Redirect(w, r, "/snippets/"+snippet.ID+"?msg=Snippet+updated+successfully", http.StatusSeeOther)
}

// MoveSnippet updates the folder of a snippet.
func (h *SnippetHandler) MoveSnippet(w http.ResponseWriter, r *http.Request) {
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

	if err := h.Store.MoveSnippet(id, user.ID, newCatID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Redirect(w, r, "/?err=Snippet+not+found", http.StatusSeeOther)
			return
		}
		if errors.Is(err, store.ErrCategoryNotFound) {
			http.Redirect(w, r, "/?err=Target+folder+not+found", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/snippets/"+id+"?msg=Snippet+moved+successfully", http.StatusSeeOther)
}

// RenameSnippet renames an existing snippet.
func (h *SnippetHandler) RenameSnippet(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	newTitle := strings.TrimSpace(r.FormValue("title"))
	if newTitle == "" {
		http.Redirect(w, r, "/?err=Title+cannot+be+empty", http.StatusSeeOther)
		return
	}

	if err := h.Store.RenameSnippet(id, user.ID, newTitle); err != nil {
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
		return
	}

	http.Redirect(w, r, "/?msg=Snippet+renamed+successfully", http.StatusSeeOther)
}

// BatchDeleteSnippets removes multiple snippets.
func (h *SnippetHandler) BatchDeleteSnippets(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?err=Invalid+request", http.StatusSeeOther)
		return
	}

	ids := r.Form["snippet_ids"]
	if len(ids) == 0 {
		if raw := r.FormValue("snippet_ids"); raw != "" {
			ids = strings.Split(raw, ",")
		}
	}

	if len(ids) == 0 {
		http.Redirect(w, r, "/?err=No+snippets+selected", http.StatusSeeOther)
		return
	}

	if err := h.Store.DeleteSnippets(ids, user.ID); err != nil {
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"success":true,"count":%d}`, len(ids))))
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/?msg=%d+snippets+deleted+successfully", len(ids)), http.StatusSeeOther)
}

