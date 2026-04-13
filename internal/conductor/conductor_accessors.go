package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/memory"
	"github.com/valksor/kvelmo/internal/varpool"
	"github.com/valksor/kvelmo/settings"
)

// State returns the current workflow state.
func (c *Conductor) State() State {
	return c.machine.State()
}

// WorkUnit returns the current work unit (alias for GetWorkUnit).
func (c *Conductor) WorkUnit() *WorkUnit {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.workUnit
}

// Repo returns the git repository for the conductor's worktree.
func (c *Conductor) Repo() *git.Repository {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.git
}

// GetWorkUnit returns the current work unit.
func (c *Conductor) GetWorkUnit() *WorkUnit {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.workUnit == nil {
		return nil
	}
	// Return a copy
	wu := *c.workUnit

	return &wu
}

// MarkDirty persists the current work unit state to disk.
// Use after modifying work unit fields like Tags or Priority directly.
func (c *Conductor) MarkDirty() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workUnit != nil {
		c.workUnit.UpdatedAt = time.Now()
	}
	c.persistState()
}

// Machine returns the state machine.
func (c *Conductor) Machine() *Machine {
	return c.machine
}

// TaskTraceID returns the current task's trace ID. Returns empty string if
// no task is loaded.
func (c *Conductor) TaskTraceID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.workUnit == nil {
		return ""
	}

	return c.workUnit.TaskTraceID
}

// SetAutoAdvance enables or disables automatic phase progression.
// When enabled, the conductor automatically advances through phases:
// plan_done → implement, implement_done → review.
func (c *Conductor) SetAutoAdvance(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoAdvance = enabled
}

// AutoAdvance returns whether automatic phase progression is enabled.
func (c *Conductor) AutoAdvance() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.autoAdvance
}

// SetSkipPhases sets per-invocation phases to skip during auto-advance.
// These are merged with config-level SkipPhases.
func (c *Conductor) SetSkipPhases(phases []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runtimeSkipPhases = phases
}

// SetContextItems attaches context references to the current work unit.
// Items are lightweight references resolved at dispatch time, not persisted content.
func (c *Conductor) SetContextItems(items []ContextItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workUnit != nil {
		c.workUnit.ContextItems = items
		c.persistState()

		// Emit event so TUI and web UI see the attached context
		if len(items) > 0 {
			refs := make([]string, 0, len(items))
			for _, item := range items {
				refs = append(refs, fmt.Sprintf("@%s %s", item.Type, item.Ref))
			}
			go c.emit(ConductorEvent{
				Type:    "context_attached",
				Message: "Context attached: " + strings.Join(refs, ", "),
			})
		}
	}
}

// SkipPhases returns the effective skip phases (runtime + config merged).
func (c *Conductor) SkipPhases() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var merged []string
	if s := c.getEffectiveSettings(); s != nil {
		merged = append(merged, s.Workflow.SkipPhases...)
	}
	for _, p := range c.runtimeSkipPhases {
		if !slices.Contains(merged, p) {
			merged = append(merged, p)
		}
	}

	return merged
}

// Suggestions returns workflow suggestions based on historical task patterns.
func (c *Conductor) Suggestions() ([]memory.SkipSuggestion, []memory.AgentSuggestion) {
	if c.store == nil {
		return nil, nil
	}

	tasks, err := c.store.ListArchivedTasks()
	if err != nil {
		slog.Debug("failed to list archived tasks for suggestions", "error", err)

		return nil, nil
	}

	return memory.DetectSkipPatterns(tasks), memory.DetectAgentPatterns(tasks)
}

// getWorkDir returns the effective working directory for operations.
// When worktree isolation is active, returns the isolated worktree path.
// Otherwise returns the main worktree (project root).
func (c *Conductor) getWorkDir() string {
	if c.workUnit != nil && c.workUnit.WorktreePath != "" {
		return c.workUnit.WorktreePath
	}

	return c.worktree
}

// getBaseBranch returns the base branch from settings or git detection.
// Returns error if neither is available (no silent fallback).
// This method is lock-free to allow calling from methods that already hold c.mu.
func (c *Conductor) getBaseBranch(ctx context.Context) (string, error) {
	// 1. Check settings override
	if settings := c.getEffectiveSettings(); settings != nil && settings.Git.BaseBranch != "" {
		return settings.Git.BaseBranch, nil
	}

	// 2. Auto-detect from git
	if c.git != nil {
		return c.git.DefaultBranch(ctx)
	}

	return "", errors.New("cannot determine base branch: git not available and git.base_branch not configured")
}

// GetBaseBranch returns the base branch from settings or git detection.
func (c *Conductor) GetBaseBranch(ctx context.Context) (string, error) {
	return c.getBaseBranch(ctx)
}

// GetEffectiveSettings returns the effective (merged) settings.
func (c *Conductor) GetEffectiveSettings() *settings.Settings {
	return c.getEffectiveSettings()
}

// getEffectiveSettings returns cached settings, loading them on first access.
// Settings are cached to avoid repeated file I/O across phases.
// This method is lock-free to allow calling from methods that already hold c.mu.
func (c *Conductor) getEffectiveSettings() *settings.Settings {
	// Fast path: return cached settings (lock-free)
	if cached := c.cachedSettings.Load(); cached != nil {
		return cached
	}

	// Slow path: load settings (only happens if ReloadSettings() was called)
	effectiveSettings, _, _, err := settings.LoadEffective(c.worktree)
	if err != nil {
		// Non-fatal: fall back to defaults when settings cannot be loaded.
		effectiveSettings = settings.DefaultSettings()
		c.logVerbosef("Warning: could not load settings: %v — using defaults", err)
	}

	// Compare-and-swap to avoid race with concurrent reload
	c.cachedSettings.CompareAndSwap(nil, effectiveSettings)

	return c.cachedSettings.Load()
}

// ReloadSettings clears the cached settings, forcing a reload on next access.
// Use this if settings have been changed and need to be refreshed.
func (c *Conductor) ReloadSettings() {
	c.cachedSettings.Store(nil)
}

// EventTypeUserPrompt is emitted when the conductor needs a yes/no answer from the user.
const EventTypeUserPrompt = "user_prompt"

// promptUser emits a user_prompt event and blocks until the socket delivers
// an answer via RespondToPrompt, or ctx is cancelled.
// Must NOT be called while holding c.mu.
func (c *Conductor) promptUser(ctx context.Context, question string) (bool, error) {
	promptID := "prompt-" + uuid.New().String()
	ch := make(chan bool, 1)

	c.mu.Lock()
	c.pendingPrompts[promptID] = ch
	c.mu.Unlock()

	c.emit(ConductorEvent{
		Type:    EventTypeUserPrompt,
		Message: question,
		Data: mustMarshalJSON(map[string]string{
			"prompt_id": promptID,
			"question":  question,
		}),
	})

	select {
	case answer := <-ch:
		return answer, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pendingPrompts, promptID)
		c.mu.Unlock()

		return false, ctx.Err()
	}
}

// PendingPromptIDs returns the IDs of all currently pending user prompts.
// Used by status to surface actionable items to CLI users.
func (c *Conductor) PendingPromptIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	ids := make([]string, 0, len(c.pendingPrompts))
	for id := range c.pendingPrompts {
		ids = append(ids, id)
	}

	return ids
}

// RespondToPrompt delivers an answer to a pending promptUser call.
// Called by the quality.respond socket handler.
func (c *Conductor) RespondToPrompt(promptID string, answer bool) error {
	c.mu.Lock()
	ch, ok := c.pendingPrompts[promptID]
	if ok {
		delete(c.pendingPrompts, promptID)
	}
	c.mu.Unlock()

	if !ok {
		return fmt.Errorf("prompt %q not found or already answered", promptID)
	}

	ch <- answer

	return nil
}

// mustMarshalJSON marshals v to JSON, panicking on error.
// Only for use with known-good data types where marshaling cannot fail.
func mustMarshalJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustMarshalJSON: %v", err))
	}

	return b
}

// phaseToScope maps a phase name to its varpool scope constant.
func phaseToScope(phase string) string {
	switch phase {
	case "plan":
		return varpool.ScopePlan
	case "implement":
		return varpool.ScopeImplement
	case "simplify":
		return varpool.ScopeSimplify
	case "optimize":
		return varpool.ScopeOptimize
	case "review":
		return varpool.ScopeReview
	default:
		return phase
	}
}

// resetPhaseState clears all per-phase transient state before (re-)entering a phase.
// This prevents iteration counts, retry counts, stale quality gate results, and
// scoped varpool data from leaking across phase re-entries.
// Quality gate is always cleared (not just on re-entry) to avoid stale results
// from a previous lifecycle influencing the current one.
// Must be called while holding c.mu.
func (c *Conductor) resetPhaseState(phase string) {
	c.iterationCount[phase] = 0
	c.retryCount[phase] = 0
	if c.workUnit != nil {
		c.workUnit.QualityGatePassed = nil
		c.workUnit.QualityGateError = ""
	}
	c.varPool.ClearScope(phaseToScope(phase))
}

// DryRunEnabled returns whether the conductor is in dry-run mode.
func (c *Conductor) DryRunEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.dryRun
}

// SetDryRun enables or disables dry-run mode.
func (c *Conductor) SetDryRun(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dryRun = v
}

// AutoFixStatus returns the current state of the quality gate auto-fix loop.
type AutoFixStatus struct {
	Active      bool   `json:"active"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	LastError   string `json:"last_error,omitempty"`
}

// GetAutoFixStatus returns the current auto-fix loop state.
func (c *Conductor) GetAutoFixStatus() AutoFixStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := c.getEffectiveSettings()
	maxAttempts := 3
	if s != nil && s.Workflow.AutoFix.MaxAttempts > 0 {
		maxAttempts = s.Workflow.AutoFix.MaxAttempts
	}

	return AutoFixStatus{
		Active:      c.autoFixAttempt > 0,
		Attempt:     c.autoFixAttempt,
		MaxAttempts: maxAttempts,
		LastError:   c.autoFixLastErr,
	}
}

// LastFailureClass returns the classification of the most recent phase failure.
func (c *Conductor) LastFailureClass() FailureClass {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastFailureClass
}

func (c *Conductor) logVerbosef(format string, args ...any) {
	if c.opts.Verbose && c.stdout != nil {
		_, _ = fmt.Fprintf(c.stdout, format+"\n", args...)
	}
}

// Status returns the current status for display.
func (c *Conductor) Status() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := map[string]any{
		"state":    c.machine.State(),
		"worktree": c.worktree,
	}

	if c.workUnit != nil {
		status["task"] = map[string]any{
			"id":          c.workUnit.ID,
			"title":       c.workUnit.Title,
			"branch":      c.workUnit.Branch,
			"checkpoints": len(c.workUnit.Checkpoints),
			"jobs":        len(c.workUnit.Jobs),
		}
	}

	return status
}

// OnEvent registers an event listener (alias for AddListener).
func (c *Conductor) OnEvent(listener EventListener) {
	c.AddListener(listener)
}

// ForceWorkUnit directly sets the work unit on the conductor.
// Intended for use in tests and internal tooling that need to
// set up a known state without going through the full Start flow.
func (c *Conductor) ForceWorkUnit(wu *WorkUnit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workUnit = wu
	c.machine.SetWorkUnit(wu)
}
