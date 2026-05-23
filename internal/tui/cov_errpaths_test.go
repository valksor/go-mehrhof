package tui

import (
	"context"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

// TestWorktreeHandlerErrorPaths drives every worktree handler that issues a
// single RPC against a stub that returns an RPC error, asserting the error is
// surfaced rather than swallowed. Handlers that validate args first are called
// with valid args so the RPC actually fires.
func TestWorktreeHandlerErrorPaths(t *testing.T) {
	cases := []struct {
		name    string
		handler worktreeHandler
		method  string
		args    string
	}{
		{"quick", wtQuick, "start", "src"},
		{"plan", wtPlan, phasePlan, ""},
		{"implement", wtImplement, phaseImplement, ""},
		{"simplify", wtSimplify, phaseSimplify, ""},
		{"optimize", wtOptimize, phaseOptimize, ""},
		{"review", wtReview, phaseReview, ""},
		{"reviewFix", wtReviewFix, phaseReview, ""},
		{"submit", wtSubmit, phaseSubmit, ""},
		{"finish", wtFinish, "task.finish", ""},
		{"abandon", wtAbandon, "abandon", ""},
		{"delete", wtDelete, "delete", ""},
		{"undo", wtUndo, "undo", ""},
		{"redo", wtRedo, "redo", ""},
		{"stop", wtStop, "stop", ""},
		{"abort", wtAbort, "abort", ""},
		{"reset", wtReset, "reset", ""},
		{"update", wtUpdate, "update", ""},
		{"status", wtStatus, "status", ""},
		{"checkpoints", wtCheckpoints, "checkpoints", ""},
		{"checkpointsGoto", wtCheckpointsGoto, "checkpoint.goto", "sha1234"},
		{"recap", wtRecap, "recap", ""},
		{"diff", wtDiff, "git.diff", ""},
		{"showSpec", wtShowSpec, "show.spec", ""},
		{"showPlan", wtShowPlan, "show.plan", ""},
		{"listSearch", wtListSearch, "task.search", "q"},
		{"list", wtList, "task.history", ""},
		{"eventlog", wtEventlog, "eventlog.query", ""},
		{"tagAdd", wtTagAdd, "task.tag", "tag"},
		{"tagRemove", wtTagRemove, "task.tag", "tag"},
		{"tags", wtTags, "task.tag", ""},
		{"queueAdd", wtQueueAdd, "queue.add", "src"},
		{"queueRemove", wtQueueRemove, "queue.remove", "id"},
		{"queueReorder", wtQueueReorder, "queue.reorder", "id 2"},
		{"queueList", wtQueueList, "queue.list", ""},
		{"forkCreate", wtForkCreate, "fork.create", "label"},
		{"forkList", wtForkList, "fork.list", ""},
		{"forkCompare", wtForkCompare, "fork.compare", ""},
		{"forkSelect", wtForkSelect, "fork.select", "fork1234"},
		{"cacheStats", wtCacheStats, "cache.stats", ""},
		{"cacheClear", wtCacheClear, "cache.clear", ""},
		{"export", wtExport, "task.export", ""},
		{"changelog", wtChangelog, "changelog.generate", "v1..v2"},
		{"changelogFull", wtChangelogFull, "changelog.generate", "v1..v2"},
		{"remoteApprove", wtRemoteApprove, "remote.approve", ""},
		{"remoteMerge", wtRemoteMerge, "remote.merge", ""},
		{"discover", wtDiscover, "discovery.scan", ""},
		{"explain", wtExplain, "chat.send", ""},
		{"approve", wtApprove, "approve", "submit"},
		{"checklistCheck", wtChecklistCheck, "review.checklist.check", "item"},
		{"checklistUncheck", wtChecklistUncheck, "review.checklist.uncheck", "item"},
		{"checklist", wtChecklist, "review.checklist.get", ""},
		{"ci", wtCI, "ci.status", ""},
		{"policy", wtPolicy, "policy.check", ""},
		{"quality", wtQuality, "quality.respond", ""},
		{"retry", wtRetry, "reset", ""},
		{"audit", wtAudit, "task.export", ""},
		{"filesSearch", wtFilesSearch, "files.search", "p"},
		{"files", wtFiles, "files.list", ""},
		{"gitStatus", wtGitStatus, "git.status", ""},
		{"gitLog", wtGitLog, "git.log", ""},
		{"cgCallers", wtCodegraphCallers, "codegraph.callers", "Sym"},
		{"cgDeps", wtCodegraphDeps, "codegraph.deps", "Sym"},
		{"cgIndex", wtCodegraphIndex, "codegraph.index", ""},
		{"cgStats", wtCodegraphStats, "codegraph.stats", ""},
		{"cgSearch", wtCodegraphSearch, "codegraph.search", "Sym"},
		{"screenshots", wtScreenshots, "screenshots.list", ""},
		{"hooks", wtHooks, "hooks.list", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newStubServer(t)
			s.onError(tc.method, socket.ErrCodeInternal, "boom")
			_, err := tc.handler(context.Background(), s.client(t), tc.args, false)
			if err == nil {
				t.Errorf("%s: expected error from %s, got nil", tc.name, tc.method)
			}
		})
	}
}

// TestGlobalHandlerErrorPaths does the same for global handlers.
func TestGlobalHandlerErrorPaths(t *testing.T) {
	cases := []struct {
		name    string
		handler globalHandler
		method  string
		args    string
	}{
		{"jobs", glJobs, "jobs.list", ""},
		{"stats", glStats, "metrics", ""},
		{"workers", glWorkers, "workers.list", ""},
		{"memorySearch", glMemorySearch, "memory.search", "q"},
		{"memoryStats", glMemoryStats, "memory.stats", ""},
		{"memoryClear", glMemoryClear, "memory.clear", ""},
		{"batch", glBatch, "tasks.batch", "plan"},
		{"report", glReport, "report.generate", ""},
		{"backup", glBackup, "backup.create", ""},
		{"groupCreate", glGroupCreate, "taskgroup.create", "g"},
		{"groupList", glGroupList, "taskgroup.list", ""},
		{"groupStatus", glGroupStatus, "taskgroup.status", "g"},
		{"groupAdd", glGroupAdd, "taskgroup.add", "g t"},
		{"groupSubmit", glGroupSubmit, "taskgroup.submit", "g"},
		{"groupRemove", glGroupRemove, "taskgroup.remove", "g"},
		{"diagnose", glDiagnose, "system.diagnose", ""},
		{"securityScan", glSecurityScan, "security.scan", ""},
		{"configCheck", glConfigCheck, "config.check", ""},
		{"configShow", glConfigShow, "settings.get", ""},
		{"configValidate", glConfigValidate, "config.validate", ""},
		{"strategy", glStrategy, "strategy.list", ""},
		{"restore", glRestore, "backup.restore", "/p.tar"},
		{"catalogList", glCatalogList, "catalog.list", ""},
		{"catalogUse", glCatalogUse, "catalog.get", "t"},
		{"workersAdd", glWorkersAdd, "workers.add", "w"},
		{"workersRemove", glWorkersRemove, "workers.remove", "w"},
		{"rpcLog", glRPCLog, "activity.query", ""},
		{"agent", glAgent, "agent.status", ""},
		{"projectsList", glProjectsList, "projects.list", ""},
		{"projectsUnregister", glProjectsUnregister, "projects.unregister", "id"},
		{"recordings", glRecordings, "recordings.list", ""},
		{"notifyTest", glNotifyTest, "notify.test", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newStubServer(t)
			s.onError(tc.method, socket.ErrCodeInternal, "boom")
			_, err := tc.handler(context.Background(), s.client(t), tc.args)
			if err == nil {
				t.Errorf("%s: expected error from %s, got nil", tc.name, tc.method)
			}
		})
	}
}
