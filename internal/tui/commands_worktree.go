package tui

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/internal/socket"
)

// worktreeHandler handles a worktree-scoped slash command.
type worktreeHandler func(ctx context.Context, client *socket.Client, args string, dryRun bool) (string, error)

// worktreeHandlers maps command names to their handler functions.
var worktreeHandlers = map[string]worktreeHandler{
	// Workflow
	"/quick":      wtQuick,
	"/plan":       wtPlan,
	"/plan!":      wtPlan,
	"/implement":  wtImplement,
	"/implement!": wtImplement,
	"/simplify":   wtSimplify,
	"/optimize":   wtOptimize,
	"/review":     wtReview,
	"/review fix": wtReviewFix,
	"/submit":     wtSubmit,
	"/finish":     wtFinish,
	"/abandon":    wtAbandon,
	"/delete":     wtDelete,

	// Control
	"/undo":   wtUndo,
	"/redo":   wtRedo,
	"/stop":   wtStop,
	"/abort":  wtAbort,
	"/reset":  wtReset,
	"/update": wtUpdate,

	// Inspection
	"/status":           wtStatus,
	"/checkpoints":      wtCheckpoints,
	"/checkpoints goto": wtCheckpointsGoto,
	"/recap":            wtRecap,
	"/diff":             wtDiff,
	"/show spec":        wtShowSpec,
	"/show plan":        wtShowPlan,
	"/list search":      wtListSearch,
	"/list":             wtList,
	"/eventlog":         wtEventlog,

	// Tags
	"/tag add":    wtTagAdd,
	"/tag remove": wtTagRemove,
	"/tags":       wtTags,

	// Queue
	"/queue add":     wtQueueAdd,
	"/queue remove":  wtQueueRemove,
	"/queue reorder": wtQueueReorder,
	"/queue list":    wtQueueList,
	"/queue":         wtQueueList,

	// Forks
	"/fork create":  wtForkCreate,
	"/fork list":    wtForkList,
	"/fork compare": wtForkCompare,
	"/fork select":  wtForkSelect,

	// Governance
	"/approve":           wtApprove,
	"/checklist check":   wtChecklistCheck,
	"/checklist uncheck": wtChecklistUncheck,
	"/checklist":         wtChecklist,
	"/ci":                wtCI,
	"/policy":            wtPolicy,
	"/quality":           wtQuality,
	"/retry":             wtRetry,
	"/audit":             wtAudit,

	// Files & Code
	"/files search":      wtFilesSearch,
	"/files":             wtFiles,
	"/git status":        wtGitStatus,
	"/git log":           wtGitLog,
	"/codegraph callers": wtCodegraphCallers,
	"/codegraph deps":    wtCodegraphDeps,
	"/codegraph index":   wtCodegraphIndex,
	"/codegraph stats":   wtCodegraphStats,
	"/codegraph search":  wtCodegraphSearch,

	// Cache
	"/cache stats": wtCacheStats,
	"/cache clear": wtCacheClear,

	// Export
	"/export": wtExport,

	// Changelog
	"/changelog":      wtChangelog,
	"/changelog full": wtChangelogFull,

	// Remote
	"/remote approve": wtRemoteApprove,
	"/remote merge":   wtRemoteMerge,

	// Discover
	"/discover": wtDiscover,

	// Chat-based
	"/explain": wtExplain,

	// Parity additions
	"/hooks":       wtHooks,
	"/screenshots": wtScreenshots,
}

// ── Workflow ────────────────────────────────────────────────────────────────

func wtQuick(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /quick <source>", nil
	}
	_, err := client.Call(ctx, "start", json.RawMessage(mustJSON(map[string]any{"source": args, "auto_advance": true})))
	if err != nil {
		return "", err
	}

	return "Quick fix started.", nil
}

func wtPlan(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	params := map[string]any{}
	if dryRun {
		params["dry_run"] = true
	}
	_, err := client.Call(ctx, "plan", json.RawMessage(mustJSON(params)))
	if err != nil {
		return "", err
	}

	return "Planning started.", nil
}

func wtImplement(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	params := map[string]any{}
	if dryRun {
		params["dry_run"] = true
	}
	_, err := client.Call(ctx, "implement", json.RawMessage(mustJSON(params)))
	if err != nil {
		return "", err
	}

	return "Implementation started.", nil
}

func wtSimplify(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	_, err := client.Call(ctx, "simplify", optDryRun(dryRun))
	if err != nil {
		return "", err
	}

	return "Simplification started.", nil
}

func wtOptimize(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	_, err := client.Call(ctx, "optimize", optDryRun(dryRun))
	if err != nil {
		return "", err
	}

	return "Optimization started.", nil
}

func wtReview(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "review", json.RawMessage(mustJSON(map[string]any{"approve": true})))
	if err != nil {
		return "", err
	}

	return "Review started.", nil
}

func wtReviewFix(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "review", json.RawMessage(mustJSON(map[string]any{"fix": true})))
	if err != nil {
		return "", err
	}

	return "Review with fixes started.", nil
}

func wtSubmit(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	_, err := client.Call(ctx, "submit", optDryRun(dryRun))
	if err != nil {
		return "", err
	}

	return "Submit started.", nil
}

func wtFinish(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "task.finish", nil)
	if err != nil {
		return "", err
	}

	return "Finish started.", nil
}

func wtAbandon(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "abandon", nil)
	if err != nil {
		return "", err
	}

	return "Task abandoned.", nil
}

func wtDelete(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "delete", nil)
	if err != nil {
		return "", err
	}

	return "Task deleted.", nil
}

// ── Control ─────────────────────────────────────────────────────────────────

func wtUndo(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "undo", nil)
	if err != nil {
		return "", err
	}

	return "Undone to previous checkpoint.", nil
}

func wtRedo(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "redo", nil)
	if err != nil {
		return "", err
	}

	return "Redone to next checkpoint.", nil
}

func wtStop(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "stop", nil)
	if err != nil {
		return "", err
	}

	return "Operation stopped.", nil
}

func wtAbort(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "abort", nil)
	if err != nil {
		return "", err
	}

	return "Operation aborted.", nil
}

func wtReset(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "reset", nil)
	if err != nil {
		return "", err
	}

	return "Task reset.", nil
}

func wtUpdate(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "update", nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Changed bool `json:"changed"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	if result.Changed {
		return "Task updated from source.", nil
	}

	return "Task is already up to date.", nil
}
