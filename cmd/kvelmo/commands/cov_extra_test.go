package commands

import (
	"strings"
	"testing"
)

func TestRunActivity_JSON(t *testing.T) {
	setBoolPtr(t, &activityJSON, true)
	shortKvelmoHome(t)
	_ = startStubGlobalSocket(t)

	out := captureStdout(t, func() {
		if err := runActivity(ActivityCmd, nil); err != nil {
			t.Errorf("runActivity json: %v", err)
		}
	})
	if !strings.Contains(out, "entries") {
		t.Errorf("activity json output:\n%s", out)
	}
}

func TestRunActivity_NotEnabled(t *testing.T) {
	setBoolPtr(t, &activityJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("activity.query", map[string]any{"entries": []any{}, "count": 0, "enabled": false})

	out := captureStdout(t, func() {
		if err := runActivity(ActivityCmd, nil); err != nil {
			t.Errorf("runActivity not enabled: %v", err)
		}
	})
	if !strings.Contains(out, "not enabled") {
		t.Errorf("activity not-enabled output = %q", out)
	}
}

func TestRunAudit_JSON(t *testing.T) {
	setBoolPtr(t, &auditJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("export", map[string]any{"tasks": []any{}, "activity": []any{}})

	out := captureStdout(t, func() {
		if err := runAudit(AuditCmd, nil); err != nil {
			t.Errorf("runAudit json: %v", err)
		}
	})
	if !strings.Contains(out, "tasks") {
		t.Errorf("audit json output:\n%s", out)
	}
}

func TestRunAudit_Populated(t *testing.T) {
	setBoolPtr(t, &auditJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("export", map[string]any{
		"tasks": []any{
			map[string]any{"id": "t1", "path": "/p", "state": "implementing"},
		},
		"activity": []any{
			map[string]any{"timestamp": "2026-05-01T10:00:00Z", "method": "ping", "duration_ms": 5, "user_id": "u1"},
		},
	})

	out := captureStdout(t, func() {
		if err := runAudit(AuditCmd, nil); err != nil {
			t.Errorf("runAudit populated: %v", err)
		}
	})
	if !strings.Contains(out, "Active Tasks (1)") {
		t.Errorf("audit populated output:\n%s", out)
	}
}

func TestRunExport_Populated(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("export", map[string]any{
		"tasks": []any{map[string]any{"id": "t1", "path": "/p", "state": "implementing"}},
		"activity": []any{
			map[string]any{"timestamp": "2026-05-01T10:00:00Z", "method": "ping", "duration_ms": 5, "user_id": "u1"},
		},
	})

	out := captureStdout(t, func() {
		if err := runExport(nil, nil); err != nil {
			t.Errorf("runExport: %v", err)
		}
	})
	if out == "" {
		t.Error("export populated produced no output")
	}
}

func TestRunExportTask_Output(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.export", map[string]any{"id": "t1", "path": "/tmp/t1.json", "format": "json"})

	out := captureStdout(t, func() {
		if err := runExportTask(nil, nil); err != nil {
			t.Errorf("runExportTask: %v", err)
		}
	})
	if out == "" {
		t.Error("export task produced no output")
	}
}

func TestRunBackup_Output(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.create", map[string]any{"path": "/b/backup.tar.gz", "size": 4096, "tasks": 3})

	out := captureStdout(t, func() {
		if err := runBackup(nil, nil); err != nil {
			t.Errorf("runBackup: %v", err)
		}
	})
	if out == "" {
		t.Error("backup produced no output")
	}
}

func TestRunRestore_Output(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.restore", map[string]any{"restored": true, "tasks": 5, "path": "/b/b.tar.gz"})

	out := captureStdout(t, func() {
		if err := runRestore(nil, []string{"/b/b.tar.gz"}); err != nil {
			t.Errorf("runRestore: %v", err)
		}
	})
	if out == "" {
		t.Error("restore produced no output")
	}
}

func TestRunReport_Output(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("report.generate", map[string]any{
		"markdown": "# Compliance Report\n\n10 tasks",
	})

	out := captureStdout(t, func() {
		if err := runReport(nil, nil); err != nil {
			t.Errorf("runReport: %v", err)
		}
	})
	if !strings.Contains(out, "Compliance Report") {
		t.Errorf("report output:\n%s", out)
	}
}

func TestRunFilesList_Populated(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("files.list", map[string]any{
		"files": []string{"main.go", "pkg/handler.go"},
	})

	out := captureStdout(t, func() {
		if err := runFilesList(filesListCmd, nil); err != nil {
			t.Errorf("runFilesList: %v", err)
		}
	})
	if !strings.Contains(out, "main.go") {
		t.Errorf("files list populated output:\n%s", out)
	}
}

func TestRunFilesSearch_Populated(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("files.search", map[string]any{
		"files": []string{"auth.go", "auth_test.go"},
	})

	out := captureStdout(t, func() {
		if err := runFilesSearch(filesSearchCmd, []string{"Auth"}); err != nil {
			t.Errorf("runFilesSearch: %v", err)
		}
	})
	if !strings.Contains(out, "auth.go") {
		t.Errorf("files search populated output:\n%s", out)
	}
}

func TestRunScreenshotsGet_Output(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.get", map[string]any{
		"id": "s1", "path": "/tmp/s1.png", "label": "home", "width": 800, "height": 600,
		"captured_at": "2026-05-01T10:00:00Z",
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsGet(screenshotsGetCmd, []string{"s1"}); err != nil {
			t.Errorf("runScreenshotsGet: %v", err)
		}
	})
	if out == "" {
		t.Error("screenshots get produced no output")
	}
}

func TestRunEventlog_Multi(t *testing.T) {
	setBoolPtr(t, &eventlogJSON, false)
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("eventlog.query", map[string]any{
		"entries": []any{
			map[string]any{"timestamp": "2026-05-01T10:00:00Z", "type": "phase_started", "phase": "plan", "message": "a"},
			map[string]any{"timestamp": "2026-05-03T11:00:00Z", "type": "phase_completed", "phase": "plan", "message": "b"},
		},
		"total": 2,
	})

	out := captureStdout(t, func() {
		if err := runEventlog(EventlogCmd, nil); err != nil {
			t.Errorf("runEventlog: %v", err)
		}
	})
	if !strings.Contains(out, "2 total") {
		t.Errorf("eventlog multi output:\n%s", out)
	}
}

func TestRunMemorySearch_FullResults(t *testing.T) {
	setBoolPtr(t, &memorySearchJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("memory.search", map[string]any{
		"results": []any{
			map[string]any{"content": "decision A about auth", "score": 0.95, "task_id": "t1", "kind": "decision", "created_at": "2026-05-01T10:00:00Z"},
			map[string]any{"content": "outcome B", "score": 0.80, "task_id": "t2", "kind": "outcome"},
		},
	})

	out := captureStdout(t, func() {
		if err := runMemorySearch(memorySearchCmd, []string{"auth"}); err != nil {
			t.Errorf("runMemorySearch: %v", err)
		}
	})
	if out == "" {
		t.Error("memory search full results produced no output")
	}
}

func TestRunDiagnoseViaRPC_JSON(t *testing.T) {
	setBoolPtr(t, &diagnoseJSON, true)
	setBoolPtr(t, &diagnoseHealth, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("system.diagnose", map[string]any{"checks": []any{}, "global_socket": "running", "providers": []any{}})

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose via rpc json: %v", err)
		}
	})
	if !strings.Contains(out, "global_socket") {
		t.Errorf("diagnose via rpc json output:\n%s", out)
	}
}

func TestRunDiagnoseHealth_Empty(t *testing.T) {
	setBoolPtr(t, &diagnoseHealth, true)
	setBoolPtr(t, &diagnoseJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("system.health", map[string]any{"worktrees": []any{}})

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose health empty: %v", err)
		}
	})
	if !strings.Contains(out, "No worktrees registered") {
		t.Errorf("diagnose health empty output = %q", out)
	}
}

func TestRunDiagnoseHealth_JSON(t *testing.T) {
	setBoolPtr(t, &diagnoseHealth, true)
	setBoolPtr(t, &diagnoseJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("system.health", map[string]any{"worktrees": []any{}})

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose health json: %v", err)
		}
	})
	if !strings.Contains(out, "worktrees") {
		t.Errorf("diagnose health json output:\n%s", out)
	}
}
