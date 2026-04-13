// Package metrics provides simple in-memory metrics for kvelmo.
package metrics

import (
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// methodCounter tracks per-method RPC stats using lock-free atomics.
type methodCounter struct {
	requests     atomic.Int64
	errors       atomic.Int64
	totalLatency atomic.Int64 // nanoseconds
}

// Metrics holds application-wide metrics.
type Metrics struct {
	// Job counters
	JobsSubmitted atomic.Int64
	JobsCompleted atomic.Int64
	JobsFailed    atomic.Int64

	// RPC counters
	RPCRequests atomic.Int64
	RPCErrors   atomic.Int64

	// Agent counters
	AgentConnects       atomic.Int64
	AgentDisconnects    atomic.Int64
	EventsDropped       atomic.Int64
	PermissionsApproved atomic.Int64
	PermissionsDenied   atomic.Int64

	// Per-method RPC metrics (lock-free via sync.Map)
	methodMetrics sync.Map // map[string]*methodCounter

	// Token tracking
	TokensConsumed atomic.Int64

	// Per-agent metrics (lock-free via sync.Map)
	agentMetrics sync.Map // map[string]*agentCounter

	// Latency tracking (simple moving average)
	mu              sync.RWMutex
	rpcLatencies    []time.Duration
	agentLatencies  []time.Duration
	maxLatencySamps int
}

// New creates a new Metrics instance.
func New() *Metrics {
	return &Metrics{
		maxLatencySamps: 100, // Keep last 100 latency samples
	}
}

// RecordJobSubmitted increments the jobs submitted counter.
func (m *Metrics) RecordJobSubmitted() {
	m.JobsSubmitted.Add(1)
}

// RecordJobCompleted increments the jobs completed counter.
func (m *Metrics) RecordJobCompleted() {
	m.JobsCompleted.Add(1)
}

// RecordJobFailed increments the jobs failed counter.
func (m *Metrics) RecordJobFailed() {
	m.JobsFailed.Add(1)
}

// RecordAgentConnect increments the agent connects counter.
func (m *Metrics) RecordAgentConnect() {
	m.AgentConnects.Add(1)
}

// RecordAgentDisconnect increments the agent disconnects counter.
func (m *Metrics) RecordAgentDisconnect() {
	m.AgentDisconnects.Add(1)
}

// RecordEventDropped increments the dropped events counter.
func (m *Metrics) RecordEventDropped() {
	m.EventsDropped.Add(1)
}

// RecordPermissionApproved increments the approved permissions counter.
func (m *Metrics) RecordPermissionApproved() {
	m.PermissionsApproved.Add(1)
}

// RecordPermissionDenied increments the denied permissions counter.
func (m *Metrics) RecordPermissionDenied() {
	m.PermissionsDenied.Add(1)
}

// RecordAgentLatency records the duration of an agent execution.
func (m *Metrics) RecordAgentLatency(d time.Duration) {
	m.mu.Lock()
	m.agentLatencies = append(m.agentLatencies, d)
	if len(m.agentLatencies) > m.maxLatencySamps {
		m.agentLatencies = m.agentLatencies[1:]
	}
	m.mu.Unlock()
}

// RecordTokens adds to the cumulative token consumption counter.
func (m *Metrics) RecordTokens(tokens int64) {
	m.TokensConsumed.Add(tokens)
}

// agentCounter tracks per-agent stats using lock-free atomics.
type agentCounter struct {
	tokens   atomic.Int64
	requests atomic.Int64
	errors   atomic.Int64
	latency  atomic.Int64 // nanoseconds total
}

// RecordAgentExecution records metrics for a completed agent execution.
func (m *Metrics) RecordAgentExecution(agentName string, tokens int64, latency time.Duration, failed bool) {
	if agentName == "" {
		return
	}
	val, _ := m.agentMetrics.LoadOrStore(agentName, &agentCounter{})
	ac, ok := val.(*agentCounter)
	if !ok {
		ac = &agentCounter{}
		m.agentMetrics.Store(agentName, ac)
	}
	ac.requests.Add(1)
	ac.tokens.Add(tokens)
	ac.latency.Add(int64(latency))
	if failed {
		ac.errors.Add(1)
	}
}

// RecordRPCRequest records an RPC request with its latency and per-method breakdown.
func (m *Metrics) RecordRPCRequest(method string, latency time.Duration, err error) {
	m.RPCRequests.Add(1)
	if err != nil {
		m.RPCErrors.Add(1)
	}

	// Per-method tracking
	if method != "" {
		val, _ := m.methodMetrics.LoadOrStore(method, &methodCounter{})
		mc, ok := val.(*methodCounter)
		if !ok {
			mc = &methodCounter{}
			m.methodMetrics.Store(method, mc)
		}
		mc.requests.Add(1)
		if err != nil {
			mc.errors.Add(1)
		}
		mc.totalLatency.Add(int64(latency))
	}

	m.mu.Lock()
	m.rpcLatencies = append(m.rpcLatencies, latency)
	if len(m.rpcLatencies) > m.maxLatencySamps {
		m.rpcLatencies = m.rpcLatencies[1:]
	}
	m.mu.Unlock()
}

// MethodSnapshot holds per-method RPC statistics.
type MethodSnapshot struct {
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// AgentSnapshot holds per-agent execution statistics.
type AgentSnapshot struct {
	Tokens       int64   `json:"tokens"`
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// Snapshot returns a point-in-time snapshot of all metrics.
type Snapshot struct {
	JobsSubmitted  int64   `json:"jobs_submitted"`
	JobsCompleted  int64   `json:"jobs_completed"`
	JobsFailed     int64   `json:"jobs_failed"`
	JobsInProgress int64   `json:"jobs_in_progress"`
	RPCRequests    int64   `json:"rpc_requests"`
	RPCErrors      int64   `json:"rpc_errors"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	P99LatencyMs   float64 `json:"p99_latency_ms"`

	// Agent metrics
	AgentConnects       int64   `json:"agent_connects"`
	AgentDisconnects    int64   `json:"agent_disconnects"`
	AgentAvgLatencyMs   float64 `json:"agent_avg_latency_ms"`
	EventsDropped       int64   `json:"events_dropped"`
	PermissionsApproved int64   `json:"permissions_approved"`
	PermissionsDenied   int64   `json:"permissions_denied"`
	TokensConsumed      int64   `json:"tokens_consumed"`

	// Per-method RPC metrics
	Methods map[string]MethodSnapshot `json:"methods,omitempty"`

	// Per-agent execution metrics
	Agents map[string]AgentSnapshot `json:"agents,omitempty"`
}

// Snapshot returns current metrics values.
func (m *Metrics) Snapshot() Snapshot {
	s := Snapshot{
		JobsSubmitted: m.JobsSubmitted.Load(),
		JobsCompleted: m.JobsCompleted.Load(),
		JobsFailed:    m.JobsFailed.Load(),
		RPCRequests:   m.RPCRequests.Load(),
		RPCErrors:     m.RPCErrors.Load(),
	}
	s.JobsInProgress = max(0, s.JobsSubmitted-s.JobsCompleted-s.JobsFailed)
	s.AgentConnects = m.AgentConnects.Load()
	s.AgentDisconnects = m.AgentDisconnects.Load()
	s.EventsDropped = m.EventsDropped.Load()
	s.PermissionsApproved = m.PermissionsApproved.Load()
	s.PermissionsDenied = m.PermissionsDenied.Load()
	s.TokensConsumed = m.TokensConsumed.Load()

	// Collect per-method metrics
	methods := make(map[string]MethodSnapshot)
	m.methodMetrics.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		mc, ok := value.(*methodCounter)
		if !ok {
			return true
		}
		reqs := mc.requests.Load()
		ms := MethodSnapshot{
			Requests: reqs,
			Errors:   mc.errors.Load(),
		}
		if reqs > 0 {
			ms.AvgLatencyMs = float64(mc.totalLatency.Load()) / float64(time.Millisecond) / float64(reqs)
		}
		methods[name] = ms

		return true
	})
	if len(methods) > 0 {
		s.Methods = methods
	}

	// Collect per-agent metrics
	agents := make(map[string]AgentSnapshot)
	m.agentMetrics.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		ac, ok := value.(*agentCounter)
		if !ok {
			return true
		}
		reqs := ac.requests.Load()
		as := AgentSnapshot{
			Tokens:   ac.tokens.Load(),
			Requests: reqs,
			Errors:   ac.errors.Load(),
		}
		if reqs > 0 {
			as.AvgLatencyMs = float64(ac.latency.Load()) / float64(time.Millisecond) / float64(reqs)
		}
		agents[name] = as

		return true
	})
	if len(agents) > 0 {
		s.Agents = agents
	}

	m.mu.RLock()
	if len(m.rpcLatencies) > 0 {
		var total time.Duration
		for _, l := range m.rpcLatencies {
			total += l
		}
		s.AvgLatencyMs = float64(total.Milliseconds()) / float64(len(m.rpcLatencies))

		// P99: 99th percentile using ceiling-based index
		if len(m.rpcLatencies) > 1 {
			sorted := make([]time.Duration, len(m.rpcLatencies))
			copy(sorted, m.rpcLatencies)
			slices.Sort(sorted)

			n := len(sorted)
			p99Idx := max(int(math.Ceil(float64(n)*0.99))-1, 0)
			if p99Idx >= n {
				p99Idx = n - 1
			}
			s.P99LatencyMs = float64(sorted[p99Idx].Milliseconds())
		}
	}
	if len(m.agentLatencies) > 0 {
		var total time.Duration
		for _, l := range m.agentLatencies {
			total += l
		}
		s.AgentAvgLatencyMs = float64(total.Milliseconds()) / float64(len(m.agentLatencies))
	}
	m.mu.RUnlock()

	return s
}

// RestoreFrom sets counter values from a previously saved snapshot.
// Used to restore metrics after a process restart.
func (m *Metrics) RestoreFrom(snap Snapshot) {
	m.JobsSubmitted.Store(snap.JobsSubmitted)
	m.JobsCompleted.Store(snap.JobsCompleted)
	m.JobsFailed.Store(snap.JobsFailed)
	m.RPCRequests.Store(snap.RPCRequests)
	m.RPCErrors.Store(snap.RPCErrors)
	m.AgentConnects.Store(snap.AgentConnects)
	m.AgentDisconnects.Store(snap.AgentDisconnects)
	m.EventsDropped.Store(snap.EventsDropped)
	m.PermissionsApproved.Store(snap.PermissionsApproved)
	m.PermissionsDenied.Store(snap.PermissionsDenied)
	m.TokensConsumed.Store(snap.TokensConsumed)
}

// Global metrics instance.
var global = New()

// Global returns the global metrics instance.
func Global() *Metrics {
	return global
}
