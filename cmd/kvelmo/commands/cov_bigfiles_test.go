package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent/recorder"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/internal/testutil"
	"github.com/valksor/kvelmo/settings"
)

// writeRecordingFixture writes a minimal JSONL recording file (header + a
// stream event + a complete event) and returns its path. This drives the
// replay agent and the recordings view/replay commands without a live agent.
func writeRecordingFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "rec.jsonl")

	now := time.Now().UTC()
	header := recorder.Header{JobID: "job-1", Agent: "claude", Model: "test", StartedAt: now}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	streamEvent, err := json.Marshal(map[string]any{"content": content})
	if err != nil {
		t.Fatalf("marshal stream event: %v", err)
	}

	records := []recorder.Record{
		{Timestamp: now, JobID: "job-1", Direction: recorder.Outbound, Type: "header", Event: headerJSON},
		{Timestamp: now, JobID: "job-1", Direction: recorder.Outbound, Type: "stream", Event: streamEvent},
		{Timestamp: now, JobID: "job-1", Direction: recorder.Outbound, Type: "complete", Event: json.RawMessage(`{}`)},
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create recording: %v", err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	for _, rec := range records {
		if err := enc.Encode(rec); err != nil {
			t.Fatalf("encode record: %v", err)
		}
	}

	return path
}

// --- pipe with replay agent (no live agent) ---

func TestRunPipe_Replay(t *testing.T) {
	origReplay, origAgent := pipeReplay, pipeAgent
	t.Cleanup(func() { pipeReplay, pipeAgent = origReplay, origAgent })

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	pipeReplay = writeRecordingFixture(t, "hello from replay")
	pipeAgent = ""

	out := captureStdout(t, func() {
		if err := runPipe(PipeCmd, []string{"summarize"}); err != nil {
			t.Errorf("runPipe replay: %v", err)
		}
	})
	if !strings.Contains(out, "hello from replay") {
		t.Errorf("pipe replay output = %q", out)
	}
}

func TestRunPipe_NoPrompt(t *testing.T) {
	origReplay := pipeReplay
	t.Cleanup(func() { pipeReplay = origReplay })
	pipeReplay = ""

	shortKvelmoHome(t)
	chdirToShortTemp(t)

	// With no args and an empty stdin (the test harness stdin), the prompt
	// resolves to empty and the command must report a usage error.
	r, w, _ := os.Pipe()
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin; _ = r.Close() })

	if err := runPipe(PipeCmd, nil); err == nil {
		t.Fatal("expected error when no prompt provided")
	}
}

// --- recordings replay / view ---

func TestRunRecordingsReplay_File(t *testing.T) {
	origFilter := recordingsTypeFilter
	t.Cleanup(func() { recordingsTypeFilter = origFilter })
	recordingsTypeFilter = ""

	path := writeRecordingFixture(t, "stream chunk")

	out := captureStdout(t, func() {
		if err := runRecordingsReplay(recordingsReplayCmd, []string{path}); err != nil {
			t.Errorf("runRecordingsReplay: %v", err)
		}
	})
	if !strings.Contains(out, "record(s)") {
		t.Errorf("replay output:\n%s", out)
	}
}

func TestRunRecordingsReplay_WithFilter(t *testing.T) {
	origFilter := recordingsTypeFilter
	t.Cleanup(func() { recordingsTypeFilter = origFilter })
	recordingsTypeFilter = "stream"

	path := writeRecordingFixture(t, "filtered chunk")

	out := captureStdout(t, func() {
		if err := runRecordingsReplay(recordingsReplayCmd, []string{path}); err != nil {
			t.Errorf("runRecordingsReplay filtered: %v", err)
		}
	})
	if !strings.Contains(out, "stream") {
		t.Errorf("filtered replay output:\n%s", out)
	}
}

func TestRunRecordingsReplay_BadPath(t *testing.T) {
	if err := runRecordingsReplay(recordingsReplayCmd, []string{"/nonexistent/rec.jsonl"}); err == nil {
		t.Fatal("expected error for missing recording file")
	}
}

func TestRunRecordingsClean_Empty(t *testing.T) {
	origDir, origOlder := recordingsDir, recordingsOlderThan
	t.Cleanup(func() { recordingsDir, recordingsOlderThan = origDir, origOlder })
	recordingsDir = t.TempDir()
	recordingsOlderThan = "30d"

	out := captureStdout(t, func() {
		if err := runRecordingsClean(recordingsCleanCmd, nil); err != nil {
			t.Errorf("runRecordingsClean: %v", err)
		}
	})
	if !strings.Contains(out, "No recordings to clean") {
		t.Errorf("clean output = %q", out)
	}
}

func TestRunRecordingsClean_BadDuration(t *testing.T) {
	origOlder := recordingsOlderThan
	t.Cleanup(func() { recordingsOlderThan = origOlder })
	recordingsOlderThan = "not-a-duration"

	if err := runRecordingsClean(recordingsCleanCmd, nil); err == nil {
		t.Fatal("expected error for bad duration")
	}
}

func TestRunRecordingsView_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("recordings.view", map[string]any{
		"header": map[string]any{
			"job_id": "job-1", "agent": "claude", "model": "test", "started_at": "2026-05-19T10:00:00Z",
		},
		"records": []any{
			map[string]any{
				"timestamp": "2026-05-19T10:00:01Z", "direction": "out", "type": "stream",
				"event": map[string]any{"content": "hi"},
			},
		},
	})

	out := captureStdout(t, func() {
		if err := runRecordingsView(recordingsViewCmd, []string{"rec.jsonl"}); err != nil {
			t.Errorf("runRecordingsView: %v", err)
		}
	})
	if !strings.Contains(out, "Recording: rec.jsonl") {
		t.Errorf("recordings view output:\n%s", out)
	}
}

func TestRunRecordingsList_WithSocket(t *testing.T) {
	origJSON := recordingsOutputJSON
	t.Cleanup(func() { recordingsOutputJSON = origJSON })
	recordingsOutputJSON = false

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("recordings.list", map[string]any{
		"recordings": []any{
			map[string]any{"path": "/r/rec.jsonl", "job_id": "job-abc", "agent": "claude", "lines": 5, "started_at": "2026-05-19T10:00:00Z"},
		},
	})

	out := captureStdout(t, func() {
		if err := runRecordingsList(recordingsListCmd, nil); err != nil {
			t.Errorf("runRecordingsList: %v", err)
		}
	})
	if !strings.Contains(out, "job-abc") || !strings.Contains(out, "Total: 1") {
		t.Errorf("recordings list output:\n%s", out)
	}
}

// --- config show/get via socket + offline ---

func TestConfigShowViaSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("settings.get", map[string]any{
		"effective": map[string]any{"workers": map[string]any{"max": 5}},
	})

	data, err := configShowViaSocket()
	if err != nil {
		t.Fatalf("configShowViaSocket: %v", err)
	}
	if !strings.Contains(string(data), "workers") {
		t.Errorf("config show data = %s", data)
	}
}

func TestConfigGetViaSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("settings.get", map[string]any{
		"effective": map[string]any{"workers": map[string]any{"max": 5}},
	})

	val, err := configGetViaSocket("workers.max")
	if err != nil {
		t.Fatalf("configGetViaSocket: %v", err)
	}
	// JSON numbers decode to float64.
	if f, ok := val.(float64); !ok || f != 5 {
		t.Errorf("configGetViaSocket = %v (%T), want 5", val, val)
	}
}

func TestConfigSetViaSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("settings.set", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		if err := configSetCmd.RunE(configSetCmd, []string{"workers.max", "8"}); err != nil {
			t.Errorf("config set: %v", err)
		}
	})
	if !strings.Contains(out, "Set workers.max = 8") {
		t.Errorf("config set output = %q", out)
	}
}

func TestConfigInitViaSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("settings.set", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		if err := configInitCmd.RunE(configInitCmd, nil); err != nil {
			t.Errorf("config init: %v", err)
		}
	})
	if !strings.Contains(out, "Configuration initialized") {
		t.Errorf("config init output = %q", out)
	}
}

func TestRunConfigEdit_NoEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")

	if err := runConfigEdit(configEditCmd, nil); err == nil {
		t.Fatal("expected error when $EDITOR not set")
	}
}

func TestRunConfigEdit_WithEditor(t *testing.T) {
	// Use `true` as a no-op editor so the command runs end-to-end.
	t.Setenv("EDITOR", "true")
	shortKvelmoHome(t)

	configEditCmd.SetContext(t.Context())
	if err := runConfigEdit(configEditCmd, nil); err != nil {
		t.Errorf("runConfigEdit with EDITOR=true: %v", err)
	}
}

func TestRunConfigDiff_NoProject(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	out := captureStdout(t, func() {
		_ = runConfigDiff(configDiffCmd, nil)
	})
	if !strings.Contains(out, "No project config") {
		t.Errorf("config diff output = %q", out)
	}
}

// --- provider login ---

func TestReadToken_NonInteractive(t *testing.T) {
	// Pipe a token via a non-terminal stdin to exercise the bufio path.
	r, w, _ := os.Pipe()
	_, _ = w.WriteString("ghp_secrettoken\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin; _ = r.Close() })

	out := captureStdout(t, func() {
		tok, err := readToken("Enter token: ")
		if err != nil {
			t.Errorf("readToken: %v", err)
		}
		if tok != "ghp_secrettoken" {
			t.Errorf("readToken = %q, want ghp_secrettoken", tok)
		}
	})
	if !strings.Contains(out, "Enter token:") {
		t.Errorf("readToken prompt not printed: %q", out)
	}
}

func TestDetectExistingToken(t *testing.T) {
	home := shortKvelmoHome(t)
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("GITHUB_TOKEN=ghp_abc123def456\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ts := detectExistingToken("GITHUB_TOKEN", settings.ScopeGlobal, "")
	if ts == nil {
		t.Fatal("expected to detect existing token")
	}
	if ts.Value == "" {
		t.Error("expected masked token value")
	}
}

func TestRunProviderLogin_NonInteractive(t *testing.T) {
	home := shortKvelmoHome(t)

	// Pipe a token in via non-terminal stdin.
	r, w, _ := os.Pipe()
	_, _ = w.WriteString("ghp_freshtoken12345\n")
	_ = w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin; _ = r.Close() })

	loginCmd := findProviderLogin(GitHubCmd)
	if loginCmd == nil {
		t.Fatal("github login subcommand missing")
	}

	// Point token validation at a stub returning 200 so the command's validation
	// step does not make a real network call to GitHub.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"login":"octocat"}`))
	}))
	t.Cleanup(srv.Close)
	withProviderURL(t, "github", srv.URL)

	_ = captureStdout(t, func() {
		if err := runProviderLogin("github")(loginCmd, nil); err != nil {
			t.Errorf("runProviderLogin: %v", err)
		}
	})

	// Token should have been saved to the global .env.
	data, err := os.ReadFile(filepath.Join(home, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if !strings.Contains(string(data), "GITHUB_TOKEN") {
		t.Errorf(".env missing GITHUB_TOKEN: %s", data)
	}
}

// --- changelog standalone (real git repo) ---

func TestRunChangelog_Standalone(t *testing.T) {
	origJSON := changelogJSON
	t.Cleanup(func() { changelogJSON = origJSON })
	changelogJSON = false

	shortKvelmoHome(t)
	repo := t.TempDir()
	testutil.InitGitRepo(t, repo)
	t.Chdir(repo)

	// No worktree socket → standalone path. With identical refs there are no
	// commits between them, so the command prints the "no commits" message and
	// returns before invoking any agent.
	out := captureStdout(t, func() {
		if err := runChangelog(ChangelogCmd, []string{"HEAD", "HEAD"}); err != nil {
			t.Errorf("runChangelog: %v", err)
		}
	})
	if !strings.Contains(out, "No commits between") {
		t.Errorf("changelog standalone output = %q", out)
	}
}

func TestRunChangelog_StandaloneJSONUnsupported(t *testing.T) {
	origJSON := changelogJSON
	t.Cleanup(func() { changelogJSON = origJSON })
	changelogJSON = true

	shortKvelmoHome(t)
	repo := t.TempDir()
	testutil.InitGitRepo(t, repo)
	t.Chdir(repo)

	err := runChangelog(ChangelogCmd, []string{"HEAD", "HEAD"})
	if err == nil {
		t.Fatal("expected error: --json unsupported in standalone mode")
	}
	if !strings.Contains(err.Error(), "standalone") {
		t.Errorf("error = %q", err.Error())
	}
}

// TestRunChangelog_StandaloneWithCommits drives the standalone path with real
// commits between two refs, exercising gatherChangelogCommits, DiffBetween, and
// resolveChangelogAgent. The agent send may fail (no live agent in CI), which is
// tolerated — the goal is to cover the gathering + agent-resolution branches.
func TestRunChangelog_StandaloneWithCommits(t *testing.T) {
	origJSON, origAgent := changelogJSON, changelogAgent
	t.Cleanup(func() { changelogJSON, changelogAgent = origJSON, origAgent })
	changelogJSON = false
	// Force a non-existent agent so resolveChangelogAgent fails fast at
	// GetOrDetect — this exercises the gather/diff/resolve branches without
	// invoking a live agent (which would make a real API call).
	changelogAgent = "no-such-agent-kvelmo-test"

	shortKvelmoHome(t)
	repo := t.TempDir()
	testutil.InitGitRepo(t, repo) // first commit
	addCommit(t, repo, "second.txt", "second commit")
	t.Chdir(repo)

	firstSHA := gitRevParse(t, repo, "HEAD~1")

	err := runChangelog(ChangelogCmd, []string{firstSHA, "HEAD"})
	if err == nil {
		t.Fatal("expected error resolving non-existent agent")
	}
	if !strings.Contains(err.Error(), "resolve agent") {
		t.Errorf("error = %q, want 'resolve agent'", err.Error())
	}
}

func TestRunChangelog_ViaSocket(t *testing.T) {
	origJSON := changelogJSON
	t.Cleanup(func() { changelogJSON = origJSON })
	changelogJSON = false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("changelog.generate", map[string]any{"markdown": "## Added\n- thing"})

	out := captureStdout(t, func() {
		if err := runChangelog(ChangelogCmd, []string{"v1", "v2"}); err != nil {
			t.Errorf("runChangelog via socket: %v", err)
		}
	})
	if !strings.Contains(out, "## Added") {
		t.Errorf("changelog socket output = %q", out)
	}
}

// --- serve helpers ---

func TestBrowserOpenCommand_BuildsOpenerWithoutLaunching(t *testing.T) {
	// MUST NOT launch a real browser (that would pop a tab in the user's default
	// browser). Build the opener command and assert its shape only — never Start it.
	cmd := browserOpenCommand("https://example.com")
	if cmd == nil {
		t.Skip("no browser opener (open/xdg-open) available on this platform")
	}
	if len(cmd.Args) < 2 {
		t.Fatalf("opener command has too few args: %v", cmd.Args)
	}
	if got := cmd.Args[len(cmd.Args)-1]; got != "https://example.com" {
		t.Errorf("opener URL arg = %q, want https://example.com", got)
	}
	if !strings.HasSuffix(cmd.Path, "open") {
		t.Errorf("opener binary = %q, want a path ending in open or xdg-open", cmd.Path)
	}

	// Non-http(s) schemes must not produce an opener command.
	if browserOpenCommand("file:///etc/passwd") != nil {
		t.Error("browserOpenCommand should reject non-http(s) URLs")
	}
}

// --- start command ---

func TestRunStart_BackgroundLoadsTask(t *testing.T) {
	origText, origFrom, origAuto := startText, startFrom, startAuto
	t.Cleanup(func() { startText, startFrom, startAuto = origText, origFrom, origAuto })
	startText, startFrom, startAuto = "", "", false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("start", map[string]any{"task_id": "t1", "state": "loaded"})

	out := captureStdout(t, func() {
		// Positional arg becomes inline text → startFrom = empty:...
		if err := runStart(StartCmd, []string{"fix", "the", "bug"}); err != nil {
			t.Errorf("runStart: %v", err)
		}
	})
	if !strings.Contains(out, "Task loaded from") {
		t.Errorf("start output = %q", out)
	}
}

func TestRunStart_SocketAlreadyRunningNoTask(t *testing.T) {
	origText, origFrom := startText, startFrom
	t.Cleanup(func() { startText, startFrom = origText, origFrom })
	startText, startFrom = "", ""

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	_ = startStubWorktreeSocket(t)

	out := captureStdout(t, func() {
		if err := runStart(StartCmd, nil); err != nil {
			t.Errorf("runStart no task: %v", err)
		}
	})
	if !strings.Contains(out, "Socket already running") {
		t.Errorf("start output = %q", out)
	}
}

func TestLoadTaskViaRPC_Success(t *testing.T) {
	origAuto, origSkip := startAuto, startSkip
	t.Cleanup(func() { startAuto, startSkip = origAuto, origSkip })
	startAuto, startSkip = true, []string{"simplify"}

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("start", map[string]any{"task_id": "t1", "state": "loaded"})

	cwd, _ := os.Getwd()
	if err := loadTaskViaRPC(socket.WorktreeSocketPath(cwd), "empty:do something"); err != nil {
		t.Errorf("loadTaskViaRPC: %v", err)
	}
}

// addCommit writes a file in the repo and commits it.
func addCommit(t *testing.T, repo, name, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(msg+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	runGit(t, repo, "add", name)
	runGit(t, repo, "commit", "-m", msg)
}

// runGit runs a git command in the repo dir, failing the test on error.
func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	full := append([]string{"-C", repo}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil { //nolint:noctx // test helper
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gitRevParse resolves a ref to a SHA.
func gitRevParse(t *testing.T, repo, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", ref).Output() //nolint:noctx // test helper
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}

	return strings.TrimSpace(string(out))
}
