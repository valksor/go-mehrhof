//go:build e2e

package conductor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v67/github"
	"github.com/valksor/kvelmo/pkg/agent"
	"github.com/valksor/kvelmo/pkg/agent/claude"
	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/storage"
	"github.com/valksor/kvelmo/pkg/worker"
	"golang.org/x/oauth2"
)

// E2E workflow tests for conductor with real Claude agent.
// Run with: go test -tags=e2e -v ./pkg/conductor/... -run TestE2E
//
// Required environment variables:
//   E2E_GITHUB_REPO  - Repository in "owner/repo" format (e.g., "ozo2003/e2e-test")
//   GITHUB_TOKEN     - Personal access token with repo scope
//   ANTHROPIC_API_KEY - API key for Claude (optional, uses Claude CLI if not set)
//
// These tests use REAL Claude agents and will incur API costs.
// Run times can be 1-5 minutes per test depending on task complexity.

func getE2EWorkflowConfig(t *testing.T) (owner, repo, token string) {
	t.Helper()

	repoFull := os.Getenv("E2E_GITHUB_REPO")
	if repoFull == "" {
		t.Skip("E2E_GITHUB_REPO not set")
	}

	parts := strings.SplitN(repoFull, "/", 2)
	if len(parts) != 2 {
		t.Fatalf("E2E_GITHUB_REPO must be in owner/repo format, got: %s", repoFull)
	}
	owner, repo = parts[0], parts[1]

	token = os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("E2E_GITHUB_TOKEN")
	}
	if token == "" {
		t.Skip("GITHUB_TOKEN or E2E_GITHUB_TOKEN not set")
	}

	return owner, repo, token
}

// newE2EGitHubClient creates a GitHub client for E2E tests.
func newE2EGitHubClient(token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(httpClient)
}

// setupE2EWorkDir creates a temporary directory with a cloned repo for E2E tests.
func setupE2EWorkDir(t *testing.T, owner, repo, token string) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "kvelmo-e2e-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}

	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" {
			os.RemoveAll(tmpDir)
		} else {
			t.Logf("Keeping temp dir: %s", tmpDir)
		}
	})

	// Clone the repo (capture output to avoid token exposure in logs)
	repoURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", token, owner, repo)
	cmd := exec.Command("git", "clone", repoURL, tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Don't log output - it may contain the token in error messages
		t.Fatalf("git clone failed: %v (check repo access and token permissions)", err)
	} else {
		t.Logf("Clone completed successfully")
		_ = output // Suppress unused warning
	}

	// Configure git user for commits
	runGitCmd(t, tmpDir, "config", "user.email", "test@e2e.local")
	runGitCmd(t, tmpDir, "config", "user.name", "E2E Test")

	return tmpDir
}

func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// checkClaudeAvailable verifies Claude CLI is installed.
func checkClaudeAvailable(t *testing.T) {
	t.Helper()
	claudeAgent := claude.New()
	if err := claudeAgent.Available(); err != nil {
		t.Skipf("Claude CLI not available: %v", err)
	}
}

// setupWorkerPool creates a worker pool with Claude agent for E2E tests.
func setupWorkerPool(t *testing.T, workDir string) *worker.Pool {
	t.Helper()

	// Create agent registry with Claude
	registry := agent.NewRegistry()
	claudeAgent := claude.New()

	// Configure Claude with the work directory (use safe type assertion)
	configured := claudeAgent.WithWorkDir(workDir)
	typedAgent, ok := configured.(*claude.Agent)
	if !ok {
		t.Fatalf("WithWorkDir returned unexpected type: %T", configured)
	}

	if err := registry.Register(typedAgent); err != nil {
		t.Fatalf("Register Claude: %v", err)
	}

	// Create worker pool
	pool := worker.NewPool(worker.PoolConfig{
		MaxWorkers: 1,
		Agents:     registry,
	})

	if err := pool.Start(); err != nil {
		t.Fatalf("Start pool: %v", err)
	}

	// Register cleanup immediately after Start() so it runs even if test skips
	t.Cleanup(func() {
		pool.Stop()
	})

	// Add a worker with Claude - use AddAgentWorker to actually connect the agent
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	w, err := pool.AddAgentWorker(ctx, "claude", false)
	if err != nil {
		// Claude CLI may fail to connect in certain environments
		// (e.g., when running inside Claude Code itself)
		t.Skipf("Could not connect Claude agent (may be running in nested Claude session): %v", err)
	}
	t.Logf("Worker created: %s (agent: %s, connected: %v)", w.ID, w.AgentName, w.Agent != nil && w.Agent.Connected())

	return pool
}

// waitForState waits for conductor to reach the expected state.
func waitForState(t *testing.T, c *Conductor, expected State, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.State() == expected {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("Timeout waiting for state %s (current: %s)", expected, c.State())
}

func TestE2E_LoadFromGitHub(t *testing.T) {
	owner, repo, token := getE2EWorkflowConfig(t)
	ctx := context.Background()

	// Create GitHub client for test setup
	client := newE2EGitHubClient(token)

	title := fmt.Sprintf("E2E Load Test %d", time.Now().Unix())
	body := "## Description\n\nSimple test task for E2E loading.\n\n## Acceptance Criteria\n\n- [ ] Task loads successfully"

	issue, _, err := client.Issues.Create(ctx, owner, repo, &github.IssueRequest{
		Title: &title,
		Body:  &body,
	})
	if err != nil {
		t.Fatalf("Create issue: %v", err)
	}
	issueNum := issue.GetNumber()
	t.Logf("Created issue #%d", issueNum)

	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" {
			state := "closed"
			_, _, _ = client.Issues.Edit(ctx, owner, repo, issueNum, &github.IssueRequest{State: &state})
			t.Logf("Closed issue #%d", issueNum)
		}
	})

	// Setup work directory
	workDir := setupE2EWorkDir(t, owner, repo, token)

	// Create conductor with settings
	effectiveSettings := &settings.Settings{
		Providers: settings.ProviderSettings{
			GitHub: settings.GitHubConfig{
				Token: token,
			},
		},
	}

	conductor, err := New(
		WithWorkDir(workDir),
		WithSettings(effectiveSettings),
	)
	if err != nil {
		t.Fatalf("New conductor: %v", err)
	}
	defer conductor.Close()

	if err := conductor.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Load the task from GitHub
	taskRef := fmt.Sprintf("github:%s/%s#%d", owner, repo, issueNum)
	err = conductor.Start(ctx, taskRef)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Verify state transitioned to loaded
	if conductor.State() != StateLoaded {
		t.Errorf("State = %v, want %v", conductor.State(), StateLoaded)
	}

	// Verify work unit
	wu := conductor.GetWorkUnit()
	if wu == nil {
		t.Fatal("WorkUnit is nil")
	}

	if wu.Title != title {
		t.Errorf("Title = %q, want %q", wu.Title, title)
	}
	if wu.Source.Provider != "github" {
		t.Errorf("Provider = %q, want github", wu.Source.Provider)
	}
	if wu.Branch == "" {
		t.Error("Branch should be set")
	}

	t.Logf("Loaded task: %s on branch %s", wu.ID, wu.Branch)

	// Cleanup: delete branch if created
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" && wu.Branch != "" {
			_, _ = client.Git.DeleteRef(ctx, owner, repo, "refs/heads/"+wu.Branch)
			t.Logf("Deleted branch: %s", wu.Branch)
		}
	})
}

func TestE2E_FullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full workflow test in short mode")
	}

	owner, repo, token := getE2EWorkflowConfig(t)
	ctx := context.Background()

	// Check Claude is available
	checkClaudeAvailable(t)

	// Create GitHub client
	client := newE2EGitHubClient(token)

	// Create a simple task that Claude can complete quickly
	title := fmt.Sprintf("E2E Full Workflow Test %d", time.Now().Unix())
	body := `## Description

Create a simple "Hello World" text file.

## Acceptance Criteria

- [ ] Create a file called hello.txt
- [ ] The file should contain "Hello, World!"

## Implementation Notes

This is a minimal task for E2E testing. Just create the file with the specified content.`

	issue, _, err := client.Issues.Create(ctx, owner, repo, &github.IssueRequest{
		Title: &title,
		Body:  &body,
	})
	if err != nil {
		t.Fatalf("Create issue: %v", err)
	}
	issueNum := issue.GetNumber()
	t.Logf("Created issue #%d: %s", issueNum, issue.GetHTMLURL())

	// Setup work directory
	workDir := setupE2EWorkDir(t, owner, repo, token)

	// Setup worker pool with Claude
	pool := setupWorkerPool(t, workDir)

	// Create conductor
	effectiveSettings := &settings.Settings{
		Providers: settings.ProviderSettings{
			GitHub: settings.GitHubConfig{
				Token: token,
			},
		},
	}

	conductor, err := New(
		WithWorkDir(workDir),
		WithSettings(effectiveSettings),
		WithPool(pool),
	)
	if err != nil {
		t.Fatalf("New conductor: %v", err)
	}
	defer conductor.Close()

	if err := conductor.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Setup storage for specifications
	store := storage.NewStore(workDir, true) // Save in project
	conductor.SetStore(store)

	// Step 1: Load task from GitHub
	t.Log("Step 1: Loading task from GitHub...")
	taskRef := fmt.Sprintf("github:%s/%s#%d", owner, repo, issueNum)
	if err := conductor.Start(ctx, taskRef); err != nil {
		t.Fatalf("Start: %v", err)
	}

	wu := conductor.GetWorkUnit()
	t.Logf("Loaded task on branch: %s", wu.Branch)

	// Cleanup branch and issue
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" {
			// Close issue
			state := "closed"
			_, _, _ = client.Issues.Edit(ctx, owner, repo, issueNum, &github.IssueRequest{State: &state})
			t.Logf("Closed issue #%d", issueNum)

			// Delete branch
			if wu.Branch != "" {
				_, _ = client.Git.DeleteRef(ctx, owner, repo, "refs/heads/"+wu.Branch)
				t.Logf("Deleted branch: %s", wu.Branch)
			}
		}
	})

	// Step 2: Planning phase
	t.Log("Step 2: Running planning phase with Claude...")
	jobID, err := conductor.Plan(ctx, false)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	t.Logf("Planning job started: %s", jobID)

	// Wait for planning to complete (timeout 3 minutes)
	waitForState(t, conductor, StatePlanned, 3*time.Minute)
	t.Log("Planning completed")

	// Verify specification was created
	wu = conductor.GetWorkUnit()
	if len(wu.Specifications) == 0 {
		t.Error("No specifications created during planning")
	} else {
		t.Logf("Specifications: %v", wu.Specifications)
	}

	// Step 3: Implementation phase
	t.Log("Step 3: Running implementation phase with Claude...")
	jobID, err = conductor.Implement(ctx, false)
	if err != nil {
		t.Fatalf("Implement: %v", err)
	}
	t.Logf("Implementation job started: %s", jobID)

	// Wait for implementation to complete (timeout 5 minutes)
	waitForState(t, conductor, StateImplemented, 5*time.Minute)
	t.Log("Implementation completed")

	// Verify hello.txt was created (or at least some changes were made)
	helloPath := filepath.Join(workDir, "hello.txt")
	if _, err := os.Stat(helloPath); os.IsNotExist(err) {
		// It might be named differently, check git status
		status := runGitCmd(t, workDir, "status", "--porcelain")
		t.Logf("Git status after implementation:\n%s", status)
		// At minimum, verify some files were created/modified
		if status == "" {
			t.Error("No changes made during implementation phase")
		}
	} else {
		content, err := os.ReadFile(helloPath)
		if err != nil {
			t.Errorf("Failed to read hello.txt: %v", err)
		} else {
			t.Logf("hello.txt content: %s", string(content))
		}
	}

	// Step 4: Push branch for PR
	t.Log("Step 4: Pushing branch...")
	runGitCmd(t, workDir, "push", "-u", "origin", wu.Branch)

	// Step 5: Review and Submit PR
	t.Log("Step 5: Reviewing and submitting PR...")
	if err := conductor.Review(ctx, false); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if err := conductor.Submit(ctx, false); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Verify PR was created
	wu = conductor.GetWorkUnit()
	t.Logf("Workflow completed! State: %s", conductor.State())

	if conductor.State() != StateSubmitted {
		t.Errorf("Final state = %v, want %v", conductor.State(), StateSubmitted)
	}

	// Cleanup: close any PRs created
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" {
			// List and close PRs for this branch
			prs, _, _ := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
				Head: fmt.Sprintf("%s:%s", owner, wu.Branch),
			})
			for _, pr := range prs {
				state := "closed"
				_, _, _ = client.PullRequests.Edit(ctx, owner, repo, pr.GetNumber(), &github.PullRequest{State: &state})
				t.Logf("Closed PR #%d", pr.GetNumber())
			}
		}
	})
}

func TestE2E_GitOperations(t *testing.T) {
	owner, repo, token := getE2EWorkflowConfig(t)
	ctx := context.Background()

	// Setup work directory
	workDir := setupE2EWorkDir(t, owner, repo, token)

	// Open git repo
	gitRepo, err := git.Open(workDir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}

	// Test getting default branch
	defaultBranch, err := gitRepo.DefaultBranch(ctx)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	t.Logf("Default branch: %s", defaultBranch)

	if defaultBranch == "" {
		t.Error("DefaultBranch should not be empty")
	}

	// Test creating a branch
	branchName := fmt.Sprintf("e2e-git-test-%d", time.Now().Unix())
	if err := gitRepo.CreateBranch(ctx, branchName, ""); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	t.Logf("Created branch: %s", branchName)

	// Test getting current branch
	currentBranch, err := gitRepo.CurrentBranch(ctx)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if currentBranch != branchName {
		t.Errorf("CurrentBranch = %q, want %q", currentBranch, branchName)
	}

	// Test commit
	testFile := filepath.Join(workDir, "e2e-git-test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGitCmd(t, workDir, "add", "e2e-git-test.txt")

	sha, err := gitRepo.Commit(ctx, "E2E test commit")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	t.Logf("Created commit: %s", sha)

	if sha == "" {
		t.Error("Commit SHA should not be empty")
	}
}

func TestE2E_PlanOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping planning test in short mode")
	}

	owner, repo, token := getE2EWorkflowConfig(t)
	ctx := context.Background()

	// Check Claude is available
	checkClaudeAvailable(t)

	// Create GitHub client
	client := newE2EGitHubClient(token)

	title := fmt.Sprintf("E2E Plan Test %d", time.Now().Unix())
	body := `## Description

Add a README.md file with project description.

## Acceptance Criteria

- [ ] Create README.md
- [ ] Include project title and description`

	issue, _, err := client.Issues.Create(ctx, owner, repo, &github.IssueRequest{
		Title: &title,
		Body:  &body,
	})
	if err != nil {
		t.Fatalf("Create issue: %v", err)
	}
	issueNum := issue.GetNumber()
	t.Logf("Created issue #%d", issueNum)

	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" {
			state := "closed"
			_, _, _ = client.Issues.Edit(ctx, owner, repo, issueNum, &github.IssueRequest{State: &state})
		}
	})

	// Setup
	workDir := setupE2EWorkDir(t, owner, repo, token)
	pool := setupWorkerPool(t, workDir)

	effectiveSettings := &settings.Settings{
		Providers: settings.ProviderSettings{
			GitHub: settings.GitHubConfig{
				Token: token,
			},
		},
	}

	conductor, err := New(
		WithWorkDir(workDir),
		WithSettings(effectiveSettings),
		WithPool(pool),
	)
	if err != nil {
		t.Fatalf("New conductor: %v", err)
	}
	defer conductor.Close()

	if err := conductor.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	store := storage.NewStore(workDir, true)
	conductor.SetStore(store)

	// Load task
	taskRef := fmt.Sprintf("github:%s/%s#%d", owner, repo, issueNum)
	if err := conductor.Start(ctx, taskRef); err != nil {
		t.Fatalf("Start: %v", err)
	}

	wu := conductor.GetWorkUnit()
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" && wu.Branch != "" {
			_, _ = client.Git.DeleteRef(ctx, owner, repo, "refs/heads/"+wu.Branch)
		}
	})

	// Run planning
	t.Log("Running planning with Claude...")
	jobID, err := conductor.Plan(ctx, false)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	t.Logf("Job ID: %s", jobID)

	// Wait for completion
	waitForState(t, conductor, StatePlanned, 3*time.Minute)

	// Verify
	wu = conductor.GetWorkUnit()
	if len(wu.Specifications) == 0 {
		t.Error("No specifications created")
	}
	t.Logf("Planning completed with %d specifications", len(wu.Specifications))
}
