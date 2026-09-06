package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filemanager/internal/handlers"
	"filemanager/internal/middleware"
	"filemanager/internal/models"
)

func TestMediaInlineViewAndRangeStreaming(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	fileHandler := handlers.NewFileHandler(base)
	shareHandler := handlers.NewShareHandler(base)

	user := &models.User{Username: "alice_media", PasswordHash: "hash"}
	if err := st.CreateUser(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. Create a dummy image file
	imgData := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR...")
	imgPath := filepath.Join(base.Config.StorageDir, "photo.png")
	if err := os.WriteFile(imgPath, imgData, 0640); err != nil {
		t.Fatalf("failed to write img: %v", err)
	}

	imgItem := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "photo.png",
		StoredPath:       imgPath,
		FileSize:         int64(len(imgData)),
		MimeType:         "image/png",
		Sha256Checksum:   "dummy_sha",
	}
	if err := st.CreateFile(imgItem); err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}

	// Test Authenticated ViewFileInline
	req := httptest.NewRequest("GET", "/files/"+imgItem.ID+"/view", nil)
	req.SetPathValue("id", imgItem.ID)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	fileHandler.ViewFileInline(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for inline image, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected Content-Type image/png, got %s", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); !strings.Contains(cd, "inline") {
		t.Errorf("expected Content-Disposition inline, got %s", cd)
	}

	// 2. Test Video with Range Streaming (206 Partial Content)
	vidData := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	vidPath := filepath.Join(base.Config.StorageDir, "clip.mp4")
	if err := os.WriteFile(vidPath, vidData, 0640); err != nil {
		t.Fatalf("failed to write video: %v", err)
	}

	vidItem := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "clip.mp4",
		StoredPath:       vidPath,
		FileSize:         int64(len(vidData)),
		MimeType:         "video/mp4",
		Sha256Checksum:   "dummy_vid_sha",
	}
	if err := st.CreateFile(vidItem); err != nil {
		t.Fatalf("failed to insert video: %v", err)
	}

	rangeReq := httptest.NewRequest("GET", "/files/"+vidItem.ID+"/view", nil)
	rangeReq.SetPathValue("id", vidItem.ID)
	rangeReq.Header.Set("Range", "bytes=0-9")
	rangeReq = rangeReq.WithContext(ctx)
	rangeRR := httptest.NewRecorder()

	fileHandler.ViewFileInline(rangeRR, rangeReq)

	if rangeRR.Code != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content for byte-range video streaming, got %d", rangeRR.Code)
	}
	if rangeRR.Body.String() != "0123456789" {
		t.Errorf("expected range body '0123456789', got '%s'", rangeRR.Body.String())
	}

	// 3. Test Public Shared Media Inline View
	shareLink := &models.ShareLink{
		Token:    "public-photo-token",
		UserID:   user.ID,
		ItemID:   imgItem.ID,
		ItemType: models.ItemTypeFile,
	}
	if err := st.CreateShareLink(shareLink); err != nil {
		t.Fatalf("failed to create share link: %v", err)
	}

	pubReq := httptest.NewRequest("GET", "/share/public-photo-token/view", nil)
	pubReq.SetPathValue("token", "public-photo-token")
	pubRR := httptest.NewRecorder()

	shareHandler.ViewSharedMediaInline(pubRR, pubReq)

	if pubRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for shared media inline, got %d", pubRR.Code)
	}
	if ct := pubRR.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected Content-Type image/png, got %s", ct)
	}
	if cd := pubRR.Header().Get("Content-Disposition"); !strings.Contains(cd, "inline") {
		t.Errorf("expected Content-Disposition inline, got %s", cd)
	}
}

func TestFileAndSnippetMoveOperations(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	fileHandler := handlers.NewFileHandler(base)
	snippetHandler := handlers.NewSnippetHandler(base)

	user := &models.User{Username: "bob_mover", PasswordHash: "hash"}
	_ = st.CreateUser(user)

	// Create Category 1 and Category 2
	cat1 := &models.Category{UserID: user.ID, Name: "Folder A"}
	_ = st.CreateCategory(cat1)
	cat2 := &models.Category{UserID: user.ID, Name: "Folder B"}
	_ = st.CreateCategory(cat2)

	// Create File in Folder A
	fPath := filepath.Join(base.Config.StorageDir, "move_file.txt")
	_ = os.WriteFile(fPath, []byte("Content to move"), 0640)

	fileItem := &models.FileItem{
		UserID:           user.ID,
		CategoryID:       &cat1.ID,
		OriginalFilename: "move_file.txt",
		StoredPath:       fPath,
		FileSize:         15,
		MimeType:         "text/plain",
		Sha256Checksum:   "sha_move",
	}
	_ = st.CreateFile(fileItem)

	ctx := context.WithValue(context.Background(), middleware.UserContextKey, user)

	// 1. Move file to Folder B
	moveForm := url.Values{"category_id": {cat2.ID}}
	req := httptest.NewRequest("POST", "/files/"+fileItem.ID+"/move", strings.NewReader(moveForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", fileItem.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	fileHandler.MoveFile(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303 after file move, got %d", rr.Code)
	}

	updatedFile, err := st.GetFileByID(fileItem.ID, user.ID)
	if err != nil {
		t.Fatalf("failed to fetch updated file: %v", err)
	}
	if updatedFile.CategoryID == nil || *updatedFile.CategoryID != cat2.ID {
		t.Errorf("expected file category to be %s, got %v", cat2.ID, updatedFile.CategoryID)
	}

	// 2. Move file back to Root (empty category_id)
	rootForm := url.Values{"category_id": {""}}
	reqRoot := httptest.NewRequest("POST", "/files/"+fileItem.ID+"/move", strings.NewReader(rootForm.Encode()))
	reqRoot.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqRoot.SetPathValue("id", fileItem.ID)
	reqRoot = reqRoot.WithContext(ctx)
	rrRoot := httptest.NewRecorder()

	fileHandler.MoveFile(rrRoot, reqRoot)
	if rrRoot.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303 after file move to root, got %d", rrRoot.Code)
	}

	rootFile, _ := st.GetFileByID(fileItem.ID, user.ID)
	if rootFile.CategoryID != nil {
		t.Errorf("expected file category to be nil (root), got %v", rootFile.CategoryID)
	}

	// 3. Snippet Move: Create Snippet in Folder A
	snippet := &models.Snippet{
		UserID:     user.ID,
		CategoryID: &cat1.ID,
		Title:      "Snippet to Move",
		Content:    "Sample snippet body",
		Language:   "go",
	}
	_ = st.CreateSnippet(snippet)

	// Move snippet to Folder B
	snMoveForm := url.Values{"category_id": {cat2.ID}}
	snReq := httptest.NewRequest("POST", "/snippets/"+snippet.ID+"/move", strings.NewReader(snMoveForm.Encode()))
	snReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	snReq.SetPathValue("id", snippet.ID)
	snReq = snReq.WithContext(ctx)
	snRR := httptest.NewRecorder()

	snippetHandler.MoveSnippet(snRR, snReq)
	if snRR.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303 after snippet move, got %d", snRR.Code)
	}

	updatedSn, _ := st.GetSnippetByID(snippet.ID, user.ID)
	if updatedSn.CategoryID == nil || *updatedSn.CategoryID != cat2.ID {
		t.Errorf("expected snippet category to be %s, got %v", cat2.ID, updatedSn.CategoryID)
	}
}

func TestSnippetEditing(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	snippetHandler := handlers.NewSnippetHandler(base)

	user := &models.User{Username: "charlie_editor", PasswordHash: "hash"}
	_ = st.CreateUser(user)
	ctx := context.WithValue(context.Background(), middleware.UserContextKey, user)

	snippet := &models.Snippet{
		UserID:   user.ID,
		Title:    "Original Title",
		Content:  "Original Content",
		Language: "text",
		Tags:     []string{"initial"},
	}
	if err := st.CreateSnippet(snippet); err != nil {
		t.Fatalf("failed to create snippet: %v", err)
	}

	// 1. GET /snippets/{id}/edit view
	editReq := httptest.NewRequest("GET", "/snippets/"+snippet.ID+"/edit", nil)
	editReq.SetPathValue("id", snippet.ID)
	editReq = editReq.WithContext(ctx)
	editRR := httptest.NewRecorder()

	snippetHandler.EditSnippetView(editRR, editReq)
	if editRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for EditSnippetView, got %d", editRR.Code)
	}
	if !strings.Contains(editRR.Body.String(), "Original Title") {
		t.Errorf("edit view missing original title")
	}

	// 2. POST /snippets/{id}/edit update
	updateForm := url.Values{
		"title":    {"Updated Markdown Notes"},
		"content":  {"# New Header\n\n- item 1\n- item 2"},
		"language": {"markdown"},
		"tags":     {"markdown, notes, updated"},
	}
	postReq := httptest.NewRequest("POST", "/snippets/"+snippet.ID+"/edit", strings.NewReader(updateForm.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.SetPathValue("id", snippet.ID)
	postReq = postReq.WithContext(ctx)
	postRR := httptest.NewRecorder()

	snippetHandler.UpdateSnippet(postRR, postReq)
	if postRR.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303 after update, got %d", postRR.Code)
	}

	updated, err := st.GetSnippetByID(snippet.ID, user.ID)
	if err != nil {
		t.Fatalf("failed to retrieve updated snippet: %v", err)
	}
	if updated.Title != "Updated Markdown Notes" {
		t.Errorf("title mismatch: %s", updated.Title)
	}
	if updated.Language != "markdown" {
		t.Errorf("language mismatch: %s", updated.Language)
	}
	if !updated.IsMarkdown() {
		t.Errorf("expected IsMarkdown() to be true")
	}
	if len(updated.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(updated.Tags))
	}
}
