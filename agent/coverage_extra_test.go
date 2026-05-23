package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent/permission"
)

// --- registry: MustRegister, GetOrDetect, Clear ---

func TestMustRegister_Succeeds(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&mockAgent{name: "ok", available: true})

	if r.Count() != 1 {
		t.Errorf("Count() = %d, want 1", r.Count())
	}
}

func TestMustRegister_PanicsOnDuplicate(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&mockAgent{name: "dup", available: true})

	defer func() {
		if recover() == nil {
			t.Error("MustRegister should panic on duplicate registration")
		}
	}()

	r.MustRegister(&mockAgent{name: "dup", available: true})
}

func TestGetOrDetect_NamedAgent(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockAgent{name: "named", available: true})

	got, err := r.GetOrDetect("named")
	if err != nil {
		t.Fatalf("GetOrDetect(named) error: %v", err)
	}
	if got.Name() != "named" {
		t.Errorf("GetOrDetect returned %q, want named", got.Name())
	}
}

func TestGetOrDetect_NamedMissing(t *testing.T) {
	r := NewRegistry()
	if _, err := r.GetOrDetect("nope"); err == nil {
		t.Error("GetOrDetect(nope) should error for unregistered name")
	}
}

func TestGetOrDetect_EmptyDetects(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockAgent{name: "unavail", available: false})
	_ = r.Register(&mockAgent{name: "avail", available: true})

	got, err := r.GetOrDetect("")
	if err != nil {
		t.Fatalf("GetOrDetect(\"\") error: %v", err)
	}
	if got.Name() != "avail" {
		t.Errorf("GetOrDetect detected %q, want avail", got.Name())
	}
}

func TestGetOrDetect_EmptyNoneAvailable(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockAgent{name: "unavail", available: false})

	if _, err := r.GetOrDetect(""); err == nil {
		t.Error("GetOrDetect(\"\") should error when none available")
	}
}

func TestRegistry_Clear(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockAgent{name: "a", available: true})
	_ = r.Register(&mockAgent{name: "b", available: true})

	r.Clear()

	if r.Count() != 0 {
		t.Errorf("Count() after Clear = %d, want 0", r.Count())
	}
	if _, err := r.GetDefault(); err == nil {
		t.Error("GetDefault after Clear should error (no agents, no fallback)")
	}
}

// --- EvaluatePermissionWithConfig ---

func TestEvaluatePermissionWithConfig_DangerousDenied(t *testing.T) {
	cfg := &Config{SafeTools: []string{"Bash"}}
	req := PermissionRequest{Tool: "Bash", Input: map[string]any{"command": "rm -rf /"}}

	result := EvaluatePermissionWithConfig(req, cfg)
	if result.Approved {
		t.Error("dangerous op should be denied even when tool is in SafeTools")
	}
	if result.DangerLevel != permission.Dangerous {
		t.Errorf("DangerLevel = %v, want Dangerous", result.DangerLevel)
	}
}

func TestEvaluatePermissionWithConfig_GlobalSafeTool(t *testing.T) {
	req := PermissionRequest{Tool: "Read"}
	result := EvaluatePermissionWithConfig(req, nil)
	if !result.Approved {
		t.Error("globally safe tool Read should be approved with nil config")
	}
}

func TestEvaluatePermissionWithConfig_ConfigSafeTool(t *testing.T) {
	cfg := &Config{SafeTools: []string{"CustomTool"}}
	req := PermissionRequest{Tool: "customtool"} // case-insensitive match

	result := EvaluatePermissionWithConfig(req, cfg)
	if !result.Approved {
		t.Error("config-level safe tool should be approved (case-insensitive)")
	}
}

func TestEvaluatePermissionWithConfig_NotSafeDenied(t *testing.T) {
	cfg := &Config{SafeTools: []string{"OtherTool"}}
	req := PermissionRequest{Tool: "Bash", Input: map[string]any{"command": "ls"}}

	result := EvaluatePermissionWithConfig(req, cfg)
	if result.Approved {
		t.Error("Bash not in safe lists should be denied")
	}
}

func TestEvaluatePermissionWithConfig_NilConfigUnsafeDenied(t *testing.T) {
	req := PermissionRequest{Tool: "Bash", Input: map[string]any{"command": "ls"}}
	result := EvaluatePermissionWithConfig(req, nil)
	if result.Approved {
		t.Error("unsafe tool with nil config should be denied")
	}
}

// --- subagent: SetEventChannel, SetDoneChannel, trySendEventTo done branch ---

func TestSubagentTracker_SetEventChannel(t *testing.T) {
	tracker := NewSubagentTracker(make(chan Event, 1))

	newCh := make(chan Event, 1)
	tracker.SetEventChannel(newCh)

	tracker.OnToolUse("c1", "Task", map[string]any{"subagent_type": "Explore"})

	select {
	case ev := <-newCh:
		if ev.Type != EventSubagent {
			t.Errorf("event type = %q, want subagent", ev.Type)
		}
	default:
		t.Error("event should be delivered to the new channel")
	}
}

func TestSubagentTracker_SetDoneChannel_AbortsCompletion(t *testing.T) {
	// An unbuffered channel with a closed done channel forces the completion
	// send to take the done branch instead of blocking forever.
	events := make(chan Event) // unbuffered, no reader
	tracker := NewSubagentTracker(events)

	done := make(chan struct{})
	tracker.SetDoneChannel(done)

	tracker.OnToolUse("c1", "Task", map[string]any{"subagent_type": "Plan"})
	// Drain the started event in the background (non-blocking send may have dropped it).
	go func() {
		select {
		case <-events:
		case <-time.After(time.Second):
		}
	}()

	close(done) // completion send should select the done branch and return quickly

	result := make(chan bool, 1)
	go func() {
		result <- tracker.OnToolResult("c1", true, "")
	}()

	select {
	case ok := <-result:
		if !ok {
			t.Error("OnToolResult should report the subagent was tracked")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OnToolResult blocked despite closed done channel")
	}
}

// --- health.now defaults to time.Now when nowFunc is nil ---

func TestHealth_NowDefaultsToTimeNow(t *testing.T) {
	h := &Health{} // nowFunc is nil
	before := time.Now()
	got := h.now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("now() = %v, expected between %v and %v", got, before, after)
	}
}

// --- ResolveCommandPath success + fallback + failure ---

func TestResolveCommandPath_FoundInPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary harness requires a POSIX environment")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fakecmd")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)

	got, err := ResolveCommandPath("fakecmd")
	if err != nil {
		t.Fatalf("ResolveCommandPath error: %v", err)
	}
	if got != bin {
		t.Errorf("ResolveCommandPath = %q, want %q", got, bin)
	}
}

func TestResolveCommandPath_NotFound(t *testing.T) {
	// Point PATH at an empty dir so LookPath fails, and use a name that won't
	// exist in the hard-coded fallback locations.
	t.Setenv("PATH", t.TempDir())

	if _, err := ResolveCommandPath("definitely-not-a-real-binary-xyz123"); err == nil {
		t.Error("ResolveCommandPath should fail for a nonexistent command")
	}
}

func TestResolveCommandPath_FallbackLocation(t *testing.T) {
	// LookPath will miss (empty PATH); fallbackCommandPaths includes /usr/bin.
	// We can't write into /usr/bin, but if `sh` exists there the fallback path
	// is exercised. This asserts the fallback branch returns an existing file
	// when PATH lookup fails but a known location has the binary.
	t.Setenv("PATH", t.TempDir())

	// /usr/bin/env exists on essentially all unix systems used for CI.
	if runtime.GOOS == "windows" {
		t.Skip("unix-only fallback paths")
	}
	if _, err := os.Stat("/usr/bin/env"); err != nil {
		t.Skip("/usr/bin/env not present; cannot exercise fallback")
	}

	got, err := ResolveCommandPath("env")
	if err != nil {
		t.Fatalf("ResolveCommandPath(env) via fallback error: %v", err)
	}
	if got != "/usr/bin/env" {
		t.Errorf("ResolveCommandPath(env) = %q, want /usr/bin/env from fallback", got)
	}
}

func TestFallbackCommandPaths_NonDarwin(t *testing.T) {
	paths := fallbackCommandPaths("tool")
	// On any platform the first three base paths are present.
	wantBase := []string{
		filepath.Join("/opt/homebrew/bin", "tool"),
		filepath.Join("/usr/local/bin", "tool"),
		filepath.Join("/usr/bin", "tool"),
	}
	for i, want := range wantBase {
		if paths[i] != want {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], want)
		}
	}
}
