package conductor

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/agenttest"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/graph"
	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/settings"
)

func TestValidateSubTaskConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  graph.SubTaskConfig
		wantErr bool
	}{
		{"no phases", graph.SubTaskConfig{Title: "x"}, true},
		{"unknown phase", graph.SubTaskConfig{Title: "x", Phases: []string{"deploy"}}, true},
		{"valid single", graph.SubTaskConfig{Title: "x", Phases: []string{"plan"}}, false},
		{"valid chain", graph.SubTaskConfig{Title: "x", Phases: []string{"plan", "implement", "review"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSubTaskConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSubTaskConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildSubTaskGraph_LinearChain(t *testing.T) {
	config := graph.SubTaskConfig{Title: "t", Phases: []string{"plan", "implement"}}
	g := buildSubTaskGraph(config)

	if g.NodeCount() != 2 {
		t.Fatalf("NodeCount = %d, want 2", g.NodeCount())
	}

	planID := subTaskNodeID("plan", 0)
	implID := subTaskNodeID("implement", 1)

	plan := g.Nodes[planID]
	if plan == nil || plan.JobType != worker.JobTypePlan || len(plan.DependsOn) != 0 {
		t.Errorf("plan node = %+v, want root node with JobTypePlan", plan)
	}

	impl := g.Nodes[implID]
	if impl == nil || impl.JobType != worker.JobTypeImplement {
		t.Fatalf("implement node = %+v, want JobTypeImplement", impl)
	}
	if len(impl.DependsOn) != 1 || impl.DependsOn[0] != planID {
		t.Errorf("implement DependsOn = %v, want [%s]", impl.DependsOn, planID)
	}

	if err := g.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestBuildSubTaskPhasePrompt(t *testing.T) {
	config := graph.SubTaskConfig{Title: "Add cache", Description: "memoize results", Phases: []string{"implement"}}
	prompt := buildSubTaskPhasePrompt(config, "implement")

	for _, want := range []string{"Add cache", "memoize results", "Phase: implement", "Implement the sub-task"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestAggregateSubTaskOutput_OrderAndSkip(t *testing.T) {
	config := graph.SubTaskConfig{Title: "t", Phases: []string{"plan", "implement", "review"}}
	g := buildSubTaskGraph(config)
	state := graph.NewStateManager(g)
	state.SetResult(subTaskNodeID("plan", 0), "PLAN OUT")
	state.SetResult(subTaskNodeID("implement", 1), "IMPL OUT")
	// review intentionally has no result and should be skipped.

	out := aggregateSubTaskOutput(config, state)

	planIdx := strings.Index(out, "PLAN OUT")
	implIdx := strings.Index(out, "IMPL OUT")
	if planIdx < 0 || implIdx < 0 {
		t.Fatalf("output missing phase results:\n%s", out)
	}
	if planIdx > implIdx {
		t.Errorf("phases out of order: plan should precede implement\n%s", out)
	}
	if strings.Contains(out, "## review") {
		t.Errorf("empty review phase should be skipped:\n%s", out)
	}
}

// setupSubTaskConductor builds a Conductor over a real git repo with a
// MockAgent-backed worker pool.
func setupSubTaskConductor(t *testing.T, events ...agent.Event) *Conductor {
	t.Helper()

	dir := t.TempDir()
	ctx := context.Background()

	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test User"},
	} {
		gitCmd(ctx, t, args...)
	}
	if err := os.WriteFile(dir+"/main.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-C", dir, "add", "."},
		{"-C", dir, "commit", "-m", "initial commit"},
	} {
		gitCmd(ctx, t, args...)
	}

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	registry := agent.NewRegistry()
	mock := agenttest.NewMockAgent("mock", events...)
	if err := registry.Register(mock); err != nil {
		t.Fatalf("register mock agent: %v", err)
	}

	pool := worker.NewPool(worker.PoolConfig{MaxWorkers: 1, Agents: registry})
	if err := pool.Start(); err != nil {
		t.Fatalf("start pool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Stop() })

	if _, err := pool.AddAgentWorker(ctx, "mock", true); err != nil {
		t.Fatalf("add mock worker: %v", err)
	}

	s := settings.DefaultSettings()
	provisionOff := false
	s.Git.Provision.Enabled = &provisionOff

	c := &Conductor{
		git:            repo,
		worktree:       dir,
		pool:           pool,
		workUnit:       &WorkUnit{ID: "task-1", Title: "Test Task"},
		events:         make(chan ConductorEvent, 64),
		pendingPrompts: make(map[string]chan bool),
	}
	c.cachedSettings.Store(s)

	// Drain events so emit never blocks.
	go func() {
		for range c.events {
		}
	}()

	return c
}

func TestRunSubTask_Integration(t *testing.T) {
	c := setupSubTaskConductor(
		t,
		agent.Event{Type: agent.EventStream, Content: "SUBTASK PLAN DONE"},
		agent.Event{Type: agent.EventComplete},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use an explicit branch so the assertion is deterministic (auto-generated
	// names carry a unique suffix) and the explicit-branch path is exercised.
	config := graph.SubTaskConfig{
		Title:  "cache layer",
		Branch: "feature/cache-layer",
		Phases: []string{"plan"},
	}

	output, err := c.RunSubTask(ctx, config)
	if err != nil {
		t.Fatalf("RunSubTask() error = %v", err)
	}

	if !strings.Contains(output, "SUBTASK PLAN DONE") {
		t.Errorf("output missing phase result:\n%s", output)
	}
	if !strings.Contains(output, "## plan") {
		t.Errorf("output missing phase heading:\n%s", output)
	}

	// The sub-task branch is retained; the worktree directory is removed.
	if !c.git.LocalBranchExists(ctx, config.Branch) {
		t.Errorf("expected sub-task branch %q to be retained", config.Branch)
	}

	worktrees, err := c.git.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	for _, wt := range worktrees {
		if strings.Contains(wt.Path, "subtask-cache-layer") {
			t.Errorf("sub-task worktree should have been removed: %s", wt.Path)
		}
	}
}

func TestRunSubTask_FailureCleansUpWorktree(t *testing.T) {
	c := setupSubTaskConductor(
		t,
		agent.Event{Type: agent.EventError, Error: "boom"},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := c.RunSubTask(ctx, graph.SubTaskConfig{Title: "fail case", Phases: []string{"plan"}})
	if err == nil {
		t.Fatal("RunSubTask() expected an error when a phase fails")
	}

	// The worktree must be torn down even when the sub-task fails.
	worktrees, lerr := c.git.ListWorktrees(ctx)
	if lerr != nil {
		t.Fatalf("ListWorktrees: %v", lerr)
	}
	for _, wt := range worktrees {
		if strings.Contains(wt.Path, "subtask-fail-case") {
			t.Errorf("sub-task worktree should be removed after failure: %s", wt.Path)
		}
	}
}

func TestRunSubTask_RejectsInvalidConfig(t *testing.T) {
	c := setupSubTaskConductor(t)

	if _, err := c.RunSubTask(context.Background(), graph.SubTaskConfig{Title: "x"}); err == nil {
		t.Error("RunSubTask() with no phases should error")
	}
	if _, err := c.RunSubTask(context.Background(), graph.SubTaskConfig{Title: "x", Phases: []string{"bogus"}}); err == nil {
		t.Error("RunSubTask() with unknown phase should error")
	}
}
