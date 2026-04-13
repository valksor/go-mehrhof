package agent

import (
	"sync"
	"testing"
	"time"
)

func TestNewHealthStartsHealthy(t *testing.T) {
	h := NewHealth()
	if !h.IsHealthy() {
		t.Fatal("new health should be healthy")
	}
	if h.FailureCount() != 0 {
		t.Fatalf("expected 0 failures, got %d", h.FailureCount())
	}
	if h.CooldownRemaining() != 0 {
		t.Fatalf("expected 0 cooldown, got %v", h.CooldownRemaining())
	}
}

func TestRecordFailureMakesUnhealthy(t *testing.T) {
	h := NewHealth()
	now := time.Now()
	h.nowFunc = func() time.Time { return now }

	h.RecordFailure()

	if h.IsHealthy() {
		t.Fatal("should be unhealthy after failure")
	}
	if h.FailureCount() != 1 {
		t.Fatalf("expected 1 failure, got %d", h.FailureCount())
	}
}

func TestCooldownProgression(t *testing.T) {
	tests := []struct {
		failures int
		expected time.Duration
	}{
		{1, 30 * time.Second},
		{2, 60 * time.Second},
		{3, 120 * time.Second},
		{4, 240 * time.Second},
	}

	for _, tt := range tests {
		got := cooldownDuration(tt.failures)
		if got != tt.expected {
			t.Errorf("cooldownDuration(%d) = %v, want %v", tt.failures, got, tt.expected)
		}
	}
}

func TestCooldownMaxFiveMinutes(t *testing.T) {
	// Failures beyond 4 should still cap at 5 minutes
	for _, failures := range []int{5, 6, 10, 100} {
		got := cooldownDuration(failures)
		if got > 5*time.Minute {
			t.Errorf("cooldownDuration(%d) = %v, want <= 5m", failures, got)
		}
	}
}

func TestRecordSuccessResets(t *testing.T) {
	h := NewHealth()
	now := time.Now()
	h.nowFunc = func() time.Time { return now }

	h.RecordFailure()
	h.RecordFailure()

	if h.IsHealthy() {
		t.Fatal("should be unhealthy after failures")
	}

	h.RecordSuccess()

	if !h.IsHealthy() {
		t.Fatal("should be healthy after success")
	}
	if h.FailureCount() != 0 {
		t.Fatalf("expected 0 failures after success, got %d", h.FailureCount())
	}
	if h.CooldownRemaining() != 0 {
		t.Fatalf("expected 0 cooldown after success, got %v", h.CooldownRemaining())
	}
}

func TestIsHealthyAfterCooldownExpires(t *testing.T) {
	h := NewHealth()
	now := time.Now()
	h.nowFunc = func() time.Time { return now }

	h.RecordFailure()

	if h.IsHealthy() {
		t.Fatal("should be unhealthy during cooldown")
	}

	// Advance past cooldown (30s for first failure)
	now = now.Add(31 * time.Second)

	if !h.IsHealthy() {
		t.Fatal("should be healthy after cooldown expires")
	}
}

func TestCooldownRemaining(t *testing.T) {
	h := NewHealth()
	now := time.Now()
	h.nowFunc = func() time.Time { return now }

	h.RecordFailure()

	remaining := h.CooldownRemaining()
	if remaining <= 0 || remaining > 30*time.Second {
		t.Fatalf("expected cooldown ~30s, got %v", remaining)
	}

	// Advance 10 seconds
	now = now.Add(10 * time.Second)
	remaining = h.CooldownRemaining()

	if remaining <= 0 || remaining > 20*time.Second {
		t.Fatalf("expected cooldown ~20s, got %v", remaining)
	}

	// Advance past cooldown
	now = now.Add(25 * time.Second)
	remaining = h.CooldownRemaining()

	if remaining != 0 {
		t.Fatalf("expected 0 cooldown after expiry, got %v", remaining)
	}
}

func TestReset(t *testing.T) {
	h := NewHealth()
	now := time.Now()
	h.nowFunc = func() time.Time { return now }

	h.RecordFailure()
	h.RecordFailure()
	h.RecordSuccess()
	h.RecordFailure()

	h.Reset()

	if !h.IsHealthy() {
		t.Fatal("should be healthy after reset")
	}
	if h.FailureCount() != 0 {
		t.Fatalf("expected 0 failures after reset, got %d", h.FailureCount())
	}
	if h.CooldownRemaining() != 0 {
		t.Fatalf("expected 0 cooldown after reset, got %v", h.CooldownRemaining())
	}
}

func TestCooldownDurationZeroFailures(t *testing.T) {
	if d := cooldownDuration(0); d != 0 {
		t.Fatalf("expected 0 for 0 failures, got %v", d)
	}
	if d := cooldownDuration(-1); d != 0 {
		t.Fatalf("expected 0 for negative failures, got %v", d)
	}
}

// --- HealthRegistry tests ---

func TestHealthRegistryCreatesLazily(t *testing.T) {
	r := NewHealthRegistry()

	h1 := r.Get("claude")
	if h1 == nil {
		t.Fatal("Get should return non-nil health")
	}
	if !h1.IsHealthy() {
		t.Fatal("new tracker should be healthy")
	}

	// Same tracker on second access
	h2 := r.Get("claude")
	if h1 != h2 {
		t.Fatal("Get should return the same tracker for the same name")
	}
}

func TestSelectHealthyPicksFirst(t *testing.T) {
	r := NewHealthRegistry()
	now := time.Now()

	// Make "claude" unhealthy
	claude := r.Get("claude")
	claude.nowFunc = func() time.Time { return now }
	claude.RecordFailure()

	// "codex" stays healthy
	_ = r.Get("codex")

	name, ok := r.SelectHealthy([]string{"claude", "codex"})
	if !ok {
		t.Fatal("should find a healthy agent")
	}
	if name != "codex" {
		t.Fatalf("expected codex, got %s", name)
	}
}

func TestSelectHealthyAllUnhealthy(t *testing.T) {
	r := NewHealthRegistry()
	now := time.Now()

	for _, name := range []string{"claude", "codex"} {
		h := r.Get(name)
		h.nowFunc = func() time.Time { return now }
		h.RecordFailure()
	}

	_, ok := r.SelectHealthy([]string{"claude", "codex"})
	if ok {
		t.Fatal("should return false when all unhealthy")
	}
}

func TestSelectHealthyEmptyList(t *testing.T) {
	r := NewHealthRegistry()

	_, ok := r.SelectHealthy(nil)
	if ok {
		t.Fatal("should return false for empty list")
	}
}

func TestHealthRegistryStatus(t *testing.T) {
	r := NewHealthRegistry()
	now := time.Now()

	// Healthy agent
	_ = r.Get("codex")

	// Unhealthy agent
	claude := r.Get("claude")
	claude.nowFunc = func() time.Time { return now }
	claude.RecordFailure()
	claude.RecordFailure()

	statuses := r.Status()

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	codexStatus := statuses["codex"]
	if !codexStatus.Healthy {
		t.Fatal("codex should be healthy")
	}
	if codexStatus.FailureCount != 0 {
		t.Fatalf("codex failure count should be 0, got %d", codexStatus.FailureCount)
	}
	if codexStatus.LastFailure != nil {
		t.Fatal("codex should have no last failure")
	}

	claudeStatus := statuses["claude"]
	if claudeStatus.Healthy {
		t.Fatal("claude should be unhealthy")
	}
	if claudeStatus.FailureCount != 2 {
		t.Fatalf("claude failure count should be 2, got %d", claudeStatus.FailureCount)
	}
	if claudeStatus.LastFailure == nil {
		t.Fatal("claude should have a last failure time")
	}
	if claudeStatus.CooldownRemaining == "" {
		t.Fatal("claude should have cooldown remaining")
	}
}

func TestConcurrentAccess(t *testing.T) {
	h := NewHealth()

	var wg sync.WaitGroup

	// Spawn goroutines that hammer the health tracker
	for range 100 {
		wg.Add(3)

		go func() {
			defer wg.Done()
			h.RecordFailure()
		}()

		go func() {
			defer wg.Done()
			h.RecordSuccess()
		}()

		go func() {
			defer wg.Done()
			_ = h.IsHealthy()
			_ = h.FailureCount()
			_ = h.CooldownRemaining()
		}()
	}

	wg.Wait()

	// Just ensure no panics or deadlocks
}

func TestHealthRegistryConcurrentAccess(t *testing.T) {
	r := NewHealthRegistry()
	names := []string{"claude", "codex", "custom", "gemini"}

	var wg sync.WaitGroup

	for range 100 {
		wg.Add(4)

		go func() {
			defer wg.Done()
			for _, name := range names {
				r.Get(name).RecordFailure()
			}
		}()

		go func() {
			defer wg.Done()
			for _, name := range names {
				r.Get(name).RecordSuccess()
			}
		}()

		go func() {
			defer wg.Done()
			r.SelectHealthy(names)
		}()

		go func() {
			defer wg.Done()
			r.Status()
		}()
	}

	wg.Wait()
}
