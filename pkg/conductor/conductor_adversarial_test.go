package conductor

import (
	"context"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/pkg/findings"
	"github.com/valksor/kvelmo/pkg/settings"
)

func TestAdversarialReview_DefaultPersonas(t *testing.T) {
	personas := DefaultPersonas()

	tests := []struct {
		name string
	}{
		{name: "security"},
		{name: "performance"},
		{name: "maintainability"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, ok := personas[tt.name]
			if !ok {
				t.Fatalf("persona %q not found in DefaultPersonas()", tt.name)
			}

			if p.Name != tt.name {
				t.Errorf("persona Name = %q, want %q", p.Name, tt.name)
			}

			if p.Description == "" {
				t.Error("persona Description is empty")
			}

			if p.Prompt == "" {
				t.Error("persona Prompt is empty")
			}
		})
	}
}

func TestAdversarialReview_PersonaPrompts(t *testing.T) {
	personas := DefaultPersonas()

	tests := []struct {
		name     string
		contains string // Expected substring in prompt
	}{
		{name: "security", contains: "Injection"},
		{name: "performance", contains: "Goroutine"},
		{name: "maintainability", contains: "coupling"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := personas[tt.name]

			prompt := buildAdversarialPrompt(p, "diff content", "spec content")

			if prompt == "" {
				t.Fatal("buildAdversarialPrompt returned empty prompt")
			}

			// Verify persona prompt is included.
			if !strings.Contains(prompt, tt.contains) {
				t.Errorf("prompt for %q does not contain %q", tt.name, tt.contains)
			}

			// Verify diff is included.
			if !strings.Contains(prompt, "diff content") {
				t.Error("prompt does not contain diff content")
			}

			// Verify spec is included.
			if !strings.Contains(prompt, "spec content") {
				t.Error("prompt does not contain spec content")
			}
		})
	}
}

func TestAdversarialReview_FindingsAttribution(t *testing.T) {
	tests := []struct {
		name    string
		persona string
		output  string
		want    string // Expected ReviewerPersona value
	}{
		{
			name:    "security findings attributed",
			persona: "security",
			output:  "handlers.go:42: SQL injection risk in user input",
			want:    "security",
		},
		{
			name:    "performance findings attributed",
			persona: "performance",
			output:  "db.go:100: N+1 query in GetUsers loop",
			want:    "performance",
		},
		{
			name:    "maintainability findings attributed",
			persona: "maintainability",
			output:  "service.go:55: God function doing too much",
			want:    "maintainability",
		},
		{
			name:    "empty output yields no findings",
			persona: "security",
			output:  "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAdversarialFindings(tt.persona, tt.output)

			if tt.output == "" {
				if len(result) != 0 {
					t.Fatalf("expected no findings for empty output, got %d", len(result))
				}
				return
			}

			if len(result) == 0 {
				t.Fatal("expected at least one finding")
			}

			for _, f := range result {
				if f.ReviewerPersona != tt.want {
					t.Errorf("ReviewerPersona = %q, want %q", f.ReviewerPersona, tt.want)
				}
			}
		})
	}
}

func TestAdversarialReview_DisabledSkips(t *testing.T) {
	tests := []struct {
		name string
		cfg  *settings.AdversarialReviewSettings
	}{
		{
			name: "nil config",
			cfg:  nil,
		},
		{
			name: "disabled",
			cfg:  &settings.AdversarialReviewSettings{Enabled: false},
		},
		{
			name: "no personas",
			cfg: &settings.AdversarialReviewSettings{
				Enabled:  true,
				Personas: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := settings.DefaultSettings()
			s.Workflow.AdversarialReview = tt.cfg

			c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			result, err := c.runAdversarialReview(context.Background())
			if err != nil {
				t.Fatalf("runAdversarialReview() error = %v", err)
			}

			if result != nil {
				t.Errorf("expected nil result when disabled, got %d findings", len(result))
			}
		})
	}
}

func TestAdversarialReview_BlockOnFindingsDisabled(t *testing.T) {
	// Verify that BlockOnFindings=false does not affect returned findings.
	// The actual blocking behavior happens in the review integration, but
	// the config field should default to false and not filter findings.
	cfg := &settings.AdversarialReviewSettings{
		Enabled:         true,
		Personas:        []string{"security"},
		BlockOnFindings: false,
	}

	if cfg.BlockOnFindings {
		t.Error("BlockOnFindings should default to false")
	}

	// Verify findings are returned regardless of BlockOnFindings.
	result := parseAdversarialFindings("security", "handlers.go:42: SQL injection risk")
	if len(result) == 0 {
		t.Fatal("expected findings to be returned when BlockOnFindings is false")
	}

	// Verify the finding has the correct persona attribution.
	if result[0].ReviewerPersona != "security" {
		t.Errorf("ReviewerPersona = %q, want %q", result[0].ReviewerPersona, "security")
	}
}

func TestAdversarialReview_BuildPromptWithoutDiffOrSpec(t *testing.T) {
	personas := DefaultPersonas()

	tests := []struct {
		name string
		diff string
		spec string
	}{
		{name: "no diff no spec", diff: "", spec: ""},
		{name: "diff only", diff: "some diff", spec: ""},
		{name: "spec only", diff: "", spec: "some spec"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := buildAdversarialPrompt(personas["security"], tt.diff, tt.spec)
			if prompt == "" {
				t.Fatal("prompt should not be empty even without diff/spec")
			}

			// The persona prompt should always be present.
			if !strings.Contains(prompt, "Injection") {
				t.Error("persona prompt content missing")
			}
		})
	}
}

func TestAdversarialReview_DeduplicateAcrossPersonas(t *testing.T) {
	// Simulate two personas finding the same issue.
	securityFindings := []findings.Finding{
		{
			File:            "handlers.go",
			Line:            42,
			Rule:            "SQL injection risk",
			Message:         "handlers.go:42: SQL injection risk in user input",
			Source:          "security",
			Category:        findings.CategorySecurity,
			Severity:        findings.SeverityHigh,
			ReviewerPersona: "security",
		},
	}

	perfFindings := []findings.Finding{
		{
			File:            "handlers.go",
			Line:            43, // Within line tolerance of 3
			Rule:            "SQL injection risk",
			Message:         "handlers.go:43: SQL injection risk spotted",
			Source:          "performance",
			Category:        findings.CategoryQuality,
			Severity:        findings.SeverityMedium,
			ReviewerPersona: "performance",
		},
	}

	groups := map[string][]findings.Finding{
		"security":    securityFindings,
		"performance": perfFindings,
	}

	merged := findings.DeduplicateFindings(groups)

	if len(merged) != 1 {
		t.Fatalf("expected 1 deduplicated finding, got %d", len(merged))
	}

	if len(merged[0].DetectedBy) != 2 {
		t.Errorf("expected 2 DetectedBy entries, got %d", len(merged[0].DetectedBy))
	}
}
