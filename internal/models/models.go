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
	IsAdmin      bool      `json:"is_admin"`
	IsApproved   bool      `json:"is_approved"` // Requires admin approval before login if false
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

// CategoryIDStr returns the category ID as a plain string, or empty string if nil.
func (f *FileItem) CategoryIDStr() string {
	if f.CategoryID != nil {
		return *f.CategoryID
	}
	return ""
}

// IsImage returns true if the file is an image.
func (f *FileItem) IsImage() bool {
	if strings.HasPrefix(strings.ToLower(f.MimeType), "image/") {
		return true
	}
	switch f.Extension() {
	case "png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico":
		return true
	}
	return false
}

// IsVideo returns true if the file is a playable video.
func (f *FileItem) IsVideo() bool {
	if strings.HasPrefix(strings.ToLower(f.MimeType), "video/") {
		return true
	}
	switch f.Extension() {
	case "mp4", "webm", "ogg", "mov", "m4v":
		return true
	}
	return false
}

// IsPDF returns true if the file is a PDF document.
func (f *FileItem) IsPDF() bool {
	return strings.ToLower(f.MimeType) == "application/pdf" || f.Extension() == "pdf"
}

// IsAudio returns true if the file is an audio file.
func (f *FileItem) IsAudio() bool {
	if f.IsMP3 || strings.HasPrefix(strings.ToLower(f.MimeType), "audio/") {
		return true
	}
	switch f.Extension() {
	case "mp3", "wav", "ogg", "m4a", "aac", "flac":
		return true
	}
	return false
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

// CategoryIDStr returns the category ID as a plain string, or empty string if nil.
func (s *Snippet) CategoryIDStr() string {
	if s.CategoryID != nil {
		return *s.CategoryID
	}
	return ""
}

// IsMarkdown returns true if the snippet language is markdown.
func (s *Snippet) IsMarkdown() bool {
	lang := strings.ToLower(strings.TrimSpace(s.Language))
	return lang == "markdown" || lang == "md"
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
	Token        string     `json:"token"`
	UserID       string     `json:"user_id"`
	ItemID       string     `json:"item_id"`
	ItemType     ItemType   `json:"item_type"`
	PasswordHash string     `json:"-"`
	MaxViews     int        `json:"max_views"` // 0 = unlimited, >0 = self-destruct after max views
	ExpiresAt    *time.Time `json:"expires_at"` // nil = never expires
	ViewCount    int        `json:"view_count"`
	CreatedAt    time.Time  `json:"created_at"`

	// Populated target items for rendering
	File    *FileItem `json:"file,omitempty"`
	Snippet *Snippet  `json:"snippet,omitempty"`
}

// HasPassword returns true if the share link requires a password to access.
func (s *ShareLink) HasPassword() bool {
	return s.PasswordHash != ""
}

// IsExhausted returns true if the share link has reached its maximum view limit.
func (s *ShareLink) IsExhausted() bool {
	return s.MaxViews > 0 && s.ViewCount >= s.MaxViews
}

// IsExpired checks if the share link's expiration date has passed.
func (s *ShareLink) IsExpired() bool {
	if s.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*s.ExpiresAt)
}

// StorageStats aggregates disk usage and counts for user dashboard reporting.
type StorageStats struct {
	TotalBytes   int64 `json:"total_bytes"`
	ImageBytes   int64 `json:"image_bytes"`
	AudioBytes   int64 `json:"audio_bytes"`
	VideoBytes   int64 `json:"video_bytes"`
	DocBytes     int64 `json:"doc_bytes"`
	OtherBytes   int64 `json:"other_bytes"`
	SnippetCount int   `json:"snippet_count"`
	FileCount    int   `json:"file_count"`
}

// FormattedTotal returns human-readable string for total storage used.
func (st *StorageStats) FormattedTotal() string {
	const unit = 1024
	if st.TotalBytes < unit {
		return fmt.Sprintf("%d B", st.TotalBytes)
	}
	div, exp := int64(unit), 0
	for n := st.TotalBytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(st.TotalBytes)/float64(div), "KMGTPE"[exp])
}

// PercentUsage returns estimated percentage against default quota of 100MB (or custom max).
func (st *StorageStats) PercentUsage(quotaBytes int64) float64 {
	if quotaBytes <= 0 {
		quotaBytes = 100 * 1024 * 1024
	}
	pct := (float64(st.TotalBytes) / float64(quotaBytes)) * 100.0
	if pct > 100.0 {
		return 100.0
	}
	return pct
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
