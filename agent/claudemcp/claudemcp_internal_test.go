package claudemcp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
)

// TestNewDefaults verifies that New() applies the documented defaults and that
// NewWithConfig backfills the same defaults when given a zero-value Config.
func TestNewDefaults(t *testing.T) {
	a := New()
	if got := a.Name(); got != AgentName {
		t.Errorf("Name() = %q, want %q", got, AgentName)
	}
	if len(a.config.Command) == 0 || a.config.Command[0] != "claude" {
		t.Errorf("Command = %v, want [claude]", a.config.Command)
	}
	// New() seeds from DefaultConfig(), whose embedded agent.DefaultConfig()
	// already sets a 30m timeout, so the claudemcp 60m fallback never fires.
	if a.config.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", a.config.Timeout)
	}
	if !a.config.StrictMCPConfig {
		t.Error("New() StrictMCPConfig = false, want true (from DefaultConfig)")
	}
	if a.config.PermissionMode != PermissionModeAcceptEdits {
		t.Errorf("PermissionMode = %q, want %q", a.config.PermissionMode, PermissionModeAcceptEdits)
	}
	want := []string{"kvelmo", "mcp", "--stdio"}
	if !slices.Equal(a.config.MCPServerCommand, want) {
		t.Errorf("MCPServerCommand = %v, want %v", a.config.MCPServerCommand, want)
	}
}

// TestNewWithConfigBackfill confirms NewWithConfig fills in each default
// independently when only some fields are set.
func TestNewWithConfigBackfill(t *testing.T) {
	a := NewWithConfig(Config{})
	if len(a.config.Command) == 0 || a.config.Command[0] != "claude" {
		t.Errorf("Command default = %v", a.config.Command)
	}
	if a.config.Timeout != 60*time.Minute {
		t.Errorf("Timeout default = %v", a.config.Timeout)
	}
	if a.config.PermissionMode != PermissionModeAcceptEdits {
		t.Errorf("PermissionMode default = %q", a.config.PermissionMode)
	}
	if !slices.Equal(a.config.MCPServerCommand, []string{"kvelmo", "mcp", "--stdio"}) {
		t.Errorf("MCPServerCommand default = %v", a.config.MCPServerCommand)
	}

	// Explicit values are preserved, not overwritten by defaults.
	custom := Config{}
	custom.Command = []string{"glm"}
	custom.Timeout = 5 * time.Minute
	custom.PermissionMode = PermissionModeBypassPermission
	custom.MCPServerCommand = []string{"my-mcp"}
	b := NewWithConfig(custom)
	if b.config.Command[0] != "glm" {
		t.Errorf("Command override lost: %v", b.config.Command)
	}
	if b.config.Timeout != 5*time.Minute {
		t.Errorf("Timeout override lost: %v", b.config.Timeout)
	}
	if b.config.PermissionMode != PermissionModeBypassPermission {
		t.Errorf("PermissionMode override lost: %v", b.config.PermissionMode)
	}
	if !slices.Equal(b.config.MCPServerCommand, []string{"my-mcp"}) {
		t.Errorf("MCPServerCommand override lost: %v", b.config.MCPServerCommand)
	}
}

// TestConnectConnectedClose drives the trivial connection-state lifecycle.
func TestConnectConnectedClose(t *testing.T) {
	a := New()
	if a.Connected() {
		t.Error("Connected() = true before Connect")
	}
	if err := a.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() = %v", err)
	}
	if !a.Connected() {
		t.Error("Connected() = false after Connect")
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if a.Connected() {
		t.Error("Connected() = true after Close")
	}
}

// TestHandlePermissionNoop verifies the documented no-op (permissions are
// handled inside the interactive claude session, not via this method).
func TestHandlePermissionNoop(t *testing.T) {
	a := New()
	if err := a.HandlePermission("req-1", true); err != nil {
		t.Errorf("HandlePermission() = %v, want nil", err)
	}
}

// TestInterruptNoSession verifies Interrupt is a safe no-op with no active PTY.
func TestInterruptNoSession(t *testing.T) {
	a := New()
	if err := a.Interrupt(); err != nil {
		t.Errorf("Interrupt() with no session = %v, want nil", err)
	}
}

// TestCloseIdempotent verifies Close is safe to call repeatedly with no
// active session.
func TestCloseIdempotent(t *testing.T) {
	a := New()
	for i := range 3 {
		if err := a.Close(); err != nil {
			t.Errorf("Close() #%d = %v", i, err)
		}
	}
}

// TestCleanupAfterSessionNoSession verifies the resource-release helper does
// nothing harmful when there is no active session.
func TestCleanupAfterSessionNoSession(t *testing.T) {
	a := New()
	a.cleanupAfterSession() // must not panic with nil rdz/pty/cancel
}

// TestBuildArgv covers each conditional branch of the claude CLI invocation.
func TestBuildArgv(t *testing.T) {
	t.Run("omit print and sdk-url, include core flags", func(t *testing.T) {
		a := New()
		argv := a.buildArgv("/tmp/mcp.json", "/tmp/sys.md", "do it")
		if argv[0] != "claude" {
			t.Errorf("argv[0] = %q, want claude", argv[0])
		}
		// Critical: must NOT route through --print / --sdk-url (those move
		// billing onto the Agent SDK credit pool).
		for _, banned := range []string{"--print", "--sdk-url"} {
			if slices.Contains(argv, banned) {
				t.Errorf("argv must not contain %q (billing regression): %v", banned, argv)
			}
		}
		assertFlagValue(t, argv, "--mcp-config", "/tmp/mcp.json")
		assertFlagValue(t, argv, "--append-system-prompt", "/tmp/sys.md")
		assertFlagValue(t, argv, "--permission-mode", PermissionModeAcceptEdits)
		// Seed prompt is the trailing positional arg.
		if argv[len(argv)-1] != "do it" {
			t.Errorf("seed prompt should be last arg, got %v", argv)
		}
	})

	t.Run("no strict flag when StrictMCPConfig is false", func(t *testing.T) {
		// A zero-embedded Config leaves StrictMCPConfig false, so the flag is
		// omitted (only DefaultConfig opts in).
		a := NewWithConfig(Config{})
		if a.config.StrictMCPConfig {
			t.Fatal("precondition: expected StrictMCPConfig false for zero Config")
		}
		argv := a.buildArgv("/m", "/s", "p")
		if slices.Contains(argv, "--strict-mcp-config") {
			t.Errorf("--strict-mcp-config present without opt-in: %v", argv)
		}
	})

	t.Run("strict mcp config flag", func(t *testing.T) {
		cfg := DefaultConfig() // DefaultConfig sets StrictMCPConfig=true
		a := NewWithConfig(cfg)
		argv := a.buildArgv("/m", "/s", "p")
		if !slices.Contains(argv, "--strict-mcp-config") {
			t.Errorf("--strict-mcp-config missing with StrictMCPConfig=true: %v", argv)
		}
	})

	t.Run("model flag added when set", func(t *testing.T) {
		cfg := Config{Model: "opus"}
		a := NewWithConfig(cfg)
		argv := a.buildArgv("/m", "/s", "p")
		assertFlagValue(t, argv, "--model", "opus")
	})

	t.Run("no model flag when unset", func(t *testing.T) {
		a := New()
		argv := a.buildArgv("/m", "/s", "p")
		if slices.Contains(argv, "--model") {
			t.Errorf("--model present with empty model: %v", argv)
		}
	})

	t.Run("extra args appended before seed prompt", func(t *testing.T) {
		cfg := Config{}
		cfg.Args = []string{"--debug", "--foo"}
		a := NewWithConfig(cfg)
		argv := a.buildArgv("/m", "/s", "seed")
		if !slices.Contains(argv, "--debug") || !slices.Contains(argv, "--foo") {
			t.Errorf("extra args missing: %v", argv)
		}
		if argv[len(argv)-1] != "seed" {
			t.Errorf("seed prompt should follow extra args, got %v", argv)
		}
	})

	t.Run("empty seed prompt omits trailing positional", func(t *testing.T) {
		a := New()
		argv := a.buildArgv("/m", "/s", "")
		if argv[len(argv)-1] == "" {
			t.Errorf("empty seed prompt should not be appended: %v", argv)
		}
	})
}

// assertFlagValue asserts argv contains flag immediately followed by want.
func assertFlagValue(t *testing.T, argv []string, flag, want string) {
	t.Helper()
	for i, a := range argv {
		if a == flag {
			if i+1 >= len(argv) {
				t.Fatalf("flag %q has no value in %v", flag, argv)
			}
			if argv[i+1] != want {
				t.Errorf("flag %q value = %q, want %q", flag, argv[i+1], want)
			}

			return
		}
	}
	t.Errorf("flag %q not found in %v", flag, argv)
}

// TestPrepareWorkDir covers both the default-root and explicit-WorkRoot
// branches and asserts the directory is created with 0700 perms.
func TestPrepareWorkDir(t *testing.T) {
	t.Run("explicit work root", func(t *testing.T) {
		root := t.TempDir()
		cfg := Config{WorkRoot: root}
		a := NewWithConfig(cfg)
		dir, err := a.prepareWorkDir("task-abc")
		if err != nil {
			t.Fatalf("prepareWorkDir: %v", err)
		}
		if filepath.Dir(filepath.Dir(dir)) != root {
			t.Errorf("dir %q not under root %q", dir, root)
		}
		if filepath.Base(filepath.Dir(dir)) != "task-abc" {
			t.Errorf("dir %q missing task segment", dir)
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if !info.IsDir() {
			t.Error("prepared work dir is not a directory")
		}
		if runtime.GOOS != "windows" {
			if mode := info.Mode().Perm(); mode != 0o700 {
				t.Errorf("work dir mode = %o, want 0700", mode)
			}
		}
	})

	t.Run("default root via KVELMO_HOME", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("KVELMO_HOME", home)
		a := New() // WorkRoot empty → falls back to paths.BaseDir()/work
		dir, err := a.prepareWorkDir("task-default")
		if err != nil {
			t.Fatalf("prepareWorkDir: %v", err)
		}
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("default-root dir not created: %v", err)
		}
		// The created dir must live under the KVELMO_HOME we set.
		rel, err := filepath.Rel(home, dir)
		if err != nil || rel == "" || rel[0] == '.' && len(rel) > 1 && rel[1] == '.' {
			t.Errorf("dir %q not under KVELMO_HOME %q (rel=%q)", dir, home, rel)
		}
	})

	t.Run("two calls produce distinct dirs", func(t *testing.T) {
		root := t.TempDir()
		a := NewWithConfig(Config{WorkRoot: root})
		d1, err := a.prepareWorkDir("t")
		if err != nil {
			t.Fatal(err)
		}
		d2, err := a.prepareWorkDir("t")
		if err != nil {
			t.Fatal(err)
		}
		if d1 == d2 {
			t.Errorf("expected distinct dirs, both = %q", d1)
		}
	})
}

// TestRandomSuffix verifies hex length and uniqueness across calls.
func TestRandomSuffix(t *testing.T) {
	s := randomSuffix(6)
	if len(s) != 12 { // 6 bytes → 12 hex chars
		t.Errorf("randomSuffix(6) len = %d, want 12 (%q)", len(s), s)
	}
	seen := map[string]bool{}
	for range 100 {
		v := randomSuffix(4)
		if len(v) != 8 {
			t.Errorf("randomSuffix(4) len = %d, want 8", len(v))
		}
		if seen[v] {
			t.Errorf("randomSuffix produced duplicate %q", v)
		}
		seen[v] = true
	}
}

// TestWithBuilders verifies each functional-option builder returns a fresh
// Agent carrying the mutation and leaves the receiver unmodified.
func TestWithBuilders(t *testing.T) {
	base := New()

	t.Run("WithEnv", func(t *testing.T) {
		got, ok := base.WithEnv("KEY", "value").(*Agent)
		if !ok {
			t.Fatalf("WithEnv did not return *Agent")
		}
		if got.config.Environment["KEY"] != "value" {
			t.Errorf("WithEnv did not set env: %v", got.config.Environment)
		}
		if base.config.Environment["KEY"] != "" {
			t.Error("WithEnv mutated the receiver's environment")
		}
		// Chaining accumulates.
		got2, ok := got.WithEnv("KEY2", "v2").(*Agent)
		if !ok {
			t.Fatalf("chained WithEnv did not return *Agent")
		}
		if got2.config.Environment["KEY"] != "value" || got2.config.Environment["KEY2"] != "v2" {
			t.Errorf("chained WithEnv lost a key: %v", got2.config.Environment)
		}
	})

	t.Run("WithArgs", func(t *testing.T) {
		got, ok := base.WithArgs("--a", "--b").(*Agent)
		if !ok {
			t.Fatalf("WithArgs did not return *Agent")
		}
		if !slices.Equal(got.config.Args, []string{"--a", "--b"}) {
			t.Errorf("WithArgs = %v", got.config.Args)
		}
		got2, ok := got.WithArgs("--c").(*Agent)
		if !ok {
			t.Fatalf("chained WithArgs did not return *Agent")
		}
		if !slices.Equal(got2.config.Args, []string{"--a", "--b", "--c"}) {
			t.Errorf("chained WithArgs = %v", got2.config.Args)
		}
		// Receiver unmodified.
		if len(got.config.Args) != 2 {
			t.Errorf("WithArgs mutated receiver: %v", got.config.Args)
		}
	})

	t.Run("WithWorkDir", func(t *testing.T) {
		got, ok := base.WithWorkDir("/work/dir").(*Agent)
		if !ok {
			t.Fatalf("WithWorkDir did not return *Agent")
		}
		if got.config.WorkDir != "/work/dir" {
			t.Errorf("WithWorkDir = %q", got.config.WorkDir)
		}
	})

	t.Run("WithTimeout", func(t *testing.T) {
		got, ok := base.WithTimeout(7 * time.Minute).(*Agent)
		if !ok {
			t.Fatalf("WithTimeout did not return *Agent")
		}
		if got.config.Timeout != 7*time.Minute {
			t.Errorf("WithTimeout = %v", got.config.Timeout)
		}
	})
}

// TestRegister confirms the agent registers under AgentName and is retrievable.
func TestRegister(t *testing.T) {
	r := agent.NewRegistry()
	if err := Register(r); err != nil {
		t.Fatalf("Register() = %v", err)
	}
	got, err := r.Get(AgentName)
	if err != nil {
		t.Fatalf("Get(%q) = %v", AgentName, err)
	}
	if got.Name() != AgentName {
		t.Errorf("registered agent Name() = %q, want %q", got.Name(), AgentName)
	}
}

// TestAvailableBinaryNotFound asserts Available returns an error when the
// configured binary cannot be resolved on PATH or in fallback locations.
func TestAvailableBinaryNotFound(t *testing.T) {
	var cfg Config
	cfg.Command = []string{"this-binary-does-not-exist-kvelmo-xyz"}
	a := NewWithConfig(cfg)
	if err := a.Available(); err == nil {
		t.Error("Available() with missing binary = nil, want error")
	}
}

// TestSendPromptErrorPaths drives SendPrompt's pre-spawn validation branches.
func TestSendPromptErrorPaths(t *testing.T) {
	t.Run("no work dir", func(t *testing.T) {
		a := New() // no WorkDir, no KVELMO_WORKTREE env
		_, err := a.SendPrompt(context.Background(), "hi")
		if err == nil {
			t.Fatal("SendPrompt without WorkDir = nil, want error")
		}
	})

	t.Run("worktree from env", func(t *testing.T) {
		// Provide worktree via env but an invalid permission mode so we still
		// fail fast before any PTY spawn — proving the env fallback resolved.
		cfg := Config{PermissionMode: "totally-bogus"}
		cfg.Environment = map[string]string{"KVELMO_WORKTREE": t.TempDir()}
		a := NewWithConfig(cfg)
		_, err := a.SendPrompt(context.Background(), "hi")
		if err == nil {
			t.Fatal("SendPrompt with invalid permission mode = nil, want error")
		}
		// Error must be the permission-mode validation, proving worktree
		// resolution succeeded past the WorkDir check.
		if got := err.Error(); !strings.Contains(got, "permission_mode") {
			t.Errorf("error = %q, want permission_mode validation", got)
		}
	})

	t.Run("invalid permission mode via WorkDir", func(t *testing.T) {
		cfg := Config{PermissionMode: "bogus"}
		cfg.WorkDir = t.TempDir()
		a := NewWithConfig(cfg)
		_, err := a.SendPrompt(context.Background(), "hi")
		if err == nil {
			t.Fatal("invalid permission mode = nil, want error")
		}
	})
}
