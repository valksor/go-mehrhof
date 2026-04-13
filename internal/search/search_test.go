package search

import (
	"testing"
	"time"
)

// mockItem implements Searchable for testing.
type mockItem struct {
	title       string
	description string
	tags        []string
	status      string
	createdAt   time.Time
	priority    int
}

func (m mockItem) SearchTitle() string        { return m.title }
func (m mockItem) SearchDescription() string  { return m.description }
func (m mockItem) SearchTags() []string       { return m.tags }
func (m mockItem) SearchStatus() string       { return m.status }
func (m mockItem) SearchCreatedAt() time.Time { return m.createdAt }
func (m mockItem) SearchPriority() int        { return m.priority }

func newItem(title, desc, status string, tags ...string) mockItem {
	return mockItem{
		title:       title,
		description: desc,
		tags:        tags,
		status:      status,
		createdAt:   time.Now(),
		priority:    0,
	}
}

func TestSearch_ExactTitleMatchRanksHighest(t *testing.T) {
	items := []mockItem{
		newItem("Fix database connection", "refactor pool", "loaded"),
		newItem("Add login page", "implement the login form", "planned"),
		newItem("login feature", "create user login", "implementing"),
	}

	results := Search(items, "login", DefaultOptions())
	if len(results) == 0 {
		t.Fatal("expected results")
	}

	// "login feature" has "login" in the title (exact match, shorter title).
	// Both title-matching items should rank above the description-only match.
	if results[0].Item.title != "login feature" && results[0].Item.title != "Add login page" {
		t.Errorf("expected a title match first, got %q", results[0].Item.title)
	}
}

func TestSearch_SubstringMatch(t *testing.T) {
	items := []mockItem{
		newItem("Implement authentication", "add OAuth flow", "planned"),
		newItem("Fix CSS layout", "responsive design", "loaded"),
	}

	results := Search(items, "auth", DefaultOptions())
	if len(results) == 0 {
		t.Fatal("expected results for substring match")
	}

	if results[0].Item.title != "Implement authentication" {
		t.Errorf("expected authentication task first, got %q", results[0].Item.title)
	}
}

func TestSearch_FuzzyMatchCatchesTypos(t *testing.T) {
	items := []mockItem{
		newItem("Implement feature", "build the new feature", "planned"),
		newItem("Fix database bug", "resolve connection timeout", "loaded"),
	}

	// "implment" is a typo for "implement".
	results := Search(items, "implment", DefaultOptions())
	if len(results) == 0 {
		t.Fatal("expected fuzzy match results for typo")
	}

	if results[0].Item.title != "Implement feature" {
		t.Errorf("expected 'Implement feature' first, got %q", results[0].Item.title)
	}
}

func TestSearch_TagsAreSearchable(t *testing.T) {
	items := []mockItem{
		newItem("Fix bug", "some bug fix", "loaded", "urgent", "backend"),
		newItem("Add feature", "new feature", "planned", "frontend"),
	}

	results := Search(items, "backend", DefaultOptions())
	if len(results) == 0 {
		t.Fatal("expected results for tag search")
	}

	if results[0].Item.title != "Fix bug" {
		t.Errorf("expected tagged item first, got %q", results[0].Item.title)
	}
}

func TestSearch_RRFCombinesSignals(t *testing.T) {
	items := []mockItem{
		newItem("Deploy service", "deploy the microservice", "loaded"),
		newItem("deploi srvice", "deployment automation", "planned"), // typo title but also text match in desc
	}

	// "deploy" should match first item via text, second via fuzzy.
	// RRF should rank the exact match higher.
	results := Search(items, "deploy", DefaultOptions())
	if len(results) == 0 {
		t.Fatal("expected results")
	}

	if results[0].Item.title != "Deploy service" {
		t.Errorf("expected exact match first, got %q", results[0].Item.title)
	}
}

func TestSearch_BoostActiveItems(t *testing.T) {
	now := time.Now()
	items := []mockItem{
		{title: "Task A", description: "matching task", status: "submitted", createdAt: now},
		{title: "Task B", description: "matching task", status: "implementing", createdAt: now},
	}

	opts := DefaultOptions()
	opts.BoostActive = true

	results := Search(items, "matching", opts)
	if len(results) < 2 {
		t.Fatal("expected 2 results")
	}

	// The implementing item should rank higher due to active boost.
	if results[0].Item.title != "Task B" {
		t.Errorf("expected active item boosted, got %q first", results[0].Item.title)
	}
}

func TestSearch_BoostRecentItems(t *testing.T) {
	items := []mockItem{
		{title: "Old matching task", description: "test", status: "loaded", createdAt: time.Now().Add(-720 * time.Hour)},
		{title: "New matching task", description: "test", status: "loaded", createdAt: time.Now()},
	}

	opts := DefaultOptions()
	opts.BoostRecent = true

	results := Search(items, "matching", opts)
	if len(results) < 2 {
		t.Fatal("expected 2 results")
	}

	if results[0].Item.title != "New matching task" {
		t.Errorf("expected recent item first, got %q", results[0].Item.title)
	}
}

func TestSearch_AdaptiveRelaxation(t *testing.T) {
	items := []mockItem{
		newItem("Implement authentication", "OAuth integration", "planned"),
		newItem("Build login page", "user login form", "loaded"),
		newItem("Fix CSS styling", "responsive layout", "loaded"),
	}

	// "authentcation" is a typo that won't text-match but should fuzzy-match.
	opts := DefaultOptions()
	opts.MinResults = 1

	results := Search(items, "authentcation", opts)
	if len(results) == 0 {
		t.Fatal("expected adaptive relaxation to produce fuzzy results")
	}

	// The authentication item should appear via fuzzy matching.
	found := false
	for _, r := range results {
		if r.Item.title == "Implement authentication" {
			found = true

			break
		}
	}

	if !found {
		t.Error("expected 'Implement authentication' in relaxed results")
	}
}

func TestSearch_EmptyQueryReturnsAll(t *testing.T) {
	items := []mockItem{
		newItem("Task A", "desc a", "loaded"),
		newItem("Task B", "desc b", "planned"),
		newItem("Task C", "desc c", "implementing"),
	}

	results := Search(items, "", DefaultOptions())
	if len(results) != 3 {
		t.Errorf("empty query: got %d results, want 3", len(results))
	}
}

func TestSearch_EmptyItemsReturnsEmpty(t *testing.T) {
	results := Search([]mockItem{}, "query", DefaultOptions())
	if len(results) != 0 {
		t.Errorf("empty items: got %d results, want 0", len(results))
	}
}

func TestSearch_NilItemsReturnsNil(t *testing.T) {
	results := Search[mockItem](nil, "query", DefaultOptions())
	if results != nil {
		t.Errorf("nil items: got %v, want nil", results)
	}
}

func TestSearch_ScoreOrderingIsCorrect(t *testing.T) {
	items := []mockItem{
		newItem("Alpha", "unrelated content", "loaded"),
		newItem("Beta search term", "also has search term", "loaded"),
		newItem("Gamma", "search term here", "loaded"),
	}

	results := Search(items, "search term", DefaultOptions())

	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: index %d (score %.4f) > index %d (score %.4f)",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	items := []mockItem{
		newItem("IMPLEMENT Feature", "BUILD the thing", "loaded"),
	}

	results := Search(items, "implement", DefaultOptions())
	if len(results) == 0 {
		t.Fatal("expected case-insensitive match")
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.TextWeight != 0.7 {
		t.Errorf("TextWeight: got %f, want 0.7", opts.TextWeight)
	}

	if opts.FuzzyWeight != 0.3 {
		t.Errorf("FuzzyWeight: got %f, want 0.3", opts.FuzzyWeight)
	}

	if opts.MinResults != 3 {
		t.Errorf("MinResults: got %d, want 3", opts.MinResults)
	}
}
