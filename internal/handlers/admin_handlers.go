package handlers

import (
	"log"
	"net/http"

	"filemanager/internal/middleware"
)

type AdminHandler struct {
	*BaseHandler
}

func NewAdminHandler(base *BaseHandler) *AdminHandler {
	return &AdminHandler{BaseHandler: base}
}

// AdminUsersView renders the user management dashboard.
func (h *AdminHandler) AdminUsersView(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.ListUsers()
	if err != nil {
		log.Printf("[ERROR] ListUsers: %v", err)
		h.Render(w, r, "admin_users.html", "User Management - File Manager", nil, "Failed to load user accounts.", "")
		return
	}

	flashSuccess := r.URL.Query().Get("msg")
	flashError := r.URL.Query().Get("err")

	h.Render(w, r, "admin_users.html", "User Management - File Manager", users, flashError, flashSuccess)
}

// ApproveUser approves a pending user account.
func (h *AdminHandler) ApproveUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/admin/users?err=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	if err := h.Store.UpdateUserApproval(id, true); err != nil {
		log.Printf("[ERROR] ApproveUser: %v", err)
		http.Redirect(w, r, "/admin/users?err=Failed+to+approve+user", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/users?msg=User+approved+successfully", http.StatusSeeOther)
}

// RejectUser revokes approval for a user account.
func (h *AdminHandler) RejectUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/admin/users?err=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	currentUser := middleware.GetUserFromContext(r.Context())
	if currentUser != nil && currentUser.ID == id {
		http.Redirect(w, r, "/admin/users?err=You+cannot+revoke+your+own+approval", http.StatusSeeOther)
		return
	}

	if err := h.Store.UpdateUserApproval(id, false); err != nil {
		log.Printf("[ERROR] RejectUser: %v", err)
		http.Redirect(w, r, "/admin/users?err=Failed+to+revoke+approval", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/users?msg=User+approval+revoked", http.StatusSeeOther)
}

// ToggleUserRole switches a user between standard and administrator roles.
func (h *AdminHandler) ToggleUserRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/admin/users?err=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	targetUser, err := h.Store.GetUserByID(id)
	if err != nil {
		http.Redirect(w, r, "/admin/users?err=User+not+found", http.StatusSeeOther)
		return
	}

	currentUser := middleware.GetUserFromContext(r.Context())
	if currentUser != nil && currentUser.ID == id && targetUser.IsAdmin {
		http.Redirect(w, r, "/admin/users?err=You+cannot+remove+admin+privileges+from+yourself", http.StatusSeeOther)
		return
	}

	newRole := !targetUser.IsAdmin
	if err := h.Store.UpdateUserRole(id, newRole); err != nil {
		log.Printf("[ERROR] ToggleUserRole: %v", err)
		http.Redirect(w, r, "/admin/users?err=Failed+to+update+user+role", http.StatusSeeOther)
		return
	}

	msg := "User promoted to administrator"
	if !newRole {
		msg = "Administrator privileges revoked"
	}
	http.Redirect(w, r, "/admin/users?msg="+msg, http.StatusSeeOther)
}

// DeleteUser permanently deletes a user account and associated resources.
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/admin/users?err=Invalid+user+ID", http.StatusSeeOther)
		return
	}

	currentUser := middleware.GetUserFromContext(r.Context())
	if currentUser != nil && currentUser.ID == id {
		http.Redirect(w, r, "/admin/users?err=You+cannot+delete+your+own+account+from+admin+panel", http.StatusSeeOther)
		return
	}

	if err := h.Store.DeleteUser(id); err != nil {
		log.Printf("[ERROR] DeleteUser: %v", err)
		http.Redirect(w, r, "/admin/users?err=Failed+to+delete+user", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/users?msg=User+account+deleted+successfully", http.StatusSeeOther)
}
