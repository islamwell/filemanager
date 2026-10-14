package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"filemanager/internal/config"
	"filemanager/internal/handlers"
	"filemanager/internal/middleware"
	"filemanager/internal/services"
	"filemanager/internal/static"
	"filemanager/internal/store"
	"filemanager/internal/templates"
)

func main() {
	cfg := config.LoadConfig()

	log.Printf("==================================================")
	log.Printf(" Starting File Manager (%s)", services.GetFooterVersion())
	log.Printf(" Storage Directory : %s", cfg.StorageDir)
	log.Printf(" Database Path     : %s", cfg.DatabasePath)
	log.Printf(" Max Upload Size   : %d MB", cfg.MaxFileSize/(1024*1024))
	log.Printf("==================================================")

	// Initialize Storage Repository
	st, err := store.NewSQLiteStore(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("[FATAL] Could not initialize database: %v", err)
	}
	defer st.Close()

	// Periodic session cleanup
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			if err := st.CleanupExpiredSessions(); err != nil {
				log.Printf("[WARN] Session cleanup failed: %v", err)
			}
		}
	}()

	// Initialize Handlers
	baseHandler, err := handlers.NewBaseHandler(st, cfg, templates.FS)
	if err != nil {
		log.Fatalf("[FATAL] Could not initialize base handler: %v", err)
	}

	authHandler := handlers.NewAuthHandler(baseHandler)
	fileHandler := handlers.NewFileHandler(baseHandler)
	snippetHandler := handlers.NewSnippetHandler(baseHandler)
	categoryHandler := handlers.NewCategoryHandler(baseHandler)
	shareHandler := handlers.NewShareHandler(baseHandler)
	searchHandler := handlers.NewSearchHandler(baseHandler)
	adminHandler := handlers.NewAdminHandler(baseHandler)

	// Build Router
	mux := http.NewServeMux()

	// Static Assets
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static.FS))))

	// PWA Manifest and Service Worker
	mux.HandleFunc("GET /manifest.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := static.FS.ReadFile("manifest.json")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, err := static.FS.ReadFile("sw.js")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		_, _ = w.Write(data)
	})

	// Offline Fallback View
	mux.HandleFunc("GET /offline", baseHandler.OfflineView)

	// In-memory rate limiter for authentication routes (15 attempts/minute/IP)
	authLimiter := middleware.NewRateLimiter(15, 1*time.Minute)

	// Health Check Route (Uptime monitors, Docker, Plesk healthchecks)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","version":"%s","time":"%s"}`+"\n", services.Version, time.Now().UTC().Format(time.RFC3339))
	})

	// Public Auth Routes
	mux.HandleFunc("GET /login", authHandler.LoginView)
	mux.Handle("POST /login", middleware.RateLimitMiddleware(authLimiter)(http.HandlerFunc(authHandler.LoginSubmit)))
	mux.HandleFunc("GET /register", authHandler.RegisterView)
	mux.Handle("POST /register", middleware.RateLimitMiddleware(authLimiter)(http.HandlerFunc(authHandler.RegisterSubmit)))
	mux.HandleFunc("POST /logout", authHandler.Logout)

	// Public Sharing Routes (No authentication required)
	mux.HandleFunc("GET /share/{token}", shareHandler.ViewSharedItem)
	mux.HandleFunc("POST /share/{token}/unlock", shareHandler.UnlockShareLink)
	mux.HandleFunc("GET /share/{token}/download", shareHandler.DownloadSharedFile)
	mux.HandleFunc("GET /share/{token}/audio", shareHandler.AudioSharedStream)
	mux.HandleFunc("GET /share/{token}/view", shareHandler.ViewSharedMediaInline)

	// Authenticated Workspace Routes
	mux.Handle("GET /", middleware.RequireAuth(http.HandlerFunc(searchHandler.DashboardView)))
	mux.Handle("GET /search", middleware.RequireAuth(http.HandlerFunc(searchHandler.SearchView)))

	// File Management Routes
	mux.Handle("POST /files/upload", middleware.RequireAuth(http.HandlerFunc(fileHandler.UploadFile)))
	mux.Handle("GET /files/{id}/download", middleware.RequireAuth(http.HandlerFunc(fileHandler.DownloadFile)))
	mux.Handle("GET /files/{id}/view", middleware.RequireAuth(http.HandlerFunc(fileHandler.ViewFileInline)))
	mux.Handle("GET /files/{id}/audio", middleware.RequireAuth(http.HandlerFunc(fileHandler.AudioStream)))
	mux.Handle("POST /files/{id}/rename", middleware.RequireAuth(http.HandlerFunc(fileHandler.RenameFile)))
	mux.Handle("POST /files/{id}/move", middleware.RequireAuth(http.HandlerFunc(fileHandler.MoveFile)))
	mux.Handle("POST /files/{id}/delete", middleware.RequireAuth(http.HandlerFunc(fileHandler.DeleteFile)))
	mux.Handle("POST /files/batch/zip", middleware.RequireAuth(http.HandlerFunc(fileHandler.BatchZipDownload)))
	mux.Handle("POST /files/batch/delete", middleware.RequireAuth(http.HandlerFunc(fileHandler.BatchDeleteFiles)))
	mux.Handle("POST /files/batch/move", middleware.RequireAuth(http.HandlerFunc(fileHandler.BatchMoveFiles)))

	// Snippet Management Routes
	mux.Handle("GET /snippets/new", middleware.RequireAuth(http.HandlerFunc(snippetHandler.NewSnippetView)))
	mux.Handle("POST /snippets", middleware.RequireAuth(http.HandlerFunc(snippetHandler.CreateSnippet)))
	mux.Handle("GET /snippets/{id}", middleware.RequireAuth(http.HandlerFunc(snippetHandler.ViewSnippet)))
	mux.Handle("GET /snippets/{id}/edit", middleware.RequireAuth(http.HandlerFunc(snippetHandler.EditSnippetView)))
	mux.Handle("POST /snippets/{id}/edit", middleware.RequireAuth(http.HandlerFunc(snippetHandler.UpdateSnippet)))
	mux.Handle("POST /snippets/{id}/rename", middleware.RequireAuth(http.HandlerFunc(snippetHandler.RenameSnippet)))
	mux.Handle("POST /snippets/{id}/move", middleware.RequireAuth(http.HandlerFunc(snippetHandler.MoveSnippet)))
	mux.Handle("POST /snippets/{id}/delete", middleware.RequireAuth(http.HandlerFunc(snippetHandler.DeleteSnippet)))
	mux.Handle("POST /snippets/batch/delete", middleware.RequireAuth(http.HandlerFunc(snippetHandler.BatchDeleteSnippets)))

	// Folder / Category Routes (Hierarchical max 4 levels)
	mux.Handle("POST /categories", middleware.RequireAuth(http.HandlerFunc(categoryHandler.CreateCategory)))
	mux.Handle("POST /categories/{id}/delete", middleware.RequireAuth(http.HandlerFunc(categoryHandler.DeleteCategory)))

	// Sharing Management Routes
	mux.Handle("POST /items/share", middleware.RequireAuth(http.HandlerFunc(shareHandler.CreateShareLink)))
	mux.Handle("POST /shares/{token}/delete", middleware.RequireAuth(http.HandlerFunc(shareHandler.DeleteShareLink)))

	// Administrator Routes
	mux.Handle("GET /admin/users", middleware.RequireAuth(middleware.RequireAdmin(http.HandlerFunc(adminHandler.AdminUsersView))))
	mux.Handle("POST /admin/users/{id}/approve", middleware.RequireAuth(middleware.RequireAdmin(http.HandlerFunc(adminHandler.ApproveUser))))
	mux.Handle("POST /admin/users/{id}/reject", middleware.RequireAuth(middleware.RequireAdmin(http.HandlerFunc(adminHandler.RejectUser))))
	mux.Handle("POST /admin/users/{id}/role", middleware.RequireAuth(middleware.RequireAdmin(http.HandlerFunc(adminHandler.ToggleUserRole))))
	mux.Handle("POST /admin/users/{id}/delete", middleware.RequireAuth(middleware.RequireAdmin(http.HandlerFunc(adminHandler.DeleteUser))))

	// Apply Middleware Stack (Outer to Inner)
	handler := middleware.LoggingAndRecovery(
		middleware.SecurityHeaders(
			middleware.SessionMiddleware(st)(
				middleware.CSRFMiddleware(cfg.SecureCookie)(mux),
			),
		),
	)

	serverAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:              serverAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // Protects against Slowloris attacks
		IdleTimeout:       120 * time.Second,
	}

	// Graceful Shutdown Channel
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on http://localhost:%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] Server listen error: %v", err)
		}
	}()

	<-shutdownChan
	log.Println("Shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server shutdown failed: %v", err)
	}
	log.Println("Server stopped.")
}
