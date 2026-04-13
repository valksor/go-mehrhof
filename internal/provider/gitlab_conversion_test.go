package provider

import (
	"context"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// ============================================================
// GitLabProvider.issueToTask tests
// ============================================================

func TestGitLabProvider_IssueToTask_BasicFields(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	issue := &gitlab.Issue{
		IID:         42,
		Title:       "Fix login bug",
		Description: "The login button is broken",
		WebURL:      "https://gitlab.com/owner/repo/-/issues/42",
		State:       "opened",
		Labels:      gitlab.Labels{"bug", "priority"},
	}

	task := gp.issueToTask("owner/repo", issue)

	if task.ID != "owner/repo#42" {
		t.Errorf("ID = %q, want %q", task.ID, "owner/repo#42")
	}
	if task.Title != "Fix login bug" {
		t.Errorf("Title = %q, want %q", task.Title, "Fix login bug")
	}
	if task.Description != "The login button is broken" {
		t.Errorf("Description mismatch")
	}
	if task.URL != "https://gitlab.com/owner/repo/-/issues/42" {
		t.Errorf("URL mismatch")
	}
	if task.Source != NameGitLab {
		t.Errorf("Source = %q, want gitlab", task.Source)
	}
	if len(task.Labels) != 2 {
		t.Errorf("Labels count = %d, want 2", len(task.Labels))
	}
	if task.Metadata("gitlab_state") != "opened" {
		t.Errorf("gitlab_state = %q, want opened", task.Metadata("gitlab_state"))
	}
	if task.Metadata("gitlab_project") != "owner/repo" {
		t.Errorf("gitlab_project = %q, want owner/repo", task.Metadata("gitlab_project"))
	}
}

func TestGitLabProvider_IssueToTask_WithAssignees(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	issue := &gitlab.Issue{
		IID:   10,
		Title: "Task with assignees",
		State: "opened",
		Assignees: []*gitlab.IssueAssignee{
			{Username: "alice"},
			{Username: "bob"},
		},
	}

	task := gp.issueToTask("group/repo", issue)

	assignees := task.Metadata("gitlab_assignees")
	if assignees == "" {
		t.Error("gitlab_assignees should not be empty")
	}
	// Should contain both usernames
	if assignees != "alice,bob" {
		t.Errorf("gitlab_assignees = %q, want alice,bob", assignees)
	}
}

func TestGitLabProvider_IssueToTask_WithMilestone(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	issue := &gitlab.Issue{
		IID:   20,
		Title: "Task with milestone",
		State: "opened",
		Milestone: &gitlab.Milestone{
			ID:    5,
			Title: "v2.0",
		},
	}

	task := gp.issueToTask("group/repo", issue)

	if task.Metadata("gitlab_milestone") != "v2.0" {
		t.Errorf("gitlab_milestone = %q, want v2.0", task.Metadata("gitlab_milestone"))
	}
	if task.Metadata("gitlab_milestone_id") == "" {
		t.Error("gitlab_milestone_id should not be empty")
	}
}

func TestGitLabProvider_IssueToTask_NoLabelsNoAssignees(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	issue := &gitlab.Issue{
		IID:   1,
		Title: "Simple issue",
		State: "opened",
	}

	task := gp.issueToTask("team/project", issue)

	if task == nil {
		t.Fatal("issueToTask returned nil")
	}
	if task.Source != NameGitLab {
		t.Errorf("Source = %q, want gitlab", task.Source)
	}
	if len(task.Labels) != 0 {
		t.Errorf("Labels count = %d, want 0", len(task.Labels))
	}
}

// ============================================================
// GitLabProvider.mrToTask tests
// ============================================================

func TestGitLabProvider_MRToTask_BasicFields(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	mr := &gitlab.MergeRequest{
		BasicMergeRequest: gitlab.BasicMergeRequest{
			IID:         7,
			Title:       "Add dark mode",
			Description: "Implements dark mode support",
			WebURL:      "https://gitlab.com/owner/repo/-/merge_requests/7",
			State:       "opened",
			Labels:      gitlab.Labels{"enhancement"},
		},
	}

	task := gp.mrToTask("owner/repo", mr)

	if task.ID != "owner/repo!7" {
		t.Errorf("ID = %q, want %q", task.ID, "owner/repo!7")
	}
	if task.Title != "Add dark mode" {
		t.Errorf("Title = %q, want Add dark mode", task.Title)
	}
	if task.Source != NameGitLab {
		t.Errorf("Source = %q, want gitlab", task.Source)
	}
	if task.Metadata("gitlab_is_mr") != "true" {
		t.Errorf("gitlab_is_mr = %q, want true", task.Metadata("gitlab_is_mr"))
	}
	if task.Metadata("gitlab_state") != "opened" {
		t.Errorf("gitlab_state = %q, want opened", task.Metadata("gitlab_state"))
	}
}

func TestGitLabProvider_MRToTask_DraftMR(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	mr := &gitlab.MergeRequest{
		BasicMergeRequest: gitlab.BasicMergeRequest{
			IID:   3,
			Title: "Draft: WIP feature",
			State: "opened",
			Draft: true,
		},
	}

	task := gp.mrToTask("group/repo", mr)

	if task.Metadata("gitlab_state") != "draft" {
		t.Errorf("gitlab_state = %q, want draft for draft MR", task.Metadata("gitlab_state"))
	}
}

func TestGitLabProvider_MRToTask_WithAssignees(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	mr := &gitlab.MergeRequest{
		BasicMergeRequest: gitlab.BasicMergeRequest{
			IID:   5,
			Title: "MR with assignees",
			State: "opened",
			Assignees: []*gitlab.BasicUser{
				{Username: "carol"},
			},
		},
	}

	task := gp.mrToTask("g/r", mr)

	if task.Metadata("gitlab_assignees") != "carol" {
		t.Errorf("gitlab_assignees = %q, want carol", task.Metadata("gitlab_assignees"))
	}
}

// ============================================================
// GitLabProvider.resolveDependencies tests
// ============================================================

func TestGitLabProvider_ResolveDependencies_NoReferences(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	task := &Task{
		ID:          "group/repo#1",
		Description: "No references here",
	}
	task.SetMetadata("gitlab_project", "group/repo")

	deps := gp.resolveDependencies(task)
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(deps))
	}
}

func TestGitLabProvider_ResolveDependencies_ShorthandRef(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	task := &Task{
		ID:          "group/repo#5",
		Description: "Depends on: #3",
	}
	task.SetMetadata("gitlab_project", "group/repo")

	deps := gp.resolveDependencies(task)
	if len(deps) == 0 {
		t.Error("expected at least 1 dependency from shorthand ref")
	}
	if len(deps) > 0 && deps[0].Source != NameGitLab {
		t.Errorf("dep source = %q, want gitlab", deps[0].Source)
	}
}

func TestGitLabProvider_ResolveDependencies_FullRef(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	task := &Task{
		ID:          "group/repo#10",
		Description: "Depends on: group/repo#5",
	}
	task.SetMetadata("gitlab_project", "group/repo")

	deps := gp.resolveDependencies(task)
	if len(deps) == 0 {
		t.Error("expected at least 1 dependency from full ref")
	}
}

// ============================================================
// GitLabProvider.UpdateStatus – parse error path
// ============================================================

func TestGitLabProvider_UpdateStatus_InvalidID(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	ctx := context.Background()
	err = gp.UpdateStatus(ctx, "invalid-id-format", "done")
	if err == nil {
		t.Error("UpdateStatus() should return error for invalid ID format")
	}
}

func TestGitLabProvider_UpdateStatus_UnsupportedStatus(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	ctx := context.Background()
	// Valid ID format but unsupported status
	err = gp.UpdateStatus(ctx, "group/repo#1", "invalid-status")
	if err == nil {
		t.Error("UpdateStatus() should return error for unsupported status")
	}
}

// ============================================================
// GitLabProvider.CreatePR – validation paths
// ============================================================

func TestGitLabProvider_CreatePR_NoProject(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	ctx := context.Background()
	// No project determinable from Head or TaskID
	_, err = gp.CreatePR(ctx, PROptions{
		Title: "test",
		Head:  "feature-branch", // no "project:" prefix
		// No TaskID
	})
	if err == nil {
		t.Error("CreatePR() should return error when project cannot be determined")
	}
}

// ============================================================
// GitLabProvider.FetchParent / FetchSiblings always return nil
// ============================================================

func TestGitLabProvider_FetchParent_AlwaysNil(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	ctx := context.Background()
	parent, err := gp.FetchParent(ctx, &Task{ID: "g/r#1"})
	if err != nil {
		t.Fatalf("FetchParent() unexpected error: %v", err)
	}
	if parent != nil {
		t.Errorf("FetchParent() = %v, want nil (GitLab has no native hierarchy)", parent)
	}
}

func TestGitLabProvider_FetchSiblings_AlwaysNil(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	ctx := context.Background()
	siblings, err := gp.FetchSiblings(ctx, &Task{ID: "g/r#1"})
	if err != nil {
		t.Fatalf("FetchSiblings() unexpected error: %v", err)
	}
	if siblings != nil {
		t.Errorf("FetchSiblings() = %v, want nil (GitLab has no native sibling fetching)", siblings)
	}
}
