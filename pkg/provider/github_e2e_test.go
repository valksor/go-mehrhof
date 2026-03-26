//go:build e2e

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v67/github"
)

// E2E tests for GitHub provider.
// Run with: go test -tags=e2e -v ./pkg/provider/... -run TestE2E
//
// Required environment variables:
//   E2E_KVELMO_TOKEN - Personal access token with repo scope
//
// Optional:
//   E2E_GITHUB_REPO  - Repository in "owner/repo" format (default: "ozo2003/e2e-test")

const defaultE2ERepo = "ozo2003/e2e-test"

func getE2EConfig(t *testing.T) (owner, repo, token string) {
	t.Helper()

	repoFull := os.Getenv("E2E_GITHUB_REPO")
	if repoFull == "" {
		repoFull = defaultE2ERepo
	}

	parts := strings.SplitN(repoFull, "/", 2)
	if len(parts) != 2 {
		t.Fatalf("E2E_GITHUB_REPO must be in owner/repo format, got: %s", repoFull)
	}
	owner, repo = parts[0], parts[1]

	token = os.Getenv("E2E_KVELMO_TOKEN")
	if token == "" {
		t.Skip("E2E_KVELMO_TOKEN not set")
	}

	return owner, repo, token
}

// e2eRepoID returns the "owner/repo" string for use with provider methods.
func e2eRepoID(owner, repo string) string {
	return owner + "/" + repo
}

// e2eTaskID returns the "owner/repo#number" string for use with provider methods.
func e2eTaskID(owner, repo string, number int) string {
	return fmt.Sprintf("%s/%s#%d", owner, repo, number)
}

func TestE2E_CreateAndCloseIssue(t *testing.T) {
	owner, repo, token := getE2EConfig(t)
	ctx := context.Background()

	p := NewGitHubProvider(token)

	// Create issue via provider
	title := fmt.Sprintf("E2E Test Issue %d", time.Now().Unix())
	body := "This is an automated test issue.\n\nIt will be closed automatically."
	task, err := p.CreateTask(ctx, CreateTaskOptions{
		Title:       title,
		Description: body,
		Team:        e2eRepoID(owner, repo),
		Labels:      []string{"test", "automated"},
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	t.Logf("Created issue: %s", task.URL)

	// Cleanup: close issue
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") != "" {
			return
		}
		_ = p.UpdateStatus(ctx, task.ID, "closed")
		t.Logf("Closed issue %s", task.ID)
	})

	// Verify created task fields
	if task.Title != title {
		t.Errorf("Title = %q, want %q", task.Title, title)
	}
	if task.Description != body {
		t.Errorf("Description mismatch")
	}
	if len(task.Labels) != 2 {
		t.Errorf("Labels = %v, want 2 labels", task.Labels)
	}

	// Fetch via provider and verify
	fetched, err := p.FetchTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("FetchTask: %v", err)
	}
	if fetched.Title != title {
		t.Errorf("FetchTask Title = %q, want %q", fetched.Title, title)
	}

	// Close via provider
	err = p.UpdateStatus(ctx, task.ID, "closed")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	// Verify closed
	closed, err := p.FetchTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("FetchTask after close: %v", err)
	}
	if closed.Metadata("github_state") != "closed" {
		t.Errorf("State = %q, want closed", closed.Metadata("github_state"))
	}
}

func TestE2E_IssueComments(t *testing.T) {
	owner, repo, token := getE2EConfig(t)
	ctx := context.Background()

	p := NewGitHubProvider(token)

	// Create issue via provider
	task, err := p.CreateTask(ctx, CreateTaskOptions{
		Title: fmt.Sprintf("E2E Comment Test %d", time.Now().Unix()),
		Team:  e2eRepoID(owner, repo),
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") != "" {
			return
		}
		_ = p.UpdateStatus(ctx, task.ID, "closed")
	})

	// Add comment via provider
	commentBody := "This is an automated test comment."
	err = p.AddComment(ctx, task.ID, commentBody)
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	t.Log("Added comment")

	// Fetch comments via provider
	comments, err := p.FetchComments(ctx, task.ID)
	if err != nil {
		t.Fatalf("FetchComments: %v", err)
	}

	if len(comments) < 1 {
		t.Fatalf("Expected at least 1 comment, got %d", len(comments))
	}

	found := false
	for _, c := range comments {
		if c.Body == commentBody {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Comment not found in list")
	}
}

func TestE2E_CreatePR(t *testing.T) {
	owner, repo, token := getE2EConfig(t)
	ctx := context.Background()

	p := NewGitHubProvider(token)
	repoID := e2eRepoID(owner, repo)

	// Use internal client for git-level fixture setup (branch + file creation).
	// These are GitHub API git operations, not provider-level task operations.
	client := newGitHubClient(token, "")

	// Get default branch
	repoInfo, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		t.Fatalf("Get repo: %v", err)
	}
	defaultBranch := repoInfo.GetDefaultBranch()
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	// Get latest commit SHA - handle empty repo by creating initial commit
	ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+defaultBranch)
	if err != nil {
		if resp != nil && resp.StatusCode == 409 {
			t.Log("Repo is empty, creating initial commit")
			readmeContent := []byte("# E2E Test Repository\n\nThis repository is used for automated E2E testing.\n")
			initMsg := "Initial commit"
			_, _, err = client.Repositories.CreateFile(ctx, owner, repo, "README.md", &github.RepositoryContentFileOptions{
				Message: &initMsg,
				Content: readmeContent,
				Branch:  &defaultBranch,
			})
			if err != nil {
				t.Fatalf("CreateFile (initial): %v", err)
			}
			time.Sleep(500 * time.Millisecond)
			ref, _, err = client.Git.GetRef(ctx, owner, repo, "refs/heads/"+defaultBranch)
			if err != nil {
				t.Fatalf("GetRef after init: %v", err)
			}
		} else {
			t.Fatalf("GetRef: %v", err)
		}
	}
	baseSHA := ref.Object.GetSHA()

	// Create a new branch
	branchName := fmt.Sprintf("e2e-test-%d", time.Now().Unix())
	refName := "refs/heads/" + branchName
	_, _, err = client.Git.CreateRef(ctx, owner, repo, &github.Reference{
		Ref:    &refName,
		Object: &github.GitObject{SHA: &baseSHA},
	})
	if err != nil {
		t.Fatalf("CreateRef: %v", err)
	}
	t.Logf("Created branch: %s", branchName)

	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") != "" {
			return
		}
		_ = p.DeleteBranch(ctx, repoID, branchName)
		t.Logf("Deleted branch: %s", branchName)
	})

	// Create a file in the branch
	content := []byte(fmt.Sprintf("# E2E Test\n\nCreated at %s\n", time.Now().Format(time.RFC3339)))
	commitMsg := "E2E test commit"
	_, _, err = client.Repositories.CreateFile(ctx, owner, repo, "e2e-test.md", &github.RepositoryContentFileOptions{
		Message: &commitMsg,
		Content: content,
		Branch:  &branchName,
	})
	if err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	// Create PR using provider
	result, err := p.CreatePR(ctx, PROptions{
		Title:  fmt.Sprintf("E2E Test PR %d", time.Now().Unix()),
		Body:   "Automated test PR",
		Head:   fmt.Sprintf("%s:%s", repoID, branchName),
		Base:   defaultBranch,
		Draft:  true,
		TaskID: fmt.Sprintf("%s#0", repoID),
	})
	if err != nil {
		t.Fatalf("CreatePR: %v", err)
	}

	t.Logf("Created PR #%d: %s", result.Number, result.URL)

	if result.Number <= 0 {
		t.Errorf("PR number = %d, want > 0", result.Number)
	}
	if result.State != "draft" && result.State != "open" {
		t.Errorf("PR state = %q, want draft or open", result.State)
	}

	// Cleanup: close the PR via provider
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") != "" {
			return
		}
		_ = p.UpdateStatus(ctx, result.ID, "closed")
		t.Logf("Closed PR #%d", result.Number)
	})
}

func TestE2E_ListIssues(t *testing.T) {
	owner, repo, token := getE2EConfig(t)
	ctx := context.Background()

	p := NewGitHubProvider(token)

	result, err := p.ListTasks(ctx, ListOptions{
		Team:   e2eRepoID(owner, repo),
		Status: "all",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	t.Logf("Found %d issues in repo", len(result.Tasks))
}
