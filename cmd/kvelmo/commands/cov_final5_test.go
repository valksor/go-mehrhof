package commands

import (
	"strings"
	"testing"
)

// --- group add / list-display ---

func TestRunGroupAdd_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.add", map[string]any{"added": true})

	out := captureStdout(t, func() {
		if err := runGroupAdd(nil, []string{"g1", "t1"}); err != nil {
			t.Errorf("runGroupAdd: %v", err)
		}
	})
	if !strings.Contains(out, "Added task t1 to group g1") {
		t.Errorf("group add output:\n%s", out)
	}
}

func TestRunGroupList_Display(t *testing.T) {
	setBoolPtr(t, &groupListJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.list", map[string]any{
		"groups": []any{
			map[string]any{
				"id": "g1", "label": "Group One", "status": "active",
				"tasks": []any{map[string]any{"task_id": "t1"}},
			},
		},
	})

	out := captureStdout(t, func() {
		if err := runGroupList(nil, nil); err != nil {
			t.Errorf("runGroupList: %v", err)
		}
	})
	if !strings.Contains(out, "Group One") {
		t.Errorf("group list display output:\n%s", out)
	}
}

func TestRunGroupSubmit_Output(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.submit", map[string]any{"submitted": true, "count": 2})

	out := captureStdout(t, func() {
		if err := runGroupSubmit(nil, []string{"g1"}); err != nil {
			t.Errorf("runGroupSubmit: %v", err)
		}
	})
	if out == "" {
		t.Error("group submit produced no output")
	}
}

func TestRunGroupRemove_Output(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("taskgroup.remove", map[string]any{"removed": true})

	out := captureStdout(t, func() {
		if err := runGroupRemove(nil, []string{"g1"}); err != nil {
			t.Errorf("runGroupRemove: %v", err)
		}
	})
	if out == "" {
		t.Error("group remove produced no output")
	}
}

// --- screenshots list display + delete ---

func TestRunScreenshotsList_FullDisplay(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.list", map[string]any{
		"screenshots": []any{
			map[string]any{
				"id": "s1", "filename": "s1.png", "width": 800, "height": 600,
				"size_bytes": 4096, "source": "browser", "step": "login", "timestamp": "2026-05-01T10:00:00Z",
			},
		},
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsList(screenshotsListCmd, nil); err != nil {
			t.Errorf("runScreenshotsList: %v", err)
		}
	})
	if !strings.Contains(out, "Screenshots (1)") || !strings.Contains(out, "s1.png") || !strings.Contains(out, "login") {
		t.Errorf("screenshots list full display:\n%s", out)
	}
}

func TestRunScreenshotsDelete_Output(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.delete", map[string]any{"deleted": true})

	out := captureStdout(t, func() {
		if err := runScreenshotsDelete(screenshotsDeleteCmd, []string{"s1"}); err != nil {
			t.Errorf("runScreenshotsDelete: %v", err)
		}
	})
	if out == "" {
		t.Error("screenshots delete produced no output")
	}
}

// --- config command RunE paths via socket ---

func TestConfigShowCmd_ViaSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("settings.get", map[string]any{
		"effective": map[string]any{"workers": map[string]any{"max": 4}},
	})

	out := captureStdout(t, func() {
		if err := configShowCmd.RunE(configShowCmd, nil); err != nil {
			t.Errorf("config show via socket: %v", err)
		}
	})
	if !strings.Contains(out, "workers") {
		t.Errorf("config show via socket output:\n%s", out)
	}
}

func TestConfigGetCmd_ViaSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("settings.get", map[string]any{
		"effective": map[string]any{"git": map[string]any{"branch_pattern": "feat/{id}"}},
	})

	out := captureStdout(t, func() {
		if err := configGetCmd.RunE(configGetCmd, []string{"git.branch_pattern"}); err != nil {
			t.Errorf("config get via socket: %v", err)
		}
	})
	if !strings.Contains(out, "feat/{id}") {
		t.Errorf("config get via socket output:\n%s", out)
	}
}

func TestConfigInitCmd_Preset(t *testing.T) {
	orig := configInitPreset
	t.Cleanup(func() { configInitPreset = orig })
	configInitPreset = "compliance"

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("settings.set", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		if err := configInitCmd.RunE(configInitCmd, nil); err != nil {
			t.Errorf("config init preset: %v", err)
		}
	})
	if !strings.Contains(out, "Applied preset: compliance") {
		t.Errorf("config init preset output:\n%s", out)
	}
}

func TestConfigInitCmd_UnknownPreset(t *testing.T) {
	orig := configInitPreset
	t.Cleanup(func() { configInitPreset = orig })
	configInitPreset = "no-such-preset"

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("settings.set", map[string]any{"ok": true})

	if err := configInitCmd.RunE(configInitCmd, nil); err == nil {
		t.Fatal("expected error for unknown preset")
	}
}

// --- recordings list/view display ---

func TestRunRecordingsView_Records(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("recordings.view", map[string]any{
		"header": map[string]any{"job_id": "j1", "agent": "claude", "model": "m", "started_at": "2026-05-01T10:00:00Z"},
		"records": []any{
			map[string]any{"timestamp": "2026-05-01T10:00:01Z", "direction": "out", "type": "stream", "event": map[string]any{"content": "hi"}},
			map[string]any{"timestamp": "2026-05-01T10:00:02Z", "direction": "in", "type": "prompt", "event": "raw"},
		},
	})

	out := captureStdout(t, func() {
		if err := runRecordingsView(recordingsViewCmd, []string{"rec.jsonl"}); err != nil {
			t.Errorf("runRecordingsView: %v", err)
		}
	})
	if !strings.Contains(out, "Recording:") {
		t.Errorf("recordings view records output:\n%s", out)
	}
}

func TestRunRecordingsList_Display(t *testing.T) {
	setBoolPtr(t, &recordingsOutputJSON, false)
	origJob, origSince := recordingsJobFilter, recordingsSinceFilter
	t.Cleanup(func() { recordingsJobFilter, recordingsSinceFilter = origJob, origSince })
	recordingsJobFilter, recordingsSinceFilter = "job-1", "24h"

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("recordings.list", map[string]any{
		"recordings": []any{
			map[string]any{"path": "/r/very-long-job-id-recording.jsonl", "job_id": "job-1-very-long-id", "agent": "claude", "lines": 10, "started_at": "2026-05-01T10:00:00Z"},
		},
	})

	out := captureStdout(t, func() {
		if err := runRecordingsList(recordingsListCmd, nil); err != nil {
			t.Errorf("runRecordingsList: %v", err)
		}
	})
	if !strings.Contains(out, "Total: 1") {
		t.Errorf("recordings list display output:\n%s", out)
	}
}
