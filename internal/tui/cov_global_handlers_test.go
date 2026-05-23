package tui

import (
	"context"
	"strings"
	"testing"
)

// glCall invokes a global handler against a stub server with the given canned
// results and returns the output string. Fails the test on handler error.
func glCall(t *testing.T, h globalHandler, args string, results map[string]any) string {
	t.Helper()
	s := newStubServer(t)
	for method, res := range results {
		s.on(method, res)
	}
	out, err := h(context.Background(), s.client(t), args)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	return out
}

func TestGlJobs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := glCall(t, glJobs, "", map[string]any{"jobs.list": map[string]any{"jobs": []any{}}})
		if out != "No jobs." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("listed", func(t *testing.T) {
		out := glCall(t, glJobs, "", map[string]any{
			"jobs.list": map[string]any{"jobs": []map[string]any{{"id": "job12345678", "type": "plan", "status": "running"}}},
		})
		if out != "job12345 [running] plan" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlStats(t *testing.T) {
	out := glCall(t, glStats, "", map[string]any{"metrics": map[string]any{"jobs": 5}})
	if !strings.Contains(out, "jobs") {
		t.Errorf("out = %q", out)
	}
}

func TestGlWorkers(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := glCall(t, glWorkers, "", map[string]any{"workers.list": map[string]any{"workers": []any{}}})
		if out != "No workers." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("listed", func(t *testing.T) {
		out := glCall(t, glWorkers, "", map[string]any{
			"workers.list": map[string]any{"workers": []map[string]any{{"name": "w1", "state": "working"}}},
		})
		if out != "w1 [working]" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlMemory(t *testing.T) {
	t.Run("search usage", func(t *testing.T) {
		if out := glCall(t, glMemorySearch, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("search none", func(t *testing.T) {
		out := glCall(t, glMemorySearch, "query", map[string]any{"memory.search": map[string]any{"results": []any{}}})
		if out != "No results." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("search results", func(t *testing.T) {
		out := glCall(t, glMemorySearch, "query", map[string]any{
			"memory.search": map[string]any{"results": []map[string]any{{"content": "match", "score": 0.95}}},
		})
		if !strings.Contains(out, "(0.95) match") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("stats", func(t *testing.T) {
		out := glCall(t, glMemoryStats, "", map[string]any{"memory.stats": map[string]any{"count": 3}})
		if !strings.Contains(out, "count") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("clear", func(t *testing.T) {
		out := glCall(t, glMemoryClear, "", map[string]any{"memory.clear": map[string]any{}})
		if out != "Memory cleared." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlBatch(t *testing.T) {
	t.Run("usage", func(t *testing.T) {
		if out := glCall(t, glBatch, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("results", func(t *testing.T) {
		out := glCall(t, glBatch, "plan", map[string]any{
			"tasks.batch": map[string]any{
				"total":   3,
				"results": []map[string]any{{"success": true}, {"success": false}, {"success": true}},
			},
		})
		if out != "Batch plan: 2/3 succeeded." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlReport(t *testing.T) {
	t.Run("with report", func(t *testing.T) {
		out := glCall(t, glReport, "", map[string]any{"report.generate": map[string]any{"report": "## Report"}})
		if out != "## Report" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("empty", func(t *testing.T) {
		out := glCall(t, glReport, "", map[string]any{"report.generate": map[string]any{}})
		if out != "Report generated." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlBackup(t *testing.T) {
	out := glCall(t, glBackup, "", map[string]any{"backup.create": map[string]any{"path": "/tmp/bk.tar"}})
	if out != "Backup created: /tmp/bk.tar" {
		t.Errorf("out = %q", out)
	}
}

func TestGlGroups(t *testing.T) {
	t.Run("create usage", func(t *testing.T) {
		if out := glCall(t, glGroupCreate, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("create", func(t *testing.T) {
		out := glCall(t, glGroupCreate, "release", map[string]any{"taskgroup.create": map[string]any{"id": "grp123"}})
		if out != "Group created: grp123" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list empty", func(t *testing.T) {
		out := glCall(t, glGroupList, "", map[string]any{"taskgroup.list": map[string]any{"groups": []any{}}})
		if out != "No groups." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list", func(t *testing.T) {
		out := glCall(t, glGroupList, "", map[string]any{
			"taskgroup.list": map[string]any{"groups": []map[string]any{{"id": "grp12345678", "label": "release"}}},
		})
		if out != "grp12345 — release" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("status usage", func(t *testing.T) {
		if out := glCall(t, glGroupStatus, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("status", func(t *testing.T) {
		out := glCall(t, glGroupStatus, "grp1", map[string]any{"taskgroup.status": map[string]any{"state": "ready"}})
		if !strings.Contains(out, "ready") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("add usage", func(t *testing.T) {
		if out := glCall(t, glGroupAdd, "grponly", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("add", func(t *testing.T) {
		out := glCall(t, glGroupAdd, "grp12345678 task1", map[string]any{"taskgroup.add": map[string]any{}})
		if out != "Task added to group grp12345." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("submit usage", func(t *testing.T) {
		if out := glCall(t, glGroupSubmit, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("submit", func(t *testing.T) {
		out := glCall(t, glGroupSubmit, "grp12345678", map[string]any{"taskgroup.submit": map[string]any{}})
		if out != "Group grp12345 submitted." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("remove usage", func(t *testing.T) {
		if out := glCall(t, glGroupRemove, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("remove", func(t *testing.T) {
		out := glCall(t, glGroupRemove, "grp12345678", map[string]any{"taskgroup.remove": map[string]any{}})
		if out != "Group grp12345 removed." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlDiagnose(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		out := glCall(t, glDiagnose, "", map[string]any{"system.diagnose": map[string]any{"checks": []any{}}})
		if out != "Diagnostics: OK" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("checks", func(t *testing.T) {
		out := glCall(t, glDiagnose, "", map[string]any{
			"system.diagnose": map[string]any{"checks": []map[string]any{
				{"name": "socket", "status": "passed"},
				{"name": "disk", "status": "failed", "detail": "low space"},
			}},
		})
		if !strings.Contains(out, "✓ socket") {
			t.Errorf("out = %q", out)
		}
		if !strings.Contains(out, "✗ disk: low space") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlSecurityScan(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		out := glCall(t, glSecurityScan, "", map[string]any{"security.scan": map[string]any{"issues": []any{}}})
		if out != "No security issues found." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("issues", func(t *testing.T) {
		out := glCall(t, glSecurityScan, "", map[string]any{
			"security.scan": map[string]any{"issues": []map[string]any{{"severity": "high", "message": "leaked key"}}},
		})
		if out != "[high] leaked key" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlConfigCheck(t *testing.T) {
	t.Run("no drift", func(t *testing.T) {
		out := glCall(t, glConfigCheck, "", map[string]any{"config.check": map[string]any{"count": 0}})
		if out != "Configuration: no drift detected." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("drift", func(t *testing.T) {
		out := glCall(t, glConfigCheck, "", map[string]any{
			"config.check": map[string]any{
				"count":  1,
				"drifts": []map[string]any{{"path": "agent.model", "expected": "claude", "actual": "codex"}},
			},
		})
		if !strings.Contains(out, "Configuration drift:") || !strings.Contains(out, "agent.model") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlConfigShow(t *testing.T) {
	out := glCall(t, glConfigShow, "", map[string]any{"settings.get": map[string]any{"effective": map[string]any{"port": 6337}}})
	if !strings.Contains(out, "6337") {
		t.Errorf("out = %q", out)
	}
}

func TestGlConfigValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		out := glCall(t, glConfigValidate, "", map[string]any{
			"config.validate": map[string]any{
				"valid":  true,
				"checks": []map[string]any{{"name": "socket", "status": "ok"}},
			},
		})
		if !strings.Contains(out, "Configuration valid") || !strings.Contains(out, "[PASS] socket") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("invalid with fix", func(t *testing.T) {
		out := glCall(t, glConfigValidate, "", map[string]any{
			"config.validate": map[string]any{
				"valid": false,
				"checks": []map[string]any{
					{"name": "agent", "status": "error", "detail": "missing", "fix": "install it"},
					{"name": "token", "status": "warning", "detail": "expiring"},
				},
			},
		})
		if !strings.Contains(out, "Configuration INVALID") {
			t.Errorf("out = %q", out)
		}
		if !strings.Contains(out, "[FAIL] agent — missing") || !strings.Contains(out, "Fix: install it") {
			t.Errorf("out = %q", out)
		}
		if !strings.Contains(out, "[WARN] token — expiring") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlStrategy(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := glCall(t, glStrategy, "", map[string]any{"strategy.list": []any{}})
		if out != "No strategies registered." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("listed", func(t *testing.T) {
		out := glCall(t, glStrategy, "", map[string]any{"strategy.list": []string{"react", "reflexion"}})
		if !strings.Contains(out, "Available strategies:") || !strings.Contains(out, "- react") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlRestore(t *testing.T) {
	t.Run("usage", func(t *testing.T) {
		if out := glCall(t, glRestore, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("restore", func(t *testing.T) {
		out := glCall(t, glRestore, "/tmp/bk.tar", map[string]any{"backup.restore": map[string]any{}})
		if out != "Restored from /tmp/bk.tar." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlCatalog(t *testing.T) {
	t.Run("list empty", func(t *testing.T) {
		out := glCall(t, glCatalogList, "", map[string]any{"catalog.list": map[string]any{"templates": []any{}}})
		if out != "No templates in catalog." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list", func(t *testing.T) {
		out := glCall(t, glCatalogList, "", map[string]any{
			"catalog.list": map[string]any{"templates": []map[string]any{{"name": "bugfix", "description": "Fix a bug"}}},
		})
		if !strings.Contains(out, "bugfix — Fix a bug") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("use usage", func(t *testing.T) {
		if out := glCall(t, glCatalogUse, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("use no source", func(t *testing.T) {
		out := glCall(t, glCatalogUse, "bugfix", map[string]any{"catalog.get": map[string]any{"name": "bugfix"}})
		if out != `Template "bugfix" has no source configured.` {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("use with source", func(t *testing.T) {
		out := glCall(t, glCatalogUse, "bugfix", map[string]any{"catalog.get": map[string]any{"name": "bugfix", "source": "gh#1"}})
		if !strings.Contains(out, "source: gh#1") || !strings.Contains(out, "/start gh#1") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlWorkersManagement(t *testing.T) {
	t.Run("add usage", func(t *testing.T) {
		if out := glCall(t, glWorkersAdd, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("add", func(t *testing.T) {
		out := glCall(t, glWorkersAdd, "claude", map[string]any{"workers.add": map[string]any{}})
		if out != `Worker "claude" added.` {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("remove usage", func(t *testing.T) {
		if out := glCall(t, glWorkersRemove, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("remove", func(t *testing.T) {
		out := glCall(t, glWorkersRemove, "w1", map[string]any{"workers.remove": map[string]any{}})
		if out != `Worker "w1" removed.` {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlRPCLog(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := glCall(t, glRPCLog, "", map[string]any{"activity.query": map[string]any{"entries": []any{}}})
		if out != "No log entries." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("entries with defaults", func(t *testing.T) {
		out := glCall(t, glRPCLog, "", map[string]any{
			"activity.query": map[string]any{"entries": []map[string]any{
				{"timestamp": "10:00", "method": "plan"},
				{"timestamp": "10:01", "level": "ERROR", "message": "boom"},
			}},
		})
		if !strings.Contains(out, "[10:00] [INFO] plan") {
			t.Errorf("default level/message missing: %q", out)
		}
		if !strings.Contains(out, "[10:01] [ERROR] boom") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlAgent(t *testing.T) {
	t.Run("available", func(t *testing.T) {
		out := glCall(t, glAgent, "", map[string]any{
			"agent.status": map[string]any{
				"agent_available": true,
				"checks":          []map[string]any{{"name": "binary", "status": "ok", "detail": "v2"}},
			},
		})
		if !strings.Contains(out, "Agent: available") || !strings.Contains(out, "binary: OK (v2)") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("simulation", func(t *testing.T) {
		out := glCall(t, glAgent, "", map[string]any{"agent.status": map[string]any{"simulation_mode": true}})
		if !strings.Contains(out, "Agent: simulation mode") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("not available", func(t *testing.T) {
		out := glCall(t, glAgent, "", map[string]any{"agent.status": map[string]any{}})
		if !strings.Contains(out, "Agent: not available") {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlProjects(t *testing.T) {
	t.Run("list empty", func(t *testing.T) {
		out := glCall(t, glProjectsList, "", map[string]any{"projects.list": map[string]any{"projects": []any{}}})
		if out != "No projects registered." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("list", func(t *testing.T) {
		out := glCall(t, glProjectsList, "", map[string]any{
			"projects.list": map[string]any{"projects": []map[string]any{
				{"id": "proj12345678", "name": "named"},
				{"id": "short", "path": "/p/path"},
			}},
		})
		if !strings.Contains(out, "proj1234 named") {
			t.Errorf("out = %q", out)
		}
		if !strings.Contains(out, "short /p/path") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("unregister usage", func(t *testing.T) {
		if out := glCall(t, glProjectsUnregister, "", nil); !strings.Contains(out, "Usage:") {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("unregister", func(t *testing.T) {
		out := glCall(t, glProjectsUnregister, "proj1", map[string]any{"projects.unregister": map[string]any{}})
		if out != "Project proj1 unregistered." {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlRecordings(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out := glCall(t, glRecordings, "", map[string]any{"recordings.list": map[string]any{"recordings": []any{}}})
		if out != "No recordings." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("listed", func(t *testing.T) {
		out := glCall(t, glRecordings, "", map[string]any{
			"recordings.list": map[string]any{"recordings": []map[string]any{{"id": "rec123456789", "path": "/r/a.json", "created_at": "today"}}},
		})
		if out != "rec12345 /r/a.json (today)" {
			t.Errorf("out = %q", out)
		}
	})
}

func TestGlNotifyTest(t *testing.T) {
	t.Run("delivered", func(t *testing.T) {
		out := glCall(t, glNotifyTest, "", map[string]any{"notify.test": map[string]any{"sent": 2}})
		if out != "Notification test sent (2 delivered)." {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("with message", func(t *testing.T) {
		out := glCall(t, glNotifyTest, "", map[string]any{"notify.test": map[string]any{"message": "no webhooks"}})
		if out != "Notification test: no webhooks" {
			t.Errorf("out = %q", out)
		}
	})
	t.Run("complete", func(t *testing.T) {
		out := glCall(t, glNotifyTest, "", map[string]any{"notify.test": map[string]any{}})
		if out != "Notification test complete." {
			t.Errorf("out = %q", out)
		}
	})
}
