package middleware

import (
	"context"
	"crypto/subtle"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"filemanager/internal/auth"
	"filemanager/internal/models"
	"filemanager/internal/store"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
	CSRFContextKey contextKey = "csrf_token"
)

// GetUserFromContext retrieves the authenticated User struct from the request context.
func GetUserFromContext(ctx context.Context) *models.User {
	if u, ok := ctx.Value(UserContextKey).(*models.User); ok {
		return u
	}
	return nil
}

// GetCSRFTokenFromContext retrieves the CSRF token from the request context.
func GetCSRFTokenFromContext(ctx context.Context) string {
	if t, ok := ctx.Value(CSRFContextKey).(string); ok {
		return t
	}
	return ""
}

// LoggingAndRecovery logs incoming HTTP requests and recovers from panics gracefully.
func LoggingAndRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[PANIC] %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		rw := &responseWriterInterceptor{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("[%s] %s -> %d (%v)", r.Method, r.URL.Path, rw.statusCode, time.Since(start))
	})
}

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterInterceptor) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SecurityHeaders applies modern web security HTTP headers to protect against clickjacking, MIME sniffing, and XSS.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; media-src 'self' blob:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:;")
		next.ServeHTTP(w, r)
	})
}

// SessionMiddleware checks for an active session cookie and injects the user into context.
func SessionMiddleware(st store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err == nil && cookie.Value != "" {
				session, err := st.GetSession(cookie.Value)
				if err == nil && session != nil {
					user, err := st.GetUserByID(session.UserID)
					if err == nil && user != nil {
						ctx = context.WithValue(ctx, UserContextKey, user)
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth ensures that the incoming request is from an authenticated user.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			if strings.HasPrefix(r.Header.Get("Accept"), "application/json") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CSRFMiddleware manages double-submit CSRF cookies and verifies them on state-changing requests.
func CSRFMiddleware(secureCookie bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if CSRF cookie already exists
			var csrfToken string
			cookie, err := r.Cookie(auth.CSRFCookieName)
			if err != nil || cookie.Value == "" {
				csrfToken, _ = auth.GenerateCSRFToken()
				auth.SetCSRFCookie(w, csrfToken, secureCookie)
			} else {
				csrfToken = cookie.Value
			}

			// Add token to context for templates
			ctx := context.WithValue(r.Context(), CSRFContextKey, csrfToken)

			// Enforce CSRF check on state-changing requests (POST, PUT, DELETE, PATCH)
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Method == http.MethodPatch {
				// Allow login and register forms (they establish the session) if desired, but double submit works everywhere!
				submittedToken := r.FormValue("_csrf")
				if submittedToken == "" {
					submittedToken = r.Header.Get("X-CSRF-Token")
				}

				// Constant time comparison to prevent timing attacks
				if subtle.ConstantTimeCompare([]byte(csrfToken), []byte(submittedToken)) != 1 {
					log.Printf("[CSRF REJECT] Method=%s Path=%s, expected=%s, got=%s", r.Method, r.URL.Path, csrfToken, submittedToken)
					http.Error(w, "Forbidden: Invalid or missing CSRF token. Please refresh and try again.", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MaxBodySize restricts the maximum request body to prevent resource exhaustion attacks.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter implements a lightweight thread-safe sliding window rate limiter per client IP.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter with the specified request count limit and sliding window duration.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}

	// Background ticker to prune inactive IP addresses and prevent unbounded memory growth
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		for range ticker.C {
			rl.mu.Lock()
			now := time.Now()
			for ip, times := range rl.attempts {
				var active []time.Time
				for _, t := range times {
					if now.Sub(t) < rl.window {
						active = append(active, t)
					}
				}
				if len(active) == 0 {
					delete(rl.attempts, ip)
				} else {
					rl.attempts[ip] = active
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

// Allow returns true if the client IP is within rate limits.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	times := rl.attempts[ip]
	var active []time.Time
	for _, t := range times {
		if now.Sub(t) < rl.window {
			active = append(active, t)
		}
	}

	if len(active) >= rl.limit {
		rl.attempts[ip] = active
		return false
	}

	rl.attempts[ip] = append(active, now)
	return true
}

// RateLimitMiddleware blocks clients exceeding the allowed request threshold.
func RateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			if !rl.Allow(ip) {
				http.Error(w, "Too many requests. Please wait a moment before trying again.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts client IP addressing proxy headers (X-Forwarded-For, X-Real-IP) and RemoteAddr.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
