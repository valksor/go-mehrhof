package commands

import (
	"testing"
)

// TestWorktreeCommands_CallErrors3 drives the RPC-call error branch (the
// `if err != nil { return fmt.Errorf(...) }` after client.Call) of additional
// worktree-socket commands by making the method return a JSON-RPC error.
func TestWorktreeCommands_CallErrors3(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"tagadd", "task.tag", func() error { return runTagAdd(tagAddCmd, []string{"backend"}) }},
		{"tagremove", "task.tag", func() error { return runTagRemove(tagRemoveCmd, []string{"backend"}) }},
		{"taglist", "task.tag", func() error { return runTagList(tagListCmd, nil) }},
		{"reviewview", "review.view", func() error { return runReviewView(reviewViewCmd, []string{"1"}) }},
		{"remoteapprove", "remote.approve", func() error { return runRemoteApprove(RemoteApproveCmd, nil) }},
		{"remotemerge", "remote.merge", func() error { return runRemoteMerge(RemoteMergeCmd, nil) }},
		{"exporttask", "task.export", func() error { return runExportTask(exportTaskCmd, nil) }},
		{"showplan", "show.plan", func() error { return runShowPlan(showPlanCmd, nil) }},
		{"queueadd", "queue.add", func() error { return runQueueAdd(queueAddCmd, []string{"src"}) }},
		{"queueremove", "queue.remove", func() error { return runQueueRemove(queueRemoveCmd, []string{"q1"}) }},
		{"queuelist", "queue.list", func() error { return runQueueList(queueListCmd, nil) }},
		{"queuereorder", "queue.reorder", func() error { return runQueueReorder(queueReorderCmd, []string{"q1", "1"}) }},
		{"screenshotsdelete", "screenshots.delete", func() error {
			return runScreenshotsDelete(screenshotsDeleteCmd, []string{"s1"})
		}},
		{"screenshotsget", "screenshots.get", func() error { return runScreenshotsGet(screenshotsGetCmd, []string{"s1"}) }},
		{"screenshotscapture", "screenshots.capture", func() error {
			return runScreenshotsCapture(screenshotsCaptureCmd, nil)
		}},
		{"reviewriskhistory", "risk.history", func() error { return runReviewRiskHistory(nil, nil) }},
		{"adversarialresults", "adversarial.results", func() error {
			return runReviewAdversarialResults(nil, nil)
		}},
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

// TestGlobalCommands_CallErrors3 drives the RPC-call error branch of additional
// global-socket commands.
func TestGlobalCommands_CallErrors3(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"workersadd", "workers.add", func() error { return runWorkersAdd(workersAddCmd, []string{"claude"}) }},
		{"workersremove", "workers.remove", func() error { return runWorkersRemove(workersRemoveCmd, []string{"w1"}) }},
		{"workersstats", "workers.stats", func() error { return runWorkersStats(workersStatsCmd, nil) }},
		{"report", "report.generate", func() error { return runReport(ReportCmd, nil) }},
		{"backup", "backup.create", func() error { return runBackup(BackupCmd, nil) }},
		{"restore", "backup.restore", func() error { return runRestore(RestoreCmd, []string{"/tmp/x.bk"}) }},
		{"audit", "export", func() error { return runAudit(AuditCmd, nil) }},
		{"activity", "activity.query", func() error { return runActivity(ActivityCmd, nil) }},
		{"export", "export", func() error { return runExport(ExportCmd, nil) }},
		{"chatsend", "chat.send", func() error { return runChatSend(chatSendCmd, []string{"hi"}) }},
		{"chatstop", "chat.stop", func() error { return runChatStop(chatStopCmd, nil) }},
		{"chatclear", "chat.clear", func() error { return runChatClear(chatClearCmd, nil) }},
		{"chathistory", "chat.history", func() error { return runChatHistory(chatHistoryCmd, nil) }},
		{"explain", "chat.send", func() error { return runExplain(ExplainCmd, nil) }},
		{"logs", "chat.history", func() error { return runLogs(LogsCmd, nil) }},
		{"filessearch", "files.search", func() error { return runFilesSearch(filesSearchCmd, []string{"q"}) }},
		{"fileslist", "files.list", func() error { return runFilesList(filesListCmd, nil) }},
		{"cataloglist", "catalog.list", func() error { return runCatalogList(nil, nil) }},
		{"cataloguse", "catalog.get", func() error { return runCatalogUse(nil, []string{"x"}) }},
		{"browse", "browse", func() error { return runBrowse(BrowseCmd, nil) }},
		{"groupadd", "taskgroup.add", func() error { return runGroupAdd(nil, []string{"g1", "t1"}) }},
		{"groupstatus", "taskgroup.status", func() error { return runGroupStatus(nil, []string{"g1"}) }},
		{"groupsubmit", "taskgroup.submit", func() error { return runGroupSubmit(nil, []string{"g1"}) }},
		{"groupremove", "taskgroup.remove", func() error { return runGroupRemove(nil, []string{"g1"}) }},
		{"providertest", "providers.test", func() error { return runProviderTest(ProviderTestCmd, []string{"github"}) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			stub := startStubGlobalSocket(t)
			stub.SetError(c.method, -32000, "boom")

			if err := c.fn(); err == nil {
				t.Errorf("%s: expected error when %s fails", c.name, c.method)
			}
		})
	}
}
