package commands

import (
	"strings"
	"testing"
)

func TestRunExportTask_MarkdownFormat(t *testing.T) {
	orig := exportTaskFormat
	t.Cleanup(func() { exportTaskFormat = orig })
	exportTaskFormat = "md"

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.export", map[string]any{"markdown": "# Task t1\n\nDetails here"})

	out := captureStdout(t, func() {
		if err := runExportTask(nil, nil); err != nil {
			t.Errorf("runExportTask md: %v", err)
		}
	})
	if !strings.Contains(out, "# Task t1") {
		t.Errorf("export task md output:\n%s", out)
	}
}

func TestRunExportTask_JSONFormat(t *testing.T) {
	orig := exportTaskFormat
	t.Cleanup(func() { exportTaskFormat = orig })
	exportTaskFormat = "json"

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.export", map[string]any{"id": "t1", "state": "finished"})

	out := captureStdout(t, func() {
		if err := runExportTask(nil, nil); err != nil {
			t.Errorf("runExportTask json: %v", err)
		}
	})
	if !strings.Contains(out, "t1") {
		t.Errorf("export task json output:\n%s", out)
	}
}

func TestRunBackup_JSON(t *testing.T) {
	setBoolPtr(t, &backupJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.create", map[string]any{"path": "/b/b.tar.gz", "size": 4096, "files": 12})

	out := captureStdout(t, func() {
		if err := runBackup(nil, nil); err != nil {
			t.Errorf("runBackup json: %v", err)
		}
	})
	if !strings.Contains(out, "path") {
		t.Errorf("backup json output:\n%s", out)
	}
}

func TestRunRestore_JSON(t *testing.T) {
	setBoolPtr(t, &restoreJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.restore", map[string]any{"target": "/t", "files": 5, "dirs": 2, "skipped": 1})

	out := captureStdout(t, func() {
		if err := runRestore(nil, []string{"/b/b.tar.gz"}); err != nil {
			t.Errorf("runRestore json: %v", err)
		}
	})
	if !strings.Contains(out, "target") {
		t.Errorf("restore json output:\n%s", out)
	}
}

func TestRunRestore_Skipped(t *testing.T) {
	setBoolPtr(t, &restoreJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.restore", map[string]any{"target": "/t", "files": 5, "dirs": 2, "skipped": 3})

	out := captureStdout(t, func() {
		if err := runRestore(nil, []string{"/b/b.tar.gz"}); err != nil {
			t.Errorf("runRestore skipped: %v", err)
		}
	})
	if !strings.Contains(out, "Skipped: 3") {
		t.Errorf("restore skipped output:\n%s", out)
	}
}

func TestRunReport_JSONFormat(t *testing.T) {
	orig := reportFormat
	t.Cleanup(func() { reportFormat = orig })
	reportFormat = "json"

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("report.generate", map[string]any{
		"report": map[string]any{"task_count": 10, "period": "2026-Q2"},
	})

	out := captureStdout(t, func() {
		if err := runReport(nil, nil); err != nil {
			t.Errorf("runReport json: %v", err)
		}
	})
	if !strings.Contains(out, "task_count") {
		t.Errorf("report json output:\n%s", out)
	}
}

func TestRunJobsGet_WithResultAndError(t *testing.T) {
	setBoolPtr(t, &jobsGetJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("jobs.get", map[string]any{
		"id": "j1", "type": "plan", "status": "failed",
		"worktree_id": "wt1", "created_at": "2026-05-01T10:00:00Z",
		"started_at": "2026-05-01T10:00:01Z", "completed_at": "2026-05-01T10:05:00Z",
		"error":  "agent crashed",
		"result": map[string]any{"output": "partial"},
	})

	out := captureStdout(t, func() {
		if err := runJobsGet(jobsGetCmd, []string{"j1"}); err != nil {
			t.Errorf("runJobsGet: %v", err)
		}
	})
	if !strings.Contains(out, "agent crashed") || !strings.Contains(out, "Result:") {
		t.Errorf("jobs get output:\n%s", out)
	}
}

func TestRunJobsGet_JSON(t *testing.T) {
	setBoolPtr(t, &jobsGetJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("jobs.get", map[string]any{"id": "j1", "type": "plan", "status": "running"})

	out := captureStdout(t, func() {
		if err := runJobsGet(jobsGetCmd, []string{"j1"}); err != nil {
			t.Errorf("runJobsGet json: %v", err)
		}
	})
	if !strings.Contains(out, "j1") {
		t.Errorf("jobs get json output:\n%s", out)
	}
}

func TestRunJobsList_JSON(t *testing.T) {
	setBoolPtr(t, &jobsListJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("jobs.list", map[string]any{"jobs": []any{
		map[string]any{"id": "j1", "type": "plan", "status": "running"},
	}})

	out := captureStdout(t, func() {
		if err := runJobsList(jobsListCmd, nil); err != nil {
			t.Errorf("runJobsList json: %v", err)
		}
	})
	if !strings.Contains(out, "jobs") {
		t.Errorf("jobs list json output:\n%s", out)
	}
}

func TestRunStatsProject_Empty(t *testing.T) {
	origH, origA := statsHistory, statsAll
	t.Cleanup(func() { statsHistory, statsAll = origH, origA })
	statsHistory, statsAll = false, false

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("task.history", map[string]any{"tasks": []any{}})

	out := captureStdout(t, func() {
		if err := runStats(StatsCmd, nil); err != nil {
			t.Errorf("runStats project empty: %v", err)
		}
	})
	if out == "" {
		t.Error("stats project empty produced no output")
	}
}

func TestRunCatalogList_JSON(t *testing.T) {
	setBoolPtr(t, &catalogListJSON, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("catalog.list", map[string]any{"templates": []any{
		map[string]any{"name": "bugfix", "description": "fix"},
	}})

	out := captureStdout(t, func() {
		if err := runCatalogList(nil, nil); err != nil {
			t.Errorf("runCatalogList json: %v", err)
		}
	})
	if !strings.Contains(out, "bugfix") {
		t.Errorf("catalog list json output:\n%s", out)
	}
}

func TestRunCatalogList_Templates(t *testing.T) {
	setBoolPtr(t, &catalogListJSON, false)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("catalog.list", map[string]any{"templates": []any{
		map[string]any{"name": "bugfix", "description": "fix a bug", "source": "github"},
	}})

	out := captureStdout(t, func() {
		if err := runCatalogList(nil, nil); err != nil {
			t.Errorf("runCatalogList templates: %v", err)
		}
	})
	if !strings.Contains(out, "bugfix") {
		t.Errorf("catalog list templates output:\n%s", out)
	}
}
