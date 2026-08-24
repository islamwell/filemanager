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
	"time"

	"filemanager/internal/handlers"
	"filemanager/internal/middleware"
	"filemanager/internal/models"
)

func TestShareTokenGenerationAndPublicAccess(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	shareHandler := handlers.NewShareHandler(base)

	user := &models.User{Username: "frank", PasswordHash: "hash"}
	_ = st.CreateUser(user)

	// Create test file on disk and in store
	tempFile := filepath.Join(base.Config.StorageDir, "share_test.txt")
	_ = os.WriteFile(tempFile, []byte("Hello shared world!"), 0644)

	fileItem := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "shared_notes.txt",
		StoredPath:       tempFile,
		FileSize:         19,
		MimeType:         "text/plain",
		Sha256Checksum:   "fakechecksum",
	}
	if err := st.CreateFile(fileItem); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// 1. Generate Share Link (with 24h expiration)
	form := url.Values{
		"item_id":   {fileItem.ID},
		"item_type": {"file"},
		"expiry":    {"24h"},
	}

	req := httptest.NewRequest("POST", "/items/share", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	shareHandler.CreateShareLink(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303 after share creation, got %d", rr.Code)
	}

	// Retrieve share link
	shareLink, err := st.GetShareLinkByItem(fileItem.ID, models.ItemTypeFile, user.ID)
	if err != nil || shareLink == nil {
		t.Fatalf("failed to find share link in store: %v", err)
	}

	if len(shareLink.Token) < 32 {
		t.Errorf("expected high-entropy token, got length %d", len(shareLink.Token))
	}
	if shareLink.ExpiresAt == nil || shareLink.IsExpired() {
		t.Errorf("expected active 24h expiry")
	}

	// 2. Test Public Access WITHOUT Authentication (Unauthenticated GET /share/{token})
	pubReq := httptest.NewRequest("GET", "/share/"+shareLink.Token, nil)
	pubReq.SetPathValue("token", shareLink.Token)
	pubRR := httptest.NewRecorder()

	shareHandler.ViewSharedItem(pubRR, pubReq)

	if pubRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for public share view, got %d", pubRR.Code)
	}
	if !strings.Contains(pubRR.Body.String(), "shared_notes.txt") {
		t.Errorf("public share view missing filename")
	}

	// 3. Test Public Direct Download (Unauthenticated GET /share/{token}/download)
	dlReq := httptest.NewRequest("GET", "/share/"+shareLink.Token+"/download", nil)
	dlReq.SetPathValue("token", shareLink.Token)
	dlRR := httptest.NewRecorder()

	shareHandler.DownloadSharedFile(dlRR, dlReq)

	if dlRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for public download, got %d", dlRR.Code)
	}
	if dlRR.Body.String() != "Hello shared world!" {
		t.Errorf("downloaded content mismatch")
	}

	// 4. Test Expired Share Link Rejection
	pastTime := time.Now().Add(-2 * time.Hour)
	expiredLink := &models.ShareLink{
		Token:     "expired_token_xyz",
		UserID:    user.ID,
		ItemID:    fileItem.ID,
		ItemType:  models.ItemTypeFile,
		ExpiresAt: &pastTime,
	}
	_ = st.CreateShareLink(expiredLink)

	expReq := httptest.NewRequest("GET", "/share/expired_token_xyz", nil)
	expReq.SetPathValue("token", "expired_token_xyz")
	expRR := httptest.NewRecorder()

	shareHandler.ViewSharedItem(expRR, expReq)

	// Should show expired notice banner (HTTP 200 with expired template)
	if !strings.Contains(expRR.Body.String(), "Share Link Expired") && !strings.Contains(expRR.Body.String(), "expired") {
		t.Errorf("expected expired link message in response")
	}

	// 5. Test Revocation / Deletion of Share Link
	revReq := httptest.NewRequest("POST", "/shares/"+shareLink.Token+"/delete", nil)
	revReq.SetPathValue("token", shareLink.Token)
	revReq = revReq.WithContext(ctx)
	revRR := httptest.NewRecorder()

	shareHandler.DeleteShareLink(revRR, revReq)
	if revRR.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect on share revocation, got %d", revRR.Code)
	}

	// Verify revoked token returns 404
	afterRevReq := httptest.NewRequest("GET", "/share/"+shareLink.Token, nil)
	afterRevReq.SetPathValue("token", shareLink.Token)
	afterRevRR := httptest.NewRecorder()
	shareHandler.ViewSharedItem(afterRevRR, afterRevReq)

	if afterRevRR.Code != http.StatusNotFound {
		t.Errorf("expected 404 for revoked share token, got %d", afterRevRR.Code)
	}
}

func TestCustomShareSlugAndVanityAliases(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	shareHandler := handlers.NewShareHandler(base)

	user := &models.User{Username: "grace", PasswordHash: "hash"}
	_ = st.CreateUser(user)

	tempFile := filepath.Join(base.Config.StorageDir, "vanity_test.txt")
	_ = os.WriteFile(tempFile, []byte("Vanity URL content"), 0644)

	fileItem := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "presentation.pdf",
		StoredPath:       tempFile,
		FileSize:         18,
		MimeType:         "application/pdf",
		Sha256Checksum:   "checksum123",
	}
	_ = st.CreateFile(fileItem)

	// 1. Create with Custom Vanity Alias: "my-q3-deck"
	form := url.Values{
		"item_id":      {fileItem.ID},
		"item_type":    {"file"},
		"custom_alias": {"my-q3-deck"},
		"expiry":       {"7d"},
	}

	req := httptest.NewRequest("POST", "/items/share", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	shareHandler.CreateShareLink(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rr.Code)
	}

	// 2. Access via Vanity URL
	pubReq := httptest.NewRequest("GET", "/share/my-q3-deck", nil)
	pubReq.SetPathValue("token", "my-q3-deck")
	pubRR := httptest.NewRecorder()

	shareHandler.ViewSharedItem(pubRR, pubReq)
	if pubRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for custom vanity share URL, got %d", pubRR.Code)
	}

	// 3. Test Invalid Characters in Alias (should reject)
	badForm := url.Values{
		"item_id":      {fileItem.ID},
		"item_type":    {"file"},
		"custom_alias": {"invalid/path?traversal"},
	}
	badReq := httptest.NewRequest("POST", "/items/share", strings.NewReader(badForm.Encode()))
	badReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badReq = badReq.WithContext(ctx)
	badRR := httptest.NewRecorder()

	shareHandler.CreateShareLink(badRR, badReq)
	if !strings.Contains(badRR.Header().Get("Location"), "err=") {
		t.Errorf("expected error redirect for invalid custom alias characters")
	}

	// 4. Test Reserved Alias (e.g. "download", "audio")
	resForm := url.Values{
		"item_id":      {fileItem.ID},
		"item_type":    {"file"},
		"custom_alias": {"download"},
	}
	resReq := httptest.NewRequest("POST", "/items/share", strings.NewReader(resForm.Encode()))
	resReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resReq = resReq.WithContext(ctx)
	resRR := httptest.NewRecorder()

	shareHandler.CreateShareLink(resRR, resReq)
	if !strings.Contains(resRR.Header().Get("Location"), "err=") {
		t.Errorf("expected error redirect for reserved custom alias")
	}
}
