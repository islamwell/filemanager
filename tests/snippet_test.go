package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"filemanager/internal/handlers"
	"filemanager/internal/middleware"
	"filemanager/internal/models"
)

func TestSnippetCRUDAndLengthValidation(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	snippetHandler := handlers.NewSnippetHandler(base)

	user := &models.User{Username: "eve", PasswordHash: "hash"}
	_ = st.CreateUser(user)

	// 1. Test Valid Snippet Creation
	form := url.Values{
		"title":    {"PostgreSQL Config"},
		"content":  {"max_connections = 100\nshared_buffers = 128MB"},
		"language": {"sql"},
		"tags":     {"database, config, sql"},
	}

	req := httptest.NewRequest("POST", "/snippets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, user)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	snippetHandler.CreateSnippet(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect StatusSeeOther (303), got %d", rr.Code)
	}

	// Verify in store
	snippets, err := st.ListSnippets(user.ID, nil, "")
	if err != nil || len(snippets) != 1 {
		t.Fatalf("expected 1 snippet in store, got %d", len(snippets))
	}

	sn := snippets[0]
	if sn.Title != "PostgreSQL Config" || sn.Language != "sql" {
		t.Errorf("snippet data mismatch: %+v", sn)
	}
	if len(sn.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(sn.Tags))
	}

	// 2. Test Exceeding 10,000 character limit
	giantContent := strings.Repeat("A", 10001)
	giantForm := url.Values{
		"title":   {"Giant Snippet"},
		"content": {giantContent},
	}
	giantReq := httptest.NewRequest("POST", "/snippets", strings.NewReader(giantForm.Encode()))
	giantReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	giantReq = giantReq.WithContext(ctx)
	giantRR := httptest.NewRecorder()

	snippetHandler.CreateSnippet(giantRR, giantReq)

	if giantRR.Code != http.StatusOK { // Re-renders new snippet page with error banner
		t.Errorf("expected 200 OK re-render with error on >10k chars, got %d", giantRR.Code)
	}

	// Verify snippet count did NOT increase
	snippetsAfter, _ := st.ListSnippets(user.ID, nil, "")
	if len(snippetsAfter) != 1 {
		t.Errorf("expected snippet count to remain 1, got %d", len(snippetsAfter))
	}

	// 3. Test Snippet View
	viewReq := httptest.NewRequest("GET", "/snippets/"+sn.ID, nil)
	viewReq.SetPathValue("id", sn.ID)
	viewReq = viewReq.WithContext(ctx)
	viewRR := httptest.NewRecorder()

	snippetHandler.ViewSnippet(viewRR, viewReq)
	if viewRR.Code != http.StatusOK {
		t.Errorf("expected 200 OK viewing snippet, got %d", viewRR.Code)
	}
	if !strings.Contains(viewRR.Body.String(), "PostgreSQL Config") {
		t.Errorf("view snippet response missing title")
	}

	// 4. Test Snippet Deletion
	delReq := httptest.NewRequest("POST", "/snippets/"+sn.ID+"/delete", nil)
	delReq.SetPathValue("id", sn.ID)
	delReq = delReq.WithContext(ctx)
	delRR := httptest.NewRecorder()

	snippetHandler.DeleteSnippet(delRR, delReq)
	if delRR.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303 on delete, got %d", delRR.Code)
	}

	remaining, _ := st.ListSnippets(user.ID, nil, "")
	if len(remaining) != 0 {
		t.Errorf("expected 0 snippets remaining, got %d", len(remaining))
	}
}
