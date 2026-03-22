package conductor

import (
	"fmt"
	"strings"

	"github.com/valksor/kvelmo/pkg/varpool"
)

// PhaseContextProfile defines what categories of information to include for a phase.
type PhaseContextProfile struct {
	IncludeTask        bool // full title + description
	IncludeTaskSummary bool // title + truncated description (first 500 chars)
	IncludeSpecs       bool // specification files
	IncludeDiff        bool // current git diff
	IncludeFindings    bool // quality gate findings
	IncludeHierarchy   bool // parent/sibling tasks
	IncludeMemory      bool // vector store retrieval (TODO: wire to memory.Store.Query)
	MaxTokenBudget     int  // soft cap for context assembly (0 = unlimited)
}

// ContextMetrics tracks what was included/excluded during context assembly.
type ContextMetrics struct {
	TokensUsed       int      `json:"tokens_used"`
	TokenBudget      int      `json:"token_budget"`
	SectionsIncluded []string `json:"sections_included"`
	SectionsExcluded []string `json:"sections_excluded"`
}

// DefaultContextProfiles returns the default context profile for each phase.
func DefaultContextProfiles() map[string]PhaseContextProfile {
	return map[string]PhaseContextProfile{
		"plan": {
			IncludeTask:      true,
			IncludeHierarchy: true,
			IncludeMemory:    true,
			MaxTokenBudget:   8000,
		},
		"implement": {
			IncludeTaskSummary: true,
			IncludeSpecs:       true,
			IncludeMemory:      true,
			MaxTokenBudget:     12000,
		},
		"simplify": {
			IncludeDiff:     true,
			IncludeFindings: true,
			MaxTokenBudget:  4000,
		},
		"optimize": {
			IncludeDiff:     true,
			IncludeFindings: true,
			MaxTokenBudget:  4000,
		},
		"review": {
			IncludeTaskSummary: true,
			IncludeSpecs:       true,
			IncludeDiff:        true,
			IncludeFindings:    true,
			MaxTokenBudget:     8000,
		},
	}
}

// BuildPhaseContext assembles context for a phase based on its profile.
// Returns the context string and metrics about what was included.
func BuildPhaseContext(profile PhaseContextProfile, wu *WorkUnit, pool *varpool.Pool) (string, ContextMetrics) {
	var sections []string
	metrics := ContextMetrics{
		TokenBudget: profile.MaxTokenBudget,
	}

	estimateTokens := func(s string) int {
		// Rough estimate: 1 token ~ 4 characters
		return len(s) / 4
	}

	budgetRemaining := profile.MaxTokenBudget
	if budgetRemaining == 0 {
		budgetRemaining = 999999 // unlimited
	}

	addSection := func(name, content string) {
		if content == "" {
			return
		}
		tokens := estimateTokens(content)
		if tokens > budgetRemaining && budgetRemaining < 999999 {
			metrics.SectionsExcluded = append(metrics.SectionsExcluded, name+" (budget exceeded)")

			return
		}
		sections = append(sections, fmt.Sprintf("## %s\n\n%s", name, content))
		metrics.SectionsIncluded = append(metrics.SectionsIncluded, name)
		budgetRemaining -= tokens
	}

	if wu == nil {
		return "", metrics
	}

	// Task description
	if profile.IncludeTask {
		content := fmt.Sprintf("**%s**\n\n%s", wu.Title, wu.Description)
		addSection("Task", content)
	} else if profile.IncludeTaskSummary {
		desc := wu.Description
		if runes := []rune(desc); len(runes) > 500 {
			desc = string(runes[:500]) + "..."
		}
		addSection("Task Summary", fmt.Sprintf("**%s**\n\n%s", wu.Title, desc))
	}

	// Specifications
	if profile.IncludeSpecs && len(wu.Specifications) > 0 {
		addSection("Specifications", fmt.Sprintf("%d specification file(s) available", len(wu.Specifications)))
	}

	// Hierarchy (parent/sibling context)
	if profile.IncludeHierarchy && wu.Hierarchy != nil {
		var parts []string
		if wu.Hierarchy.Parent != nil {
			parts = append(parts, "Parent: "+wu.Hierarchy.Parent.Title)
		}
		if len(wu.Hierarchy.Siblings) > 0 {
			titles := make([]string, 0, len(wu.Hierarchy.Siblings))
			for _, s := range wu.Hierarchy.Siblings {
				titles = append(titles, s.Title)
			}
			parts = append(parts, "Siblings: "+strings.Join(titles, ", "))
		}
		if len(parts) > 0 {
			addSection("Hierarchy", strings.Join(parts, "\n"))
		}
	}

	// Diff
	if profile.IncludeDiff && pool != nil {
		if diff := pool.GetScopedString("sys", "last_diff"); diff != "" {
			addSection("Current Changes", diff)
		}
	}

	// Findings
	if profile.IncludeFindings && pool != nil {
		if findings := pool.GetScopedString("sys", "last_findings"); findings != "" {
			addSection("Quality Findings", findings)
		}
	}

	result := strings.Join(sections, "\n\n")
	metrics.TokensUsed = estimateTokens(result)

	return result, metrics
}
