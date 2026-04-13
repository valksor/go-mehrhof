package metrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestoreFrom(t *testing.T) {
	m := New()
	snap := Snapshot{
		JobsSubmitted:       10,
		JobsCompleted:       7,
		JobsFailed:          2,
		RPCRequests:         100,
		RPCErrors:           5,
		AgentConnects:       3,
		AgentDisconnects:    1,
		EventsDropped:       4,
		PermissionsApproved: 20,
		PermissionsDenied:   6,
	}

	m.RestoreFrom(snap)

	if got := m.JobsSubmitted.Load(); got != 10 {
		t.Errorf("JobsSubmitted = %d, want 10", got)
	}
	if got := m.JobsCompleted.Load(); got != 7 {
		t.Errorf("JobsCompleted = %d, want 7", got)
	}
	if got := m.JobsFailed.Load(); got != 2 {
		t.Errorf("JobsFailed = %d, want 2", got)
	}
	if got := m.RPCRequests.Load(); got != 100 {
		t.Errorf("RPCRequests = %d, want 100", got)
	}
	if got := m.RPCErrors.Load(); got != 5 {
		t.Errorf("RPCErrors = %d, want 5", got)
	}
	if got := m.AgentConnects.Load(); got != 3 {
		t.Errorf("AgentConnects = %d, want 3", got)
	}
	if got := m.AgentDisconnects.Load(); got != 1 {
		t.Errorf("AgentDisconnects = %d, want 1", got)
	}
	if got := m.EventsDropped.Load(); got != 4 {
		t.Errorf("EventsDropped = %d, want 4", got)
	}
	if got := m.PermissionsApproved.Load(); got != 20 {
		t.Errorf("PermissionsApproved = %d, want 20", got)
	}
	if got := m.PermissionsDenied.Load(); got != 6 {
		t.Errorf("PermissionsDenied = %d, want 6", got)
	}
}

func TestPersisterLoadNonExistentFile(t *testing.T) {
	m := New()
	p := NewPersister(m, filepath.Join(t.TempDir(), "does-not-exist.json"), time.Minute)

	// Should be a no-op, no panic
	p.Load()

	if got := m.JobsSubmitted.Load(); got != 0 {
		t.Errorf("JobsSubmitted = %d after loading non-existent file, want 0", got)
	}
}

func TestPersisterSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")

	// Create metrics with some values
	m1 := New()
	m1.JobsSubmitted.Store(42)
	m1.JobsCompleted.Store(30)
	m1.JobsFailed.Store(5)
	m1.RPCRequests.Store(200)
	m1.RPCErrors.Store(10)
	m1.AgentConnects.Store(8)
	m1.AgentDisconnects.Store(2)
	m1.EventsDropped.Store(3)
	m1.PermissionsApproved.Store(15)
	m1.PermissionsDenied.Store(7)

	// Save via persister
	p1 := NewPersister(m1, path, time.Minute)
	p1.save()

	// Verify file was created
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("metrics file not created: %v", err)
	}

	// Load into fresh metrics
	m2 := New()
	p2 := NewPersister(m2, path, time.Minute)
	p2.Load()

	// Verify all counters match
	snap1 := m1.Snapshot()
	snap2 := m2.Snapshot()

	if snap2.JobsSubmitted != snap1.JobsSubmitted {
		t.Errorf("JobsSubmitted = %d, want %d", snap2.JobsSubmitted, snap1.JobsSubmitted)
	}
	if snap2.JobsCompleted != snap1.JobsCompleted {
		t.Errorf("JobsCompleted = %d, want %d", snap2.JobsCompleted, snap1.JobsCompleted)
	}
	if snap2.JobsFailed != snap1.JobsFailed {
		t.Errorf("JobsFailed = %d, want %d", snap2.JobsFailed, snap1.JobsFailed)
	}
	if snap2.RPCRequests != snap1.RPCRequests {
		t.Errorf("RPCRequests = %d, want %d", snap2.RPCRequests, snap1.RPCRequests)
	}
	if snap2.RPCErrors != snap1.RPCErrors {
		t.Errorf("RPCErrors = %d, want %d", snap2.RPCErrors, snap1.RPCErrors)
	}
	if snap2.AgentConnects != snap1.AgentConnects {
		t.Errorf("AgentConnects = %d, want %d", snap2.AgentConnects, snap1.AgentConnects)
	}
	if snap2.AgentDisconnects != snap1.AgentDisconnects {
		t.Errorf("AgentDisconnects = %d, want %d", snap2.AgentDisconnects, snap1.AgentDisconnects)
	}
	if snap2.EventsDropped != snap1.EventsDropped {
		t.Errorf("EventsDropped = %d, want %d", snap2.EventsDropped, snap1.EventsDropped)
	}
	if snap2.PermissionsApproved != snap1.PermissionsApproved {
		t.Errorf("PermissionsApproved = %d, want %d", snap2.PermissionsApproved, snap1.PermissionsApproved)
	}
	if snap2.PermissionsDenied != snap1.PermissionsDenied {
		t.Errorf("PermissionsDenied = %d, want %d", snap2.PermissionsDenied, snap1.PermissionsDenied)
	}
}

func TestPersisterLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")

	// Write corrupt JSON
	if err := os.WriteFile(path, []byte("{not valid json!!!"), 0o640); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	m := New()
	m.JobsSubmitted.Store(99)

	p := NewPersister(m, path, time.Minute)
	p.Load()

	// Counters should remain unchanged (corrupt file ignored)
	if got := m.JobsSubmitted.Load(); got != 99 {
		t.Errorf("JobsSubmitted = %d after loading corrupt file, want 99", got)
	}
}

func TestPersisterStartSavesOnCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")

	m := New()
	m.JobsSubmitted.Store(77)

	p := NewPersister(m, path, time.Hour) // Long interval so only shutdown save fires

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		p.Start(ctx)
		close(done)
	}()

	// Cancel immediately to trigger shutdown save
	cancel()
	<-done

	// Verify file was written on shutdown
	m2 := New()
	p2 := NewPersister(m2, path, time.Minute)
	p2.Load()

	if got := m2.JobsSubmitted.Load(); got != 77 {
		t.Errorf("JobsSubmitted = %d after shutdown save, want 77", got)
	}
}
