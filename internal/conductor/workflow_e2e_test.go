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

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/claude"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/settings"
)

// E2E workflow tests for conductor with real Claude agent.
// Run with: go test -tags=e2e -v ./internal/conductor/... -run TestE2E
//
// Required environment variables:
//   E2E_KVELMO_TOKEN - Personal access token with repo scope
//
// Optional:
//   E2E_GITHUB_REPO  - Repository in "owner/repo" format (default: "ozo2003/e2e-test")

const defaultE2ERepo = "ozo2003/e2e-test"

func getE2EWorkflowConfig(t *testing.T) (repoID, token string) {
	t.Helper()

	repoID = os.Getenv("E2E_GITHUB_REPO")
	if repoID == "" {
		repoID = defaultE2ERepo
	}

	parts := strings.SplitN(repoID, "/", 2)
	if len(parts) != 2 {
		t.Fatalf("E2E_GITHUB_REPO must be in owner/repo format, got: %s", repoID)
	}

	token = os.Getenv("E2E_KVELMO_TOKEN")
	if token == "" {
		t.Skip("E2E_KVELMO_TOKEN not set")
	}

	return repoID, token
}

// newE2EProvider creates a GitHubProvider for E2E tests.
func newE2EProvider(token string) *provider.GitHubProvider {
	return provider.NewGitHubProvider(token)
}

// setupE2EWorkDir creates a temporary directory with a cloned repo for E2E tests.
func setupE2EWorkDir(t *testing.T, repoID, token string) string {
	t.Helper()

	parts := strings.SplitN(repoID, "/", 2)
	owner, repo := parts[0], parts[1]

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
		_ = output
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

	// Configure Claude with the work directory
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

	t.Cleanup(func() {
		pool.Stop()
	})

	// Add a worker with Claude
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	w, err := pool.AddAgentWorker(ctx, "claude", false)
	if err != nil {
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
	repoID, token := getE2EWorkflowConfig(t)
	ctx := context.Background()

	p := newE2EProvider(token)

	// Create issue via provider
	title := fmt.Sprintf("E2E Load Test %d", time.Now().Unix())
	body := "## Description\n\nSimple test task for E2E loading.\n\n## Acceptance Criteria\n\n- [ ] Task loads successfully"
	task, err := p.CreateTask(ctx, provider.CreateTaskOptions{
		Title:       title,
		Description: body,
		Team:        repoID,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	t.Logf("Created issue: %s", task.ID)

	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" {
			_ = p.UpdateStatus(ctx, task.ID, "closed")
			t.Logf("Closed issue %s", task.ID)
		}
	})

	// Setup work directory
	workDir := setupE2EWorkDir(t, repoID, token)

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
	taskRef := fmt.Sprintf("github:%s", task.ID)
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

	// Cleanup: delete branch
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" && wu.Branch != "" {
			_ = p.DeleteBranch(ctx, repoID, wu.Branch)
			t.Logf("Deleted branch: %s", wu.Branch)
		}
	})
}

func TestE2E_FullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full workflow test in short mode")
	}

	repoID, token := getE2EWorkflowConfig(t)
	ctx := context.Background()

	// Check Claude is available
	checkClaudeAvailable(t)

	p := newE2EProvider(token)

	// Create a simple task via provider
	title := fmt.Sprintf("E2E Full Workflow Test %d", time.Now().Unix())
	body := `## Description

Create a simple "Hello World" text file.

## Acceptance Criteria

- [ ] Create a file called hello.txt
- [ ] The file should contain "Hello, World!"

## Implementation Notes

This is a minimal task for E2E testing. Just create the file with the specified content.`

	task, err := p.CreateTask(ctx, provider.CreateTaskOptions{
		Title:       title,
		Description: body,
		Team:        repoID,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	t.Logf("Created issue: %s (%s)", task.ID, task.URL)

	// Setup work directory
	workDir := setupE2EWorkDir(t, repoID, token)

	// Setup worker pool with Claude
	pool := setupWorkerPool(t, workDir)

	// Create conductor — disable external review to avoid interactive prompts in tests
	effectiveSettings := &settings.Settings{
		Providers: settings.ProviderSettings{
			GitHub: settings.GitHubConfig{
				Token: token,
			},
		},
		Workflow: settings.WorkflowSettings{
			ExternalReview: settings.ExternalReviewConfig{
				Mode: settings.ExternalReviewNever,
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
	store := storage.NewStore(workDir, true)
	conductor.SetStore(store)

	// Step 1: Load task from GitHub
	t.Log("Step 1: Loading task from GitHub...")
	taskRef := fmt.Sprintf("github:%s", task.ID)
	if err := conductor.Start(ctx, taskRef); err != nil {
		t.Fatalf("Start: %v", err)
	}

	wu := conductor.GetWorkUnit()
	t.Logf("Loaded task on branch: %s", wu.Branch)

	// Cleanup branch and issue
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" {
			_ = p.UpdateStatus(ctx, task.ID, "closed")
			t.Logf("Closed issue %s", task.ID)

			if wu.Branch != "" {
				_ = p.DeleteBranch(ctx, repoID, wu.Branch)
				t.Logf("Deleted branch: %s", wu.Branch)
			}
		}
	})

	// Step 2: Planning phase
	t.Log("Step 2: Running planning phase with Claude...")
	jobID, err := conductor.Plan(ctx)
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
	jobID, err = conductor.Implement(ctx)
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
		status := runGitCmd(t, workDir, "status", "--porcelain")
		t.Logf("Git status after implementation:\n%s", status)
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
}

func TestE2E_GitOperations(t *testing.T) {
	repoID, token := getE2EWorkflowConfig(t)
	ctx := context.Background()

	// Setup work directory
	workDir := setupE2EWorkDir(t, repoID, token)

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

	repoID, token := getE2EWorkflowConfig(t)
	ctx := context.Background()

	// Check Claude is available
	checkClaudeAvailable(t)

	p := newE2EProvider(token)

	task, err := p.CreateTask(ctx, provider.CreateTaskOptions{
		Title: fmt.Sprintf("E2E Plan Test %d", time.Now().Unix()),
		Description: `## Description

Add a README.md file with project description.

## Acceptance Criteria

- [ ] Create README.md
- [ ] Include project title and description`,
		Team: repoID,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	t.Logf("Created issue: %s", task.ID)

	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" {
			_ = p.UpdateStatus(ctx, task.ID, "closed")
		}
	})

	// Setup
	workDir := setupE2EWorkDir(t, repoID, token)
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
	taskRef := fmt.Sprintf("github:%s", task.ID)
	if err := conductor.Start(ctx, taskRef); err != nil {
		t.Fatalf("Start: %v", err)
	}

	wu := conductor.GetWorkUnit()
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" && wu.Branch != "" {
			_ = p.DeleteBranch(ctx, repoID, wu.Branch)
		}
	})

	// Run planning
	t.Log("Running planning with Claude...")
	jobID, err := conductor.Plan(ctx)
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
