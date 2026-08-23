package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"filemanager/internal/models"
	"filemanager/internal/services"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements the Store interface using SQLite.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore opens the SQLite database, configures pragmas, and migrates the schema.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)

	s := &SQLiteStore{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

// Close closes the underlying SQLite database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) initSchema() error {
	schema := `
	PRAGMA foreign_keys = ON;

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL COLLATE NOCASE,
		password_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS categories (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		parent_id TEXT REFERENCES categories(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		depth INTEGER NOT NULL CHECK(depth >= 1 AND depth <= 4),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, parent_id, name)
	);

	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		category_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
		original_filename TEXT NOT NULL,
		stored_path TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		mime_type TEXT NOT NULL,
		sha256_checksum TEXT NOT NULL,
		is_mp3 BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS snippets (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		category_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		language TEXT NOT NULL DEFAULT 'text',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS tags (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name TEXT NOT NULL COLLATE NOCASE,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, name)
	);

	CREATE TABLE IF NOT EXISTS item_tags (
		item_id TEXT NOT NULL,
		item_type TEXT NOT NULL CHECK(item_type IN ('file', 'snippet')),
		tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
		PRIMARY KEY(item_id, item_type, tag_id)
	);

	CREATE TABLE IF NOT EXISTS share_links (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		item_id TEXT NOT NULL,
		item_type TEXT NOT NULL CHECK(item_type IN ('file', 'snippet')),
		expires_at DATETIME,
		view_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_files_user ON files(user_id, category_id);
	CREATE INDEX IF NOT EXISTS idx_files_name ON files(original_filename COLLATE NOCASE);
	CREATE INDEX IF NOT EXISTS idx_snippets_user ON snippets(user_id, category_id);
	CREATE INDEX IF NOT EXISTS idx_snippets_title ON snippets(title COLLATE NOCASE);
	CREATE INDEX IF NOT EXISTS idx_categories_user ON categories(user_id, parent_id);
	CREATE INDEX IF NOT EXISTS idx_item_tags ON item_tags(tag_id, item_type);
	CREATE INDEX IF NOT EXISTS idx_share_links_token ON share_links(token);
	`
	_, err := s.db.Exec(schema)
	return err
}

// -------------------------------------------------------------
// User Operations
// -------------------------------------------------------------

func (s *SQLiteStore) CreateUser(user *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.CreatedAt = time.Now().UTC()
	user.UpdatedAt = user.CreatedAt

	query := `INSERT INTO users (id, username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, user.ID, user.Username, user.PasswordHash, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrUserExists
		}
		return err
	}
	return nil
}

func (s *SQLiteStore) GetUserByUsername(username string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, username, password_hash, created_at, updated_at FROM users WHERE username = ? COLLATE NOCASE`
	row := s.db.QueryRow(query, username)

	var u models.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *SQLiteStore) GetUserByID(id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, username, password_hash, created_at, updated_at FROM users WHERE id = ?`
	row := s.db.QueryRow(query, id)

	var u models.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *SQLiteStore) CountUsers() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// -------------------------------------------------------------
// Session Operations
// -------------------------------------------------------------

func (s *SQLiteStore) CreateSession(session *models.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.CreatedAt = time.Now().UTC()
	query := `INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, session.Token, session.UserID, session.ExpiresAt, session.CreatedAt)
	return err
}

func (s *SQLiteStore) GetSession(token string) (*models.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = ?`
	row := s.db.QueryRow(query, token)

	var sess models.Session
	err := row.Scan(&sess.Token, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if sess.IsExpired() {
		return nil, ErrNotFound
	}
	return &sess, nil
}

func (s *SQLiteStore) DeleteSession(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *SQLiteStore) CleanupExpiredSessions() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP`)
	return err
}

// -------------------------------------------------------------
// Category Operations (Hierarchical up to 4 levels)
// -------------------------------------------------------------

func (s *SQLiteStore) CreateCategory(cat *models.Category) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cat.Name = strings.TrimSpace(cat.Name)
	if cat.Name == "" {
		return errors.New("category name cannot be empty")
	}

	depth := 1
	if cat.ParentID != nil && *cat.ParentID != "" {
		var parentDepth int
		var parentUserID string
		err := s.db.QueryRow(`SELECT depth, user_id FROM categories WHERE id = ?`, *cat.ParentID).Scan(&parentDepth, &parentUserID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCategoryNotFound
		}
		if err != nil {
			return err
		}
		if parentUserID != cat.UserID {
			return errors.New("unauthorized parent category")
		}
		if parentDepth >= 4 {
			return ErrMaxDepthExceeded
		}
		depth = parentDepth + 1
	}

	if cat.ID == "" {
		cat.ID = uuid.New().String()
	}
	cat.Depth = depth
	cat.CreatedAt = time.Now().UTC()

	query := `INSERT INTO categories (id, user_id, parent_id, name, depth, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, cat.ID, cat.UserID, cat.ParentID, cat.Name, cat.Depth, cat.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errors.New("category with this name already exists in this parent folder")
		}
		return err
	}

	path, _, _ := s.getCategoryPathInternal(cat.ID, cat.UserID)
	cat.Path = path
	return nil
}

func (s *SQLiteStore) getCategoryPathInternal(id, userID string) (string, int, error) {
	var names []string
	currID := id
	depth := 0

	for currID != "" && depth < 10 {
		var name string
		var parentID sql.NullString
		var d int
		err := s.db.QueryRow(`SELECT name, parent_id, depth FROM categories WHERE id = ? AND user_id = ?`, currID, userID).Scan(&name, &parentID, &d)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return "", 0, err
		}
		names = append([]string{name}, names...)
		if parentID.Valid && parentID.String != "" {
			currID = parentID.String
			depth++
		} else {
			break
		}
	}
	if len(names) == 0 {
		return "", 0, ErrNotFound
	}
	return strings.Join(names, " / "), len(names), nil
}

func (s *SQLiteStore) GetCategoryPath(id, userID string) (string, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getCategoryPathInternal(id, userID)
}

func (s *SQLiteStore) GetCategoryByID(id, userID string) (*models.Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cat models.Category
	var parentID sql.NullString
	query := `SELECT id, user_id, parent_id, name, depth, created_at FROM categories WHERE id = ? AND user_id = ?`
	err := s.db.QueryRow(query, id, userID).Scan(&cat.ID, &cat.UserID, &parentID, &cat.Name, &cat.Depth, &cat.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		cat.ParentID = &parentID.String
	}
	cat.Path, _, _ = s.getCategoryPathInternal(cat.ID, cat.UserID)
	return &cat, nil
}

func (s *SQLiteStore) ListCategories(userID string) ([]*models.Category, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, user_id, parent_id, name, depth, created_at FROM categories WHERE user_id = ? ORDER BY depth ASC, name ASC`
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}

	var list []*models.Category
	for rows.Next() {
		var cat models.Category
		var parentID sql.NullString
		if err := rows.Scan(&cat.ID, &cat.UserID, &parentID, &cat.Name, &cat.Depth, &cat.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if parentID.Valid {
			cat.ParentID = &parentID.String
		}
		list = append(list, &cat)
	}
	rows.Close()

	for _, c := range list {
		c.Path, _, _ = s.getCategoryPathInternal(c.ID, userID)
	}

	return list, nil
}

func (s *SQLiteStore) DeleteCategory(id, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec(`DELETE FROM categories WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// -------------------------------------------------------------
// Tag Operations
// -------------------------------------------------------------

func (s *SQLiteStore) assignItemTagsInternal(itemID string, itemType models.ItemType, tags []string, userID string) error {
	var cleanTags []string
	seen := make(map[string]bool)
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" && !seen[strings.ToLower(t)] {
			seen[strings.ToLower(t)] = true
			cleanTags = append(cleanTags, t)
		}
	}

	_, err := s.db.Exec(`DELETE FROM item_tags WHERE item_id = ? AND item_type = ?`, itemID, itemType)
	if err != nil {
		return err
	}

	for _, tag := range cleanTags {
		var tagID string
		err := s.db.QueryRow(`SELECT id FROM tags WHERE user_id = ? AND name = ? COLLATE NOCASE`, userID, tag).Scan(&tagID)
		if errors.Is(err, sql.ErrNoRows) {
			tagID = uuid.New().String()
			_, err = s.db.Exec(`INSERT INTO tags (id, user_id, name, created_at) VALUES (?, ?, ?, ?)`, tagID, userID, tag, time.Now().UTC())
			if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return err
			}
			if err != nil {
				_ = s.db.QueryRow(`SELECT id FROM tags WHERE user_id = ? AND name = ? COLLATE NOCASE`, userID, tag).Scan(&tagID)
			}
		} else if err != nil {
			return err
		}

		_, err = s.db.Exec(`INSERT OR IGNORE INTO item_tags (item_id, item_type, tag_id) VALUES (?, ?, ?)`, itemID, itemType, tagID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) AssignItemTags(itemID string, itemType models.ItemType, tags []string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.assignItemTagsInternal(itemID, itemType, tags, userID)
}

func (s *SQLiteStore) getItemTagsInternal(itemID string, itemType models.ItemType) ([]string, error) {
	query := `
	SELECT t.name 
	FROM tags t 
	JOIN item_tags it ON t.id = it.tag_id 
	WHERE it.item_id = ? AND it.item_type = ? 
	ORDER BY t.name ASC`

	rows, err := s.db.Query(query, itemID, itemType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, nil
}

func (s *SQLiteStore) GetItemTags(itemID string, itemType models.ItemType) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getItemTagsInternal(itemID, itemType)
}

func (s *SQLiteStore) ListUserTags(userID string) ([]models.Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT t.id, t.user_id, t.name, t.created_at, COUNT(it.item_id) as item_count
	FROM tags t
	LEFT JOIN item_tags it ON t.id = it.tag_id
	WHERE t.user_id = ?
	GROUP BY t.id
	ORDER BY t.name ASC`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []models.Tag
	for rows.Next() {
		var t models.Tag
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.CreatedAt, &t.ItemCount); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

// -------------------------------------------------------------
// File Operations
// -------------------------------------------------------------

func (s *SQLiteStore) CreateFile(file *models.FileItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if file.ID == "" {
		file.ID = uuid.New().String()
	}
	file.CreatedAt = time.Now().UTC()
	file.UpdatedAt = file.CreatedAt

	ext := strings.ToLower(filepath.Ext(file.OriginalFilename))
	if ext == ".mp3" || strings.Contains(strings.ToLower(file.MimeType), "audio/mpeg") || strings.Contains(strings.ToLower(file.MimeType), "audio/mp3") {
		file.IsMP3 = true
	}

	query := `
	INSERT INTO files (id, user_id, category_id, original_filename, stored_path, file_size, mime_type, sha256_checksum, is_mp3, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query,
		file.ID, file.UserID, file.CategoryID, file.OriginalFilename,
		file.StoredPath, file.FileSize, file.MimeType, file.Sha256Checksum,
		file.IsMP3, file.CreatedAt, file.UpdatedAt)
	if err != nil {
		return err
	}

	if len(file.Tags) > 0 {
		if err := s.assignItemTagsInternal(file.ID, models.ItemTypeFile, file.Tags, file.UserID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) getFileByIDInternal(id, userID string) (*models.FileItem, error) {
	query := `
	SELECT f.id, f.user_id, f.category_id, f.original_filename, f.stored_path, f.file_size, f.mime_type, f.sha256_checksum, f.is_mp3, f.created_at, f.updated_at,
	       c.name as category_name, sl.token as share_token, sl.expires_at as share_expires_at
	FROM files f
	LEFT JOIN categories c ON f.category_id = c.id
	LEFT JOIN share_links sl ON f.id = sl.item_id AND sl.item_type = 'file'
	WHERE f.id = ? AND f.user_id = ?`

	row := s.db.QueryRow(query, id, userID)
	var f models.FileItem
	var catID, catName, shareToken sql.NullString
	var shareExpiresAt sql.NullTime

	err := row.Scan(&f.ID, &f.UserID, &catID, &f.OriginalFilename, &f.StoredPath, &f.FileSize, &f.MimeType, &f.Sha256Checksum, &f.IsMP3, &f.CreatedAt, &f.UpdatedAt,
		&catName, &shareToken, &shareExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if catID.Valid {
		f.CategoryID = &catID.String
		f.CategoryName = catName.String
		f.CategoryPath, _, _ = s.getCategoryPathInternal(catID.String, userID)
	}
	if shareToken.Valid {
		f.ShareToken = &shareToken.String
		if shareExpiresAt.Valid {
			f.ShareExpiresAt = &shareExpiresAt.Time
		}
	}

	tags, _ := s.getItemTagsInternal(f.ID, models.ItemTypeFile)
	f.Tags = tags
	return &f, nil
}

func (s *SQLiteStore) GetFileByID(id, userID string) (*models.FileItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getFileByIDInternal(id, userID)
}

func (s *SQLiteStore) getFileByIDPublicInternal(id string) (*models.FileItem, error) {
	query := `
	SELECT f.id, f.user_id, f.category_id, f.original_filename, f.stored_path, f.file_size, f.mime_type, f.sha256_checksum, f.is_mp3, f.created_at, f.updated_at
	FROM files f
	WHERE f.id = ?`

	row := s.db.QueryRow(query, id)
	var f models.FileItem
	var catID sql.NullString

	err := row.Scan(&f.ID, &f.UserID, &catID, &f.OriginalFilename, &f.StoredPath, &f.FileSize, &f.MimeType, &f.Sha256Checksum, &f.IsMP3, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if catID.Valid {
		f.CategoryID = &catID.String
	}
	tags, _ := s.getItemTagsInternal(f.ID, models.ItemTypeFile)
	f.Tags = tags
	return &f, nil
}

func (s *SQLiteStore) GetFileByIDPublic(id string) (*models.FileItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getFileByIDPublicInternal(id)
}

func (s *SQLiteStore) ListFiles(userID string, categoryID *string, tagFilter string) ([]*models.FileItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(`
	SELECT DISTINCT f.id, f.user_id, f.category_id, f.original_filename, f.stored_path, f.file_size, f.mime_type, f.sha256_checksum, f.is_mp3, f.created_at, f.updated_at,
	       c.name as category_name, sl.token as share_token, sl.expires_at as share_expires_at
	FROM files f
	LEFT JOIN categories c ON f.category_id = c.id
	LEFT JOIN share_links sl ON f.id = sl.item_id AND sl.item_type = 'file'
	`)

	var args []interface{}
	var whereClauses []string

	whereClauses = append(whereClauses, "f.user_id = ?")
	args = append(args, userID)

	if categoryID != nil {
		if *categoryID == "" {
			whereClauses = append(whereClauses, "f.category_id IS NULL")
		} else {
			whereClauses = append(whereClauses, "f.category_id = ?")
			args = append(args, *categoryID)
		}
	}

	if tagFilter != "" {
		sb.WriteString(` JOIN item_tags it ON f.id = it.item_id AND it.item_type = 'file' `)
		sb.WriteString(` JOIN tags t ON it.tag_id = t.id `)
		whereClauses = append(whereClauses, "t.name = ? COLLATE NOCASE")
		args = append(args, tagFilter)
	}

	sb.WriteString(" WHERE ")
	sb.WriteString(strings.Join(whereClauses, " AND "))
	sb.WriteString(" ORDER BY f.created_at DESC")

	rows, err := s.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}

	var files []*models.FileItem
	for rows.Next() {
		var f models.FileItem
		var catID, catName, shareToken sql.NullString
		var shareExpiresAt sql.NullTime

		if err := rows.Scan(&f.ID, &f.UserID, &catID, &f.OriginalFilename, &f.StoredPath, &f.FileSize, &f.MimeType, &f.Sha256Checksum, &f.IsMP3, &f.CreatedAt, &f.UpdatedAt,
			&catName, &shareToken, &shareExpiresAt); err != nil {
			rows.Close()
			return nil, err
		}

		if catID.Valid {
			f.CategoryID = &catID.String
			f.CategoryName = catName.String
		}
		if shareToken.Valid {
			f.ShareToken = &shareToken.String
			if shareExpiresAt.Valid {
				f.ShareExpiresAt = &shareExpiresAt.Time
			}
		}
		files = append(files, &f)
	}
	rows.Close()

	// Populate category paths and tags AFTER closing the main rows iterator
	for _, f := range files {
		if f.CategoryID != nil && *f.CategoryID != "" {
			f.CategoryPath, _, _ = s.getCategoryPathInternal(*f.CategoryID, userID)
		}
		tags, _ := s.getItemTagsInternal(f.ID, models.ItemTypeFile)
		f.Tags = tags
	}

	return files, nil
}

func (s *SQLiteStore) DeleteFile(id, userID string) (*models.FileItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := s.getFileByIDInternal(id, userID)
	if err != nil {
		return nil, err
	}

	_, err = s.db.Exec(`DELETE FROM files WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (s *SQLiteStore) GetFileByChecksum(userID, checksum string) (*models.FileItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, user_id, category_id, original_filename, stored_path, file_size, mime_type, sha256_checksum, is_mp3, created_at, updated_at FROM files WHERE user_id = ? AND sha256_checksum = ? LIMIT 1`
	row := s.db.QueryRow(query, userID, checksum)

	var f models.FileItem
	var catID sql.NullString
	err := row.Scan(&f.ID, &f.UserID, &catID, &f.OriginalFilename, &f.StoredPath, &f.FileSize, &f.MimeType, &f.Sha256Checksum, &f.IsMP3, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if catID.Valid {
		f.CategoryID = &catID.String
	}
	return &f, nil
}

// -------------------------------------------------------------
// Snippet Operations
// -------------------------------------------------------------

func (s *SQLiteStore) CreateSnippet(snippet *models.Snippet) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if snippet.ID == "" {
		snippet.ID = uuid.New().String()
	}
	snippet.CreatedAt = time.Now().UTC()
	snippet.UpdatedAt = snippet.CreatedAt
	if snippet.Language == "" {
		snippet.Language = "text"
	}

	query := `INSERT INTO snippets (id, user_id, category_id, title, content, language, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, snippet.ID, snippet.UserID, snippet.CategoryID, snippet.Title, snippet.Content, snippet.Language, snippet.CreatedAt, snippet.UpdatedAt)
	if err != nil {
		return err
	}

	if len(snippet.Tags) > 0 {
		if err := s.assignItemTagsInternal(snippet.ID, models.ItemTypeSnippet, snippet.Tags, snippet.UserID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) getSnippetByIDInternal(id, userID string) (*models.Snippet, error) {
	query := `
	SELECT sn.id, sn.user_id, sn.category_id, sn.title, sn.content, sn.language, sn.created_at, sn.updated_at,
	       c.name as category_name, sl.token as share_token, sl.expires_at as share_expires_at
	FROM snippets sn
	LEFT JOIN categories c ON sn.category_id = c.id
	LEFT JOIN share_links sl ON sn.id = sl.item_id AND sl.item_type = 'snippet'
	WHERE sn.id = ? AND sn.user_id = ?`

	row := s.db.QueryRow(query, id, userID)
	var sn models.Snippet
	var catID, catName, shareToken sql.NullString
	var shareExpiresAt sql.NullTime

	err := row.Scan(&sn.ID, &sn.UserID, &catID, &sn.Title, &sn.Content, &sn.Language, &sn.CreatedAt, &sn.UpdatedAt,
		&catName, &shareToken, &shareExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if catID.Valid {
		sn.CategoryID = &catID.String
		sn.CategoryName = catName.String
		sn.CategoryPath, _, _ = s.getCategoryPathInternal(catID.String, userID)
	}
	if shareToken.Valid {
		sn.ShareToken = &shareToken.String
		if shareExpiresAt.Valid {
			sn.ShareExpiresAt = &shareExpiresAt.Time
		}
	}

	tags, _ := s.getItemTagsInternal(sn.ID, models.ItemTypeSnippet)
	sn.Tags = tags
	return &sn, nil
}

func (s *SQLiteStore) GetSnippetByID(id, userID string) (*models.Snippet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getSnippetByIDInternal(id, userID)
}

func (s *SQLiteStore) getSnippetByIDPublicInternal(id string) (*models.Snippet, error) {
	query := `SELECT id, user_id, category_id, title, content, language, created_at, updated_at FROM snippets WHERE id = ?`
	row := s.db.QueryRow(query, id)

	var sn models.Snippet
	var catID sql.NullString
	err := row.Scan(&sn.ID, &sn.UserID, &catID, &sn.Title, &sn.Content, &sn.Language, &sn.CreatedAt, &sn.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if catID.Valid {
		sn.CategoryID = &catID.String
	}
	tags, _ := s.getItemTagsInternal(sn.ID, models.ItemTypeSnippet)
	sn.Tags = tags
	return &sn, nil
}

func (s *SQLiteStore) GetSnippetByIDPublic(id string) (*models.Snippet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getSnippetByIDPublicInternal(id)
}

func (s *SQLiteStore) ListSnippets(userID string, categoryID *string, tagFilter string) ([]*models.Snippet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(`
	SELECT DISTINCT sn.id, sn.user_id, sn.category_id, sn.title, sn.content, sn.language, sn.created_at, sn.updated_at,
	       c.name as category_name, sl.token as share_token, sl.expires_at as share_expires_at
	FROM snippets sn
	LEFT JOIN categories c ON sn.category_id = c.id
	LEFT JOIN share_links sl ON sn.id = sl.item_id AND sl.item_type = 'snippet'
	`)

	var args []interface{}
	var whereClauses []string

	whereClauses = append(whereClauses, "sn.user_id = ?")
	args = append(args, userID)

	if categoryID != nil {
		if *categoryID == "" {
			whereClauses = append(whereClauses, "sn.category_id IS NULL")
		} else {
			whereClauses = append(whereClauses, "sn.category_id = ?")
			args = append(args, *categoryID)
		}
	}

	if tagFilter != "" {
		sb.WriteString(` JOIN item_tags it ON sn.id = it.item_id AND it.item_type = 'snippet' `)
		sb.WriteString(` JOIN tags t ON it.tag_id = t.id `)
		whereClauses = append(whereClauses, "t.name = ? COLLATE NOCASE")
		args = append(args, tagFilter)
	}

	sb.WriteString(" WHERE ")
	sb.WriteString(strings.Join(whereClauses, " AND "))
	sb.WriteString(" ORDER BY sn.created_at DESC")

	rows, err := s.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}

	var snippets []*models.Snippet
	for rows.Next() {
		var sn models.Snippet
		var catID, catName, shareToken sql.NullString
		var shareExpiresAt sql.NullTime

		if err := rows.Scan(&sn.ID, &sn.UserID, &catID, &sn.Title, &sn.Content, &sn.Language, &sn.CreatedAt, &sn.UpdatedAt,
			&catName, &shareToken, &shareExpiresAt); err != nil {
			rows.Close()
			return nil, err
		}

		if catID.Valid {
			sn.CategoryID = &catID.String
			sn.CategoryName = catName.String
		}
		if shareToken.Valid {
			sn.ShareToken = &shareToken.String
			if shareExpiresAt.Valid {
				sn.ShareExpiresAt = &shareExpiresAt.Time
			}
		}

		snippets = append(snippets, &sn)
	}
	rows.Close()

	// Populate category paths and tags AFTER closing the main rows iterator
	for _, sn := range snippets {
		if sn.CategoryID != nil && *sn.CategoryID != "" {
			sn.CategoryPath, _, _ = s.getCategoryPathInternal(*sn.CategoryID, userID)
		}
		tags, _ := s.getItemTagsInternal(sn.ID, models.ItemTypeSnippet)
		sn.Tags = tags
	}

	return snippets, nil
}

func (s *SQLiteStore) DeleteSnippet(id, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec(`DELETE FROM snippets WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// -------------------------------------------------------------
// Share Link Operations
// -------------------------------------------------------------

func (s *SQLiteStore) CreateShareLink(link *models.ShareLink) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	link.CreatedAt = time.Now().UTC()
	_, _ = s.db.Exec(`DELETE FROM share_links WHERE user_id = ? AND item_id = ? AND item_type = ?`, link.UserID, link.ItemID, link.ItemType)

	query := `INSERT INTO share_links (token, user_id, item_id, item_type, expires_at, view_count, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, link.Token, link.UserID, link.ItemID, link.ItemType, link.ExpiresAt, link.ViewCount, link.CreatedAt)
	return err
}

func (s *SQLiteStore) GetShareLink(token string) (*models.ShareLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT token, user_id, item_id, item_type, expires_at, view_count, created_at FROM share_links WHERE token = ?`
	row := s.db.QueryRow(query, token)

	var sl models.ShareLink
	var expiresAt sql.NullTime
	var itemTypeStr string

	err := row.Scan(&sl.Token, &sl.UserID, &sl.ItemID, &itemTypeStr, &expiresAt, &sl.ViewCount, &sl.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	sl.ItemType = models.ItemType(itemTypeStr)
	if expiresAt.Valid {
		sl.ExpiresAt = &expiresAt.Time
	}

	if sl.IsExpired() {
		return nil, ErrShareExpired
	}

	if sl.ItemType == models.ItemTypeFile {
		file, err := s.getFileByIDPublicInternal(sl.ItemID)
		if err == nil {
			sl.File = file
		}
	} else if sl.ItemType == models.ItemTypeSnippet {
		snippet, err := s.getSnippetByIDPublicInternal(sl.ItemID)
		if err == nil {
			sl.Snippet = snippet
		}
	}

	return &sl, nil
}

func (s *SQLiteStore) DeleteShareLink(token, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec(`DELETE FROM share_links WHERE token = ? AND user_id = ?`, token, userID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) IncrementShareView(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE share_links SET view_count = view_count + 1 WHERE token = ?`, token)
	return err
}

func (s *SQLiteStore) GetShareLinkByItem(itemID string, itemType models.ItemType, userID string) (*models.ShareLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT token, user_id, item_id, item_type, expires_at, view_count, created_at FROM share_links WHERE item_id = ? AND item_type = ? AND user_id = ?`
	row := s.db.QueryRow(query, itemID, itemType, userID)

	var sl models.ShareLink
	var expiresAt sql.NullTime
	var itemTypeStr string

	err := row.Scan(&sl.Token, &sl.UserID, &sl.ItemID, &itemTypeStr, &expiresAt, &sl.ViewCount, &sl.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	sl.ItemType = models.ItemType(itemTypeStr)
	if expiresAt.Valid {
		sl.ExpiresAt = &expiresAt.Time
	}
	return &sl, nil
}

// -------------------------------------------------------------
// Fuzzy Search Implementation
// -------------------------------------------------------------

func (s *SQLiteStore) SearchItems(userID, query string, tagFilter string) ([]models.SearchResult, error) {
	files, err := s.ListFiles(userID, nil, tagFilter)
	if err != nil {
		return nil, err
	}

	snippets, err := s.ListSnippets(userID, nil, tagFilter)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	q := strings.TrimSpace(query)

	for _, f := range files {
		score := 0
		matched := false

		if q == "" {
			matched = true
			score = 100
		} else {
			if m, sc := services.FuzzyMatch(q, f.OriginalFilename); m {
				matched = true
				if sc > score {
					score = sc
				}
			}
			if f.CategoryPath != "" {
				if m, sc := services.FuzzyMatch(q, f.CategoryPath); m {
					matched = true
					if sc-50 > score {
						score = sc - 50
					}
				}
			}
			if m, sc := services.MatchTags(q, f.Tags); m {
				matched = true
				if sc > score {
					score = sc
				}
			}
		}

		if matched {
			results = append(results, models.SearchResult{
				ID:           f.ID,
				Type:         models.ItemTypeFile,
				Title:        f.OriginalFilename,
				Subtitle:     fmt.Sprintf("%s • %s", f.FormattedSize(), f.MimeType),
				CategoryPath: f.CategoryPath,
				Tags:         f.Tags,
				Score:        score,
				IsMP3:        f.IsMP3,
				CreatedAt:    f.CreatedAt,
				SizeOrLength: f.FormattedSize(),
			})
		}
	}

	for _, sn := range snippets {
		score := 0
		matched := false

		if q == "" {
			matched = true
			score = 100
		} else {
			if m, sc := services.FuzzyMatch(q, sn.Title); m {
				matched = true
				if sc > score {
					score = sc
				}
			}
			if m, sc := services.FuzzyMatch(q, sn.SnippetPreview(200)); m {
				matched = true
				if sc-100 > score {
					score = sc - 100
				}
			}
			if sn.CategoryPath != "" {
				if m, sc := services.FuzzyMatch(q, sn.CategoryPath); m {
					matched = true
					if sc-50 > score {
						score = sc - 50
					}
				}
			}
			if m, sc := services.MatchTags(q, sn.Tags); m {
				matched = true
				if sc > score {
					score = sc
				}
			}
		}

		if matched {
			results = append(results, models.SearchResult{
				ID:           sn.ID,
				Type:         models.ItemTypeSnippet,
				Title:        sn.Title,
				Subtitle:     fmt.Sprintf("%s (%d chars)", sn.Language, sn.CharacterCount()),
				CategoryPath: sn.CategoryPath,
				Tags:         sn.Tags,
				Score:        score,
				IsMP3:        false,
				CreatedAt:    sn.CreatedAt,
				SizeOrLength: fmt.Sprintf("%d chars", sn.CharacterCount()),
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}
