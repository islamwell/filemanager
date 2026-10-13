package store

import (
	"errors"
	"filemanager/internal/models"
)

var (
	ErrNotFound         = errors.New("record not found")
	ErrUserExists       = errors.New("username already exists")
	ErrMaxDepthExceeded = errors.New("category depth limit of 4 levels exceeded")
	ErrCategoryNotFound = errors.New("parent category not found")
	ErrCategoryCycle    = errors.New("category hierarchy cycle detected")
	ErrShareExpired     = errors.New("share link has expired or is invalid")
)

// Store defines the storage repository interface for the application.
type Store interface {
	Close() error

	// Users
	CreateUser(user *models.User) error
	GetUserByUsername(username string) (*models.User, error)
	GetUserByID(id string) (*models.User, error)
	CountUsers() (int, error)

	// Sessions
	CreateSession(session *models.Session) error
	GetSession(token string) (*models.Session, error)
	DeleteSession(token string) error
	CleanupExpiredSessions() error

	// Categories
	CreateCategory(category *models.Category) error
	GetCategoryByID(id, userID string) (*models.Category, error)
	ListCategories(userID string) ([]*models.Category, error)
	DeleteCategory(id, userID string) error
	GetCategoryPath(id, userID string) (string, int, error)

	// Files
	CreateFile(file *models.FileItem) error
	GetFileByID(id, userID string) (*models.FileItem, error)
	GetFileByIDPublic(id string) (*models.FileItem, error)
	ListFiles(userID string, categoryID *string, tagFilter string) ([]*models.FileItem, error)
	DeleteFile(id, userID string) (*models.FileItem, error)
	GetFileByChecksum(userID, checksum string) (*models.FileItem, error)
	MoveFile(id, userID string, newCategoryID *string) error
	RenameFile(id, userID, newFilename string) error
	GetFilesByIDs(ids []string, userID string) ([]*models.FileItem, error)
	DeleteFiles(ids []string, userID string) ([]*models.FileItem, error)
	MoveFiles(ids []string, userID string, newCategoryID *string) error

	// Snippets
	CreateSnippet(snippet *models.Snippet) error
	GetSnippetByID(id, userID string) (*models.Snippet, error)
	GetSnippetByIDPublic(id string) (*models.Snippet, error)
	ListSnippets(userID string, categoryID *string, tagFilter string) ([]*models.Snippet, error)
	UpdateSnippet(snippet *models.Snippet) error
	DeleteSnippet(id, userID string) error
	MoveSnippet(id, userID string, newCategoryID *string) error
	RenameSnippet(id, userID, newTitle string) error
	DeleteSnippets(ids []string, userID string) error

	// Tags
	AssignItemTags(itemID string, itemType models.ItemType, tags []string, userID string) error
	GetItemTags(itemID string, itemType models.ItemType) ([]string, error)
	ListUserTags(userID string) ([]models.Tag, error)

	// Share Links
	CreateShareLink(link *models.ShareLink) error
	GetShareLink(token string) (*models.ShareLink, error)
	DeleteShareLink(token, userID string) error
	IncrementShareView(token string) error
	GetShareLinkByItem(itemID string, itemType models.ItemType, userID string) (*models.ShareLink, error)

	// Fuzzy Search
	SearchItems(userID, query string, tagFilter string) ([]models.SearchResult, error)

	// Storage Analytics
	GetUserStorageStats(userID string) (*models.StorageStats, error)
}
