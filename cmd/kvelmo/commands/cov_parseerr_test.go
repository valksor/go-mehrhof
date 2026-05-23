package commands

import (
	"testing"
)

// badJSON is a response payload that marshals to a JSON string, which fails to
// unmarshal into the struct/map shapes the commands expect — driving each
// command's "invalid/parse response" error branch.
const badJSON = "not-a-valid-object-for-this-shape"

// TestWorktreeCommands_ParseErrors drives the parse-error branch of worktree
// commands by returning a malformed (string) payload for the RPC they parse.
func TestWorktreeCommands_ParseErrors(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"plan", "plan", func() error { return runPlan(PlanCmd, nil) }},
		{"implement", "implement", func() error { return runImplement(ImplementCmd, nil) }},
		{"simplify", "simplify", func() error { return runSimplify(SimplifyCmd, nil) }},
		{"optimize", "optimize", func() error { return runOptimize(OptimizeCmd, nil) }},
		{"checkpoints", "checkpoints", func() error { return runCheckpoints(CheckpointsCmd, nil) }},
		{"checkpointsgoto", "checkpoint.goto", func() error { return runCheckpointsGoto(checkpointsGotoCmd, []string{"x"}) }},
		{"eventlog", "eventlog.query", func() error { return runEventlog(EventlogCmd, nil) }},
		{"ci", "ci.status", func() error { return runCIStatus(CICmd, nil) }},
		{"hooks", "hooks.list", func() error { return runHooks(HooksCmd, nil) }},
		{"codegraphstats", "codegraph.stats", func() error { return runCodegraphStats(nil, nil) }},
		{"codegraphsearch", "codegraph.search", func() error { return runCodegraphSearch(nil, []string{"x"}) }},
		{"codegraphcallers", "codegraph.callers", func() error { return runCodegraphCallers(nil, []string{"x"}) }},
		{"codegraphdeps", "codegraph.deps", func() error { return runCodegraphDeps(nil, []string{"x"}) }},
		{"forklist", "fork.list", func() error { return runForkList(nil, nil) }},
		{"forkcompare", "fork.compare", func() error { return runForkCompare(nil, nil) }},
		{"policy", "policy.check", func() error { return runPolicyCheck(nil, nil) }},
		{"recap", "recap", func() error { return runRecap(RecapCmd, nil) }},
		{"showspec", "show.spec", func() error { return runShowSpec(showSpecCmd, nil) }},
		{"reviewlist", "review.list", func() error { return runReviewList(ReviewCmd, nil) }},
		{"taglist", "task.tag", func() error { return runTagList(tagListCmd, nil) }},
		{"checklist", "review.checklist.get", func() error { return runChecklist(ChecklistCmd, nil) }},
		{"gitstatus", "git.status", func() error { return runGitStatus(gitStatusCmd, nil) }},
		{"gitdiff", "git.diff", func() error { return runGitDiff(gitDiffCmd, nil) }},
		{"gitlog", "git.log", func() error { return runGitLog(gitLogCmd, nil) }},
		{"queuelist", "queue.list", func() error { return runQueueList(queueListCmd, nil) }},
		{"taskhistory", "task.history", func() error { return runTaskHistory(false) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			stub := startStubWorktreeSocket(t)
			stub.SetResponse(c.method, badJSON)

			if err := c.fn(); err == nil {
				t.Errorf("%s: expected parse error for malformed %s response", c.name, c.method)
			}
		})
	}
}

// TestGlobalCommands_ParseErrors drives the parse-error branch of global
// commands.
func TestGlobalCommands_ParseErrors(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"list", "tasks.list", func() error { return runList(ListCmd, nil) }},
		{"projects", "projects.list", func() error { return runProjects(ProjectsCmd, nil) }},
		{"agent", "agent.status", func() error { return runAgentStatus(AgentCmd, nil) }},
		{"strategy", "strategy.list", func() error { return runStrategyList(StrategyCmd, nil) }},
		{"jobslist", "jobs.list", func() error { return runJobsList(jobsListCmd, nil) }},
		{"jobsget", "jobs.get", func() error { return runJobsGet(jobsGetCmd, []string{"j1"}) }},
		{"security", "security.scan", func() error { return runSecurityScan(SecurityCmd, nil) }},
		{"backuplist", "backup.list", func() error { return runBackupList(nil, nil) }},
		{"recordingslist", "recordings.list", func() error { return runRecordingsList(recordingsListCmd, nil) }},
		{"recordingsview", "recordings.view", func() error { return runRecordingsView(recordingsViewCmd, []string{"r"}) }},
		{"chathistory", "chat.history", func() error { return runChatHistory(chatHistoryCmd, nil) }},
		{"logs", "chat.history", func() error { return runLogs(LogsCmd, nil) }},
		{"configcheck", "config.check", func() error { return runConfigCheck(nil, nil) }},
		{"providertest", "providers.test", func() error { return runProviderTest(ProviderTestCmd, []string{"github"}) }},
		{"diagnosehealth", "system.health", func() error {
			setBoolPtr(t, &diagnoseHealth, true)

			return runDiagnose(DiagnoseCmd, nil)
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			stub := startStubGlobalSocket(t)
			stub.SetResponse(c.method, badJSON)

			if err := c.fn(); err == nil {
				t.Errorf("%s: expected parse error for malformed %s response", c.name, c.method)
			}
		})
	}
}
