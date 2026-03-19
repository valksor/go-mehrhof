package search

import (
	"testing"
)

func TestLevenshtein_Identical(t *testing.T) {
	if d := levenshtein("hello", "hello"); d != 0 {
		t.Errorf("identical strings: got %d, want 0", d)
	}
}

func TestLevenshtein_SingleEdit(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"cat", "bat", 1},   // substitution
		{"cat", "cats", 1},  // insertion
		{"cats", "cat", 1},  // deletion
		{"kitten", "sitten", 1},
	}

	for _, tt := range tests {
		if d := levenshtein(tt.a, tt.b); d != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, d, tt.want)
		}
	}
}

func TestLevenshtein_CompletelyDifferent(t *testing.T) {
	d := levenshtein("abc", "xyz")
	if d != 3 {
		t.Errorf("completely different: got %d, want 3", d)
	}
}

func TestLevenshtein_EmptyStrings(t *testing.T) {
	if d := levenshtein("", "hello"); d != 5 {
		t.Errorf("empty vs non-empty: got %d, want 5", d)
	}

	if d := levenshtein("hello", ""); d != 5 {
		t.Errorf("non-empty vs empty: got %d, want 5", d)
	}

	if d := levenshtein("", ""); d != 0 {
		t.Errorf("both empty: got %d, want 0", d)
	}
}

func TestLevenshtein_Unicode(t *testing.T) {
	// Each rune counts as one character.
	if d := levenshtein("cafe", "caf\u00e9"); d != 1 {
		t.Errorf("unicode substitution: got %d, want 1", d)
	}
}

func TestSimilarity_Exact(t *testing.T) {
	s := similarity("Hello", "hello")
	if s != 1.0 {
		t.Errorf("case-insensitive exact match: got %f, want 1.0", s)
	}
}

func TestSimilarity_OneEdit(t *testing.T) {
	s := similarity("cat", "bat")
	// distance=1, maxLen=3 => 1 - 1/3 = 0.666...
	want := 1.0 - 1.0/3.0
	if diff := s - want; diff > 0.001 || diff < -0.001 {
		t.Errorf("one edit on short string: got %f, want %f", s, want)
	}
}

func TestSimilarity_EmptyStrings(t *testing.T) {
	if s := similarity("", ""); s != 1.0 {
		t.Errorf("both empty: got %f, want 1.0", s)
	}
}

func TestFuzzyMatch_AboveThreshold(t *testing.T) {
	score, ok := fuzzyMatch("implement", "implment", 0.5)
	if !ok {
		t.Error("expected match above threshold")
	}

	if score < 0.5 {
		t.Errorf("score %f below threshold 0.5", score)
	}
}

func TestFuzzyMatch_BelowThreshold(t *testing.T) {
	_, ok := fuzzyMatch("implement", "zzzzzzzzz", 0.5)
	if ok {
		t.Error("expected no match for completely different strings")
	}
}

func TestBestFuzzyMatch_MultipleFields(t *testing.T) {
	score := bestFuzzyMatch("implment", "implement feature", "some description", "bug")
	if score < 0.7 {
		t.Errorf("expected high score for near-match in field, got %f", score)
	}
}

func TestBestFuzzyMatch_ExactWordInField(t *testing.T) {
	score := bestFuzzyMatch("login", "implement login page", "user authentication")
	if score < 0.99 {
		t.Errorf("expected ~1.0 for exact word match, got %f", score)
	}
}
