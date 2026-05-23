package commands

import (
	"testing"
)

// TestWorktreeCommands_ServerErrors drives the error branch of every
// worktree-socket command by making the underlying RPC method return a
// JSON-RPC error. Each command must surface (not swallow) the error.
func TestWorktreeCommands_ServerErrors(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"implement", "implement", func() error { return runImplement(ImplementCmd, nil) }},
		{"simplify", "simplify", func() error { return runSimplify(SimplifyCmd, nil) }},
		{"optimize", "optimize", func() error { return runOptimize(OptimizeCmd, nil) }},
		{"submit", "submit", func() error { return runSubmit(SubmitCmd, nil) }},
		{"redo", "redo", func() error { return runRedo(RedoCmd, nil) }},
		{"abort", "abort", func() error {
			_ = AbortCmd.Flags().Set("force", "true")
			defer func() { _ = AbortCmd.Flags().Set("force", "false") }()

			return runAbort(AbortCmd, nil)
		}},
		{"reset", "reset", func() error {
			_ = ResetCmd.Flags().Set("force", "true")
			defer func() { _ = ResetCmd.Flags().Set("force", "false") }()

			return runReset(ResetCmd, nil)
		}},
		{"stop", "stop", func() error { return runStop(StopCmd, nil) }},
		{"abandon", "abandon", func() error { return runAbandon(AbandonCmd, nil) }},
		{"delete", "delete", func() error { return runDelete(DeleteCmd, nil) }},
		{"update", "update", func() error { return runUpdate(UpdateCmd, nil) }},
		{"finish", "task.finish", func() error { return runFinish(FinishCmd, nil) }},
		{"refresh", "task.refresh", func() error { return runRefresh(RefreshCmd, nil) }},
		{"checkpoints", "checkpoints", func() error { return runCheckpoints(CheckpointsCmd, nil) }},
		{"eventlog", "eventlog.query", func() error { return runEventlog(EventlogCmd, nil) }},
		{"ci", "ci.status", func() error { return runCIStatus(CICmd, nil) }},
		{"hooks", "hooks.list", func() error { return runHooks(HooksCmd, nil) }},
		{"discover", "discovery.scan", func() error { return runDiscover(DiscoverCmd, nil) }},
		{"cachestats", "cache.stats", func() error { return runCacheStats(nil, nil) }},
		{"quality", "autofix.status", func() error { return runQuality(QualityCmd, nil) }},
		{"qualityfailclass", "failclass.stats", func() error { return runQualityFailclass(nil, nil) }},
		{"codegraphstats", "codegraph.stats", func() error { return runCodegraphStats(nil, nil) }},
		{"codegraphsearch", "codegraph.search", func() error { return runCodegraphSearch(nil, []string{"x"}) }},
		{"codegraphcallers", "codegraph.callers", func() error { return runCodegraphCallers(nil, []string{"x"}) }},
		{"codegraphdeps", "codegraph.deps", func() error { return runCodegraphDeps(nil, []string{"x"}) }},
		{"forkcreate", "fork.create", func() error { return runForkCreate(nil, []string{"x"}) }},
		{"forklist", "fork.list", func() error { return runForkList(nil, nil) }},
		{"policycheck", "policy.check", func() error { return runPolicyCheck(nil, nil) }},
		{"recap", "recap", func() error { return runRecap(RecapCmd, nil) }},
		{"showspec", "show.spec", func() error { return runShowSpec(showSpecCmd, nil) }},
		{"reviewlist", "review.list", func() error { return runReviewList(ReviewCmd, nil) }},
		{"reviewrisk", "risk.evaluate", func() error { return runReviewRisk(nil, nil) }},
		{"undo", "undo", func() error { return runUndo(UndoCmd, nil) }},
		{"gitstatus", "git.status", func() error { return runGitStatus(gitStatusCmd, nil) }},
		{"gitdiff", "git.diff", func() error { return runGitDiff(gitDiffCmd, nil) }},
		{"gitlog", "git.log", func() error { return runGitLog(gitLogCmd, nil) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			stub := startStubWorktreeSocket(t)
			stub.SetError(c.method, -32000, "boom")

			if err := c.fn(); err == nil {
				t.Errorf("%s: expected error when %s fails", c.name, c.method)
			}
		})
	}
}

// TestGlobalCommands_ServerErrors drives the error branch of global-socket
// commands.
func TestGlobalCommands_ServerErrors(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"list", "tasks.list", func() error { return runList(ListCmd, nil) }},
		{"projects", "projects.list", func() error { return runProjects(ProjectsCmd, nil) }},
		{"workers", "workers.list", func() error { return runWorkers(WorkersCmd, nil) }},
		{"agent", "agent.status", func() error { return runAgentStatus(AgentCmd, nil) }},
		{"strategy", "strategy.list", func() error { return runStrategyList(StrategyCmd, nil) }},
		{"jobslist", "jobs.list", func() error { return runJobsList(jobsListCmd, nil) }},
		{"jobsget", "jobs.get", func() error { return runJobsGet(jobsGetCmd, []string{"j1"}) }},
		{"memorystats", "memory.stats", func() error { return runMemoryStats(memoryStatsCmd, nil) }},
		{"memorysearch", "memory.search", func() error { return runMemorySearch(memorySearchCmd, []string{"x"}) }},
		{"memoryoutcomes", "memory.outcomes", func() error { return runMemoryOutcomes(nil, nil) }},
		{"security", "security.scan", func() error { return runSecurityScan(SecurityCmd, nil) }},
		{"notify", "notify.test", func() error { return runNotifyTest(nil, nil) }},
		{"backuplist", "backup.list", func() error { return runBackupList(nil, nil) }},
		{"catalog", "catalog.list", func() error { return runCatalogList(nil, nil) }},
		{"groupcreate", "taskgroup.create", func() error { return runGroupCreate(nil, []string{"g"}) }},
		{"grouplist", "taskgroup.list", func() error { return runGroupList(nil, nil) }},
		{"recordingslist", "recordings.list", func() error { return runRecordingsList(recordingsListCmd, nil) }},
		{"recordingsview", "recordings.view", func() error { return runRecordingsView(recordingsViewCmd, []string{"r"}) }},
		{"strategyerr", "strategy.list", func() error { return runStrategyList(StrategyCmd, nil) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			stub := startStubGlobalSocket(t)
			stub.SetError(c.method, -32000, "boom")

			if err := c.fn(); err == nil {
				t.Errorf("%s: expected error when %s fails", c.name, c.method)
			}
		})
	}
}
