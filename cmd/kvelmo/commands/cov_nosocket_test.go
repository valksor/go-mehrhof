package commands

import (
	"strings"
	"testing"
)

// TestWorktreeCommands_NoSocket drives the no-worktree-socket error branch for
// every worktree-socket command (the dial/SocketExists failure path).
func TestWorktreeCommands_NoSocket(t *testing.T) {
	cases := []struct {
		name string
		fn   func() error
	}{
		{"plan", func() error { return runPlan(PlanCmd, nil) }},
		{"implement", func() error { return runImplement(ImplementCmd, nil) }},
		{"simplify", func() error { return runSimplify(SimplifyCmd, nil) }},
		{"optimize", func() error { return runOptimize(OptimizeCmd, nil) }},
		{"submit", func() error { return runSubmit(SubmitCmd, nil) }},
		{"undo", func() error { return runUndo(UndoCmd, nil) }},
		{"redo", func() error { return runRedo(RedoCmd, nil) }},
		{"abandon", func() error { return runAbandon(AbandonCmd, nil) }},
		{"delete", func() error { return runDelete(DeleteCmd, nil) }},
		{"update", func() error { return runUpdate(UpdateCmd, nil) }},
		{"finish", func() error { return runFinish(FinishCmd, nil) }},
		{"refresh", func() error { return runRefresh(RefreshCmd, nil) }},
		{"checkpoints", func() error { return runCheckpoints(CheckpointsCmd, nil) }},
		{"checkpointsgoto", func() error { return runCheckpointsGoto(checkpointsGotoCmd, []string{"abc"}) }},
		{"eventlog", func() error { return runEventlog(EventlogCmd, nil) }},
		{"ci", func() error { return runCIStatus(CICmd, nil) }},
		{"hooks", func() error { return runHooks(HooksCmd, nil) }},
		{"discover", func() error { return runDiscover(DiscoverCmd, nil) }},
		{"cachestats", func() error { return runCacheStats(nil, nil) }},
		{"cacheclear", func() error { return runCacheClear(nil, nil) }},
		{"quality", func() error { return runQuality(QualityCmd, nil) }},
		{"qualityfailclass", func() error { return runQualityFailclass(nil, nil) }},
		{"codegraphstats", func() error { return runCodegraphStats(nil, nil) }},
		{"codegraphindex", func() error { return runCodegraphIndex(CodegraphCmd, nil) }},
		{"codegraphsearch", func() error { return runCodegraphSearch(nil, []string{"x"}) }},
		{"codegraphcallers", func() error { return runCodegraphCallers(nil, []string{"x"}) }},
		{"codegraphdeps", func() error { return runCodegraphDeps(nil, []string{"x"}) }},
		{"forkcreate", func() error { return runForkCreate(nil, []string{"x"}) }},
		{"forklist", func() error { return runForkList(nil, nil) }},
		{"forkcompare", func() error { return runForkCompare(nil, nil) }},
		{"forkselect", func() error { return runForkSelect(nil, []string{"f1"}) }},
		{"policycheck", func() error { return runPolicyCheck(nil, nil) }},
		{"recap", func() error { return runRecap(RecapCmd, nil) }},
		{"showspec", func() error { return runShowSpec(showSpecCmd, nil) }},
		{"showplan", func() error { return runShowPlan(showPlanCmd, nil) }},
		{"reviewlist", func() error { return runReviewList(ReviewCmd, nil) }},
		{"reviewrisk", func() error { return runReviewRisk(nil, nil) }},
		{"reviewriskhistory", func() error { return runReviewRiskHistory(nil, nil) }},
		{"tagadd", func() error { return runTagAdd(tagAddCmd, []string{"x"}) }},
		{"taglist", func() error { return runTagList(tagListCmd, nil) }},
		{"approve", func() error { return runApprove(ApproveCmd, []string{"submit"}) }},
		{"checklist", func() error { return runChecklist(ChecklistCmd, nil) }},
		{"diff", func() error { return runDiff(DiffCmd, nil) }},
		{"exporttask", func() error { return runExportTask(nil, nil) }},
		{"report", func() error { return runReport(nil, nil) }},
		{"taskhistory", func() error { return runTaskHistory(false) }},
		{"statsproject", func() error { return runStatsProject() }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			if err := c.fn(); err == nil {
				t.Errorf("%s: expected error with no worktree socket", c.name)
			}
		})
	}
}

// TestRunQualityRespond_FlagValidation covers the --yes/--no validation errors.
func TestRunQualityRespond_FlagValidation(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	// Neither flag set → error.
	if err := runQualityRespond(qualityRespondCmd, nil); err == nil {
		t.Error("expected error when neither --yes nor --no set")
	}

	// Both flags set → error.
	if err := qualityRespondCmd.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}
	if err := qualityRespondCmd.Flags().Set("no", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = qualityRespondCmd.Flags().Set("yes", "false")
		_ = qualityRespondCmd.Flags().Set("no", "false")
	})
	if err := runQualityRespond(qualityRespondCmd, nil); err == nil {
		t.Error("expected error when both --yes and --no set")
	}
}

// TestGlobalCommands_NoSocket drives the no-global-socket error branch.
func TestGlobalCommands_NoSocket(t *testing.T) {
	cases := []struct {
		name string
		fn   func() error
	}{
		{"list", func() error { return runList(ListCmd, nil) }},
		{"projects", func() error { return runProjects(ProjectsCmd, nil) }},
		{"workers", func() error { return runWorkers(WorkersCmd, nil) }},
		{"agent", func() error { return runAgentStatus(AgentCmd, nil) }},
		{"strategy", func() error { return runStrategyList(StrategyCmd, nil) }},
		{"jobslist", func() error { return runJobsList(jobsListCmd, nil) }},
		{"memoryoutcomes", func() error { return runMemoryOutcomes(nil, nil) }},
		{"security", func() error { return runSecurityScan(SecurityCmd, nil) }},
		{"backuplist", func() error { return runBackupList(nil, nil) }},
		{"catalog", func() error { return runCatalogList(nil, nil) }},
		{"catalcoguse", func() error { return runCatalogUse(nil, []string{"x"}) }},
		{"groupcreate", func() error { return runGroupCreate(nil, []string{"g"}) }},
		{"grouplist", func() error { return runGroupList(nil, nil) }},
		{"groupstatus", func() error { return runGroupStatus(nil, []string{"g"}) }},
		{"recordingslist", func() error { return runRecordingsList(recordingsListCmd, nil) }},
		{"recordingsview", func() error { return runRecordingsView(recordingsViewCmd, []string{"r"}) }},
		{"export", func() error { return runExport(nil, nil) }},
		{"audit", func() error { return runAudit(AuditCmd, nil) }},
		{"activity", func() error { return runActivity(ActivityCmd, nil) }},
		{"configcheck", func() error { return runConfigCheck(nil, nil) }},
		{"chathistory", func() error { return runChatHistory(chatHistoryCmd, nil) }},
		{"logs", func() error { return runLogs(LogsCmd, nil) }},
		{"diagnosehealth", func() error { return runDiagnoseHealth() }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			if err := c.fn(); err == nil {
				t.Errorf("%s: expected error with no global socket", c.name)
			}
		})
	}
}

// TestRunDiagnose_OfflineFull exercises the offline diagnostics path (no socket)
// including the formatted output of preflight checks and provider tokens.
func TestRunDiagnose_OfflineFull(t *testing.T) {
	origJSON, origHealth := diagnoseJSON, diagnoseHealth
	t.Cleanup(func() { diagnoseJSON, diagnoseHealth = origJSON, origHealth })
	diagnoseJSON, diagnoseHealth = false, false

	shortKvelmoHome(t)
	chdirToShortTemp(t)

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose offline: %v", err)
		}
	})
	if !strings.Contains(out, "Diagnostics") || !strings.Contains(out, "Providers:") {
		t.Errorf("diagnose offline output:\n%s", out)
	}
}

func TestRunDiagnose_OfflineJSON(t *testing.T) {
	origJSON := diagnoseJSON
	t.Cleanup(func() { diagnoseJSON = origJSON })
	diagnoseJSON = true

	shortKvelmoHome(t)
	chdirToShortTemp(t)

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose offline json: %v", err)
		}
	})
	if !strings.Contains(out, "\"global_socket\"") {
		t.Errorf("diagnose offline json output:\n%s", out)
	}
}

// TestRunDiagnoseViaRPC_Server exercises the server-side diagnose display path.
func TestRunDiagnoseViaRPC_Server(t *testing.T) {
	origJSON := diagnoseJSON
	t.Cleanup(func() { diagnoseJSON = origJSON })
	diagnoseJSON = false

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("system.diagnose", map[string]any{
		"checks": []any{
			map[string]any{"name": "git", "status": "ok", "detail": "2.40"},
			map[string]any{"name": "claude", "status": "fail"},
		},
		"global_socket": "running",
		"providers": []any{
			map[string]any{"name": "GitHub", "configured": true},
			map[string]any{"name": "GitLab", "configured": false},
		},
		"issues": []any{"do the thing"},
	})

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose via rpc: %v", err)
		}
	})
	if !strings.Contains(out, "via server") || !strings.Contains(out, "Next steps") {
		t.Errorf("diagnose via rpc output:\n%s", out)
	}
}

func TestRunDiagnoseHealth_WithSocket(t *testing.T) {
	origHealth := diagnoseHealth
	t.Cleanup(func() { diagnoseHealth = origHealth })
	diagnoseHealth = true

	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	healthy := true
	stub.SetResponse("system.health", map[string]any{
		"worktrees": []any{
			map[string]any{"id": "wt1", "path": "/p1", "state": "implementing", "healthy": &healthy},
		},
	})

	out := captureStdout(t, func() {
		if err := runDiagnose(DiagnoseCmd, nil); err != nil {
			t.Errorf("runDiagnose health: %v", err)
		}
	})
	if !strings.Contains(out, "Worktree health") {
		t.Errorf("diagnose health output:\n%s", out)
	}
	diagnoseHealth = false
}

// TestRunCleanup_GitFlag exercises the orphaned-worktree detection path.
func TestRunCleanup_GitFlag(t *testing.T) {
	if err := CleanupCmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	if err := CleanupCmd.Flags().Set("git", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = CleanupCmd.Flags().Set("dry-run", "false")
		_ = CleanupCmd.Flags().Set("git", "false")
	})

	shortKvelmoHome(t)
	chdirToShortTemp(t) // not a git repo → git.Open warns, continues

	out := captureStdout(t, func() {
		if err := runCleanup(CleanupCmd, nil); err != nil {
			t.Errorf("runCleanup git: %v", err)
		}
	})
	if !strings.Contains(out, "No stale sockets") {
		t.Errorf("cleanup git output = %q", out)
	}
}
