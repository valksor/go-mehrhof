package trello

import (
	"context"
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
)

func TestInfo(t *testing.T) {
	info := Info()

	if info.Name != ProviderName {
		t.Errorf("Name = %q, want %q", info.Name, ProviderName)
	}
	if info.Name != "trello" {
		t.Errorf("Name = %q, want %q", info.Name, "trello")
	}

	// Check schemes
	schemes := info.Schemes
	if len(schemes) != 2 {
		t.Fatalf("Schemes length = %d, want 2", len(schemes))
	}
	if schemes[0] != "trello" || schemes[1] != "tr" {
		t.Errorf("Schemes = %v, want [trello tr]", schemes)
	}

	// Check capabilities
	caps := info.Capabilities
	expectedCaps := []provider.Capability{
		provider.CapRead,
		provider.CapList,
		provider.CapFetchComments,
		provider.CapComment,
		provider.CapUpdateStatus,
		provider.CapManageLabels,
		provider.CapDownloadAttachment,
		provider.CapSnapshot,
	}
	for _, cap := range expectedCaps {
		if !caps.Has(cap) {
			t.Errorf("expected capability %q", cap)
		}
	}
}

func TestNew(t *testing.T) {
	cfg := provider.NewConfig().
		Set("api_key", "test-key").
		Set("token", "test-token").
		Set("board", "test-board")

	p, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	prov, ok := p.(*Provider)
	if !ok {
		t.Fatal("expected *Provider type")
	}

	if prov.boardID != "test-board" {
		t.Errorf("boardID = %q, want %q", prov.boardID, "test-board")
	}
	if prov.client == nil {
		t.Error("client should not be nil")
	}
}

func TestMatch(t *testing.T) {
	cfg := provider.NewConfig().
		Set("api_key", "test").
		Set("token", "test")

	p, _ := New(context.Background(), cfg)
	prov := p.(*Provider)

	tests := []struct {
		input string
		want  bool
	}{
		{"trello:abc123", true},
		{"tr:abc123", true},
		{"trello:https://trello.com/c/abc123/card", true},
		{"tr:https://trello.com/c/abc123/card", true},
		{"github:123", false},
		{"abc123", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := prov.Match(tt.input)
			if got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "full card ID",
			input: "trello:507f1f77bcf86cd799439011",
			want:  "507f1f77bcf86cd799439011",
		},
		{
			name:  "short link",
			input: "tr:abc12345",
			want:  "abc12345",
		},
		{
			name:  "URL format",
			input: "trello:https://trello.com/c/abc12345/my-card-name",
			want:  "abc12345",
		},
		{
			name:  "bare URL",
			input: "https://trello.com/c/xyz99999/some-card",
			want:  "xyz99999",
		},
		{
			name:  "bare card ID",
			input: "507f1f77bcf86cd799439011",
			want:  "507f1f77bcf86cd799439011",
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "just scheme",
			input:   "trello:",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "trello:not-valid-reference!!!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseReference(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.CardID != tt.want {
				t.Errorf("CardID = %q, want %q", ref.CardID, tt.want)
			}
		})
	}
}

func TestMapTrelloListToStatus(t *testing.T) {
	tests := []struct {
		listName string
		want     provider.Status
	}{
		{"To Do", provider.StatusOpen},
		{"TODO", provider.StatusOpen},
		{"Backlog", provider.StatusOpen},
		{"In Progress", provider.StatusInProgress},
		{"Doing", provider.StatusInProgress},
		{"WIP", provider.StatusInProgress},
		{"In Review", provider.StatusReview},
		{"Review", provider.StatusReview},
		{"Done", provider.StatusDone},
		{"Completed", provider.StatusDone},
		{"Cancelled", provider.StatusClosed},
		{"Unknown List", provider.StatusOpen}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.listName, func(t *testing.T) {
			got := mapTrelloListToStatus(tt.listName)
			if got != tt.want {
				t.Errorf("mapTrelloListToStatus(%q) = %v, want %v", tt.listName, got, tt.want)
			}
		})
	}
}

func TestMapProviderStatusToListName(t *testing.T) {
	tests := []struct {
		status provider.Status
		want   string
	}{
		{provider.StatusOpen, "To Do"},
		{provider.StatusInProgress, "Doing"},
		{provider.StatusReview, "In Review"},
		{provider.StatusDone, "Done"},
		{provider.StatusClosed, "Done"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := mapProviderStatusToListName(tt.status)
			if got != tt.want {
				t.Errorf("mapProviderStatusToListName(%v) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestExtractLabels(t *testing.T) {
	card := &Card{
		Labels: []Label{
			{Name: "bug", Color: "red"},
			{Name: "feature", Color: "green"},
			{Name: "", Color: "blue"}, // No name, should use color
		},
	}

	labels := extractLabels(card)
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	if labels[0] != "bug" {
		t.Errorf("labels[0] = %q, want %q", labels[0], "bug")
	}
	if labels[1] != "feature" {
		t.Errorf("labels[1] = %q, want %q", labels[1], "feature")
	}
	if labels[2] != "blue" {
		t.Errorf("labels[2] = %q, want %q (color fallback)", labels[2], "blue")
	}
}

func TestExtractMembers(t *testing.T) {
	card := &Card{
		Members: []Member{
			{ID: "1", FullName: "John Doe", Username: "johndoe"},
			{ID: "2", FullName: "Jane Smith", Username: "janesmith"},
		},
	}

	members := extractMembers(card)
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	if members[0].Name != "John Doe" {
		t.Errorf("members[0].Name = %q, want %q", members[0].Name, "John Doe")
	}
	if members[1].ID != "2" {
		t.Errorf("members[1].ID = %q, want %q", members[1].ID, "2")
	}
}

func TestHasAnyLabel(t *testing.T) {
	card := Card{
		Labels: []Label{
			{Name: "bug", Color: "red"},
			{Name: "urgent", Color: "orange"},
		},
	}

	tests := []struct {
		labels []string
		want   bool
	}{
		{[]string{"bug"}, true},
		{[]string{"urgent"}, true},
		{[]string{"feature"}, false},
		{[]string{"feature", "bug"}, true}, // Has bug
		{[]string{"red"}, true},            // Color match
		{[]string{"BUG"}, true},            // Case insensitive
	}

	for _, tt := range tests {
		got := hasAnyLabel(card, tt.labels)
		if got != tt.want {
			t.Errorf("hasAnyLabel(card, %v) = %v, want %v", tt.labels, got, tt.want)
		}
	}
}

func TestIsCardID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"507f1f77bcf86cd799439011", true},  // Valid 24-char hex
		{"507F1F77BCF86CD799439011", true},  // Uppercase hex
		{"abc12345", false},                 // Too short
		{"507f1f77bcf86cd79943901z", false}, // Invalid char
	}

	for _, tt := range tests {
		got := isCardID(tt.input)
		if got != tt.want {
			t.Errorf("isCardID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsShortLink(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abc12345", true},                  // Valid 8-char alphanumeric
		{"ABC12345", true},                  // Uppercase
		{"abcd1234", true},                  // Mixed
		{"abc1234", false},                  // Too short
		{"abc123456", false},                // Too long
		{"abc1234!", false},                 // Invalid char
		{"507f1f77bcf86cd799439011", false}, // Card ID, not short link
	}

	for _, tt := range tests {
		got := isShortLink(tt.input)
		if got != tt.want {
			t.Errorf("isShortLink(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestRefString(t *testing.T) {
	tests := []struct {
		ref  Ref
		want string
	}{
		{Ref{CardID: "abc12345"}, "trello:abc12345"},
		{Ref{CardID: "abc12345", URL: "https://trello.com/c/abc12345"}, "trello:https://trello.com/c/abc12345"},
	}

	for _, tt := range tests {
		got := tt.ref.String()
		if got != tt.want {
			t.Errorf("Ref.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestBuildSnapshotContent(t *testing.T) {
	card := &Card{
		Name: "Test Card",
		Desc: "This is a test description.",
		Labels: []Label{
			{Name: "bug", Color: "red"},
		},
		Checklists: []Checklist{
			{
				Name: "Tasks",
				CheckItems: []CheckItem{
					{Name: "Task 1", State: "complete"},
					{Name: "Task 2", State: "incomplete"},
				},
			},
		},
	}

	content := buildSnapshotContent(card)

	// Check title
	if !containsSubstring(content, "# Test Card") {
		t.Error("expected title in snapshot")
	}

	// Check description
	if !containsSubstring(content, "This is a test description.") {
		t.Error("expected description in snapshot")
	}

	// Check labels
	if !containsSubstring(content, "## Labels") {
		t.Error("expected labels section in snapshot")
	}
	if !containsSubstring(content, "- bug") {
		t.Error("expected bug label in snapshot")
	}

	// Check checklists
	if !containsSubstring(content, "### Tasks") {
		t.Error("expected checklist name in snapshot")
	}
	if !containsSubstring(content, "- [x] Task 1") {
		t.Error("expected completed item in snapshot")
	}
	if !containsSubstring(content, "- [ ] Task 2") {
		t.Error("expected incomplete item in snapshot")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestResolveAPIKey(t *testing.T) {
	// Provided key should be returned
	if got := ResolveAPIKey("provided-key"); got != "provided-key" {
		t.Errorf("ResolveAPIKey(provided) = %q, want %q", got, "provided-key")
	}

	// Empty should attempt env lookup (returns empty if not set)
	got := ResolveAPIKey("")
	// Can't easily test env vars without setting them, but empty should not panic
	_ = got
}

func TestResolveToken(t *testing.T) {
	// Provided token should be returned
	if got := ResolveToken("provided-token"); got != "provided-token" {
		t.Errorf("ResolveToken(provided) = %q, want %q", got, "provided-token")
	}
}
