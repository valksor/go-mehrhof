// Package search provides hybrid text and fuzzy search with Reciprocal Rank
// Fusion (RRF) scoring. It operates entirely in-memory with no external
// dependencies, combining exact substring matching and Levenshtein-based
// fuzzy matching to rank results.
package search

import (
	"strings"
	"unicode/utf8"
)

// levenshtein computes the edit distance between two strings using the
// classic dynamic-programming algorithm. Both inputs should already be
// lowercased for case-insensitive comparison.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return utf8.RuneCountInString(b)
	}
	if len(b) == 0 {
		return utf8.RuneCountInString(a)
	}

	aRunes := []rune(a)
	bRunes := []rune(b)
	aLen := len(aRunes)
	bLen := len(bRunes)

	// Single-row DP: prev holds the previous row of distances.
	prev := make([]int, bLen+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= aLen; i++ {
		curr := make([]int, bLen+1)
		curr[0] = i

		for j := 1; j <= bLen; j++ {
			cost := 1
			if aRunes[i-1] == bRunes[j-1] {
				cost = 0
			}

			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost

			curr[j] = min(del, min(ins, sub))
		}

		prev = curr
	}

	return prev[bLen]
}

// similarity returns a 0-1 score based on edit distance.
// 1.0 means exact match, 0.0 means completely different.
func similarity(a, b string) float64 {
	al := strings.ToLower(a)
	bl := strings.ToLower(b)

	if al == bl {
		return 1.0
	}

	dist := levenshtein(al, bl)
	maxLen := max(utf8.RuneCountInString(al), utf8.RuneCountInString(bl))
	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - float64(dist)/float64(maxLen)
}

// fuzzyMatch checks if query fuzzy-matches text with a minimum threshold.
// Returns the similarity score and whether it meets the threshold.
func fuzzyMatch(text, query string, threshold float64) (float64, bool) {
	score := similarity(text, query)

	return score, score >= threshold
}

// bestFuzzyMatch finds the best fuzzy match score for query across multiple
// fields. It compares the query against each field as a whole and also against
// individual words within each field, returning the highest score found.
func bestFuzzyMatch(query string, fields ...string) float64 {
	ql := strings.ToLower(query)
	best := 0.0

	for _, f := range fields {
		fl := strings.ToLower(f)

		// Compare against full field.
		s := similarity(ql, fl)
		if s > best {
			best = s
		}

		// Compare against individual words in the field.
		for _, word := range strings.Fields(fl) {
			s = similarity(ql, word)
			if s > best {
				best = s
			}
		}
	}

	return best
}
