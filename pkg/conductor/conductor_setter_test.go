package conductor

import (
	"slices"
	"testing"

	"github.com/valksor/kvelmo/pkg/agent/strategy"
	"github.com/valksor/kvelmo/pkg/eventlog"
)

func TestSetAutoAdvance(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	c.SetAutoAdvance(true)
	if !c.AutoAdvance() {
		t.Error("AutoAdvance() = false after SetAutoAdvance(true)")
	}

	c.SetAutoAdvance(false)
	if c.AutoAdvance() {
		t.Error("AutoAdvance() = true after SetAutoAdvance(false)")
	}
}

func TestSetDryRun(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	c.SetDryRun(true)
	if !c.DryRunEnabled() {
		t.Error("DryRunEnabled() = false after SetDryRun(true)")
	}

	c.SetDryRun(false)
	if c.DryRunEnabled() {
		t.Error("DryRunEnabled() = true after SetDryRun(false)")
	}
}

func TestSetSkipPhases(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	phases := []string{"simplify", "optimize"}
	c.SetSkipPhases(phases)

	got := c.SkipPhases()
	for _, p := range phases {
		if !slices.Contains(got, p) {
			t.Errorf("SkipPhases() missing %q", p)
		}
	}
}

func TestSetStrategy(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Get the "direct" strategy which is always registered.
	s, ok := strategy.Get("direct")
	if !ok {
		t.Fatal("strategy.Get(\"direct\") not found")
	}

	c.SetStrategy(s)

	c.mu.RLock()
	got := c.strategy
	c.mu.RUnlock()

	if got == nil {
		t.Fatal("strategy is nil after SetStrategy")
	}
	if got.Name() != "direct" {
		t.Errorf("strategy.Name() = %q, want %q", got.Name(), "direct")
	}
}

func TestSetPhaseStrategy(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	s, ok := strategy.Get("direct")
	if !ok {
		t.Fatal("strategy.Get(\"direct\") not found")
	}

	c.SetPhaseStrategy("plan", s)

	c.mu.RLock()
	got, exists := c.phaseStrategies["plan"]
	c.mu.RUnlock()

	if !exists {
		t.Fatal("phaseStrategies[\"plan\"] not set")
	}
	if got.Name() != "direct" {
		t.Errorf("phaseStrategies[\"plan\"].Name() = %q, want %q", got.Name(), "direct")
	}

	// Clear override by setting nil.
	c.SetPhaseStrategy("plan", nil)

	c.mu.RLock()
	_, exists = c.phaseStrategies["plan"]
	c.mu.RUnlock()

	if exists {
		t.Error("phaseStrategies[\"plan\"] still exists after SetPhaseStrategy(nil)")
	}
}

func TestSetEventLog(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	dir := t.TempDir()
	log, err := eventlog.New(dir)
	if err != nil {
		t.Fatalf("eventlog.New() error: %v", err)
	}

	c.SetEventLog(log)

	c.mu.RLock()
	got := c.eventLog
	c.mu.RUnlock()

	if got == nil {
		t.Error("eventLog is nil after SetEventLog")
	}
}

func TestSetNotifier_Nil(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Should not panic when called with nil.
	c.SetNotifier(nil)

	c.mu.RLock()
	got := c.notifier
	c.mu.RUnlock()

	if got != nil {
		t.Error("notifier should be nil after SetNotifier(nil)")
	}
}

func TestVarPool(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	pool := c.VarPool()
	if pool == nil {
		t.Error("VarPool() returned nil on initialized conductor")
	}
}
