package search

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"time"
)

const rrfK = 60.0 // RRF smoothing constant (matches tracigo pattern)

// Searchable defines the interface for items that can be searched.
type Searchable interface {
	SearchTitle() string
	SearchDescription() string
	SearchTags() []string
	SearchStatus() string
	SearchCreatedAt() time.Time
	SearchPriority() int
}

// Result wraps a search result with its score.
type Result[T Searchable] struct {
	Item  T       `json:"item"`
	Score float64 `json:"score"`
}

// Options configures search behavior.
type Options struct {
	TextWeight  float64 // weight for text signal in RRF (default 0.7)
	FuzzyWeight float64 // weight for fuzzy signal in RRF (default 0.3)
	BoostActive bool    // boost items with active-like statuses
	BoostRecent bool    // boost recently created items
	MinResults  int     // adaptive relaxation threshold (default 3)
}

// DefaultOptions returns sensible default search options.
func DefaultOptions() Options {
	return Options{
		TextWeight:  0.7,
		FuzzyWeight: 0.3,
		BoostActive: false,
		BoostRecent: false,
		MinResults:  3,
	}
}

// normalize fills zero-valued option fields with defaults.
func (o Options) normalize() Options {
	if o.TextWeight == 0 && o.FuzzyWeight == 0 {
		o.TextWeight = 0.7
		o.FuzzyWeight = 0.3
	}

	if o.MinResults <= 0 {
		o.MinResults = 3
	}

	return o
}

// activeStatuses lists statuses considered "active" for boost purposes.
var activeStatuses = map[string]bool{
	"loaded":       true,
	"planning":     true,
	"planned":      true,
	"implementing": true,
	"implemented":  true,
	"simplifying":  true,
	"optimizing":   true,
	"reviewing":    true,
}

// Search performs hybrid text+fuzzy search with Reciprocal Rank Fusion ranking.
//
// The algorithm:
//  1. Text signal: exact case-insensitive substring match in title (2x weight),
//     description, and tags.
//  2. Fuzzy signal: Levenshtein-based similarity across all searchable fields.
//  3. Merge via RRF: score = sum(weight / (rrfK + rank)) for each signal.
//  4. Apply optional boosts (active status, recency).
//  5. Sort by score descending.
//  6. If results < MinResults, relax to fuzzy-only and retry.
func Search[T Searchable](items []T, query string, opts Options) []Result[T] {
	if len(items) == 0 {
		return nil
	}

	opts = opts.normalize()

	// Empty query returns all items sorted by priority then recency.
	if strings.TrimSpace(query) == "" {
		return allItemsSorted(items, opts)
	}

	ql := strings.ToLower(strings.TrimSpace(query))

	// Gather text signal rankings.
	textRanked := textSignal(items, ql)

	// Gather fuzzy signal rankings.
	fuzzyRanked := fuzzySignal(items, ql)

	// Merge via RRF.
	scores := make(map[int]float64, len(items))

	for rank, idx := range textRanked {
		scores[idx] += opts.TextWeight / (rrfK + float64(rank+1))
	}

	for rank, idx := range fuzzyRanked {
		scores[idx] += opts.FuzzyWeight / (rrfK + float64(rank+1))
	}

	// Build results from items that have a non-zero score.
	results := make([]Result[T], 0, len(scores))
	for idx, score := range scores {
		if score <= 0 {
			continue
		}

		score = applyBoosts(score, items[idx], opts)
		results = append(results, Result[T]{Item: items[idx], Score: score})
	}

	slices.SortFunc(results, func(a, b Result[T]) int {
		return cmp.Compare(b.Score, a.Score)
	})

	// Adaptive relaxation: if too few results, fall back to fuzzy-only.
	if len(results) < opts.MinResults && len(fuzzyRanked) > 0 {
		results = fuzzyOnlyResults(items, fuzzyRanked, opts)
	}

	return results
}

// textSignal scores items by case-insensitive substring matching and returns
// indices sorted by score descending. Title matches receive 2x weight.
func textSignal[T Searchable](items []T, query string) []int {
	type scored struct {
		idx   int
		score float64
	}

	var matched []scored

	for i, item := range items {
		s := 0.0

		title := strings.ToLower(item.SearchTitle())
		desc := strings.ToLower(item.SearchDescription())

		if strings.Contains(title, query) {
			s += 2.0 // title boost
		}

		if strings.Contains(desc, query) {
			s += 1.0
		}

		for _, tag := range item.SearchTags() {
			if strings.Contains(strings.ToLower(tag), query) {
				s += 1.0

				break // one tag match is enough
			}
		}

		if s > 0 {
			matched = append(matched, scored{idx: i, score: s})
		}
	}

	slices.SortFunc(matched, func(a, b scored) int {
		return cmp.Compare(b.score, a.score)
	})

	indices := make([]int, len(matched))
	for i, m := range matched {
		indices[i] = m.idx
	}

	return indices
}

// fuzzySignal scores items by Levenshtein similarity and returns indices sorted
// by score descending. Only items with similarity >= 0.3 threshold are included.
func fuzzySignal[T Searchable](items []T, query string) []int {
	const threshold = 0.3

	type scored struct {
		idx   int
		score float64
	}

	var matched []scored

	for i, item := range items {
		fields := []string{item.SearchTitle(), item.SearchDescription()}
		fields = append(fields, item.SearchTags()...)

		s := bestFuzzyMatch(query, fields...)
		if s >= threshold {
			matched = append(matched, scored{idx: i, score: s})
		}
	}

	slices.SortFunc(matched, func(a, b scored) int {
		return cmp.Compare(b.score, a.score)
	})

	indices := make([]int, len(matched))
	for i, m := range matched {
		indices[i] = m.idx
	}

	return indices
}

// applyBoosts adds optional score adjustments for active status and recency.
func applyBoosts[T Searchable](score float64, item T, opts Options) float64 {
	if opts.BoostActive && activeStatuses[strings.ToLower(item.SearchStatus())] {
		score += 0.01
	}

	if opts.BoostRecent {
		age := time.Since(item.SearchCreatedAt())
		// Decay: items created within the last 24h get up to +0.01,
		// falling off logarithmically.
		if age > 0 {
			hours := age.Hours()
			if hours < 1 {
				hours = 1
			}

			score += 0.01 / math.Log2(1+hours)
		}
	}

	return score
}

// fuzzyOnlyResults builds a result set using only fuzzy rankings when the
// hybrid pass produced too few results.
func fuzzyOnlyResults[T Searchable](items []T, fuzzyRanked []int, opts Options) []Result[T] {
	results := make([]Result[T], 0, len(fuzzyRanked))

	for rank, idx := range fuzzyRanked {
		score := 1.0 / (rrfK + float64(rank+1))
		score = applyBoosts(score, items[idx], opts)
		results = append(results, Result[T]{Item: items[idx], Score: score})
	}

	slices.SortFunc(results, func(a, b Result[T]) int {
		return cmp.Compare(b.Score, a.Score)
	})

	return results
}

// allItemsSorted returns all items when the query is empty, ordered by priority
// (descending) then creation time (most recent first).
func allItemsSorted[T Searchable](items []T, opts Options) []Result[T] {
	results := make([]Result[T], len(items))

	for i, item := range items {
		score := float64(item.SearchPriority()) / 100.0
		score = applyBoosts(score, item, opts)
		results[i] = Result[T]{Item: item, Score: score}
	}

	slices.SortFunc(results, func(a, b Result[T]) int {
		if a.Score != b.Score {
			return cmp.Compare(b.Score, a.Score)
		}
		// Tie-break: most recent first.
		return a.Item.SearchCreatedAt().Compare(b.Item.SearchCreatedAt()) * -1
	})

	return results
}
