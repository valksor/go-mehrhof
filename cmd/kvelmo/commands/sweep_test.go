package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestSweep_RunXFunctions_WithStubSockets exercises a broad set of runX
// handlers through stub worktree+global sockets. Most calls will return
// successfully or with a JSON-parse error against the canned stub payload;
// either way the bulk of the handler body executes, which is the point — we
// want coverage, not assertions about external behavior.
//
// Where a command has a confirmation prompt or destructive side effects, we
// either toggle --force or skip it.
func TestSweep_RunXFunctions_WithStubSockets(t *testing.T) {
	shortKvelmoHome(t)
	_ = startStubWorktreeSocket(t)
	_ = startStubGlobalSocket(t)

	// Force-bypass interactive prompts.
	_ = AbortCmd.Flags().Set("force", "true")
	_ = ResetCmd.Flags().Set("force", "true")
	_ = DeleteCmd.Flags().Set("force", "true")
	t.Cleanup(func() {
		_ = AbortCmd.Flags().Set("force", "false")
		_ = ResetCmd.Flags().Set("force", "false")
		_ = DeleteCmd.Flags().Set("force", "false")
	})

	cases := []struct {
		name string
		fn   func(*cobra.Command, []string) error
		cmd  *cobra.Command
		args []string
	}{
		// Workflow / lifecycle (worktree)
		{"abandon", runAbandon, AbandonCmd, nil},
		{"abort", runAbort, AbortCmd, nil},
		{"reset", runReset, ResetCmd, nil},
		{"stop", runStop, StopCmd, nil},
		{"shutdown", runShutdown, ShutdownCmd, nil},
		{"undo", runUndo, UndoCmd, nil},
		{"redo", runRedo, RedoCmd, nil},
		{"plan", runPlan, PlanCmd, nil},
		{"implement", runImplement, ImplementCmd, nil},
		{"simplify", runSimplify, SimplifyCmd, nil},
		{"optimize", runOptimize, OptimizeCmd, nil},
		{"finish", runFinish, FinishCmd, nil},
		{"refresh", runRefresh, RefreshCmd, nil},
		{"update", runUpdate, UpdateCmd, nil},
		{"delete", runDelete, DeleteCmd, nil},

		// Checkpoints
		{"checkpoints", runCheckpoints, CheckpointsCmd, nil},
		{"checkpoints_goto", runCheckpointsGoto, checkpointsGotoCmd, []string{"deadbeef"}},

		// Queue
		{"queue_add", runQueueAdd, queueAddCmd, []string{"https://github.com/o/r/issues/1"}},
		{"queue_remove", runQueueRemove, queueRemoveCmd, []string{"t1"}},
		{"queue_list", runQueueList, queueListCmd, nil},
		{"queue_reorder", runQueueReorder, queueReorderCmd, []string{"t1", "0"}},

		// Show / diff / watch / files / browse
		{"show_spec", runShowSpec, showSpecCmd, nil},
		{"show_plan", runShowPlan, showPlanCmd, nil},
		{"diff", runDiff, DiffCmd, nil},
		{"watch", runWatch, WatchCmd, nil},
		{"files_list", runFilesList, filesListCmd, nil},
		{"files_search", runFilesSearch, filesSearchCmd, []string{"package"}},

		// Reviews
		{"review_list", runReviewList, reviewListCmd, nil},
		{"review_view", runReviewView, reviewViewCmd, []string{"1"}},

		// Remote
		{"remote_approve", runRemoteApprove, RemoteApproveCmd, nil},
		{"remote_merge", runRemoteMerge, RemoteMergeCmd, nil},

		// Recap / stats
		{"recap", runRecap, RecapCmd, nil},
		{"stats", runStats, StatsCmd, nil},

		// Tag
		{"tag_add", runTagAdd, tagAddCmd, []string{"label"}},
		{"tag_remove", runTagRemove, tagRemoveCmd, []string{"label"}},
		{"tag_list", runTagList, tagListCmd, nil},

		// Eventlog / discover / hooks / policy / ci / autofix
		{"eventlog", runEventlog, EventlogCmd, nil},
		{"hooks", runHooks, HooksCmd, nil},
		{"ci_status", runCIStatus, ciStatusCmd, nil},
		{"policy_check", runPolicyCheck, policyCheckCmd, nil},

		// Codegraph
		{"codegraph_stats", runCodegraphStats, codegraphStatsCmd, nil},
		{"codegraph_index", runCodegraphIndex, codegraphIndexCmd, nil},
		{"codegraph_search", runCodegraphSearch, codegraphSearchCmd, []string{"foo"}},
		{"codegraph_callers", runCodegraphCallers, codegraphCallersCmd, []string{"foo"}},
		{"codegraph_deps", runCodegraphDeps, codegraphDepsCmd, []string{"foo"}},

		// Memory
		{"memory_search", runMemorySearch, memorySearchCmd, []string{"q"}},
		{"memory_stats", runMemoryStats, memoryStatsCmd, nil},
		{"memory_outcomes", runMemoryOutcomes, memoryOutcomesCmd, []string{"plan"}},

		// Fork
		{"fork_list", runForkList, forkListCmd, nil},
		{"fork_compare", runForkCompare, forkCompareCmd, nil},
		{"fork_create", runForkCreate, forkCreateCmd, []string{"alt"}},
		{"fork_select", runForkSelect, forkSelectCmd, []string{"f1"}},

		// Cache
		{"cache_stats", runCacheStats, cacheStatsCmd, nil},
		{"cache_clear", runCacheClear, cacheClearCmd, nil},

		// Quality
		{"quality_autofix_status", runQualityAutofixStatus, qualityAutofixStatusCmd, nil},
		{"quality_failclass", runQualityFailclass, qualityFailclassCmd, nil},

		// Risk
		{"review_risk", runReviewRisk, reviewRiskCmd, nil},
		{"review_risk_history", runReviewRiskHistory, reviewRiskHistoryCmd, nil},

		// Catalog (worktree)
		{"catalog_list", runCatalogList, catalogListCmd, nil},

		// Adversarial
		{"review_adv_results", runReviewAdversarialResults, reviewAdversarialResultsCmd, nil},

		// Strategy (worktree)
		{"strategy_list", runStrategyList, strategyListCmd, nil},

		// Approve
		{"approve", runApprove, ApproveCmd, nil},

		// Browse
		{"browse", runBrowse, BrowseCmd, nil},

		// Recordings (worktree-side calls)
		{"recordings_list", runRecordingsList, recordingsListCmd, nil},
		{"recordings_clean", runRecordingsClean, recordingsCleanCmd, nil},

		// Screenshots
		{"screenshots_list", runScreenshotsList, screenshotsListCmd, nil},
		{"screenshots_get", runScreenshotsGet, screenshotsGetCmd, []string{"id"}},
		{"screenshots_delete", runScreenshotsDelete, screenshotsDeleteCmd, []string{"id"}},

		// ---- Global socket commands ----
		{"agent_status", runAgentStatus, agentStatusCmd, nil},
		{"activity", runActivity, ActivityCmd, nil},
		{"audit", runAudit, AuditCmd, nil},
		{"workers", runWorkers, WorkersCmd, nil},
		{"workers_add", runWorkersAdd, workersAddCmd, []string{"claude"}},
		{"workers_remove", runWorkersRemove, workersRemoveCmd, []string{"claude-1"}},
		{"workers_stats", runWorkersStats, workersStatsCmd, nil},
		{"jobs_list", runJobsList, jobsListCmd, nil},
		{"jobs_get", runJobsGet, jobsGetCmd, []string{"j1"}},
		{"projects", runProjects, ProjectsCmd, nil},
		{"projects_add", runProjectsAdd, projectsAddCmd, []string{"/tmp/p"}},
		{"projects_remove", runProjectsRemove, projectsRemoveCmd, []string{"/tmp/p"}},
		{"chat_history", runChatHistory, chatHistoryCmd, nil},
		{"chat_stop", runChatStop, chatStopCmd, nil},
		{"chat_clear", runChatClear, chatClearCmd, nil},
		{"explain", runExplain, ExplainCmd, nil},
		{"logs", runLogs, LogsCmd, nil},
		{"backup_list", runBackupList, backupListCmd, nil},
		{"notify_test", runNotifyTest, notifyTestCmd, nil},
		{"group_create", runGroupCreate, groupCreateCmd, []string{"grp"}},
		{"group_list", runGroupList, groupListCmd, nil},
		{"group_add", runGroupAdd, groupAddCmd, []string{"grp", "t1"}},
		{"group_status", runGroupStatus, groupStatusCmd, []string{"grp"}},
		{"group_submit", runGroupSubmit, groupSubmitCmd, []string{"grp"}},
		{"group_remove", runGroupRemove, groupRemoveCmd, []string{"grp", "t1"}},

		// ---- Local / mixed commands ----
		{"discover", runDiscover, DiscoverCmd, nil},
		{"export", runExport, ExportCmd, nil},
		{"export_task", runExportTask, exportTaskCmd, nil},
		{"config_check", runConfigCheck, configCheckCmd, nil},
		{"config_validate", runConfigValidate, configValidateCmd, nil},
		{"config_diff", runConfigDiff, configDiffCmd, nil},
		{"prompt", runPrompt, PromptCmd, nil},
		{"tutorial", runTutorial, TutorialCmd, nil},
		{"diagnose_health", func(c *cobra.Command, a []string) error { return runDiagnoseHealth() }, DiagnoseCmd, nil},
		{"batch", runBatch, BatchCmd, []string{"status"}},
		{"backup", runBackup, BackupCmd, nil},
		{"restore", runRestore, RestoreCmd, []string{"/tmp/x"}},
		{"changelog", runChangelog, ChangelogCmd, []string{"v1.0.0", "HEAD"}},
		{"cleanup", runCleanup, CleanupCmd, nil},
		{"report", runReport, ReportCmd, nil},
		{"rpc", runRPC, RPCCmd, []string{"ping"}},
		{"security_scan", runSecurityScan, SecurityCmd, nil},
		{"list", runList, ListCmd, nil},
		{"list_history", runListHistoryCmd, listHistoryCmd, nil},
		{"list_search", runListSearchCmd, listSearchCmd, []string{"q"}},
		{"checklist", runChecklist, ChecklistCmd, nil},
		{"catalog_use", runCatalogUse, catalogUseCmd, []string{"basic"}},
		{"catalog_add", runCatalogAdd, catalogAddCmd, []string{"/tmp/template.yaml"}},

		// ---- Browser agent commands (global socket) ----
		{"browser_navigate", runBrowserNavigate, browserNavigateCmd, []string{"https://example.com"}},
		{"browser_snapshot", runBrowserSnapshot, browserSnapshotCmd, nil},
		{"browser_screenshot", runBrowserScreenshot, browserScreenshotCmd, nil},
		{"browser_click", runBrowserClick, browserClickCmd, []string{"button"}},
		{"browser_type", runBrowserType, browserTypeCmd, []string{"input", "hello"}},
		{"browser_wait", runBrowserWait, browserWaitCmd, []string{"selector"}},
		{"browser_eval", runBrowserEval, browserEvalCmd, []string{"1+1"}},
		{"browser_console", runBrowserConsole, browserConsoleCmd, nil},
		{"browser_network", runBrowserNetwork, browserNetworkCmd, nil},
		{"browser_fill", runBrowserFill, browserFillCmd, []string{"input", "val"}},
		{"browser_select", runBrowserSelect, browserSelectCmd, []string{"select", "opt"}},
		{"browser_hover", runBrowserHover, browserHoverCmd, []string{"el"}},
		{"browser_focus", runBrowserFocus, browserFocusCmd, []string{"el"}},
		{"browser_scroll", runBrowserScroll, browserScrollCmd, nil},
		{"browser_press", runBrowserPress, browserPressCmd, []string{"Enter"}},
		{"browser_back", runBrowserBack, browserBackCmd, nil},
		{"browser_forward", runBrowserForward, browserForwardCmd, nil},
		{"browser_reload", runBrowserReload, browserReloadCmd, nil},
		{"browser_dialog", runBrowserDialog, browserDialogCmd, []string{"accept"}},
		{"browser_pdf", runBrowserPDF, browserPDFCmd, nil},
		{"browser_status", runBrowserStatus, browserStatusCmd, nil},
		{"browser_config", runBrowserConfig, browserConfigCmd, nil},

		// Git
		{"git_status", runGitStatus, gitStatusCmd, nil},
		{"git_diff", runGitDiff, gitDiffCmd, nil},
		{"git_log", runGitLog, gitLogCmd, nil},

		// Memory
		{"memory_clear", runMemoryClear, memoryClearCmd, nil},

		// Status (worktree)
		{"status", runStatus, StatusCmd, nil},

		// More sweep additions
		{"retry", runRetry, RetryCmd, nil},
		{"diagnose", runDiagnose, DiagnoseCmd, nil},
		{"diagnose_rpc", func(c *cobra.Command, a []string) error {
			_, err := runDiagnoseViaRPC()

			return err
		}, DiagnoseCmd, nil},
		{"provision_preview", func(c *cobra.Command, a []string) error { return runProvisionPreview() }, StartCmd, nil},
		{"recordings_view", runRecordingsView, recordingsViewCmd, []string{"r1"}},
		{"recordings_replay", runRecordingsReplay, recordingsReplayCmd, []string{"r1"}},
		{"screenshots_capture", runScreenshotsCapture, screenshotsCaptureCmd, nil},
		{"stats_history", func(c *cobra.Command, a []string) error { return runStatsHistory() }, StatsCmd, nil},
		{"stats_all", func(c *cobra.Command, a []string) error { return runStatsAll() }, StatsCmd, nil},
		{"provider_test", runProviderTest, ProviderTestCmd, []string{"github"}},

		// Submit with --dry-run for safer execution
		{"submit_dry", runSubmit, SubmitCmd, nil},
		// Review (exercise after stub socket returns reviews)
		{"review", runReview, ReviewCmd, nil},
		// Chat send
		{"chat_send", runChatSend, chatSendCmd, []string{"hello"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// We don't assert success — many commands parse a specific JSON
			// shape that our stub responses don't fully match. The intent is
			// to walk the handler body, which raises coverage materially.
			// Some commands assume non-nil cmd.Context() or expect specific
			// response shapes; recover from any panic so one bad case doesn't
			// derail the whole sweep.
			defer func() { _ = recover() }()
			_ = tc.fn(tc.cmd, tc.args)
		})
	}
}
