package commands

import (
	"testing"
)

// More parse-error coverage: each command parses a malformed (string) payload
// and must surface the parse error.

func TestWorktreeCommands_ParseErrors2(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"reviewview", "review.view", func() error { return runReviewView(reviewViewCmd, []string{"1"}) }},
		{"diff", "checkpoints", func() error { return runDiff(DiffCmd, nil) }},
		{"screenshotslist", "screenshots.list", func() error { return runScreenshotsList(screenshotsListCmd, nil) }},
		{"screenshotsget", "screenshots.get", func() error { return runScreenshotsGet(screenshotsGetCmd, []string{"s1"}) }},
		{"screenshotscapture", "screenshots.capture", func() error { return runScreenshotsCapture(screenshotsCaptureCmd, nil) }},
		{"statsproject", "task.history", func() error { return runStatsProject() }},
		{"filessearch", "files.search", func() error { return runFilesSearch(filesSearchCmd, []string{"q"}) }},
		{"fileslist", "files.list", func() error { return runFilesList(filesListCmd, nil) }},
		{"finish", "task.finish", func() error { return runFinish(FinishCmd, nil) }},
		{"refresh", "task.refresh", func() error { return runRefresh(RefreshCmd, nil) }},
		{"discover", "discovery.scan", func() error { return runDiscover(DiscoverCmd, nil) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			stub := startStubWorktreeSocket(t)
			stub.SetResponse(c.method, badJSON)
			if err := c.fn(); err == nil {
				t.Errorf("%s: expected parse error", c.name)
			}
		})
	}
}

func TestGlobalCommands_ParseErrors2(t *testing.T) {
	cases := []struct {
		name   string
		method string
		fn     func() error
	}{
		{"workers", "workers.list", func() error { return runWorkers(WorkersCmd, nil) }},
		{"memorysearch", "memory.search", func() error { return runMemorySearch(memorySearchCmd, []string{"q"}) }},
		{"memorystats", "memory.stats", func() error { return runMemoryStats(memoryStatsCmd, nil) }},
		{"audit", "export", func() error { return runAudit(AuditCmd, nil) }},
		{"activity", "activity.query", func() error { return runActivity(ActivityCmd, nil) }},
		{"backup", "backup.create", func() error { return runBackup(nil, nil) }},
		{"restore", "backup.restore", func() error { return runRestore(nil, []string{"/b.tar.gz"}) }},
		{"groupcreate", "taskgroup.create", func() error { return runGroupCreate(nil, []string{"g"}) }},
		{"groupstatus", "taskgroup.status", func() error { return runGroupStatus(nil, []string{"g"}) }},
		{"catalogget", "catalog.get", func() error { return runCatalogUse(nil, []string{"x"}) }},
		{"stats", "metrics.history", func() error {
			origH := statsHistory
			statsHistory = true
			defer func() { statsHistory = origH }()

			return runStatsHistory()
		}},
		{"batch", "tasks.batch", func() error { return runBatch(BatchCmd, []string{"status"}) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			stub := startStubGlobalSocket(t)
			stub.SetResponse(c.method, badJSON)
			if err := c.fn(); err == nil {
				t.Errorf("%s: expected parse error", c.name)
			}
		})
	}
}
