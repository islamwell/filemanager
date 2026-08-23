package tests

import (
	"errors"
	"testing"

	"filemanager/internal/models"
	"filemanager/internal/store"
)

func TestCategoryHierarchyAndMaxDepth(t *testing.T) {
	st, _ := setupTestEnv(t)
	defer st.Close()

	user := &models.User{Username: "charlie", PasswordHash: "hash"}
	if err := st.CreateUser(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Level 1: Root Folder
	cat1 := &models.Category{UserID: user.ID, Name: "Documents"}
	if err := st.CreateCategory(cat1); err != nil {
		t.Fatalf("failed to create level 1 category: %v", err)
	}
	if cat1.Depth != 1 {
		t.Errorf("expected depth 1, got %d", cat1.Depth)
	}

	// Level 2: Sub-collection
	cat2 := &models.Category{UserID: user.ID, ParentID: &cat1.ID, Name: "Work"}
	if err := st.CreateCategory(cat2); err != nil {
		t.Fatalf("failed to create level 2 category: %v", err)
	}
	if cat2.Depth != 2 {
		t.Errorf("expected depth 2, got %d", cat2.Depth)
	}

	// Level 3: Nested folder
	cat3 := &models.Category{UserID: user.ID, ParentID: &cat2.ID, Name: "2026"}
	if err := st.CreateCategory(cat3); err != nil {
		t.Fatalf("failed to create level 3 category: %v", err)
	}
	if cat3.Depth != 3 {
		t.Errorf("expected depth 3, got %d", cat3.Depth)
	}

	// Level 4: Deepest allowable folder
	cat4 := &models.Category{UserID: user.ID, ParentID: &cat3.ID, Name: "Audio Recordings"}
	if err := st.CreateCategory(cat4); err != nil {
		t.Fatalf("failed to create level 4 category: %v", err)
	}
	if cat4.Depth != 4 {
		t.Errorf("expected depth 4, got %d", cat4.Depth)
	}

	// Level 5: Must be REJECTED (exceeds max 4 levels)
	cat5 := &models.Category{UserID: user.ID, ParentID: &cat4.ID, Name: "Sub-tracks"}
	err := st.CreateCategory(cat5)
	if !errors.Is(err, store.ErrMaxDepthExceeded) {
		t.Fatalf("expected ErrMaxDepthExceeded when inserting 5th level, got: %v", err)
	}

	// Verify Breadcrumb Path computation
	path, depth, err := st.GetCategoryPath(cat4.ID, user.ID)
	if err != nil {
		t.Fatalf("failed to get category path: %v", err)
	}
	expectedPath := "Documents / Work / 2026 / Audio Recordings"
	if path != expectedPath {
		t.Errorf("expected path '%s', got '%s'", expectedPath, path)
	}
	if depth != 4 {
		t.Errorf("expected depth 4, got %d", depth)
	}
}
