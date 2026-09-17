package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"filemanager/internal/services"
)

// Config encapsulates runtime configuration parameters for the file manager server.
type Config struct {
	Port         int
	Host         string
	StorageDir   string
	DatabasePath string
	MaxFileSize  int64 // Max file size in bytes (e.g., 100MB)
	SecureCookie bool  // Whether session cookies require HTTPS
	AppVersion   string
}

// LoadConfig parses command line flags and environment variables with sensible production defaults.
func LoadConfig() *Config {
	cfg := &Config{
		Port:         8080,
		Host:         "0.0.0.0",
		StorageDir:   "./data/storage",
		DatabasePath: "./data/filemanager.db",
		MaxFileSize:  100 * 1024 * 1024, // 100 MB default
		SecureCookie: false,             // Set to true behind production HTTPS
		AppVersion:   "v1.0.9",
	}

	// Environment variable overrides
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		if p, err := strconv.Atoi(portEnv); err == nil {
			cfg.Port = p
		}
	}
	if hostEnv := os.Getenv("HOST"); hostEnv != "" {
		cfg.Host = hostEnv
	}
	if storageEnv := os.Getenv("STORAGE_DIR"); storageEnv != "" {
		cfg.StorageDir = storageEnv
	}
	if dbEnv := os.Getenv("DATABASE_PATH"); dbEnv != "" {
		cfg.DatabasePath = dbEnv
	}
	if maxFileEnv := os.Getenv("MAX_FILE_SIZE"); maxFileEnv != "" {
		if m, err := strconv.ParseInt(maxFileEnv, 10, 64); err == nil {
			cfg.MaxFileSize = m
		}
	}
	if secEnv := os.Getenv("SECURE_COOKIE"); secEnv == "true" || secEnv == "1" {
		cfg.SecureCookie = true
	}

	// CLI flags override environment variables
	flag.IntVar(&cfg.Port, "port", cfg.Port, "HTTP server listening port")
	flag.StringVar(&cfg.Host, "host", cfg.Host, "HTTP server listening host")
	flag.StringVar(&cfg.StorageDir, "storage", cfg.StorageDir, "Directory path for physical file storage")
	flag.StringVar(&cfg.DatabasePath, "db", cfg.DatabasePath, "Path to SQLite database file")
	flag.Int64Var(&cfg.MaxFileSize, "max-file-size", cfg.MaxFileSize, "Maximum file upload size in bytes")
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print application version and exit")
	flag.BoolVar(&showVersion, "v", false, "Print application version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(services.GetFooterVersion())
		os.Exit(0)
	}

	// Ensure directories exist
	dbDir := filepath.Dir(cfg.DatabasePath)
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		// Log warning or continue; store initialization will verify
	}
	if err := os.MkdirAll(cfg.StorageDir, 0750); err != nil {
		// Store initialization will verify
	}

	return cfg
}
