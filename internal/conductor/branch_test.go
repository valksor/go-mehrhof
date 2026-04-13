package conductor

import (
	"testing"

	"github.com/valksor/kvelmo/settings"
)

// ─── slugify ─────────────────────────────────────────────────────────────────

func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "simple lowercase",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "mixed case to lowercase",
			input: "Hello World",
			want:  "hello-world",
		},
		{
			name:  "spaces become hyphens",
			input: "add new feature",
			want:  "add-new-feature",
		},
		{
			name:  "underscores become hyphens",
			input: "fix_the_bug",
			want:  "fix-the-bug",
		},
		{
			name:  "special characters removed",
			input: "feat(api): add endpoint!",
			want:  "featapi-add-endpoint",
		},
		{
			name:  "multiple consecutive hyphens collapsed",
			input: "hello   world",
			want:  "hello-world",
		},
		{
			name:  "trailing hyphens trimmed",
			input: "hello-world-",
			want:  "hello-world",
		},
		{
			name:  "leading hyphens trimmed",
			input: "-hello-world",
			want:  "hello-world",
		},
		{
			name:  "numbers preserved",
			input: "fix issue 42",
			want:  "fix-issue-42",
		},
		{
			name:  "string over 50 chars gets truncated",
			input: "this is a very long title that exceeds fifty characters in total length",
			want:  "this-is-a-very-long-title-that-exceeds-fifty-chara",
		},
		{
			name:  "truncation does not end with hyphen",
			input: "this is a very long title that ends right on hyphen boundary ok",
			// Just verify no trailing hyphen — exact value depends on truncation point
			want: func() string {
				result := slugify("this is a very long title that ends right on hyphen boundary ok")

				return result
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
			// Verify result never ends with a hyphen
			if len(got) > 0 && got[len(got)-1] == '-' {
				t.Errorf("slugify(%q) = %q: result must not end with hyphen", tt.input, got)
			}
			// Verify result is at most 50 chars
			if len([]rune(got)) > 50 {
				t.Errorf("slugify(%q) = %q: length %d exceeds 50", tt.input, got, len([]rune(got)))
			}
		})
	}
}

// ─── generateBranchName ──────────────────────────────────────────────────────

// newConductorWithSettings creates a minimal conductor and injects settings directly.
func newConductorWithSettings(t *testing.T, s *settings.Settings) *Conductor {
	t.Helper()
	c, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	c.cachedSettings.Store(s)

	return c
}

func TestGenerateBranchName(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wu      *WorkUnit
		want    string
	}{
		{
			name:    "default pattern with key and slug",
			pattern: "feature/{key}--{slug}",
			wu: &WorkUnit{
				ID:         "task-1",
				ExternalID: "PROJ-42",
				Title:      "Add login page",
				Source:     &Source{Provider: "github"},
			},
			want: "feature/PROJ-42-add-login-page",
		},
		{
			name:    "pattern with type interpolation",
			pattern: "{type}/{key}-{slug}",
			wu: &WorkUnit{
				ID:         "task-2",
				ExternalID: "99",
				Title:      "Fix bug",
				Source:     &Source{Provider: "gitlab"},
			},
			want: "gitlab/99-fix-bug",
		},
		{
			name:    "no ExternalID uses WorkUnit ID as key",
			pattern: "feature/{key}--{slug}",
			wu: &WorkUnit{
				ID:    "internal-abc",
				Title: "Update readme",
			},
			want: "feature/internal-abc-update-readme",
		},
		{
			name:    "nil Source sets type to local",
			pattern: "{type}/{slug}",
			wu: &WorkUnit{
				ID:    "t3",
				Title: "Local task",
			},
			want: "local/local-task",
		},
		{
			name:    "empty key in pattern removes key prefix",
			pattern: "feature/{key}--{slug}",
			wu: &WorkUnit{
				ID:    "",
				Title: "No key task",
			},
			want: "feature/no-key-task",
		},
		{
			name:    "empty pattern falls back to default",
			pattern: "",
			wu: &WorkUnit{
				ID:         "t4",
				ExternalID: "TASK-7",
				Title:      "Simple task",
				Source:     &Source{Provider: "wrike"},
			},
			want: "feature/TASK-7-simple-task",
		},
		{
			name:    "title with special characters slugified",
			pattern: "fix/{key}--{slug}",
			wu: &WorkUnit{
				ID:         "t5",
				ExternalID: "BUG-1",
				Title:      "Fix: broken API (v2)!",
				Source:     &Source{Provider: "github"},
			},
			want: "fix/BUG-1-fix-broken-api-v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := settings.DefaultSettings()
			s.Git.BranchPattern = tt.pattern
			c := newConductorWithSettings(t, s)
			c.ForceWorkUnit(tt.wu)

			got := c.generateBranchName(tt.wu)
			if got != tt.want {
				t.Errorf("generateBranchName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ─── validateBranchName ───────────────────────────────────────────────────────

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		pattern string
		wantErr bool
	}{
		{"empty pattern allows any", "anything", "", false},
		{"matching pattern", "feature/PROJ-123-add-login", `^(feature|bugfix)/`, false},
		{"non-matching pattern", "my-branch", `^(feature|bugfix)/`, true},
		{"invalid regex", "anything", `[invalid`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBranchName(tt.branch, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBranchName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ─── shouldPostTicketComment ──────────────────────────────────────────────────

func TestShouldPostTicketComment(t *testing.T) {
	tests := []struct {
		name   string
		wu     *WorkUnit
		github bool
		gitlab bool
		wrike  bool
		linear bool
		want   bool
	}{
		{
			name: "nil WorkUnit",
			wu:   nil,
			want: false,
		},
		{
			name: "nil Source",
			wu:   &WorkUnit{},
			want: false,
		},
		{
			name:   "github provider with AllowTicketComment true",
			wu:     &WorkUnit{Source: &Source{Provider: "github"}},
			github: true,
			want:   true,
		},
		{
			name:   "github provider with AllowTicketComment false",
			wu:     &WorkUnit{Source: &Source{Provider: "github"}},
			github: false,
			want:   false,
		},
		{
			name:   "gitlab provider with AllowTicketComment true",
			wu:     &WorkUnit{Source: &Source{Provider: "gitlab"}},
			gitlab: true,
			want:   true,
		},
		{
			name:   "gitlab provider with AllowTicketComment false",
			wu:     &WorkUnit{Source: &Source{Provider: "gitlab"}},
			gitlab: false,
			want:   false,
		},
		{
			name:  "wrike provider with AllowTicketComment true",
			wu:    &WorkUnit{Source: &Source{Provider: "wrike"}},
			wrike: true,
			want:  true,
		},
		{
			name:  "wrike provider with AllowTicketComment false",
			wu:    &WorkUnit{Source: &Source{Provider: "wrike"}},
			wrike: false,
			want:  false,
		},
		{
			name:   "linear provider with AllowTicketComment true",
			wu:     &WorkUnit{Source: &Source{Provider: "linear"}},
			linear: true,
			want:   true,
		},
		{
			name:   "linear provider with AllowTicketComment false",
			wu:     &WorkUnit{Source: &Source{Provider: "linear"}},
			linear: false,
			want:   false,
		},
		{
			name: "file provider (no ticket comments)",
			wu:   &WorkUnit{Source: &Source{Provider: "file"}},
			want: false,
		},
		{
			name: "unknown provider (no ticket comments)",
			wu:   &WorkUnit{Source: &Source{Provider: "unknown-provider"}},
			want: false,
		},
		{
			name:   "github disabled even when other providers enabled",
			wu:     &WorkUnit{Source: &Source{Provider: "github"}},
			github: false,
			gitlab: true,
			wrike:  true,
			linear: true,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := settings.DefaultSettings()
			s.Providers.GitHub.AllowTicketComment = tt.github
			s.Providers.GitLab.AllowTicketComment = tt.gitlab
			s.Providers.Wrike.AllowTicketComment = tt.wrike
			s.Providers.Linear.AllowTicketComment = tt.linear

			c := newConductorWithSettings(t, s)
			c.ForceWorkUnit(tt.wu)

			got := c.shouldPostTicketComment()
			if got != tt.want {
				t.Errorf("shouldPostTicketComment() = %v, want %v", got, tt.want)
			}
		})
	}
}
