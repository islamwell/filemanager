package tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"filemanager/internal/auth"
	"filemanager/internal/middleware"
)

func TestCSRFProtectionMiddleware(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	csrfMiddleware := middleware.CSRFMiddleware(false)
	protected := csrfMiddleware(dummyHandler)

	// 1. Test GET Request sets CSRF Cookie
	getReq := httptest.NewRequest("GET", "/", nil)
	getRR := httptest.NewRecorder()
	protected.ServeHTTP(getRR, getReq)

	if getRR.Code != http.StatusOK {
		t.Errorf("expected GET 200 OK, got %d", getRR.Code)
	}

	var csrfCookie *http.Cookie
	for _, c := range getRR.Result().Cookies() {
		if c.Name == auth.CSRFCookieName {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil || csrfCookie.Value == "" {
		t.Fatalf("expected CSRF cookie to be issued on GET")
	}

	// 2. Test POST with matching CSRF Token -> Should SUCCEED (200 OK)
	postForm := url.Values{"_csrf": {csrfCookie.Value}}
	postReq := httptest.NewRequest("POST", "/test-action", strings.NewReader(postForm.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(csrfCookie)
	postRR := httptest.NewRecorder()

	protected.ServeHTTP(postRR, postReq)

	if postRR.Code != http.StatusOK {
		t.Errorf("expected POST with valid CSRF to succeed with 200 OK, got %d", postRR.Code)
	}

	// 3. Test POST with INVALID or MISMATCHED CSRF Token -> Should be FORBIDDEN (403)
	badPostForm := url.Values{"_csrf": {"invalid_or_stolen_token"}}
	badPostReq := httptest.NewRequest("POST", "/test-action", strings.NewReader(badPostForm.Encode()))
	badPostReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badPostReq.AddCookie(csrfCookie)
	badPostRR := httptest.NewRecorder()

	protected.ServeHTTP(badPostRR, badPostReq)

	if badPostRR.Code != http.StatusForbidden {
		t.Errorf("expected POST with mismatched CSRF token to be rejected with 403 Forbidden, got %d", badPostRR.Code)
	}

	// 4. Test POST with MISSING CSRF Form field -> Should be FORBIDDEN (403)
	missingReq := httptest.NewRequest("POST", "/test-action", strings.NewReader(""))
	missingReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	missingReq.AddCookie(csrfCookie)
	missingRR := httptest.NewRecorder()

	protected.ServeHTTP(missingRR, missingReq)

	if missingRR.Code != http.StatusForbidden {
		t.Errorf("expected POST with missing CSRF token to be rejected with 403 Forbidden, got %d", missingRR.Code)
	}
}
