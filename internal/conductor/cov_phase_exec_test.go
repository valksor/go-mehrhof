package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/internal/varpool"
	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/settings"
)

// delayAgent is a minimal agent.Agent that emits a configured sequence of events
// after a small startup delay. The delay ensures the conductor's watchJob
// goroutine attaches to the job stream before the job completes — the
// MockAgent completes near-instantly, which can race watchJob's Stream() call.
type delayAgent struct {
	name   string
	delay  time.Duration
	events []agent.Event
}

func newDelayAgent(name string, delay time.Duration, events ...agent.Event) *delayAgent {
	return &delayAgent{name: name, delay: delay, events: events}
}

func (a *delayAgent) Name() string                        { return a.name }
func (a *delayAgent) Available() error                    { return nil }
func (a *delayAgent) Connect(context.Context) error       { return nil }
func (a *delayAgent) Connected() bool                     { return true }
func (a *delayAgent) HandlePermission(string, bool) error { return nil }
func (a *delayAgent) Interrupt() error                    { return nil }
func (a *delayAgent) Close() error                        { return nil }

func (a *delayAgent) SendPrompt(_ context.Context, _ string) (<-chan agent.Event, error) {
	events := a.events
	if len(events) == 0 {
		events = []agent.Event{{Type: agent.EventComplete}}
	}
	ch := make(chan agent.Event, len(events)+1)
	delay := a.delay
	go func() {
		defer close(ch)
		time.Sleep(delay)
		terminal := false
		for _, e := range events {
			if e.Timestamp.IsZero() {
				e.Timestamp = time.Now()
			}
			ch <- e
			if e.Type == agent.EventComplete || e.Type == agent.EventError {
				terminal = true

				break
			}
		}
		if !terminal {
			ch <- agent.Event{Type: agent.EventComplete, Timestamp: time.Now()}
		}
	}()

	return ch, nil
}

func (a *delayAgent) WithEnv(string, string) agent.Agent { return a }
func (a *delayAgent) WithArgs(...string) agent.Agent     { return a }
func (a *delayAgent) WithWorkDir(string) agent.Agent     { return a }
func (a *delayAgent) WithTimeout(time.Duration) agent.Agent {
	return a
}

// setupExecConductor builds a fully-wired Conductor over a real git repo with a
// store and a MockAgent-backed worker pool. The mock emits the given events for
// each job. Auto-advance is left disabled so phase transitions are explicit.
func setupExecConductor(t *testing.T, events ...agent.Event) (*Conductor, string) {
	t.Helper()

	dir := t.TempDir()
	ctx := context.Background()

	for _, args := range [][]string{
		{"init", dir},
		{"-C", dir, "config", "user.email", "test@test.com"},
		{"-C", dir, "config", "user.name", "Test User"},
		{"-C", dir, "config", "commit.gpgsign", "false"},
	} {
		gitCmd(ctx, t, args...)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
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
	if len(events) == 0 {
		events = []agent.Event{
			{Type: agent.EventStream, Content: "done"},
			{Type: agent.EventComplete},
		}
	}
	mock := newDelayAgent("mock", 150*time.Millisecond, events...)
	if err := registry.Register(mock); err != nil {
		t.Fatalf("register mock: %v", err)
	}

	pool := worker.NewPool(worker.PoolConfig{MaxWorkers: 1, Agents: registry})
	if err := pool.Start(); err != nil {
		t.Fatalf("start pool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Stop() })
	if _, err := pool.AddAgentWorker(ctx, "mock", true); err != nil {
		t.Fatalf("add worker: %v", err)
	}

	s := settings.DefaultSettings()
	provisionOff := false
	s.Git.Provision.Enabled = &provisionOff
	s.Git.BaseBranch = "main"
	// Disable external review so quality gates never block on a user prompt.
	s.Workflow.ExternalReview.Mode = settings.ExternalReviewNever

	c := &Conductor{
		machine:            NewMachine(),
		git:                repo,
		worktree:           dir,
		pool:               pool,
		providers:          provider.NewRegistry(s),
		events:             make(chan ConductorEvent, 256),
		pendingPrompts:     make(map[string]chan bool),
		iterationCount:     make(map[string]int),
		retryCount:         make(map[string]int),
		maxIterations:      3,
		phasePolicies:      defaultPhasePolicies(),
		router:             NewDefaultRouter(),
		lifecycleCtx:       ctx,
		varPool:            varpool.New(),
		progressCalibrator: NewProgressCalibrator(),
	}
	c.cachedSettings.Store(s)
	c.store = storage.NewStore(dir, true)

	wu := &WorkUnit{
		ID:          "exec-task",
		Title:       "Exec Task",
		Description: "implement the exec task",
		Branch:      "feature/exec",
		Source:      &Source{Provider: "github", Reference: "owner/repo#1"},
	}
	c.workUnit = wu
	c.machine.SetWorkUnit(wu)
	c.machine.AddListener(c.onStateChanged)

	// Drain events so emit never blocks.
	go func() {
		for range c.events {
		}
	}()

	return c, dir
}

// waitForStateExec polls until the conductor reaches want or the deadline passes.
func waitForStateExec(t *testing.T, c *Conductor, want State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.State() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for state %s (current: %s)", want, c.State())
}

func TestImplement_DrivesToImplemented(t *testing.T) {
	c, dir := setupExecConductor(t)
	c.machine.ForceState(StatePlanned)
	c.workUnit.Specifications = []string{"spec.md"}

	// Make a change so the implement job produces a checkpoint.
	if err := os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n\nfunc Feature() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	jobID, err := c.Implement(ctx)
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
	if jobID == "" {
		t.Error("expected non-empty job ID")
	}

	waitForStateExec(t, c, StateImplemented, 15*time.Second)

	wu := c.GetWorkUnit()
	if !wu.HasImplemented {
		t.Error("HasImplemented should be true after implement completes")
	}
	if len(wu.Checkpoints) == 0 {
		t.Error("expected at least one checkpoint after implement")
	}
}

func TestImplement_SkipPlanFromLoaded(t *testing.T) {
	c, dir := setupExecConductor(t)
	c.machine.ForceState(StateLoaded)

	if err := os.WriteFile(filepath.Join(dir, "skip.go"), []byte("package main\n\nfunc Skip() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := c.Implement(ctx); err != nil {
		t.Fatalf("Implement() from loaded error = %v", err)
	}
	waitForStateExec(t, c, StateImplemented, 15*time.Second)
}

func TestImplement_MinSpecPolicyBlocks(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StatePlanned)
	c.workUnit.Specifications = nil
	s := c.GetEffectiveSettings()
	s.Workflow.Policy.MinSpecSections = 2
	c.cachedSettings.Store(s)

	_, err := c.Implement(context.Background())
	if err == nil {
		t.Fatal("expected min-spec policy to block implement")
	}
	if c.State() != StatePlanned {
		t.Errorf("state should remain planned after policy block, got %s", c.State())
	}
}

func TestOptimize_DrivesBackToImplemented(t *testing.T) {
	c, dir := setupExecConductor(t)
	c.machine.ForceState(StateImplemented)
	c.workUnit.HasImplemented = true

	if err := os.WriteFile(filepath.Join(dir, "opt.go"), []byte("package main\n\nfunc Opt() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := c.Optimize(ctx); err != nil {
		t.Fatalf("Optimize() error = %v", err)
	}
	// Optimize completes back to Implemented.
	waitForStateExec(t, c, StateImplemented, 15*time.Second)
}

func TestSimplify_DrivesBackToImplemented(t *testing.T) {
	c, dir := setupExecConductor(t)
	c.machine.ForceState(StateImplemented)
	c.workUnit.HasImplemented = true

	if err := os.WriteFile(filepath.Join(dir, "simp.go"), []byte("package main\n\nfunc Simp() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := c.Simplify(ctx); err != nil {
		t.Fatalf("Simplify() error = %v", err)
	}
	waitForStateExec(t, c, StateImplemented, 15*time.Second)
}

func TestPlan_DrivesToPlanned_WithSpecFile(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StateLoaded)

	// Pre-seed a specification file so detectSpecificationFiles finds the
	// required deliverable when the plan phase completes.
	specDir := c.store.SpecificationsDir(c.workUnit.ID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specDir, "specification-1.md")
	if err := os.WriteFile(specPath, []byte("# Spec\n\nDo the thing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	jobID, err := c.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if jobID == "" {
		t.Error("expected non-empty scheduler ID")
	}

	waitForStateExec(t, c, StatePlanned, 15*time.Second)

	wu := c.GetWorkUnit()
	if len(wu.Specifications) == 0 {
		t.Error("expected specification to be detected after plan")
	}
}

func TestImplement_AgentFailureRollsBack(t *testing.T) {
	c, _ := setupExecConductor(t, agent.Event{Type: agent.EventError, Error: "agent boom"})
	c.machine.ForceState(StatePlanned)
	c.workUnit.Specifications = []string{"spec.md"}
	// Force fail-fast policy so a single failure rolls back instead of retrying.
	c.phasePolicies[PhaseImplement] = PhasePolicy{Policy: FailurePolicyFail}

	ctx := context.Background()
	if _, err := c.Implement(ctx); err != nil {
		t.Fatalf("Implement() submit error = %v", err)
	}

	// On agent failure with fail-fast policy the machine rolls back to Planned
	// (the default error target for implementing) and does NOT advance.
	waitForStateExec(t, c, StatePlanned, 15*time.Second)
	if c.GetWorkUnit().HasImplemented {
		t.Error("HasImplemented should remain false on failure")
	}
}

func TestReview_TransitionsToReviewing(t *testing.T) {
	c, _ := setupExecConductor(t)
	c.machine.ForceState(StateImplemented)
	c.workUnit.HasImplemented = true
	// Disable external review and async quality gate side effects via defaults.

	ctx := context.Background()
	if err := c.Review(ctx, false); err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if c.State() != StateReviewing {
		t.Errorf("state = %s, want reviewing", c.State())
	}
	waitForQualityGate(t, c)
}

// waitForQualityGate blocks until the async quality gate started by Review
// completes, so the test's TempDir cleanup does not race the gate goroutine.
func waitForQualityGate(t *testing.T, c *Conductor) {
	t.Helper()
	c.mu.RLock()
	ch := c.qualityGateCh
	c.mu.RUnlock()
	if ch != nil {
		select {
		case <-ch:
		case <-time.After(15 * time.Second):
		}
	}
	time.Sleep(150 * time.Millisecond)
}
