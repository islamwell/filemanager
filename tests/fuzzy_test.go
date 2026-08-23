package tests

import (
	"testing"

	"filemanager/internal/models"
	"filemanager/internal/services"
)

func TestFuzzyAlgorithm(t *testing.T) {
	// Exact match
	m, scoreExact := services.FuzzyMatch("report", "report")
	if !m || scoreExact < 800 {
		t.Errorf("expected high score for exact match, got %v, %d", m, scoreExact)
	}

	// Prefix match
	m, scorePrefix := services.FuzzyMatch("repo", "report_2026.pdf")
	if !m || scorePrefix < 500 {
		t.Errorf("expected high score for prefix match, got %v, %d", m, scorePrefix)
	}

	// Subsequence match (e.g. "djapp" matches "django_application.py")
	m, scoreSubseq := services.FuzzyMatch("djapp", "django_application.py")
	if !m || scoreSubseq <= 0 {
		t.Errorf("expected subsequence match, got %v, %d", m, scoreSubseq)
	}

	// Typo tolerance (e.g. "configration" for "configuration")
	m, scoreTypo := services.FuzzyMatch("configration", "configuration")
	if !m || scoreTypo <= 0 {
		t.Errorf("expected typo match for Levenshtein distance, got %v, %d", m, scoreTypo)
	}

	// Non-matching
	m, _ = services.FuzzyMatch("zzzzzz", "financial_report.xlsx")
	if m {
		t.Errorf("expected completely unrelated strings not to match")
	}
}

func TestStoreFuzzySearchRanking(t *testing.T) {
	st, _ := setupTestEnv(t)
	defer st.Close()

	user := &models.User{Username: "grace", PasswordHash: "hash"}
	_ = st.CreateUser(user)

	// Create items with various titles and tags
	f1 := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "audio_podcast_episode_1.mp3",
		StoredPath:       "/tmp/test1",
		FileSize:         1000,
		MimeType:         "audio/mpeg",
		Sha256Checksum:   "abc1",
		Tags:             []string{"podcast", "music"},
	}
	_ = st.CreateFile(f1)

	f2 := &models.FileItem{
		UserID:           user.ID,
		OriginalFilename: "budget_2026.xlsx",
		StoredPath:       "/tmp/test2",
		FileSize:         2000,
		MimeType:         "application/vnd.ms-excel",
		Sha256Checksum:   "abc2",
		Tags:             []string{"finance"},
	}
	_ = st.CreateFile(f2)

	sn1 := &models.Snippet{
		UserID:   user.ID,
		Title:    "Podcast Notes",
		Content:  "Topics discussed in the audio episode",
		Language: "markdown",
		Tags:     []string{"podcast"},
	}
	_ = st.CreateSnippet(sn1)

	// Search for "podcast" -> Should return f1 and sn1, but not f2
	results, err := st.SearchItems(user.ID, "podcast", "")
	if err != nil {
		t.Fatalf("search error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'podcast', got %d", len(results))
	}

	// Search for typo "pdcast" -> Should still fuzzy match!
	fuzzyResults, err := st.SearchItems(user.ID, "pdcast", "")
	if err != nil {
		t.Fatalf("fuzzy search error: %v", err)
	}
	if len(fuzzyResults) < 1 {
		t.Errorf("expected typo 'pdcast' to match items, got %d results", len(fuzzyResults))
	}
}
