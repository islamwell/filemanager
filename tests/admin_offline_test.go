package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"filemanager/internal/auth"
	"filemanager/internal/handlers"
	"filemanager/internal/middleware"
	"filemanager/internal/models"
	"filemanager/internal/static"
)

func TestAdminApprovalFlow(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	authHandler := handlers.NewAuthHandler(base)
	adminHandler := handlers.NewAdminHandler(base)

	// Step 1: Register First User (Alice) -> Should automatically become Admin & Approved
	aliceForm := url.Values{
		"username":         {"alice_admin"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	}
	req1 := httptest.NewRequest("POST", "/register", strings.NewReader(aliceForm.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr1 := httptest.NewRecorder()
	authHandler.RegisterSubmit(rr1, req1)

	alice, err := st.GetUserByUsername("alice_admin")
	if err != nil {
		t.Fatalf("failed to find alice: %v", err)
	}
	if !alice.IsAdmin || !alice.IsApproved {
		t.Fatalf("expected alice to be both Admin and Approved, got admin=%v approved=%v", alice.IsAdmin, alice.IsApproved)
	}

	// Step 2: Register Second User (Bob) -> Should be standard User and NOT approved
	bobForm := url.Values{
		"username":         {"bob_user"},
		"password":         {"password456"},
		"confirm_password": {"password456"},
	}
	req2 := httptest.NewRequest("POST", "/register", strings.NewReader(bobForm.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr2 := httptest.NewRecorder()
	authHandler.RegisterSubmit(rr2, req2)

	bob, err := st.GetUserByUsername("bob_user")
	if err != nil {
		t.Fatalf("failed to find bob: %v", err)
	}
	if bob.IsAdmin || bob.IsApproved {
		t.Fatalf("expected bob to be non-admin and unapproved, got admin=%v approved=%v", bob.IsAdmin, bob.IsApproved)
	}

	// Step 3: Unapproved Bob attempts to log in -> Login MUST be rejected with pending approval notice
	bobLogin := url.Values{
		"username": {"bob_user"},
		"password": {"password456"},
	}
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader(bobLogin.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRR := httptest.NewRecorder()
	authHandler.LoginSubmit(loginRR, loginReq)

	if loginRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK (re-rendered login page), got %d", loginRR.Code)
	}
	if !strings.Contains(loginRR.Body.String(), "pending administrator approval") {
		t.Fatalf("expected pending administrator approval warning in body, got: %s", loginRR.Body.String())
	}
	// Verify no session cookie was issued
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			t.Fatalf("expected no session cookie for unapproved user")
		}
	}

	// Step 4: Non-admin access to /admin/users -> Must receive 403 Forbidden
	nonAdminReq := httptest.NewRequest("GET", "/admin/users", nil)
	nonAdminCtx := context.WithValue(nonAdminReq.Context(), middleware.UserContextKey, bob)
	nonAdminReq = nonAdminReq.WithContext(nonAdminCtx)
	nonAdminRR := httptest.NewRecorder()

	adminProtected := middleware.RequireAuth(middleware.RequireAdmin(http.HandlerFunc(adminHandler.AdminUsersView)))
	adminProtected.ServeHTTP(nonAdminRR, nonAdminReq)

	if nonAdminRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-admin user, got %d", nonAdminRR.Code)
	}

	// Step 5: Admin accesses /admin/users -> 200 OK and lists users
	adminReq := httptest.NewRequest("GET", "/admin/users", nil)
	adminCtx := context.WithValue(adminReq.Context(), middleware.UserContextKey, alice)
	adminReq = adminReq.WithContext(adminCtx)
	adminRR := httptest.NewRecorder()

	adminProtected.ServeHTTP(adminRR, adminReq)
	if adminRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for admin, got %d", adminRR.Code)
	}
	if !strings.Contains(adminRR.Body.String(), "bob_user") {
		t.Fatalf("expected admin users page to list bob_user")
	}

	// Step 6: Admin approves Bob
	approveReq := httptest.NewRequest("POST", "/admin/users/"+bob.ID+"/approve", nil)
	approveReq.SetPathValue("id", bob.ID)
	approveReq = approveReq.WithContext(context.WithValue(approveReq.Context(), middleware.UserContextKey, alice))
	approveRR := httptest.NewRecorder()
	adminHandler.ApproveUser(approveRR, approveReq)

	if approveRR.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after approval, got %d", approveRR.Code)
	}

	// Verify Bob is now approved in database
	bobUpdated, err := st.GetUserByID(bob.ID)
	if err != nil || !bobUpdated.IsApproved {
		t.Fatalf("expected bob to be approved, got error=%v approved=%v", err, bobUpdated.IsApproved)
	}

	// Step 7: Approved Bob logs in -> Success and session cookie set
	approvedLoginRR := httptest.NewRecorder()
	authHandler.LoginSubmit(approvedLoginRR, loginReq)
	if approvedLoginRR.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after successful login, got %d", approvedLoginRR.Code)
	}

	var sessionCookie *http.Cookie
	for _, c := range approvedLoginRR.Result().Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected session cookie for approved user")
	}

	// Step 8: Admin revokes Bob's approval -> Active session is rejected by SessionMiddleware
	rejectReq := httptest.NewRequest("POST", "/admin/users/"+bob.ID+"/reject", nil)
	rejectReq.SetPathValue("id", bob.ID)
	rejectReq = rejectReq.WithContext(context.WithValue(rejectReq.Context(), middleware.UserContextKey, alice))
	rejectRR := httptest.NewRecorder()
	adminHandler.RejectUser(rejectRR, rejectReq)

	if rejectRR.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after reject, got %d", rejectRR.Code)
	}

	// Verify SessionMiddleware no longer injects revoked user into request context
	testReq := httptest.NewRequest("GET", "/", nil)
	testReq.AddCookie(sessionCookie)
	var injectedUser *models.User
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		injectedUser = middleware.GetUserFromContext(r.Context())
	})
	sessionMw := middleware.SessionMiddleware(st)(testHandler)
	sessionMw.ServeHTTP(httptest.NewRecorder(), testReq)

	if injectedUser != nil {
		t.Fatalf("expected revoked user to NOT be injected into context by session middleware")
	}

	// Step 9: Admin promotes Bob to Admin, then deletes Bob
	roleReq := httptest.NewRequest("POST", "/admin/users/"+bob.ID+"/role", nil)
	roleReq.SetPathValue("id", bob.ID)
	roleReq = roleReq.WithContext(context.WithValue(roleReq.Context(), middleware.UserContextKey, alice))
	roleRR := httptest.NewRecorder()
	adminHandler.ToggleUserRole(roleRR, roleReq)

	bobPromoted, _ := st.GetUserByID(bob.ID)
	if !bobPromoted.IsAdmin {
		t.Fatalf("expected bob to be promoted to admin")
	}

	deleteReq := httptest.NewRequest("POST", "/admin/users/"+bob.ID+"/delete", nil)
	deleteReq.SetPathValue("id", bob.ID)
	deleteReq = deleteReq.WithContext(context.WithValue(deleteReq.Context(), middleware.UserContextKey, alice))
	deleteRR := httptest.NewRecorder()
	adminHandler.DeleteUser(deleteRR, deleteReq)

	_, err = st.GetUserByID(bob.ID)
	if err == nil {
		t.Fatalf("expected bob to be deleted from store")
	}
}

func TestPWAAndOfflineEndpoints(t *testing.T) {
	st, base := setupTestEnv(t)
	defer st.Close()

	// 1. Verify Manifest Content
	manifestData, err := static.FS.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("failed to read embedded manifest.json: %v", err)
	}
	if !strings.Contains(string(manifestData), "FileManager") {
		t.Fatalf("manifest.json missing expected content")
	}

	// 2. Verify Service Worker Content
	swData, err := static.FS.ReadFile("sw.js")
	if err != nil {
		t.Fatalf("failed to read embedded sw.js: %v", err)
	}
	if !strings.Contains(string(swData), "filemanager-cache") {
		t.Fatalf("sw.js missing expected cache logic")
	}

	// 3. Verify Offline View Rendering
	offlineReq := httptest.NewRequest("GET", "/offline", nil)
	offlineRR := httptest.NewRecorder()
	base.OfflineView(offlineRR, offlineReq)

	if offlineRR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /offline, got %d", offlineRR.Code)
	}
	if !strings.Contains(offlineRR.Body.String(), "Offline Mode Active") {
		t.Fatalf("offline template missing header")
	}
	if !strings.Contains(offlineRR.Body.String(), "Offline Cached Snippets") {
		t.Fatalf("offline template missing cached snippets section")
	}
}
