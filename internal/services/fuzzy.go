package services

import (
	"strings"
	"unicode"
)

// FuzzyMatch tests if the search query matches the target string and computes a relevance score.
// Returns (matched bool, score int). Higher scores indicate closer relevance.
func FuzzyMatch(query, target string) (bool, int) {
	q := strings.ToLower(strings.TrimSpace(query))
	t := strings.ToLower(strings.TrimSpace(target))

	if q == "" {
		return true, 100
	}
	if t == "" {
		return false, 0
	}

	// 1. Exact match bonus
	if t == q {
		return true, 1000
	}

	// 2. Exact prefix match bonus
	if strings.HasPrefix(t, q) {
		return true, 800 + (len(q) * 10)
	}

	// 3. Substring containment bonus
	if strings.Contains(t, q) {
		idx := strings.Index(t, q)
		// Early occurrence gets higher score
		return true, 500 - (idx * 5) + (len(q) * 8)
	}

	// 4. Word-boundary / CamelCase / separator matching
	words := strings.FieldsFunc(t, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, word := range words {
		if strings.HasPrefix(word, q) {
			return true, 450 + (len(q) * 8)
		}
	}

	// 5. Subsequence matching (e.g. "mp3pl" matches "my_audio_mp3_player.mp3")
	qRunes := []rune(q)
	tRunes := []rune(t)
	qIdx := 0
	tIdx := 0
	consecutiveBonus := 0
	subseqScore := 0

	for qIdx < len(qRunes) && tIdx < len(tRunes) {
		if qRunes[qIdx] == tRunes[tIdx] {
			subseqScore += 10 + consecutiveBonus
			consecutiveBonus += 5
			qIdx++
		} else {
			consecutiveBonus = 0
		}
		tIdx++
	}

	if qIdx == len(qRunes) {
		// All query characters appeared in order
		return true, 200 + subseqScore - len(tRunes)
	}

	// 6. Levenshtein edit distance for short strings / typo tolerance
	if len(qRunes) >= 3 && len(tRunes) >= 3 {
		dist := LevenshteinDistance(q, t)
		maxAllowedDist := 2
		if len(qRunes) > 6 {
			maxAllowedDist = 3
		}
		if dist <= maxAllowedDist {
			score := 150 - (dist * 30)
			if score > 0 {
				return true, score
			}
		}
	}

	return false, 0
}

// MatchTags returns the highest score from matching query against a list of tags.
func MatchTags(query string, tags []string) (bool, int) {
	bestScore := 0
	matched := false
	for _, tag := range tags {
		if m, score := FuzzyMatch(query, tag); m {
			matched = true
			if score+100 > bestScore { // Extra bonus for tag matches
				bestScore = score + 100
			}
		}
	}
	return matched, bestScore
}

// LevenshteinDistance calculates the minimum single-character edits between two strings.
func LevenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	len1, len2 := len(r1), len(r2)

	column := make([]int, len1+1)
	for y := 1; y <= len1; y++ {
		column[y] = y
	}

	for x := 1; x <= len2; x++ {
		column[0] = x
		lastkey := x - 1
		for y := 1; y <= len1; y++ {
			oldkey := column[y]
			cost := 0
			if r1[y-1] != r2[x-1] {
				cost = 1
			}
			column[y] = min3(column[y]+1, column[y-1]+1, lastkey+cost)
			lastkey = oldkey
		}
	}
	return column[len1]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
