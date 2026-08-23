package models

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// User represents an authenticated account in the system.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Session tracks an active authenticated user session.
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired checks whether the session has passed its expiration time.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// Category represents a hierarchical folder/collection up to 4 levels deep.
type Category struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	ParentID  *string     `json:"parent_id"` // Nullable for root categories
	Name      string      `json:"name"`
	Depth     int         `json:"depth"` // 1 (root), 2, 3, or 4 (max)
	Path      string      `json:"path"`  // Full computed breadcrumb path, e.g., "Projects / 2026 / Audio"
	CreatedAt time.Time   `json:"created_at"`
	Children  []*Category `json:"children,omitempty"`
}

// FileItem represents an uploaded file stored on disk with metadata in SQLite.
type FileItem struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	CategoryID       *string   `json:"category_id"`
	CategoryName     string    `json:"category_name,omitempty"`
	CategoryPath     string    `json:"category_path,omitempty"`
	OriginalFilename string    `json:"original_filename"`
	StoredPath       string    `json:"-"` // Relative or absolute disk storage path
	FileSize         int64     `json:"file_size"`
	MimeType         string    `json:"mime_type"`
	Sha256Checksum   string    `json:"sha256_checksum"`
	IsMP3            bool      `json:"is_mp3"`
	Tags             []string  `json:"tags"`
	ShareToken       *string   `json:"share_token,omitempty"`
	ShareExpiresAt   *time.Time`json:"share_expires_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// FormattedSize returns human-readable file size (e.g., 2.4 MB).
func (f *FileItem) FormattedSize() string {
	const unit = 1024
	if f.FileSize < unit {
		return fmt.Sprintf("%d B", f.FileSize)
	}
	div, exp := int64(unit), 0
	for n := f.FileSize / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(f.FileSize)/float64(div), "KMGTPE"[exp])
}

// Extension returns the lowercase file extension.
func (f *FileItem) Extension() string {
	ext := filepath.Ext(f.OriginalFilename)
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// Snippet represents a text paste up to 10,000 runes.
type Snippet struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	CategoryID     *string    `json:"category_id"`
	CategoryName   string     `json:"category_name,omitempty"`
	CategoryPath   string     `json:"category_path,omitempty"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	Language       string     `json:"language"` // e.g. "text", "go", "json", "python", "markdown"
	Tags           []string   `json:"tags"`
	ShareToken     *string    `json:"share_token,omitempty"`
	ShareExpiresAt *time.Time `json:"share_expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CharacterCount returns the number of UTF-8 runes in the snippet content.
func (s *Snippet) CharacterCount() int {
	return len([]rune(s.Content))
}

// SnippetPreview returns a truncated snippet string for dashboard display.
func (s *Snippet) SnippetPreview(maxLen int) string {
	runes := []rune(s.Content)
	if len(runes) <= maxLen {
		return s.Content
	}
	return string(runes[:maxLen]) + "..."
}

// Tag represents a keyword label attached to files and snippets.
type Tag struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	ItemCount int       `json:"item_count,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ItemType defines whether a shareable target is a file or a text snippet.
type ItemType string

const (
	ItemTypeFile    ItemType = "file"
	ItemTypeSnippet ItemType = "snippet"
)

// ShareLink represents a public access token for a specific file or snippet.
type ShareLink struct {
	Token     string     `json:"token"`
	UserID    string     `json:"user_id"`
	ItemID    string     `json:"item_id"`
	ItemType  ItemType   `json:"item_type"`
	ExpiresAt *time.Time `json:"expires_at"` // nil = never expires
	ViewCount int        `json:"view_count"`
	CreatedAt time.Time  `json:"created_at"`

	// Populated target items for rendering
	File    *FileItem `json:"file,omitempty"`
	Snippet *Snippet  `json:"snippet,omitempty"`
}

// IsExpired checks if the share link's expiration date has passed.
func (s *ShareLink) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}

// SearchResult is a unified item representation for fuzzy search results with relevance score.
type SearchResult struct {
	ID           string    `json:"id"`
	Type         ItemType  `json:"type"` // "file" or "snippet"
	Title        string    `json:"title"`
	Subtitle     string    `json:"subtitle"`
	CategoryPath string    `json:"category_path"`
	Tags         []string  `json:"tags"`
	Score        int       `json:"score"`
	IsMP3        bool      `json:"is_mp3"`
	CreatedAt    time.Time `json:"created_at"`
	SizeOrLength string    `json:"size_or_length"`
}
