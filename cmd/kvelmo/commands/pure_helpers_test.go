package commands

import (
	"encoding/json"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/internal/update"
)

func TestIsBlockedTask(t *testing.T) {
	cases := []struct {
		name string
		task socket.TaskListSummary
		want bool
	}{
		{"failed state", socket.TaskListSummary{State: stateFailed}, true},
		{"pending prompt", socket.TaskListSummary{PendingPromptID: "p1"}, true},
		{"hard stop", socket.TaskListSummary{LastFailureClass: "hard_stop"}, true},
		{"running", socket.TaskListSummary{State: "implementing"}, false},
		{"empty", socket.TaskListSummary{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBlockedTask(tc.task); got != tc.want {
				t.Errorf("isBlockedTask(%v) = %v, want %v", tc.task, got, tc.want)
			}
		})
	}
}

func TestStatusFlag(t *testing.T) {
	cases := []struct {
		name string
		task socket.TaskListSummary
		want string
	}{
		{"failed", socket.TaskListSummary{State: stateFailed}, "FAIL"},
		{"prompt", socket.TaskListSummary{PendingPromptID: "p1"}, "PROMPT"},
		{"hard stop", socket.TaskListSummary{LastFailureClass: "hard_stop"}, "BLOCK"},
		{"warning", socket.TaskListSummary{LastError: "transient"}, "WARN"},
		{"clean", socket.TaskListSummary{}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusFlag(tc.task); got != tc.want {
				t.Errorf("statusFlag(%v) = %q, want %q", tc.task, got, tc.want)
			}
		})
	}
}

func TestReleaseURLs(t *testing.T) {
	status := &update.UpdateStatus{
		AssetURL: "https://github.com/valksor/kvelmo/releases/download/v1.2.3/kvelmo-darwin-arm64.tar.gz",
	}
	checks, sig := releaseURLs(status)
	wantBase := "https://github.com/valksor/kvelmo/releases/download/v1.2.3"
	if checks != wantBase+"/checksums.txt" {
		t.Errorf("checks = %q", checks)
	}
	if sig != wantBase+"/checksums.txt.minisig" {
		t.Errorf("sig = %q", sig)
	}
}

func TestReleaseURLs_NoSlash(t *testing.T) {
	status := &update.UpdateStatus{AssetURL: "no-slash-here"}
	checks, sig := releaseURLs(status)
	// Without a slash, base stays unchanged.
	if checks != "no-slash-here/checksums.txt" {
		t.Errorf("checks = %q", checks)
	}
	_ = sig
}

func TestResolveGitHubToken_NoEnv(t *testing.T) {
	// With KVELMO_HOME pointed at a temp dir and no .env, returns "".
	shortKvelmoHome(t)
	if tok := resolveGitHubToken(); tok != "" {
		t.Errorf("expected empty token, got %q", tok)
	}
}

func TestBuildQuickContextItems(t *testing.T) {
	old := quickContextFiles
	t.Cleanup(func() { quickContextFiles = old })

	quickContextFiles = []string{"x.go"}
	quickContextSymbol = []string{"Sym"}
	quickContextCommit = []string{"abc"}
	t.Cleanup(func() {
		quickContextFiles = nil
		quickContextSymbol = nil
		quickContextCommit = nil
	})

	items := buildQuickContextItems()
	if len(items) != 3 {
		t.Errorf("len = %d, want 3", len(items))
	}
}

func TestOutputCSV_Empty(t *testing.T) {
	if err := outputCSV(json.RawMessage(`{}`)); err != nil {
		t.Errorf("empty input: %v", err)
	}
}

func TestOutputCSV_TasksAndActivity(t *testing.T) {
	raw := json.RawMessage(`{
		"tasks": [
			{"id": "t1", "path": "/p", "state": "implementing"}
		],
		"activity": [
			{"timestamp": "2026-05-19T10:00:00Z", "method": "ping", "duration_ms": 5, "user_id": "u1", "task_id": "t1", "agent_model": "claude"}
		]
	}`)
	if err := outputCSV(raw); err != nil {
		t.Errorf("outputCSV: %v", err)
	}
}

func TestOutputCSV_Malformed(t *testing.T) {
	if err := outputCSV(json.RawMessage(`{not valid`)); err == nil {
		t.Error("expected parse error for malformed JSON")
	}
}
