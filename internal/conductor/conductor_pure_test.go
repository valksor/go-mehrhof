package conductor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/settings"
)

// ─── getStatusFromLabels ──────────────────────────────────────────────────────

func TestGetStatusFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{
			name:   "empty slice returns empty string",
			labels: []string{},
			want:   "",
		},
		{
			name:   "nil slice returns empty string",
			labels: nil,
			want:   "",
		},
		{
			name:   "labels without status prefix return empty string",
			labels: []string{"bug", "urgent"},
			want:   "",
		},
		{
			name:   "label with status prefix returns value",
			labels: []string{"bug", "status:active"},
			want:   "active",
		},
		{
			name:   "status with empty value returns empty string",
			labels: []string{"status:"},
			want:   "",
		},
		{
			name:   "multiple status labels returns first match",
			labels: []string{"status:blocked", "status:active"},
			want:   "blocked",
		},
		{
			name:   "status label mixed with other labels",
			labels: []string{"bug", "priority:high", "status:in-review", "urgent"},
			want:   "in-review",
		},
		{
			name:   "status with hyphenated value",
			labels: []string{"status:in-progress"},
			want:   "in-progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStatusFromLabels(tt.labels)
			if got != tt.want {
				t.Errorf("getStatusFromLabels(%v) = %q, want %q", tt.labels, got, tt.want)
			}
		})
	}
}

// ─── PendingPromptIDs ─────────────────────────────────────────────────────────

func TestPendingPromptIDs_NewConductor(t *testing.T) {
	c, _ := New()
	ids := c.PendingPromptIDs()
	if ids == nil {
		t.Error("PendingPromptIDs() should return non-nil slice for new conductor")
	}
	if len(ids) != 0 {
		t.Errorf("PendingPromptIDs() = %v, want empty slice", ids)
	}
}

func TestPendingPromptIDs_WithEntries(t *testing.T) {
	c, _ := New()

	// Insert entries directly (same-package access)
	c.mu.Lock()
	c.pendingPrompts["prompt-aaa"] = make(chan bool, 1)
	c.pendingPrompts["prompt-bbb"] = make(chan bool, 1)
	c.mu.Unlock()

	ids := c.PendingPromptIDs()
	if len(ids) != 2 {
		t.Errorf("PendingPromptIDs() len = %d, want 2", len(ids))
	}
	// Verify both IDs are present (order is not guaranteed due to map iteration)
	found := make(map[string]bool)
	for _, id := range ids {
		found[id] = true
	}
	if !found["prompt-aaa"] {
		t.Error("PendingPromptIDs() missing prompt-aaa")
	}
	if !found["prompt-bbb"] {
		t.Error("PendingPromptIDs() missing prompt-bbb")
	}
}

// ─── RespondToPrompt ──────────────────────────────────────────────────────────

func TestRespondToPrompt_UnknownID(t *testing.T) {
	c, _ := New()
	err := c.RespondToPrompt("nonexistent-prompt-id", true)
	if err == nil {
		t.Error("RespondToPrompt() with unknown ID should return error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("RespondToPrompt() error = %q, want 'not found'", err.Error())
	}
}

func TestRespondToPrompt_ValidDelivery(t *testing.T) {
	c, _ := New()

	// Insert a pending prompt channel with buffered capacity
	ch := make(chan bool, 1)
	c.mu.Lock()
	c.pendingPrompts["prompt-test-xyz"] = ch
	c.mu.Unlock()

	// Respond with true
	if err := c.RespondToPrompt("prompt-test-xyz", true); err != nil {
		t.Fatalf("RespondToPrompt() unexpected error: %v", err)
	}

	// Channel should have received the answer
	select {
	case answer := <-ch:
		if !answer {
			t.Error("RespondToPrompt() delivered false, want true")
		}
	default:
		t.Error("RespondToPrompt() did not deliver answer to channel")
	}

	// The prompt should have been removed from pendingPrompts
	c.mu.Lock()
	_, stillPending := c.pendingPrompts["prompt-test-xyz"]
	c.mu.Unlock()
	if stillPending {
		t.Error("RespondToPrompt() should remove prompt from pendingPrompts")
	}
}

func TestRespondToPrompt_DeliversFalse(t *testing.T) {
	c, _ := New()

	ch := make(chan bool, 1)
	c.mu.Lock()
	c.pendingPrompts["prompt-false-test"] = ch
	c.mu.Unlock()

	if err := c.RespondToPrompt("prompt-false-test", false); err != nil {
		t.Fatalf("RespondToPrompt() unexpected error: %v", err)
	}

	select {
	case answer := <-ch:
		if answer {
			t.Error("RespondToPrompt() delivered true, want false")
		}
	default:
		t.Error("RespondToPrompt() did not deliver answer to channel")
	}
}

// ─── ReloadSettings ───────────────────────────────────────────────────────────

func TestReloadSettings_ClearsCache(t *testing.T) {
	c, _ := New()

	// Inject known settings so we can detect a reload
	s := settings.DefaultSettings()
	s.Git.BaseBranch = "sentinel-branch"
	c.cachedSettings.Store(s)

	// Verify settings are cached
	if cached := c.cachedSettings.Load(); cached == nil {
		t.Fatal("cachedSettings should not be nil before reload")
	}

	// ReloadSettings clears the cache
	c.ReloadSettings()

	// After reload, cachedSettings is nil (cleared)
	if cached := c.cachedSettings.Load(); cached != nil {
		t.Error("ReloadSettings() should set cachedSettings to nil")
	}
}

func TestReloadSettings_RefetchesOnNextAccess(t *testing.T) {
	c, _ := New()

	// Clear the cache
	c.ReloadSettings()
	if cached := c.cachedSettings.Load(); cached != nil {
		t.Fatal("cachedSettings should be nil after ReloadSettings")
	}

	// getEffectiveSettings should reload from defaults (no settings file in tmp)
	got := c.getEffectiveSettings()
	if got == nil {
		t.Error("getEffectiveSettings() after reload should return non-nil settings")
	}
	// Should be re-cached now
	if cached := c.cachedSettings.Load(); cached == nil {
		t.Error("cachedSettings should be repopulated after getEffectiveSettings()")
	}
}

// ─── TaskHistory ──────────────────────────────────────────────────────────────

func TestTaskHistory_NilStore(t *testing.T) {
	c, _ := New()
	// store is nil by default

	tasks, err := c.TaskHistory()
	if err != nil {
		t.Errorf("TaskHistory() with nil store error = %v, want nil", err)
	}
	if tasks != nil {
		t.Errorf("TaskHistory() with nil store = %v, want nil", tasks)
	}
}

// ─── MarshalQueue ─────────────────────────────────────────────────────────────

func TestMarshalQueue_EmptyQueue(t *testing.T) {
	c, _ := New()

	raw, err := c.MarshalQueue()
	if err != nil {
		t.Fatalf("MarshalQueue() error = %v", err)
	}
	if raw == nil {
		t.Fatal("MarshalQueue() returned nil")
	}

	// Should be valid JSON (empty array)
	var out []any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Errorf("MarshalQueue() produced invalid JSON: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("MarshalQueue() empty queue = %v, want empty array", out)
	}
}

func TestMarshalQueue_WithItems(t *testing.T) {
	c, _ := New()
	_, _ = c.QueueTask("github:owner/repo#1", "First task")
	_, _ = c.QueueTask("github:owner/repo#2", "Second task")

	raw, err := c.MarshalQueue()
	if err != nil {
		t.Fatalf("MarshalQueue() error = %v", err)
	}

	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("MarshalQueue() produced invalid JSON: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("MarshalQueue() len = %d, want 2", len(items))
	}
}

// ─── getBaseBranch ────────────────────────────────────────────────────────────

func TestGetBaseBranch_FromSettings(t *testing.T) {
	s := settings.DefaultSettings()
	s.Git.BaseBranch = "develop"
	c, _ := New(WithSettings(s))

	branch, err := c.getBaseBranch(context.Background())
	if err != nil {
		t.Fatalf("getBaseBranch() error = %v", err)
	}
	if branch != "develop" {
		t.Errorf("getBaseBranch() = %q, want %q", branch, "develop")
	}
}

func TestGetBaseBranch_NoSettingsNoGit(t *testing.T) {
	// New conductor: no git (non-git dir), no settings override → error
	c, _ := New(WithWorkDir(t.TempDir()))
	// git is nil (non-git directory), settings BaseBranch is empty

	_, err := c.getBaseBranch(context.Background())
	if err == nil {
		t.Error("getBaseBranch() with no settings and no git should return error")
	}
}

func TestGetBaseBranch_MainBranch(t *testing.T) {
	s := settings.DefaultSettings()
	s.Git.BaseBranch = "main"
	c, _ := New(WithSettings(s))

	branch, err := c.getBaseBranch(context.Background())
	if err != nil {
		t.Fatalf("getBaseBranch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("getBaseBranch() = %q, want main", branch)
	}
}
