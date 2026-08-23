package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"filemanager/internal/auth"
	"filemanager/internal/middleware"
	"filemanager/internal/models"
	"filemanager/internal/store"
)

type AuthHandler struct {
	*BaseHandler
}

func NewAuthHandler(base *BaseHandler) *AuthHandler {
	return &AuthHandler{BaseHandler: base}
}

// LoginView renders the login form.
func (h *AuthHandler) LoginView(w http.ResponseWriter, r *http.Request) {
	if user := middleware.GetUserFromContext(r.Context()); user != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.Render(w, r, "login.html", "Sign In - File Manager", map[string]string{
		"Next": r.URL.Query().Get("next"),
	}, "", "")
}

// LoginSubmit authenticates the user and issues a secure session cookie.
func (h *AuthHandler) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	nextURL := r.FormValue("next")

	if username == "" || password == "" {
		h.Render(w, r, "login.html", "Sign In - File Manager", map[string]string{"Username": username, "Next": nextURL}, "Username and password are required.", "")
		return
	}

	user, err := h.Store.GetUserByUsername(username)
	if err != nil || !auth.CheckPassword(password, user.PasswordHash) {
		h.Render(w, r, "login.html", "Sign In - File Manager", map[string]string{"Username": username, "Next": nextURL}, "Invalid username or password.", "")
		return
	}

	// Create session
	token, err := auth.GenerateSessionToken()
	if err != nil {
		h.Render(w, r, "login.html", "Sign In - File Manager", map[string]string{"Username": username, "Next": nextURL}, "Failed to generate session.", "")
		return
	}

	expiresAt := time.Now().Add(auth.SessionDuration)
	session := &models.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
	}

	if err := h.Store.CreateSession(session); err != nil {
		h.Render(w, r, "login.html", "Sign In - File Manager", map[string]string{"Username": username, "Next": nextURL}, "Failed to create session.", "")
		return
	}

	auth.SetSessionCookie(w, token, expiresAt, h.Config.SecureCookie)

	// Validate nextURL to prevent Open Redirect vulnerability
	target := "/"
	if nextURL != "" && strings.HasPrefix(nextURL, "/") && !strings.HasPrefix(nextURL, "//") {
		if parsed, err := url.Parse(nextURL); err == nil && parsed.Host == "" {
			target = nextURL
		}
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// RegisterView renders the registration page.
func (h *AuthHandler) RegisterView(w http.ResponseWriter, r *http.Request) {
	if user := middleware.GetUserFromContext(r.Context()); user != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.Render(w, r, "register.html", "Register Account - File Manager", nil, "", "")
}

// RegisterSubmit validates and creates a new user account.
func (h *AuthHandler) RegisterSubmit(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if username == "" || password == "" {
		h.Render(w, r, "register.html", "Register Account - File Manager", map[string]string{"Username": username}, "All fields are required.", "")
		return
	}

	if len(username) < 3 || len(username) > 32 {
		h.Render(w, r, "register.html", "Register Account - File Manager", map[string]string{"Username": username}, "Username must be between 3 and 32 characters.", "")
		return
	}

	if password != confirmPassword {
		h.Render(w, r, "register.html", "Register Account - File Manager", map[string]string{"Username": username}, "Passwords do not match.", "")
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		h.Render(w, r, "register.html", "Register Account - File Manager", map[string]string{"Username": username}, err.Error(), "")
		return
	}

	user := &models.User{
		Username:     username,
		PasswordHash: hash,
	}

	if err := h.Store.CreateUser(user); err != nil {
		if errors.Is(err, store.ErrUserExists) {
			h.Render(w, r, "register.html", "Register Account - File Manager", map[string]string{"Username": username}, "Username is already taken.", "")
			return
		}
		h.Render(w, r, "register.html", "Register Account - File Manager", map[string]string{"Username": username}, "Failed to create account. Please try again.", "")
		return
	}

	// Auto log in after registration
	token, _ := auth.GenerateSessionToken()
	expiresAt := time.Now().Add(auth.SessionDuration)
	_ = h.Store.CreateSession(&models.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
	})
	auth.SetSessionCookie(w, token, expiresAt, h.Config.SecureCookie)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout invalidates the active session and clears the cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = h.Store.DeleteSession(cookie.Value)
	}
	auth.ClearSessionCookie(w, h.Config.SecureCookie)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
