package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

const (
	// SessionCookieName is the cookie identifier for authenticated user sessions.
	SessionCookieName = "fm_session"

	// CSRFCookieName is the cookie identifier for CSRF protection.
	CSRFCookieName = "fm_csrf"

	// SessionDuration defines the session validity duration (7 days).
	SessionDuration = 7 * 24 * time.Hour
)

// GenerateRandomToken generates a cryptographically secure random base64url string with n bytes of entropy.
func GenerateRandomToken(byteLen int) (string, error) {
	bytes := make([]byte, byteLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("crypto rand failure: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// GenerateShareToken creates a 32-byte secure random token for public file/snippet share links.
func GenerateShareToken() (string, error) {
	return GenerateRandomToken(32)
}

// GenerateSessionToken creates a 32-byte random session token.
func GenerateSessionToken() (string, error) {
	return GenerateRandomToken(32)
}

// GenerateCSRFToken creates a 32-byte random token for CSRF protection.
func GenerateCSRFToken() (string, error) {
	return GenerateRandomToken(32)
}

// SetSessionCookie writes a hardened session cookie to the HTTP response.
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie invalidates the session cookie in the client browser.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// SetCSRFCookie writes a cookie containing the current CSRF token.
func SetCSRFCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(SessionDuration),
		HttpOnly: false, // Accessible to client scripts if needed, but validated via double-submit
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
