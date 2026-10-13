package handlers

import (
	"net/http"
	"strings"

	"filemanager/internal/middleware"
	"filemanager/internal/models"
)

type SearchHandler struct {
	*BaseHandler
}

func NewSearchHandler(base *BaseHandler) *SearchHandler {
	return &SearchHandler{BaseHandler: base}
}

// DashboardView renders the main file manager workspace with files, snippets, categories, and tags.
func (h *SearchHandler) DashboardView(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	categoryIDStr := strings.TrimSpace(r.URL.Query().Get("category_id"))
	var categoryID *string
	if categoryIDStr != "" {
		categoryID = &categoryIDStr
	}

	tagFilter := strings.TrimSpace(r.URL.Query().Get("tag"))
	flashMsg := r.URL.Query().Get("msg")
	flashErr := r.URL.Query().Get("err")

	categories, err := h.Store.ListCategories(user.ID)
	if err != nil {
		categories = []*models.Category{}
	}

	files, err := h.Store.ListFiles(user.ID, categoryID, tagFilter)
	if err != nil {
		files = []*models.FileItem{}
	}

	snippets, err := h.Store.ListSnippets(user.ID, categoryID, tagFilter)
	if err != nil {
		snippets = []*models.Snippet{}
	}

	tags, err := h.Store.ListUserTags(user.ID)
	if err != nil {
		tags = []models.Tag{}
	}

	var activeCategory *models.Category
	if categoryID != nil && *categoryID != "" {
		activeCategory, _ = h.Store.GetCategoryByID(*categoryID, user.ID)
	}

	storageStats, _ := h.Store.GetUserStorageStats(user.ID)

	data := map[string]interface{}{
		"Categories":     categories,
		"Files":          files,
		"Snippets":       snippets,
		"Tags":           tags,
		"ActiveCatID":    categoryIDStr,
		"ActiveCategory": activeCategory,
		"ActiveTag":      tagFilter,
		"TotalFiles":     len(files),
		"TotalSnippets":  len(snippets),
		"StorageStats":   storageStats,
	}

	h.Render(w, r, "dashboard.html", "Dashboard - File Manager", data, flashErr, flashMsg)
}

// SearchView handles fuzzy search requests across files, snippets, categories, and tags.
func (h *SearchHandler) SearchView(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	tagFilter := strings.TrimSpace(r.URL.Query().Get("tag"))

	results, err := h.Store.SearchItems(user.ID, query, tagFilter)
	if err != nil {
		results = []models.SearchResult{}
	}

	tags, _ := h.Store.ListUserTags(user.ID)

	data := map[string]interface{}{
		"Query":        query,
		"TagFilter":    tagFilter,
		"Results":      results,
		"ResultCount":  len(results),
		"Tags":         tags,
	}

	h.Render(w, r, "search_results.html", "Search Results - File Manager", data, "", "")
}
