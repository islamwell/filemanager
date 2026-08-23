package handlers

import (
	"errors"
	"net/http"
	"strings"

	"filemanager/internal/middleware"
	"filemanager/internal/models"
	"filemanager/internal/store"
)

type CategoryHandler struct {
	*BaseHandler
}

func NewCategoryHandler(base *BaseHandler) *CategoryHandler {
	return &CategoryHandler{BaseHandler: base}
}

// CreateCategory adds a new folder/category ensuring maximum hierarchy depth <= 4.
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	parentIDStr := strings.TrimSpace(r.FormValue("parent_id"))

	if name == "" {
		http.Redirect(w, r, "/?err=Category+name+cannot+be+empty", http.StatusSeeOther)
		return
	}

	var parentID *string
	if parentIDStr != "" {
		parentID = &parentIDStr
	}

	cat := &models.Category{
		UserID:   user.ID,
		ParentID: parentID,
		Name:     name,
	}

	if err := h.Store.CreateCategory(cat); err != nil {
		if errors.Is(err, store.ErrMaxDepthExceeded) {
			http.Redirect(w, r, "/?err=Maximum+folder+nesting+limit+reached+(maximum+4+levels)", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/?err="+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?msg=Folder+created+successfully", http.StatusSeeOther)
}

// DeleteCategory deletes a category.
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := r.PathValue("id")
	if err := h.Store.DeleteCategory(id, user.ID); err != nil {
		http.Redirect(w, r, "/?err=Failed+to+delete+folder", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/?msg=Folder+deleted+successfully", http.StatusSeeOther)
}
