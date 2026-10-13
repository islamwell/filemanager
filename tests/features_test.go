package tests

import (
	"archive/zip"
	"bytes"
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

func TestFileAndSnippetRename(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	fileHandler := handlers.NewFileHandler(base)
	snippetHandler := handlers.NewSnippetHandler(base)

	user := &models.User{Username: "rename_user", PasswordHash: "hash"}
	if err := st.CreateUser(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. Test File Rename
	diskPath := filepath.Join(base.Config.StorageDir, "sample.txt")
	_ = os.WriteFile(diskPath, []byte("contents"), 0644)

	file := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "old_name.txt",
		StoredPath:       diskPath,
		FileSize:         8,
		MimeType:         "text/plain",
		Sha256Checksum:   "dummy",
	}
	if err := st.CreateFile(file); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	form := url.Values{"filename": {"new_audio.mp3"}}
	req := httptest.NewRequest("POST", "/files/"+file.ID+"/rename", strings.NewReader(form.Encode()))
	req.SetPathValue("id", file.ID)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, user))
	rr := httptest.NewRecorder()

	fileHandler.RenameFile(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect on rename, got %d", rr.Code)
	}

	updatedFile, err := st.GetFileByID(file.ID, user.ID)
	if err != nil {
		t.Fatalf("failed to get renamed file: %v", err)
	}
	if updatedFile.OriginalFilename != "new_audio.mp3" {
		t.Errorf("expected filename 'new_audio.mp3', got %s", updatedFile.OriginalFilename)
	}
	if !updatedFile.IsMP3 {
		t.Errorf("expected IsMP3 to be true after renaming to .mp3")
	}

	// 2. Test Snippet Rename
	snippet := &models.Snippet{
		UserID:   user.ID,
		Title:    "Old Snippet Title",
		Content:  "fmt.Println(123)",
		Language: "go",
	}
	if err := st.CreateSnippet(snippet); err != nil {
		t.Fatalf("failed to create snippet: %v", err)
	}

	snForm := url.Values{"title": {"Renamed Snippet Title"}}
	snReq := httptest.NewRequest("POST", "/snippets/"+snippet.ID+"/rename", strings.NewReader(snForm.Encode()))
	snReq.SetPathValue("id", snippet.ID)
	snReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	snReq = snReq.WithContext(context.WithValue(snReq.Context(), middleware.UserContextKey, user))
	snRR := httptest.NewRecorder()

	snippetHandler.RenameSnippet(snRR, snReq)
	if snRR.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect on snippet rename, got %d", snRR.Code)
	}

	updatedSnippet, err := st.GetSnippetByID(snippet.ID, user.ID)
	if err != nil {
		t.Fatalf("failed to get renamed snippet: %v", err)
	}
	if updatedSnippet.Title != "Renamed Snippet Title" {
		t.Errorf("expected snippet title 'Renamed Snippet Title', got %s", updatedSnippet.Title)
	}
}

func TestBatchOperations(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	fileHandler := handlers.NewFileHandler(base)

	user := &models.User{Username: "batch_user", PasswordHash: "hash"}
	_ = st.CreateUser(user)

	// Create test category
	cat := &models.Category{UserID: user.ID, Name: "BatchTarget", Depth: 1}
	_ = st.CreateCategory(cat)

	// Create 3 files
	var fileIDs []string
	for i := 1; i <= 3; i++ {
		path := filepath.Join(base.Config.StorageDir, "batch_file_"+string(rune('0'+i))+".txt")
		_ = os.WriteFile(path, []byte("Content of file "+string(rune('0'+i))), 0644)
		f := &models.FileItem{
			UserID:           user.ID,
			OriginalFilename: "file_" + string(rune('0'+i)) + ".txt",
			StoredPath:       path,
			FileSize:         17,
			MimeType:         "text/plain",
			Sha256Checksum:   "checksum",
		}
		_ = st.CreateFile(f)
		fileIDs = append(fileIDs, f.ID)
	}

	// 1. Batch ZIP Download
	zipForm := url.Values{
		"file_ids": fileIDs,
	}
	req := httptest.NewRequest("POST", "/files/batch/zip", strings.NewReader(zipForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, user))
	rr := httptest.NewRecorder()

	fileHandler.BatchZipDownload(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 on batch zip, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/zip" {
		t.Errorf("expected application/zip content type, got %s", rr.Header().Get("Content-Type"))
	}

	// Verify zip contents
	zipReader, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("failed to read generated zip: %v", err)
	}
	if len(zipReader.File) != 3 {
		t.Errorf("expected 3 files inside zip, got %d", len(zipReader.File))
	}

	// 2. Batch Move Files
	moveForm := url.Values{
		"file_ids":    fileIDs[:2],
		"category_id": {cat.ID},
	}
	moveReq := httptest.NewRequest("POST", "/files/batch/move", strings.NewReader(moveForm.Encode()))
	moveReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	moveReq = moveReq.WithContext(context.WithValue(moveReq.Context(), middleware.UserContextKey, user))
	moveRR := httptest.NewRecorder()

	fileHandler.BatchMoveFiles(moveRR, moveReq)
	if moveRR.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on batch move, got %d", moveRR.Code)
	}

	f1, _ := st.GetFileByID(fileIDs[0], user.ID)
	if f1.CategoryID == nil || *f1.CategoryID != cat.ID {
		t.Errorf("expected file 1 to be moved to category %s", cat.ID)
	}

	// 3. Batch Delete Files
	delForm := url.Values{
		"file_ids": fileIDs[2:],
	}
	delReq := httptest.NewRequest("POST", "/files/batch/delete", strings.NewReader(delForm.Encode()))
	delReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), middleware.UserContextKey, user))
	delRR := httptest.NewRecorder()

	fileHandler.BatchDeleteFiles(delRR, delReq)
	if delRR.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on batch delete, got %d", delRR.Code)
	}

	_, err = st.GetFileByID(fileIDs[2], user.ID)
	if err == nil {
		t.Errorf("expected file 3 to be deleted")
	}

	// 4. Batch Delete Snippets
	var snIDs []string
	for i := 1; i <= 2; i++ {
		sn := &models.Snippet{
			UserID:  user.ID,
			Title:   "Snippet " + string(rune('0'+i)),
			Content: "Test code",
		}
		_ = st.CreateSnippet(sn)
		snIDs = append(snIDs, sn.ID)
	}
	if err := st.DeleteSnippets(snIDs, user.ID); err != nil {
		t.Fatalf("failed to delete snippets: %v", err)
	}
	snLeft, _ := st.ListSnippets(user.ID, nil, "")
	if len(snLeft) != 0 {
		t.Errorf("expected 0 snippets left, got %d", len(snLeft))
	}
}

func TestPasswordProtectedShareAndMaxViews(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	shareHandler := handlers.NewShareHandler(base)

	user := &models.User{Username: "share_sec_user", PasswordHash: "hash"}
	_ = st.CreateUser(user)

	path := filepath.Join(base.Config.StorageDir, "sec_file.txt")
	_ = os.WriteFile(path, []byte("Top Secret Data"), 0644)
	file := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "secret.txt",
		StoredPath:       path,
		FileSize:         15,
		MimeType:         "text/plain",
		Sha256Checksum:   "dummy",
	}
	_ = st.CreateFile(file)

	// Create share link with password "pass123" and max_views 2
	shareForm := url.Values{
		"item_id":   {file.ID},
		"item_type": {"file"},
		"password":  {"pass123"},
		"max_views": {"2"},
		"expiry":    {"never"},
	}
	req := httptest.NewRequest("POST", "/items/share", strings.NewReader(shareForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, user))
	rr := httptest.NewRecorder()

	shareHandler.CreateShareLink(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect on create share link, got %d", rr.Code)
	}

	link, err := st.GetShareLinkByItem(file.ID, models.ItemTypeFile, user.ID)
	if err != nil {
		t.Fatalf("failed to get share link: %v", err)
	}
	if !link.HasPassword() {
		t.Errorf("expected link to have password")
	}
	if link.MaxViews != 2 {
		t.Errorf("expected max views 2, got %d", link.MaxViews)
	}

	// Verify unauthenticated view without password shows password prompt
	viewReq := httptest.NewRequest("GET", "/share/"+link.Token, nil)
	viewReq.SetPathValue("token", link.Token)
	viewRR := httptest.NewRecorder()
	shareHandler.ViewSharedItem(viewRR, viewReq)
	if !strings.Contains(viewRR.Body.String(), "Password Protected Link") {
		t.Errorf("expected password protected prompt, got body: %s", viewRR.Body.String())
	}

	// Try unlock with wrong password
	unlockWrong := url.Values{"password": {"wrongpass"}}
	unReq := httptest.NewRequest("POST", "/share/"+link.Token+"/unlock", strings.NewReader(unlockWrong.Encode()))
	unReq.SetPathValue("token", link.Token)
	unReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unRR := httptest.NewRecorder()
	shareHandler.UnlockShareLink(unRR, unReq)
	if !strings.Contains(unRR.Body.String(), "Incorrect password") {
		t.Errorf("expected incorrect password error, got body: %s", unRR.Body.String())
	}

	// Unlock with correct password
	unlockCorrect := url.Values{"password": {"pass123"}}
	unReq2 := httptest.NewRequest("POST", "/share/"+link.Token+"/unlock", strings.NewReader(unlockCorrect.Encode()))
	unReq2.SetPathValue("token", link.Token)
	unReq2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unRR2 := httptest.NewRecorder()
	shareHandler.UnlockShareLink(unRR2, unReq2)
	if unRR2.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on successful unlock, got %d", unRR2.Code)
	}

	// Check cookie was set
	cookies := unRR2.Result().Cookies()
	var unlockCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "unlock_"+link.Token {
			unlockCookie = c
			break
		}
	}
	if unlockCookie == nil {
		t.Fatalf("expected unlock cookie to be set")
	}

	// View with cookie: view 1
	authViewReq := httptest.NewRequest("GET", "/share/"+link.Token, nil)
	authViewReq.SetPathValue("token", link.Token)
	authViewReq.AddCookie(unlockCookie)
	authViewRR := httptest.NewRecorder()
	shareHandler.ViewSharedItem(authViewRR, authViewReq)
	if !strings.Contains(authViewRR.Body.String(), "secret.txt") {
		t.Errorf("expected content to be visible after unlock, got: %s", authViewRR.Body.String())
	}

	// View 2 (hits max_views limit of 2)
	authViewReq2 := httptest.NewRequest("GET", "/share/"+link.Token, nil)
	authViewReq2.SetPathValue("token", link.Token)
	authViewReq2.AddCookie(unlockCookie)
	authViewRR2 := httptest.NewRecorder()
	shareHandler.ViewSharedItem(authViewRR2, authViewReq2)

	// View 3 should now show exhausted
	authViewReq3 := httptest.NewRequest("GET", "/share/"+link.Token, nil)
	authViewReq3.SetPathValue("token", link.Token)
	authViewReq3.AddCookie(unlockCookie)
	authViewRR3 := httptest.NewRecorder()
	shareHandler.ViewSharedItem(authViewRR3, authViewReq3)
	if !strings.Contains(authViewRR3.Body.String(), "View Limit Reached") {
		t.Errorf("expected view limit reached on 3rd access, got: %s", authViewRR3.Body.String())
	}
}

func TestStorageStats(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	user := &models.User{Username: "stats_user", PasswordHash: "hash"}
	_ = st.CreateUser(user)

	// Add an image (50KB)
	img := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "photo.jpg",
		StoredPath:       filepath.Join(base.Config.StorageDir, "photo.jpg"),
		FileSize:         50 * 1024,
		MimeType:         "image/jpeg",
		Sha256Checksum:   "ck1",
	}
	_ = st.CreateFile(img)

	// Add an audio file (100KB)
	aud := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "track.mp3",
		StoredPath:       filepath.Join(base.Config.StorageDir, "track.mp3"),
		FileSize:         100 * 1024,
		MimeType:         "audio/mpeg",
		Sha256Checksum:   "ck2",
	}
	_ = st.CreateFile(aud)

	// Add snippet
	sn := &models.Snippet{
		UserID:  user.ID,
		Title:   "Snippet",
		Content: "test snippet code",
	}
	_ = st.CreateSnippet(sn)

	stats, err := st.GetUserStorageStats(user.ID)
	if err != nil {
		t.Fatalf("failed to get storage stats: %v", err)
	}

	if stats.FileCount != 2 {
		t.Errorf("expected 2 files, got %d", stats.FileCount)
	}
	if stats.SnippetCount != 1 {
		t.Errorf("expected 1 snippet, got %d", stats.SnippetCount)
	}
	if stats.ImageBytes != 50*1024 {
		t.Errorf("expected 50KB image bytes, got %d", stats.ImageBytes)
	}
	if stats.AudioBytes != 100*1024 {
		t.Errorf("expected 100KB audio bytes, got %d", stats.AudioBytes)
	}
	if stats.TotalBytes != 150*1024 {
		t.Errorf("expected 150KB total bytes, got %d", stats.TotalBytes)
	}
	if stats.FormattedTotal() != "150.0 KB" {
		t.Errorf("expected '150.0 KB', got %s", stats.FormattedTotal())
	}
	pct := stats.PercentUsage(100 * 1024 * 1024)
	if pct <= 0 || pct > 1 {
		t.Errorf("expected pct around 0.15%%, got %.4f", pct)
	}
}
