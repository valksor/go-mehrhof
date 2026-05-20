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

func TestMetrics_AgentCounters(t *testing.T) {
	m := New()

	m.RecordAgentConnect()
	m.RecordAgentConnect()
	m.RecordAgentConnect()
	m.RecordAgentDisconnect()

	s := m.Snapshot()
	if s.AgentConnects != 3 {
		t.Errorf("AgentConnects = %d, want 3", s.AgentConnects)
	}
	if s.AgentDisconnects != 1 {
		t.Errorf("AgentDisconnects = %d, want 1", s.AgentDisconnects)
	}
}

func TestMetrics_EventsDropped(t *testing.T) {
	m := New()
	for range 5 {
		m.RecordEventDropped()
	}
	if got := m.Snapshot().EventsDropped; got != 5 {
		t.Errorf("EventsDropped = %d, want 5", got)
	}
}

func TestMetrics_PermissionCounters(t *testing.T) {
	m := New()
	m.RecordPermissionApproved()
	m.RecordPermissionApproved()
	m.RecordPermissionDenied()

	s := m.Snapshot()
	if s.PermissionsApproved != 2 {
		t.Errorf("PermissionsApproved = %d, want 2", s.PermissionsApproved)
	}
	if s.PermissionsDenied != 1 {
		t.Errorf("PermissionsDenied = %d, want 1", s.PermissionsDenied)
	}
}

func TestMetrics_Tokens(t *testing.T) {
	m := New()
	m.RecordTokens(100)
	m.RecordTokens(250)

	if got := m.Snapshot().TokensConsumed; got != 350 {
		t.Errorf("TokensConsumed = %d, want 350", got)
	}
}

func TestMetrics_AgentLatency(t *testing.T) {
	m := New()
	m.RecordAgentLatency(10 * time.Millisecond)
	m.RecordAgentLatency(30 * time.Millisecond)
	m.RecordAgentLatency(50 * time.Millisecond)

	if got := m.Snapshot().AgentAvgLatencyMs; got != 30 {
		t.Errorf("AgentAvgLatencyMs = %f, want 30", got)
	}
}

func TestMetrics_AgentLatencyRingBuffer(t *testing.T) {
	m := New()
	// Push more samples than maxLatencySamps (100) — oldest should be dropped.
	for i := range 150 {
		m.RecordAgentLatency(time.Duration(i+1) * time.Millisecond)
	}
	// After 150 inserts, samples are 51ms..150ms; avg = 100.5ms (rounded down to 100 by Milliseconds()).
	got := m.Snapshot().AgentAvgLatencyMs
	if got < 99 || got > 101 {
		t.Errorf("AgentAvgLatencyMs = %f, want roughly 100", got)
	}
}

func TestMetrics_AgentExecution(t *testing.T) {
	m := New()

	m.RecordAgentExecution("claude", 100, 50*time.Millisecond, false)
	m.RecordAgentExecution("claude", 200, 70*time.Millisecond, true)
	m.RecordAgentExecution("codex", 50, 30*time.Millisecond, false)

	s := m.Snapshot()

	if s.Agents == nil {
		t.Fatal("Agents map is nil")
	}
	if len(s.Agents) != 2 {
		t.Errorf("Agents count = %d, want 2", len(s.Agents))
	}

	claude := s.Agents["claude"]
	if claude.Requests != 2 {
		t.Errorf("claude.Requests = %d, want 2", claude.Requests)
	}
	if claude.Errors != 1 {
		t.Errorf("claude.Errors = %d, want 1", claude.Errors)
	}
	if claude.Tokens != 300 {
		t.Errorf("claude.Tokens = %d, want 300", claude.Tokens)
	}
	if claude.AvgLatencyMs != 60 {
		t.Errorf("claude.AvgLatencyMs = %f, want 60", claude.AvgLatencyMs)
	}

	codex := s.Agents["codex"]
	if codex.Requests != 1 {
		t.Errorf("codex.Requests = %d, want 1", codex.Requests)
	}
	if codex.Errors != 0 {
		t.Errorf("codex.Errors = %d, want 0", codex.Errors)
	}
}

func TestMetrics_AgentExecutionEmptyName(t *testing.T) {
	m := New()
	m.RecordAgentExecution("", 100, 50*time.Millisecond, false)

	if s := m.Snapshot(); len(s.Agents) != 0 {
		t.Errorf("empty agent name should be ignored, got %d agents", len(s.Agents))
	}
}

func TestMetrics_RPCLatencyP99(t *testing.T) {
	m := New()
	// 100 requests at 1ms..100ms; p99 ceiling index = ceil(100*0.99)-1 = 98 → 99ms.
	for i := 1; i <= 100; i++ {
		m.RecordRPCRequest("ping", time.Duration(i)*time.Millisecond, nil)
	}

	s := m.Snapshot()
	if s.P99LatencyMs != 99 {
		t.Errorf("P99LatencyMs = %f, want 99", s.P99LatencyMs)
	}
}

func TestMetrics_RPCRequestEmptyMethod(t *testing.T) {
	m := New()
	m.RecordRPCRequest("", 10*time.Millisecond, nil)

	s := m.Snapshot()
	if s.RPCRequests != 1 {
		t.Errorf("RPCRequests = %d, want 1", s.RPCRequests)
	}
	if len(s.Methods) != 0 {
		t.Errorf("Methods should be empty for blank method, got %d", len(s.Methods))
	}
}

func TestMetrics_RPCLatencyRingBuffer(t *testing.T) {
	m := New()
	// Push more samples than maxLatencySamps (100); only the last 100 should be averaged.
	for i := range 200 {
		m.RecordRPCRequest("x", time.Duration(i+1)*time.Millisecond, nil)
	}
	// Samples retained: 101ms..200ms; avg ≈ 150ms.
	got := m.Snapshot().AvgLatencyMs
	if got < 149 || got > 151 {
		t.Errorf("AvgLatencyMs = %f, want ~150", got)
	}
}
