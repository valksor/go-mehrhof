package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/valksor/kvelmo/internal/memory"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/internal/varpool"
)

// ContextType identifies what kind of reference a ContextItem holds.
type ContextType string

const (
	ContextTypeFile     ContextType = "file"     // File path (relative to worktree root)
	ContextTypeSymbol   ContextType = "symbol"   // Code symbol (class, function, method)
	ContextTypeCommit   ContextType = "commit"   // Git commit SHA or ref
	ContextTypeBranch   ContextType = "branch"   // Git branch name
	ContextTypeTerminal ContextType = "terminal" // Terminal output snippet
	ContextTypeURL      ContextType = "url"      // External documentation URL
)

// ContextItem is a lightweight reference to contextual information attached to a task.
// Only Type, Ref, and Label are persisted in WorkUnit. Content is resolved
// ephemerally at dispatch time via BuildPhaseContext and never serialized to task.yaml.
type ContextItem struct {
	Type  ContextType `json:"type" yaml:"type"`
	Ref   string      `json:"ref" yaml:"ref"`     // e.g. "agent/agent.go:42", "abc123", "main"
	Label string      `json:"label" yaml:"label"` // Human-readable display name
}

// PhaseContextProfile defines what categories of information to include for a phase.
type PhaseContextProfile struct {
	IncludeTask         bool // full title + description
	IncludeTaskSummary  bool // title + truncated description (first 500 chars)
	IncludeSpecs        bool // specification files
	IncludeDiff         bool // current git diff
	IncludeFindings     bool // quality gate findings
	IncludeHierarchy    bool // parent/sibling tasks
	IncludeMemory       bool // vector store retrieval for similar past context
	IncludeContextItems bool // user-attached context references (@-mentions)
	MaxTokenBudget      int  // soft cap for context assembly (0 = unlimited)
}

// ContextMetrics tracks what was included/excluded during context assembly.
type ContextMetrics struct {
	TokensUsed       int      `json:"tokens_used"`
	TokenBudget      int      `json:"token_budget"`
	SectionsIncluded []string `json:"sections_included"`
	SectionsExcluded []string `json:"sections_excluded"`
}

// buildContextDeps assembles optional dependencies for context assembly.
// Must be called with c.mu held (at least RLock).
func (c *Conductor) buildContextDeps() contextDeps {
	var deps contextDeps
	if c.workUnit != nil && c.store != nil && len(c.workUnit.Specifications) > 0 {
		specStore := storage.NewSpecStore(c.store)
		if content, err := specStore.GatherSpecificationsContent(c.workUnit.ID); err == nil && content != "" {
			deps.SpecContent = content
		}
	}

	// Attach resolver for context items if we have a worktree
	if c.worktree != "" {
		deps.Resolver = &ContextResolver{
			WorktreeRoot: c.worktree,
			Repo:         c.git,
			// Graph is nil — symbol resolution uses grep fallback.
			// Wire codegraph.Graph here when available on Conductor.
		}
	}

	// Attach memory store for semantic retrieval
	if c.memoryIndexer != nil {
		deps.MemoryStore = c.memoryIndexer.Store()
	}

	return deps
}

// DefaultContextProfiles returns the default context profile for each phase.
func DefaultContextProfiles() map[string]PhaseContextProfile {
	return map[string]PhaseContextProfile{
		PhasePlan: {
			IncludeTask:         true,
			IncludeHierarchy:    true,
			IncludeMemory:       true,
			IncludeContextItems: true,
			MaxTokenBudget:      8000,
		},
		PhaseImplement: {
			IncludeTaskSummary:  true,
			IncludeSpecs:        true,
			IncludeMemory:       true,
			IncludeContextItems: true,
			MaxTokenBudget:      12000,
		},
		PhaseSimplify: {
			IncludeDiff:     true,
			IncludeFindings: true,
			MaxTokenBudget:  4000,
		},
		PhaseOptimize: {
			IncludeDiff:     true,
			IncludeFindings: true,
			MaxTokenBudget:  4000,
		},
		PhaseReview: {
			IncludeTaskSummary: true,
			IncludeSpecs:       true,
			IncludeDiff:        true,
			IncludeFindings:    true,
			MaxTokenBudget:     8000,
		},
	}
}

// contextDeps provides optional dependencies for context assembly.
// Fields are nil-safe — missing deps cause the corresponding section to be skipped.
type contextDeps struct {
	SpecContent string              // Pre-gathered specification content (from storage.GatherSpecificationsContent)
	Resolver    *ContextResolver    // Resolves ContextItems to content (nil = skip context items)
	MemoryStore *memory.VectorStore // Vector store for semantic memory retrieval (nil = skip memory)
}

// BuildPhaseContext assembles context for a phase based on its profile.
// Returns the context string and metrics about what was included.
func BuildPhaseContext(ctx context.Context, profile PhaseContextProfile, wu *WorkUnit, pool *varpool.Pool, deps ...contextDeps) (string, ContextMetrics) {
	var dep contextDeps
	if len(deps) > 0 {
		dep = deps[0]
	}
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
		if dep.SpecContent != "" {
			addSection("Specifications", dep.SpecContent)
		} else {
			addSection("Specifications", fmt.Sprintf("%d specification file(s) available", len(wu.Specifications)))
		}
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
		if diff := pool.GetScopedString(varpool.ScopeSystem, "last_diff"); diff != "" {
			addSection("Current Changes", diff)
		}
	}

	// Findings
	if profile.IncludeFindings && pool != nil {
		if findings := pool.GetScopedString(varpool.ScopeSystem, "last_findings"); findings != "" {
			addSection("Quality Findings", findings)
		}
	}

	// User-attached context items (@-mentions)
	if profile.IncludeContextItems && len(wu.ContextItems) > 0 {
		if dep.Resolver != nil {
			resolved, errs := dep.Resolver.ResolveAll(ctx, wu.ContextItems)
			for _, err := range errs {
				slog.Warn("context item resolution failed", "error", err)
			}
			for _, rc := range resolved {
				addSection("Context: "+rc.Label, rc.Content)
			}
		} else {
			slog.Warn("context items attached but no resolver available — items will not be included in prompt",
				"count", len(wu.ContextItems))
		}
	}

	// Memory retrieval from vector store
	if profile.IncludeMemory && dep.MemoryStore == nil {
		slog.Debug("memory retrieval requested but no memory store available")
	} else if profile.IncludeMemory && dep.MemoryStore != nil {
		query := fmt.Sprintf("%s %s", wu.Title, wu.Description)
		if runes := []rune(query); len(runes) > 512 {
			query = string(runes[:512])
		}
		results, err := dep.MemoryStore.Search(ctx, query, memory.SearchOptions{
			Limit:    5,
			MinScore: 0.70,
			DocumentTypes: []memory.DocumentType{
				memory.TypeSpecification,
				memory.TypeSolution,
				memory.TypeDecision,
			},
		})
		if err != nil {
			slog.Warn("memory search failed", "error", err)
		} else if len(results) > 0 {
			var sb strings.Builder
			for _, r := range results {
				content := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(r.Document.Content)
				if runes := []rune(content); len(runes) > 500 {
					content = string(runes[:500]) + "..."
				}
				fmt.Fprintf(&sb, "- [%s] (%.0f%% match): %s\n", r.Document.Type, r.Score*100, content)
			}
			addSection("Related Memory", sb.String())
		}
	}

	result := strings.Join(sections, "\n\n")
	metrics.TokensUsed = estimateTokens(result)

	return result, metrics
}
