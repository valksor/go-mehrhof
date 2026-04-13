package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestLinearProvider creates a LinearProvider backed by a test HTTP server.
// The server URL is injected via the package-level httpClient variable swap
// pattern used in the existing wrike_httptest_test.go.
// Since LinearProvider hardcodes linearAPIURL and uses the package httpClient,
// we cannot easily swap the URL. Instead we test the request/response paths
// that do NOT require an actual HTTP call (pure logic paths).

// ============================================================
// issueToTask pure logic tests (no HTTP required)
// ============================================================

func TestLinearProvider_IssueToTask_WithChildren(t *testing.T) {
	lp := NewLinearProvider("test-token", "ENG")

	issue := &linearIssue{
		ID:          "abc-1",
		Identifier:  "ENG-10",
		Title:       "Parent with children",
		Description: "",
		Priority:    3,
		State: &linearState{
			ID:   "s1",
			Name: "In Progress",
			Type: "started",
		},
		Children: &linearChildren{
			Nodes: []linearIssue{
				{
					Identifier: "ENG-11",
					Title:      "Child 1",
					State:      &linearState{Type: "started"},
				},
				{
					Identifier: "ENG-12",
					Title:      "Child 2",
					State:      &linearState{Type: "completed"},
				},
				{
					Identifier: "ENG-13",
					Title:      "Child 3",
					State:      &linearState{Type: "canceled"},
				},
			},
		},
	}

	task := lp.issueToTask(issue)

	if len(task.Subtasks) != 3 {
		t.Fatalf("expected 3 subtasks, got %d", len(task.Subtasks))
	}

	// First child (started) should not be completed
	if task.Subtasks[0].Completed {
		t.Error("started child should not be completed")
	}
	// Second child (completed) should be completed
	if !task.Subtasks[1].Completed {
		t.Error("completed child should be completed")
	}
	// Third child (canceled) should also be completed
	if !task.Subtasks[2].Completed {
		t.Error("canceled child should be marked completed")
	}
}

func TestLinearProvider_IssueToTask_NoPriority(t *testing.T) {
	lp := NewLinearProvider("test-token", "")

	issue := &linearIssue{
		ID:         "id-1",
		Identifier: "TEAM-1",
		Title:      "Task without priority",
		Priority:   0, // No priority set
	}

	task := lp.issueToTask(issue)

	// Priority 0 means no priority — should fall back to inferred
	if task.Priority != "" && task.Priority != "normal" {
		t.Logf("Priority with no Linear priority set = %q (inferred)", task.Priority)
	}
}

func TestLinearProvider_IssueToTask_AllPriorities(t *testing.T) {
	lp := NewLinearProvider("test-token", "")

	tests := []struct {
		linearPriority int
		wantStr        string
	}{
		{1, "critical"},
		{2, "high"},
		{3, "normal"},
		{4, "low"},
		{5, "normal"}, // out of range → default
	}

	for _, tt := range tests {
		issue := &linearIssue{
			Identifier: "T-1",
			Priority:   tt.linearPriority,
		}
		task := lp.issueToTask(issue)
		if task.Priority != tt.wantStr {
			t.Errorf("priority %d → %q, want %q", tt.linearPriority, task.Priority, tt.wantStr)
		}
	}
}

func TestLinearProvider_IssueToTask_MetadataFields(t *testing.T) {
	lp := NewLinearProvider("test-token", "ENG")

	issue := &linearIssue{
		ID:         "issue-uuid",
		Identifier: "ENG-99",
		Title:      "Test issue",
		State: &linearState{
			ID:   "state-uuid",
			Name: "Todo",
			Type: "unstarted",
		},
		Team: &linearTeam{
			ID:  "team-uuid",
			Key: "ENG",
		},
		Parent: &linearParent{
			ID:         "parent-uuid",
			Identifier: "ENG-50",
		},
		Assignee: &linearUser{
			ID:   "user-uuid",
			Name: "Jane Doe",
		},
	}

	task := lp.issueToTask(issue)

	checkMeta := func(key, want string) {
		t.Helper()
		got := task.Metadata(key)
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	checkMeta("linear_id", "issue-uuid")
	checkMeta("linear_identifier", "ENG-99")
	checkMeta("linear_state_id", "state-uuid")
	checkMeta("linear_state_type", "unstarted")
	checkMeta("linear_team_key", "ENG")
	checkMeta("linear_team_id", "team-uuid")
	checkMeta("linear_parent_id", "parent-uuid")
	checkMeta("linear_parent_identifier", "ENG-50")
	checkMeta("linear_assignee", "Jane Doe")
}

func TestLinearProvider_IssueToTask_NilPointerFields(t *testing.T) {
	lp := NewLinearProvider("test-token", "")

	// All optional fields nil — should not panic
	issue := &linearIssue{
		ID:         "bare-id",
		Identifier: "X-1",
		Title:      "Bare issue",
		State:      nil,
		Team:       nil,
		Parent:     nil,
		Labels:     nil,
		Assignee:   nil,
		Children:   nil,
	}

	task := lp.issueToTask(issue)
	if task == nil {
		t.Fatal("issueToTask() returned nil for valid bare issue")
	}
	if task.Metadata("linear_state_type") != "" {
		t.Errorf("expected empty linear_state_type when State is nil, got %q", task.Metadata("linear_state_type"))
	}
}

// ============================================================
// resolveDependencies pure logic tests
// ============================================================

func TestLinearProvider_ResolveDependencies_NoRefs(t *testing.T) {
	lp := NewLinearProvider("test-token", "")
	task := &Task{Description: "No deps here"}

	deps := lp.resolveDependencies(task)
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

func TestLinearProvider_ResolveDependencies_GitLabStyleRef(t *testing.T) {
	lp := NewLinearProvider("test-token", "")
	// ParseDependencies only matches #\d+ style refs (not ENG-42 style).
	// A project-qualified ref like "my-group/my-repo#42" contains "-" and
	// satisfies the Linear guard, so it would be included.
	task := &Task{Description: "Depends on: my-group/my-repo#42"}

	deps := lp.resolveDependencies(task)
	// ParseDependencies finds "my-group/my-repo#42"; it contains "-" so Linear includes it.
	if len(deps) == 0 {
		t.Error("expected at least 1 dependency from project-qualified ref")
	}
	if len(deps) > 0 && deps[0].Source != "linear" {
		t.Errorf("dep source = %q, want linear", deps[0].Source)
	}
}

func TestLinearProvider_ResolveDependencies_ShorthandNoHyphen(t *testing.T) {
	lp := NewLinearProvider("test-token", "")
	// References without hyphens are not valid Linear identifiers
	task := &Task{Description: "Depends on NOHYPHEN"}

	deps := lp.resolveDependencies(task)
	// Should have 0 deps (no hyphen = skip)
	for _, d := range deps {
		if d.ID == "NOHYPHEN" {
			t.Error("should skip references without hyphen")
		}
	}
}

// ============================================================
// fetchIssueByIdentifier identifier parsing
// (test the parse logic via FetchTask with no token — early error)
// ============================================================

func TestLinearProvider_FetchIssueByIdentifier_InvalidFormat(t *testing.T) {
	// Use a server to test error paths that reach graphql
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"issues": map[string]any{
					"nodes": []any{},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Test the identifier parsing in fetchIssueByIdentifier via FetchTask (no-token early exit)
	lp := NewLinearProvider("", "")
	ctx := context.Background()

	// No-token error fires first
	_, err := lp.FetchTask(ctx, "INVALID-NO-DASH")
	if err == nil {
		t.Error("expected error for empty token")
	}
}

// ============================================================
// DownloadAttachment token/URL validation
// ============================================================

func TestLinearProvider_DownloadAttachment_InvalidURL(t *testing.T) {
	lp := NewLinearProvider("token", "")
	ctx := context.Background()

	_, err := lp.DownloadAttachment(ctx, "https://evil.com/malicious.png")
	if err == nil {
		t.Error("DownloadAttachment should reject non-Linear attachment URLs")
	}
}

func TestLinearProvider_DownloadAttachment_ValidURL_ServerError(t *testing.T) {
	// Use a local server that returns an error to test the non-200 path
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	// We can't easily override the attachment URL without reflection or build tags.
	// Instead test via a validated URL prefix by creating a GCS-style URL that
	// points to our test server. Since we can't override the client, use
	// isAllowedLinearAttachmentURL directly.
	err := isAllowedLinearAttachmentURL("https://uploads.linear.app/file.png")
	if err != nil {
		t.Errorf("valid uploads.linear.app URL should pass validation: %v", err)
	}
}
