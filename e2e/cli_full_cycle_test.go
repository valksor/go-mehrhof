//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/socket"
)

// TestCLIFullCycle tests the complete kvelmo workflow via CLI.
// Uses local Ollama server (llama3.1) and real GitHub API.
//
// Run with:
//
//	KVELMO_E2E_TOKEN=github_pat_... \
//	go test -tags=e2e -v -timeout=30m ./e2e/... -run TestCLIFullCycle
func TestCLIFullCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full cycle test in short mode")
	}

	// 1. Check prerequisites (Ollama check only when using Ollama agent)
	if os.Getenv("KVELMO_E2E_AGENT") == "" || os.Getenv("KVELMO_E2E_AGENT") == "ollama" {
		checkOllamaAvailable(t)
	}
	token, repo := getE2EConfig(t)

	// 2. Build kvelmo binary
	kvelmoPath := buildKvelmo(t)

	// 3. Create test issue via provider
	p := newE2EProvider(token)
	issueNum := createTestIssue(t, p, repo)
	taskID := fmt.Sprintf("%s#%d", repo, issueNum)
	t.Logf("Created test issue #%d", issueNum)

	// Setup cleanup
	var createdBranch string
	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") != "" {
			t.Log("Skipping cleanup (E2E_SKIP_CLEANUP set)")
			return
		}

		ctx := context.Background()

		// Delete branch if created
		if createdBranch != "" {
			_ = p.DeleteBranch(ctx, repo, createdBranch)
			t.Logf("Deleted branch: %s", createdBranch)
		}

		// Close issue
		_ = p.UpdateStatus(ctx, taskID, "closed")
		t.Logf("Closed issue #%d", issueNum)
	})

	// 4. Setup isolated HOME with agent config + work directory
	// Always use isolated HOME to avoid conflicting with real kvelmo sockets.
	// For Claude/Codex, copy auth config from real HOME so CLI agents can authenticate.
	testHome := setupTestHome(t)
	realHome, _ := os.UserHomeDir()
	// Copy Claude auth config if it exists (needed for Claude CLI agent)
	claudeDir := filepath.Join(realHome, ".claude")
	if _, err := os.Stat(claudeDir); err == nil {
		destClaude := filepath.Join(testHome, ".claude")
		if cpErr := copyDir(claudeDir, destClaude); cpErr != nil {
			t.Logf("Warning: could not copy .claude config: %v", cpErr)
		}
	}
	t.Setenv("HOME", testHome)
	parts := strings.SplitN(repo, "/", 2)
	workDir := setupWorkDir(t, token, parts[0], parts[1])

	// 5. Start kvelmo socket in background
	t.Log("Step 1: Starting kvelmo socket in background...")
	stopKvelmo := startKvelmoBackground(t, kvelmoPath, workDir, token, repo, issueNum)
	defer stopKvelmo()

	// Wait for socket to be ready
	waitForSocket(t, workDir)

	// Test infrastructure commands early (before task loads)
	t.Log("Testing infrastructure commands...")
	runKvelmo(t, kvelmoPath, workDir, "config", "show")
	runKvelmo(t, kvelmoPath, workDir, "config", "path")
	runKvelmo(t, kvelmoPath, workDir, "config", "validate")
	runKvelmo(t, kvelmoPath, workDir, "diagnose")
	runKvelmo(t, kvelmoPath, workDir, "strategy")
	runKvelmo(t, kvelmoPath, workDir, "discover")

	// 6. Wait for task to be loaded (start command loads it asynchronously while holding lock)
	// Use waitForState with timeout since the conductor holds a lock during GitHub fetch
	waitForState(t, kvelmoPath, workDir, "loaded", 60*time.Second)
	t.Log("Task loaded successfully")

	// Test status variants and listing commands
	runKvelmo(t, kvelmoPath, workDir, "status")
	runKvelmo(t, kvelmoPath, workDir, "status", "--json")
	runKvelmo(t, kvelmoPath, workDir, "list")
	tryRunKvelmo(t, kvelmoPath, workDir, "hooks")

	// Test provision preview (dry run mode)
	tryRunKvelmo(t, kvelmoPath, workDir, "start", "--provision-preview")

	// Get branch name for cleanup
	statusOutput := runKvelmoCapture(t, kvelmoPath, workDir, "status", "--json")
	var statusResult map[string]any
	if err := json.Unmarshal([]byte(statusOutput), &statusResult); err == nil {
		if branch, ok := statusResult["branch"].(string); ok {
			createdBranch = branch
			t.Logf("Created branch: %s", createdBranch)
		}
	}

	// 7. Run planning phase
	t.Log("Step 2: Running planning phase...")
	runKvelmo(t, kvelmoPath, workDir, "plan")
	waitForState(t, kvelmoPath, workDir, "planned", 10*time.Minute)
	t.Log("Planning completed")

	// Verify checkpoints, spec content, and agent logs
	runKvelmo(t, kvelmoPath, workDir, "checkpoints")
	runKvelmo(t, kvelmoPath, workDir, "show", "spec")
	runKvelmo(t, kvelmoPath, workDir, "eventlog")
	tryRunKvelmo(t, kvelmoPath, workDir, "logs")

	// 8. Run implementation phase
	t.Log("Step 3: Running implementation phase...")
	runKvelmo(t, kvelmoPath, workDir, "implement")
	waitForState(t, kvelmoPath, workDir, "implemented", 10*time.Minute)
	t.Log("Implementation completed")

	// Test context and inspection commands after implementation
	t.Log("Testing context and inspection commands...")
	runKvelmo(t, kvelmoPath, workDir, "git", "status")
	runKvelmo(t, kvelmoPath, workDir, "git", "log")
	runKvelmo(t, kvelmoPath, workDir, "files", "list", ".")
	runKvelmo(t, kvelmoPath, workDir, "diff")
	runKvelmo(t, kvelmoPath, workDir, "jobs", "list")
	runKvelmo(t, kvelmoPath, workDir, "workers", "list")
	runKvelmo(t, kvelmoPath, workDir, "workers", "stats")
	runKvelmo(t, kvelmoPath, workDir, "agent")
	runKvelmo(t, kvelmoPath, workDir, "recap")
	runKvelmo(t, kvelmoPath, workDir, "activity")
	runKvelmo(t, kvelmoPath, workDir, "eventlog")
	tryRunKvelmo(t, kvelmoPath, workDir, "stats")
	tryRunKvelmo(t, kvelmoPath, workDir, "memory", "stats")
	tryRunKvelmo(t, kvelmoPath, workDir, "policy")
	tryRunKvelmo(t, kvelmoPath, workDir, "audit")
	tryRunKvelmo(t, kvelmoPath, workDir, "security", "scan")
	tryRunKvelmo(t, kvelmoPath, workDir, "codegraph", "stats")

	// Test new feature commands (cache, quality diagnostics)
	t.Log("Testing new feature diagnostic commands...")
	tryRunKvelmo(t, kvelmoPath, workDir, "cache", "stats")
	tryRunKvelmo(t, kvelmoPath, workDir, "quality", "autofix-status")
	tryRunKvelmo(t, kvelmoPath, workDir, "quality", "failclass")

	// Test task organization commands
	t.Log("Testing task organization...")
	runKvelmo(t, kvelmoPath, workDir, "tag", "add", "e2e-test")
	runKvelmo(t, kvelmoPath, workDir, "tag", "list")
	runKvelmo(t, kvelmoPath, workDir, "checklist", "list")

	// 9. Run simplify pass (optional but test it).
	// Timeout doubled from the original 5m to 10m — the prior value was
	// marginal and flaked when Claude took longer than median on the
	// implemented task.
	t.Log("Step 4: Running simplify pass...")
	runKvelmo(t, kvelmoPath, workDir, "simplify")
	waitForState(t, kvelmoPath, workDir, "implemented", 10*time.Minute)
	t.Log("Simplify completed")

	// 10. Run optimize pass (optional but test it). Same timeout rationale.
	t.Log("Step 5: Running optimize pass...")
	runKvelmo(t, kvelmoPath, workDir, "optimize")
	waitForState(t, kvelmoPath, workDir, "implemented", 10*time.Minute)
	t.Log("Optimize completed")

	// 11. Test undo/redo functionality (skip if less than 2 checkpoints - need previous state to undo to)
	t.Log("Step 6: Testing undo/redo and checkpoint navigation...")
	checkpointsOutput := runKvelmoCapture(t, kvelmoPath, workDir, "checkpoints")
	// Count checkpoint lines (they start with "  " or "* " followed by a number)
	checkpointCount := strings.Count(checkpointsOutput, ". ")
	if strings.Contains(checkpointsOutput, "No checkpoints") || checkpointCount < 2 {
		t.Logf("Skipping undo/redo - need at least 2 checkpoints, got %d", checkpointCount)
	} else {
		runKvelmo(t, kvelmoPath, workDir, "undo")
		runKvelmo(t, kvelmoPath, workDir, "redo")
		t.Log("Undo/redo completed")
	}

	// 12. Run quality check explicitly before review
	t.Log("Step 7: Running quality check...")
	runKvelmo(t, kvelmoPath, workDir, "quality")

	// 12b. Test fork operations (create, list, compare)
	t.Log("Testing fork operations...")
	tryRunKvelmo(t, kvelmoPath, workDir, "fork", "create", "test-fork-a")
	tryRunKvelmo(t, kvelmoPath, workDir, "fork", "list")
	tryRunKvelmo(t, kvelmoPath, workDir, "fork", "list", "--json")
	tryRunKvelmo(t, kvelmoPath, workDir, "fork", "compare")

	// 13. Review, export, and submit
	t.Log("Step 8: Reviewing and submitting...")
	runKvelmo(t, kvelmoPath, workDir, "review", "--approve")
	runKvelmo(t, kvelmoPath, workDir, "show", "review")
	runKvelmo(t, kvelmoPath, workDir, "checklist", "list")

	// Test review subcommands (risk evaluation, adversarial results)
	t.Log("Testing review diagnostic commands...")
	tryRunKvelmo(t, kvelmoPath, workDir, "review", "risk")
	tryRunKvelmo(t, kvelmoPath, workDir, "review", "risk-history")
	tryRunKvelmo(t, kvelmoPath, workDir, "review", "adversarial-results")

	runKvelmo(t, kvelmoPath, workDir, "export", "--format", "json")
	tryRunKvelmo(t, kvelmoPath, workDir, "submit", "--dry-run")
	runKvelmo(t, kvelmoPath, workDir, "submit")

	submitState := getKvelmoState(t, kvelmoPath, workDir)
	if submitState != "submitted" {
		t.Fatalf("Expected state 'submitted', got '%s'", submitState)
	}
	t.Log("PR submitted")

	// 14. Approve and merge via remote commands
	// Note: Self-approval is not allowed on GitHub, so we skip approval and merge directly.
	// In a real workflow, approval would come from a different user/token.
	t.Log("Step 9: Merging PR (skipping approval - GitHub doesn't allow self-approval)...")
	runKvelmo(t, kvelmoPath, workDir, "remote", "merge", "--method", "squash")
	t.Log("PR merged")

	// 15. Refresh and finish
	t.Log("Step 10: Refreshing and finishing...")
	runKvelmo(t, kvelmoPath, workDir, "refresh")
	runKvelmo(t, kvelmoPath, workDir, "finish")

	// 16. Test backup before final verification
	t.Log("Step 11: Testing backup...")
	backupPath := filepath.Join(workDir, "e2e-backup.tar.gz")
	tryRunKvelmo(t, kvelmoPath, workDir, "backup", backupPath)

	// 17. Verify final state
	finalState := getKvelmoState(t, kvelmoPath, workDir)
	if finalState != "none" {
		t.Fatalf("Expected final state 'none', got '%s'", finalState)
	}

	// Post-workflow: test commands that work without an active task
	t.Log("Step 12: Testing post-workflow commands...")
	runKvelmo(t, kvelmoPath, workDir, "status")
	runKvelmo(t, kvelmoPath, workDir, "list")
	tryRunKvelmo(t, kvelmoPath, workDir, "cleanup")
	tryRunKvelmo(t, kvelmoPath, workDir, "projects")

	// Test task group operations (global socket, no active task needed)
	t.Log("Testing task group operations...")
	tryRunKvelmo(t, kvelmoPath, workDir, "group", "create", "e2e-test-group")
	tryRunKvelmo(t, kvelmoPath, workDir, "group", "list")

	// Test memory outcomes (global socket)
	tryRunKvelmo(t, kvelmoPath, workDir, "memory", "outcomes")

	// Test cache cleanup
	tryRunKvelmo(t, kvelmoPath, workDir, "cache", "clear")

	t.Log("Workflow completed successfully!")

	// Branch was deleted by finish, clear for cleanup
	createdBranch = ""
}

// TestCLIPipe tests the pipe command (one-shot agent, no server required).
func TestCLIPipe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping pipe test in short mode")
	}

	e2eAgent := os.Getenv("KVELMO_E2E_AGENT")
	if e2eAgent == "" {
		e2eAgent = "ollama"
	}
	if e2eAgent == "ollama" {
		checkOllamaAvailable(t)
	}

	kvelmoPath := buildKvelmo(t)
	workDir := t.TempDir()

	// Initialize a git repo so pipe has context
	runGitCmd(t, workDir, "init")
	runGitCmd(t, workDir, "config", "user.email", "test@e2e.local")
	runGitCmd(t, workDir, "config", "user.name", "E2E Test")
	if err := os.WriteFile(filepath.Join(workDir, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, workDir, "add", "-A")
	runGitCmd(t, workDir, "commit", "-m", "init")

	// Pipe sends a one-shot prompt to the agent
	output := runKvelmoCapture(t, kvelmoPath, workDir, "pipe", "What files are in this directory? Just list them.")
	if output == "" {
		t.Fatal("pipe returned empty output")
	}
	t.Logf("Pipe output: %s", output[:min(len(output), 200)])
}

// checkOllamaAvailable verifies Ollama server is reachable.
func checkOllamaAvailable(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:11434/api/tags", nil)
	if err != nil {
		t.Skipf("Ollama not available: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("Ollama server not reachable at localhost:11434: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Availability check

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Ollama server returned status %d", resp.StatusCode)
	}
}

// getE2EConfig gets configuration from environment.
func getE2EConfig(t *testing.T) (token, repo string) {
	t.Helper()

	repo = os.Getenv("E2E_GITHUB_REPO")
	if repo == "" {
		repo = "ozo2003/e2e-test" // Default test repo; override with E2E_GITHUB_REPO
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		t.Fatalf("E2E_GITHUB_REPO must be in owner/repo format, got: %s", repo)
	}

	token = os.Getenv("E2E_KVELMO_TOKEN")
	if token == "" {
		t.Skip("E2E_KVELMO_TOKEN not set (set a GitHub PAT with repo scope)")
	}

	return token, repo
}

// newE2EProvider creates a GitHub provider for E2E tests.
func newE2EProvider(token string) *provider.GitHubProvider {
	return provider.NewGitHubProvider(token)
}

// buildKvelmo compiles the kvelmo binary and returns the path.
func buildKvelmo(t *testing.T) string {
	t.Helper()

	// Build to temp directory
	tmpDir, err := os.MkdirTemp("", "kvelmo-e2e-bin-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}

	binaryPath := filepath.Join(tmpDir, "kvelmo")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/kvelmo")
	cmd.Dir = findProjectRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build kvelmo: %v\n%s", err, output)
	}

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return binaryPath
}

// findProjectRoot finds the kvelmo project root.
func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from current directory and walk up to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (go.mod)")
		}
		dir = parent
	}
}

// createTestIssue creates a realistic test issue with multiple files and code to simplify/optimize.
// Uses a unique timestamp suffix to avoid conflicts between test runs.
func createTestIssue(t *testing.T, p *provider.GitHubProvider, repo string) int {
	t.Helper()

	timestamp := time.Now().Unix()
	title := fmt.Sprintf("Build bookmark manager API (%d)", timestamp)
	body := fmt.Sprintf(`## Description

Build a bookmark manager HTTP API in Go. (Run ID: %d)

The API stores bookmarks in memory and serves a single-page HTML dashboard for browsing them.

A bookmark has a URL, title, optional tags, and timestamps for when it was created and last updated.

## Requirements

The API should support:
- Listing all bookmarks as JSON
- Adding a new bookmark (URL, title, tags)
- Getting a single bookmark by ID
- Updating a bookmark
- Deleting a bookmark
- Searching bookmarks by title or URL substring (case-insensitive)
- Serving an HTML dashboard at the root that lists bookmarks, has a form to add new ones, and a search input

The HTML dashboard should be embedded in the binary using Go embed. It should show bookmarks in a table with clickable titles, tags, and creation dates. Include basic styling (centered layout, simple table, form).

## Acceptance Criteria

- [ ] API endpoints work for all CRUD operations plus search
- [ ] HTML dashboard renders with bookmark list, add form, and search
- [ ] Tests verify every endpoint works and edge cases return proper errors
- [ ] `+"`go build ./...`"+` passes
- [ ] `+"`go test ./...`"+` passes`, timestamp)

	task, err := p.CreateTask(context.Background(), provider.CreateTaskOptions{
		Title:       title,
		Description: body,
		Team:        repo,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Extract issue number from task ID (format: "owner/repo#number")
	parts := strings.SplitN(task.ID, "#", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected task ID format: %s", task.ID)
	}
	var num int
	fmt.Sscanf(parts[1], "%d", &num)
	return num
}

// setupTestHome creates an isolated HOME directory with global kvelmo config for Ollama.
// Returns the path. kvelmo's serve.go reads global config from ~/.valksor/kvelmo/kvelmo.yaml,
// so we need the agent config at the global level (not just project level).
func setupTestHome(t *testing.T) string {
	t.Helper()

	homeDir, err := os.MkdirTemp("", "kvelmo-e2e-home-*")
	if err != nil {
		t.Fatalf("MkdirTemp home: %v", err)
	}

	t.Cleanup(func() {
		if os.Getenv("E2E_SKIP_CLEANUP") == "" {
			os.RemoveAll(homeDir)
		}
	})

	// Create global kvelmo config directory
	configDir := filepath.Join(homeDir, ".valksor", "kvelmo")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("Create global config dir: %v", err)
	}

	// Agent is configurable via KVELMO_E2E_AGENT (default: ollama)
	agentName := os.Getenv("KVELMO_E2E_AGENT")
	if agentName == "" {
		agentName = "ollama"
	}
	t.Logf("Using agent: %s", agentName)

	globalConfig := fmt.Sprintf(`agent:
  default: %s
  ollama:
    model: llama3.1
`, agentName)
	if err := os.WriteFile(filepath.Join(configDir, "kvelmo.yaml"), []byte(globalConfig), 0o644); err != nil {
		t.Fatalf("Write global kvelmo.yaml: %v", err)
	}

	return homeDir
}

// setupWorkDir clones the repo to a temp directory.
func setupWorkDir(t *testing.T, token, owner, repo string) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "kvelmo-e2e-work-*")
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

	// Clone the repo
	repoURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", token, owner, repo)
	cmd := exec.Command("git", "clone", repoURL, tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v (check repo access and token permissions)", err)
	} else {
		t.Log("Clone completed successfully")
		_ = output
	}

	// Configure git user for commits
	runGitCmd(t, tmpDir, "config", "user.email", "test@e2e.local")
	runGitCmd(t, tmpDir, "config", "user.name", "E2E Test")

	// Create .valksor directory with config
	valksorDir := filepath.Join(tmpDir, ".valksor")
	if err := os.MkdirAll(valksorDir, 0o755); err != nil {
		t.Fatalf("Create .valksor dir: %v", err)
	}

	// Write .env with the GitHub token (kvelmo loads tokens from .env files)
	envContent := fmt.Sprintf("GITHUB_TOKEN=%s\n", token)
	if err := os.WriteFile(filepath.Join(valksorDir, ".env"), []byte(envContent), 0o600); err != nil {
		t.Fatalf("Write .env file: %v", err)
	}

	// Write kvelmo.yaml: isolate state, configure agent, disable interactive prompts
	e2eAgent := os.Getenv("KVELMO_E2E_AGENT")
	if e2eAgent == "" {
		e2eAgent = "ollama"
	}
	settingsContent := fmt.Sprintf(`storage:
  save_in_project: true
agent:
  default: %s
  ollama:
    model: llama3.1
workflow:
  external_review:
    mode: never
`, e2eAgent)
	if err := os.WriteFile(filepath.Join(valksorDir, "kvelmo.yaml"), []byte(settingsContent), 0o644); err != nil {
		t.Fatalf("Write kvelmo.yaml: %v", err)
	}

	return tmpDir
}

// startKvelmoBackground starts kvelmo in foreground mode as a subprocess and returns a stop function.
func startKvelmoBackground(t *testing.T, kvelmoPath, workDir, token, repo string, issueNum int) func() {
	t.Helper()

	// Start kvelmo with the --from flag to load the task
	cmd := exec.Command(kvelmoPath, "start", "--foreground", "--from", fmt.Sprintf("github:%s#%d", repo, issueNum))
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+token)

	// Create a pipe to capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("StderrPipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start kvelmo: %v", err)
	}
	t.Logf("Started kvelmo (PID: %d)", cmd.Process.Pid)

	// Channel to signal goroutines to stop logging
	stopLogging := make(chan struct{})
	var logWg sync.WaitGroup

	// Read output in background for debugging
	logWg.Add(2)
	go func() {
		defer logWg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				select {
				case <-stopLogging:
					return
				default:
					t.Logf("kvelmo stdout: %s", buf[:n])
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer logWg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				select {
				case <-stopLogging:
					return
				default:
					t.Logf("kvelmo stderr: %s", buf[:n])
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Return stop function
	return func() {
		t.Log("Stopping kvelmo...")

		// Signal logging goroutines to stop
		close(stopLogging)

		if cmd.Process != nil {
			// Send SIGTERM for graceful shutdown
			_ = cmd.Process.Signal(syscall.SIGTERM)

			// Wait with timeout
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()

			select {
			case <-done:
				t.Log("kvelmo stopped gracefully")
			case <-time.After(5 * time.Second):
				t.Log("kvelmo didn't stop, killing...")
				_ = cmd.Process.Kill()
			}
		}

		// Wait for logging goroutines to finish
		logWg.Wait()
	}
}

// waitForSocket waits for the kvelmo socket to be ready.
func waitForSocket(t *testing.T, workDir string) {
	t.Helper()

	socketPath := socket.WorktreeSocketPath(workDir)
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		if socket.SocketExists(socketPath) {
			t.Logf("Socket ready: %s", socketPath)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("Timeout waiting for socket: %s", socketPath)
}

// runGitCmd runs a git command.
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

// tryRunKvelmo runs kvelmo but only logs on failure (for commands that may legitimately fail
// when there's no data, e.g., logs before chat, memory before indexing).
func tryRunKvelmo(t *testing.T, kvelmoPath, workDir string, args ...string) {
	t.Helper()

	cmd := exec.Command(kvelmoPath, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Logf("Running (best-effort): kvelmo %s", strings.Join(args, " "))

	if err := cmd.Run(); err != nil {
		t.Logf("kvelmo %v returned error (non-fatal): %v — %s", args, err, stderr.String())

		return
	}

	if stdout.Len() > 0 {
		t.Logf("stdout: %s", stdout.String())
	}
}

// runKvelmo runs kvelmo and checks for errors.
func runKvelmo(t *testing.T, kvelmoPath, workDir string, args ...string) {
	t.Helper()

	cmd := exec.Command(kvelmoPath, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Logf("Running: kvelmo %s", strings.Join(args, " "))

	if err := cmd.Run(); err != nil {
		t.Fatalf("kvelmo %v: %v\nstdout: %s\nstderr: %s",
			args, err, stdout.String(), stderr.String())
	}

	if stdout.Len() > 0 {
		t.Logf("stdout: %s", stdout.String())
	}
}

// runKvelmoCapture runs kvelmo and returns stdout.
func runKvelmoCapture(t *testing.T, kvelmoPath, workDir string, args ...string) string {
	t.Helper()

	cmd := exec.Command(kvelmoPath, args...)
	cmd.Dir = workDir

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("kvelmo %v: %v\nstderr: %s", args, err, exitErr.Stderr)
		}
		t.Fatalf("kvelmo %v: %v", args, err)
	}

	return string(output)
}

// getKvelmoState returns the current kvelmo state.
func getKvelmoState(t *testing.T, kvelmoPath, workDir string) string {
	t.Helper()

	output := runKvelmoCapture(t, kvelmoPath, workDir, "status", "--json")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Parse status JSON: %v\nOutput: %s", err, output)
	}

	state, ok := result["state"].(string)
	if !ok {
		t.Fatalf("No state in status output: %v", result)
	}

	return state
}

// tryGetKvelmoState tries to get the state, returning empty string on error.
func tryGetKvelmoState(kvelmoPath, workDir string) string {
	cmd := exec.Command(kvelmoPath, "status", "--json", "--timeout", "10s")
	cmd.Dir = workDir

	output, err := cmd.Output()
	if err != nil {
		return "" // Return empty on error, caller will retry
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		return ""
	}

	state, _ := result["state"].(string)
	return state
}

// waitForState polls until the expected state is reached.
func waitForState(t *testing.T, kvelmoPath, workDir string, expected string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		// Use tryGetKvelmoState to handle temporary timeouts during task loading
		state := tryGetKvelmoState(kvelmoPath, workDir)
		if state == expected {
			return
		}

		// Check for failure states
		if state == "failed" {
			t.Fatalf("Task entered failed state while waiting for %s", expected)
		}

		// Empty state means error/timeout, just retry
		time.Sleep(pollInterval)
	}

	t.Fatalf("Timeout waiting for state %s (current: %s)", expected, getKvelmoState(t, kvelmoPath, workDir))
}

// copyDir recursively copies a directory, preserving file permissions.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode().Perm())
	})
}
