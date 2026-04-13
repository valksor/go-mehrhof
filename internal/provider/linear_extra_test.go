package provider

import (
	"testing"

	"github.com/google/go-github/v67/github"
)

func TestIsAllowedLinearAttachmentURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"linear uploads CDN", "https://uploads.linear.app/some/file.png", false},
		{"linear cdn host", "https://cdn.linear.app/file.png", false},
		{"GCS uploads.linear.app prefix", "https://storage.googleapis.com/uploads.linear.app/path", false},
		{"GCS public.linear.app prefix", "https://storage.googleapis.com/public.linear.app/path", false},
		{"GCS imports.linear.app prefix", "https://storage.googleapis.com/imports.linear.app/path", false},
		{"GCS europe-west1 uploads", "https://storage.googleapis.com/linear-uploads-europe-west1/path", false},
		{"GCS europe-west1 imports", "https://storage.googleapis.com/linear-imports-europe-west1/path", false},
		{"disallowed host", "https://evil.com/file.png", true},
		{"disallowed GCS path", "https://storage.googleapis.com/other-bucket/path", true},
		{"empty URL", "", true},
		{"invalid URL", "not a url %%", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isAllowedLinearAttachmentURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("isAllowedLinearAttachmentURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestGitHubProvider_IssueToTask(t *testing.T) {
	gp := NewGitHubProvider("")

	title := "Fix login button"
	body := "The login button is broken on mobile"
	number := 42
	htmlURL := "https://github.com/myorg/myrepo/issues/42"
	state := stateOpen
	labelName := "bug"

	issue := &github.Issue{
		Number:  &number,
		Title:   &title,
		Body:    &body,
		HTMLURL: &htmlURL,
		State:   &state,
		Labels: []*github.Label{
			{Name: &labelName},
		},
	}

	task := gp.issueToTask("myorg", "myrepo", issue)

	if task.ID != "myorg/myrepo#42" {
		t.Errorf("ID = %s, want myorg/myrepo#42", task.ID)
	}
	if task.Title != "Fix login button" {
		t.Errorf("Title = %s, want 'Fix login button'", task.Title)
	}
	if task.Description != body {
		t.Errorf("Description mismatch")
	}
	if task.URL != htmlURL {
		t.Errorf("URL = %s, want %s", task.URL, htmlURL)
	}
	if task.Source != NameGitHub {
		t.Errorf("Source = %s, want github", task.Source)
	}
	if len(task.Labels) != 1 || task.Labels[0] != "bug" {
		t.Errorf("Labels = %v, want [bug]", task.Labels)
	}
	if task.Metadata("github_state") != stateOpen {
		t.Errorf("github_state = %s, want open", task.Metadata("github_state"))
	}
	if task.Metadata("github_owner") != "myorg" {
		t.Errorf("github_owner = %s, want myorg", task.Metadata("github_owner"))
	}
	if task.Metadata("github_repo") != "myrepo" {
		t.Errorf("github_repo = %s, want myrepo", task.Metadata("github_repo"))
	}
}

func TestGitHubProvider_PrToTask(t *testing.T) {
	gp := NewGitHubProvider("")

	title := "Add dark mode"
	body := "Implements dark mode support"
	number := 99
	htmlURL := "https://github.com/myorg/myrepo/pull/99"
	state := stateOpen
	draft := true
	labelName := "enhancement"

	pr := &github.PullRequest{
		Number:  &number,
		Title:   &title,
		Body:    &body,
		HTMLURL: &htmlURL,
		State:   &state,
		Draft:   &draft,
		Labels: []*github.Label{
			{Name: &labelName},
		},
	}

	task := gp.prToTask("myorg", "myrepo", pr)

	if task.ID != "myorg/myrepo#99" {
		t.Errorf("ID = %s, want myorg/myrepo#99", task.ID)
	}
	if task.Title != "Add dark mode" {
		t.Errorf("Title = %s, want 'Add dark mode'", task.Title)
	}
	if task.Source != NameGitHub {
		t.Errorf("Source = %s, want github", task.Source)
	}
	if task.Metadata("github_state") != "draft" {
		t.Errorf("github_state = %s, want draft (draft PR)", task.Metadata("github_state"))
	}
	if task.Metadata("github_is_pr") != "true" {
		t.Errorf("github_is_pr = %s, want true", task.Metadata("github_is_pr"))
	}
	if len(task.Labels) != 1 || task.Labels[0] != "enhancement" {
		t.Errorf("Labels = %v, want [enhancement]", task.Labels)
	}
}

func TestGitHubProvider_PrToTask_NotDraft(t *testing.T) {
	gp := NewGitHubProvider("")

	title := "Normal PR"
	body := ""
	number := 100
	htmlURL := "https://github.com/o/r/pull/100"
	state := stateOpen
	draft := false

	pr := &github.PullRequest{
		Number:  &number,
		Title:   &title,
		Body:    &body,
		HTMLURL: &htmlURL,
		State:   &state,
		Draft:   &draft,
	}

	task := gp.prToTask("o", "r", pr)

	if task.Metadata("github_state") != stateOpen {
		t.Errorf("github_state = %s, want open (non-draft PR)", task.Metadata("github_state"))
	}
}

func TestGitHubProvider_IssueToTask_WithAssignees(t *testing.T) {
	gp := NewGitHubProvider("")

	title := "Task with assignees"
	number := 10
	htmlURL := "https://github.com/o/r/issues/10"
	state := stateOpen
	login1 := "alice"
	login2 := "bob"

	issue := &github.Issue{
		Number:  &number,
		Title:   &title,
		HTMLURL: &htmlURL,
		State:   &state,
		Assignees: []*github.User{
			{Login: &login1},
			{Login: &login2},
		},
	}

	task := gp.issueToTask("o", "r", issue)

	if task.Metadata("github_assignees") != "alice,bob" {
		t.Errorf("github_assignees = %s, want alice,bob", task.Metadata("github_assignees"))
	}
}

func TestNewGitHubProviderWithHost(t *testing.T) {
	gp := NewGitHubProviderWithHost("token", "https://github.example.com")
	if gp == nil {
		t.Fatal("NewGitHubProviderWithHost returned nil")
	}
	if gp.Name() != NameGitHub {
		t.Errorf("Name() = %s, want github", gp.Name())
	}
	if gp.host != "https://github.example.com" {
		t.Errorf("host = %s, want https://github.example.com", gp.host)
	}
}

func TestNewGitLabProviderWithHost(t *testing.T) {
	gp, err := NewGitLabProviderWithHost("token", "https://gitlab.example.com")
	if err != nil {
		t.Fatalf("NewGitLabProviderWithHost() error = %v", err)
	}
	if gp == nil {
		t.Fatal("NewGitLabProviderWithHost returned nil")
	}
	if gp.Name() != "gitlab" {
		t.Errorf("Name() = %s, want gitlab", gp.Name())
	}
}
