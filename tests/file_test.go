package tests

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"filemanager/internal/handlers"
	"filemanager/internal/middleware"
	"filemanager/internal/models"
)

func TestFileUploadDownloadAndRangeStreaming(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	fileHandler := handlers.NewFileHandler(base)

	user := &models.User{Username: "dave", PasswordHash: "fakehash"}
	if err := st.CreateUser(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. Prepare multipart upload with sample MP3 content
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "test_song.mp3")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}

	fakeAudioContent := strings.Repeat("ID3fakeaudiostreamdata1234567890", 100)
	if _, err := part.Write([]byte(fakeAudioContent)); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	_ = writer.WriteField("tags", "music, pop, test")
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/files/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	t.Log("Step 1: Uploading file")
	fileHandler.UploadFile(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect StatusSeeOther (303), got %d", rr.Code)
	}

	t.Log("Step 2: Listing files in store")
	files, err := st.ListFiles(user.ID, nil, "")
	if err != nil || len(files) != 1 {
		t.Fatalf("expected 1 file in store, got %d (err: %v)", len(files), err)
	}

	fileItem := files[0]
	if fileItem.OriginalFilename != "test_song.mp3" {
		t.Errorf("expected filename 'test_song.mp3', got '%s'", fileItem.OriginalFilename)
	}
	if !fileItem.IsMP3 {
		t.Errorf("expected IsMP3 to be true")
	}
	if fileItem.FileSize != int64(len(fakeAudioContent)) {
		t.Errorf("expected size %d, got %d", len(fakeAudioContent), fileItem.FileSize)
	}
	if len(fileItem.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(fileItem.Tags))
	}

	t.Log("Step 3: Downloading file")
	dlReq := httptest.NewRequest("GET", "/files/"+fileItem.ID+"/download", nil)
	dlReq.SetPathValue("id", fileItem.ID)
	dlReq = dlReq.WithContext(ctx)
	dlRR := httptest.NewRecorder()

	fileHandler.DownloadFile(dlRR, dlReq)

	if dlRR.Code != http.StatusOK {
		t.Fatalf("expected download 200 OK, got %d", dlRR.Code)
	}
	if dlRR.Body.String() != fakeAudioContent {
		t.Errorf("download body content mismatch")
	}

	t.Log("Step 4: Range streaming audio")
	audioReq := httptest.NewRequest("GET", "/files/"+fileItem.ID+"/audio", nil)
	audioReq.SetPathValue("id", fileItem.ID)
	audioReq.Header.Set("Range", "bytes=0-19")
	audioReq = audioReq.WithContext(ctx)
	audioRR := httptest.NewRecorder()

	fileHandler.AudioStream(audioRR, audioReq)

	if audioRR.Code != http.StatusPartialContent {
		t.Fatalf("expected HTTP 206 Partial Content, got %d", audioRR.Code)
	}
	if audioRR.Header().Get("Accept-Ranges") != "bytes" {
		t.Errorf("expected Accept-Ranges: bytes header")
	}
	if audioRR.Body.Len() != 20 {
		t.Errorf("expected 20 bytes returned for range 0-19, got %d", audioRR.Body.Len())
	}

	t.Log("Step 5: Deleting file")
	delReq := httptest.NewRequest("POST", "/files/"+fileItem.ID+"/delete", nil)
	delReq.SetPathValue("id", fileItem.ID)
	delReq = delReq.WithContext(ctx)
	delRR := httptest.NewRecorder()

	fileHandler.DeleteFile(delRR, delReq)

	if delRR.Code != http.StatusSeeOther {
		t.Fatalf("expected delete redirect 303, got %d", delRR.Code)
	}

	t.Log("Step 6: Verifying disk file removed")
	if _, err := os.Stat(fileItem.StoredPath); !os.IsNotExist(err) {
		t.Errorf("expected disk file to be removed upon deletion")
	}
}

func TestFilenameSanitization(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"normal.txt", "normal.txt"},
		{"../../../etc/passwd", "passwd"},
		{"..\\..\\windows\\system32\\cmd.exe", "cmd.exe"},
		{"song & dance (2026) [remix].mp3", "song _ dance (2026) [remix].mp3"},
		{"", "file_"},
	}

	for _, c := range cases {
		sanitized := handlers.SanitizeFilename(c.input)
		if !strings.HasPrefix(sanitized, c.expected) && sanitized != c.expected {
			t.Errorf("SanitizeFilename(%q) = %q; expected prefix/match %q", c.input, sanitized, c.expected)
		}
		if strings.Contains(sanitized, "/") || strings.Contains(sanitized, "\\") || strings.Contains(sanitized, "..") {
			t.Errorf("SanitizeFilename(%q) contains directory traversal: %q", c.input, sanitized)
		}
	}
}
