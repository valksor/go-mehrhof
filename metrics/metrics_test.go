package metrics

import (
	"errors"
	"testing"
	"time"
)

func TestMetrics_JobCounters(t *testing.T) {
	m := New()

	m.RecordJobSubmitted()
	m.RecordJobSubmitted()
	m.RecordJobCompleted()
	m.RecordJobFailed()

	s := m.Snapshot()

	if s.JobsSubmitted != 2 {
		t.Errorf("JobsSubmitted = %d, want 2", s.JobsSubmitted)
	}
	if s.JobsCompleted != 1 {
		t.Errorf("JobsCompleted = %d, want 1", s.JobsCompleted)
	}
	if s.JobsFailed != 1 {
		t.Errorf("JobsFailed = %d, want 1", s.JobsFailed)
	}
	if s.JobsInProgress != 0 {
		t.Errorf("JobsInProgress = %d, want 0", s.JobsInProgress)
	}
}

func TestMetrics_RPCCounters(t *testing.T) {
	m := New()

	m.RecordRPCRequest("ping", 10*time.Millisecond, nil)
	m.RecordRPCRequest("status", 20*time.Millisecond, nil)
	m.RecordRPCRequest("ping", 30*time.Millisecond, errors.New("test error"))

	s := m.Snapshot()

	if s.RPCRequests != 3 {
		t.Errorf("RPCRequests = %d, want 3", s.RPCRequests)
	}
	if s.RPCErrors != 1 {
		t.Errorf("RPCErrors = %d, want 1", s.RPCErrors)
	}
	if s.AvgLatencyMs != 20 {
		t.Errorf("AvgLatencyMs = %f, want 20", s.AvgLatencyMs)
	}

	// Verify per-method metrics
	if s.Methods == nil {
		t.Fatal("Methods is nil")
	}
	if len(s.Methods) != 2 {
		t.Errorf("Methods count = %d, want 2", len(s.Methods))
	}

	ping := s.Methods["ping"]
	if ping.Requests != 2 {
		t.Errorf("ping.Requests = %d, want 2", ping.Requests)
	}
	if ping.Errors != 1 {
		t.Errorf("ping.Errors = %d, want 1", ping.Errors)
	}

	status := s.Methods["status"]
	if status.Requests != 1 {
		t.Errorf("status.Requests = %d, want 1", status.Requests)
	}
	if status.Errors != 0 {
		t.Errorf("status.Errors = %d, want 0", status.Errors)
	}
}

func TestMetrics_Global(t *testing.T) {
	g := Global()
	if g == nil {
		t.Error("Global() returned nil")
	}
}
