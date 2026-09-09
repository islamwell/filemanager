package tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"filemanager/internal/auth"
	"filemanager/internal/config"
	"filemanager/internal/handlers"
	"filemanager/internal/middleware"
	"filemanager/internal/models"
	"filemanager/internal/store"
	"filemanager/internal/templates"
)

func setupTestEnv(t *testing.T) (store.Store, *handlers.BaseHandler) {
	st, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory sqlite store: %v", err)
	}

	cfg := &config.Config{
		Port:         8080,
		StorageDir:   t.TempDir(),
		DatabasePath: ":memory:",
		MaxFileSize:  100 * 1024 * 1024,
		SecureCookie: false,
	}

	base, err := handlers.NewBaseHandler(st, cfg, templates.FS)
	if err != nil {
		t.Fatalf("failed to create base handler: %v", err)
	}

	return st, base
}

func TestPasswordHashingAndVerification(t *testing.T) {
	password := "supersecret123"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !auth.CheckPassword(password, hash) {
		t.Errorf("expected password to match hash")
	}

	if auth.CheckPassword("wrongpassword", hash) {
		t.Errorf("expected wrong password to fail check")
	}

	// Test short password
	_, err = auth.HashPassword("short")
	if err == nil {
		t.Errorf("expected error for password < 8 chars")
	}
}

func TestUserRegistrationAndLogin(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	authHandler := handlers.NewAuthHandler(base)

	// Test Registration
	form := url.Values{
		"username":         {"alice"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	}

	req := httptest.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	authHandler.RegisterSubmit(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect StatusSeeOther (303), got %d", rr.Code)
	}

	// Verify user exists in database
	user, err := st.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("failed to find user 'alice': %v", err)
	}

	if user.Username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", user.Username)
	}

	// Test Duplicate User Registration
	rrDup := httptest.NewRecorder()
	authHandler.RegisterSubmit(rrDup, req)
	if rrDup.Code != http.StatusOK { // Re-renders form with error
		t.Errorf("expected 200 OK re-render on duplicate user, got %d", rrDup.Code)
	}

	// Test Login Submit
	loginForm := url.Values{
		"username": {"alice"},
		"password": {"password123"},
	}
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader(loginForm.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()

	authHandler.LoginSubmit(loginRR, loginReq)

	if loginRR.Code != http.StatusSeeOther {
		t.Fatalf("expected login redirect 303, got %d", loginRR.Code)
	}

	cookies := loginRR.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected session cookie to be set")
	}

	// Verify session in store
	session, err := st.GetSession(sessionCookie.Value)
	if err != nil || session.UserID != user.ID {
		t.Fatalf("failed to retrieve valid session from store")
	}
}

func TestSessionExpiration(t *testing.T) {
	st, _ := setupTestEnv(t)
	defer st.Close()

	user := &models.User{Username: "bob", PasswordHash: "fakehash"}
	_ = st.CreateUser(user)

	// Create expired session
	expiredSession := &models.Session{
		Token:     "expired_token_123",
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	_ = st.CreateSession(expiredSession)

	_, err := st.GetSession("expired_token_123")
	if err == nil {
		t.Errorf("expected error fetching expired session, got nil")
	}
}

func TestAuthRateLimiter(t *testing.T) {
	rl := middleware.NewRateLimiter(3, 500*time.Millisecond)

	ip := "192.168.1.100"
	if !rl.Allow(ip) {
		t.Errorf("expected 1st attempt to be allowed")
	}
	if !rl.Allow(ip) {
		t.Errorf("expected 2nd attempt to be allowed")
	}
	if !rl.Allow(ip) {
		t.Errorf("expected 3rd attempt to be allowed")
	}
	if rl.Allow(ip) {
		t.Errorf("expected 4th attempt to be blocked")
	}

	// Another IP should not be blocked
	if !rl.Allow("192.168.1.101") {
		t.Errorf("expected different IP to be allowed")
	}

	// Wait for window to expire
	time.Sleep(550 * time.Millisecond)
	if !rl.Allow(ip) {
		t.Errorf("expected attempt to be allowed after window expires")
	}
}
