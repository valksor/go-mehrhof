package metrics

import (
	"fmt"
	"slices"
	"strings"
)

// RenderPrometheus renders a Snapshot in Prometheus text exposition format.
func RenderPrometheus(snap Snapshot) string {
	var b []byte

	// Helper to write a metric
	write := func(name, help, mtype string, value any) {
		b = fmt.Appendf(b, "# HELP %s %s\n# TYPE %s %s\n%s %v\n", name, help, name, mtype, name, value)
	}

	// Helper to write a labeled metric
	writeLabeled := func(name, labels string, value any) {
		b = fmt.Appendf(b, "%s{%s} %v\n", name, labels, value)
	}

	// Job metrics (counters)
	write("kvelmo_jobs_submitted_total", "Total jobs submitted.", "counter", snap.JobsSubmitted)
	write("kvelmo_jobs_completed_total", "Total jobs completed.", "counter", snap.JobsCompleted)
	write("kvelmo_jobs_failed_total", "Total jobs failed.", "counter", snap.JobsFailed)
	// In-progress is a gauge (derived)
	write("kvelmo_jobs_in_progress", "Jobs currently in progress.", "gauge", snap.JobsInProgress)

	// RPC metrics
	write("kvelmo_rpc_requests_total", "Total RPC requests.", "counter", snap.RPCRequests)
	write("kvelmo_rpc_errors_total", "Total RPC errors.", "counter", snap.RPCErrors)

	// Latency (as gauge for avg/p99)
	write("kvelmo_rpc_latency_avg_ms", "Average RPC latency in milliseconds.", "gauge", snap.AvgLatencyMs)
	write("kvelmo_rpc_latency_p99_ms", "P99 RPC latency in milliseconds.", "gauge", snap.P99LatencyMs)

	// Agent metrics
	write("kvelmo_agent_connects_total", "Total agent connections.", "counter", snap.AgentConnects)
	write("kvelmo_agent_disconnects_total", "Total agent disconnections.", "counter", snap.AgentDisconnects)
	write("kvelmo_agent_latency_avg_ms", "Average agent execution latency in milliseconds.", "gauge", snap.AgentAvgLatencyMs)
	write("kvelmo_events_dropped_total", "Total events dropped.", "counter", snap.EventsDropped)
	write("kvelmo_permissions_approved_total", "Total permissions approved.", "counter", snap.PermissionsApproved)
	write("kvelmo_permissions_denied_total", "Total permissions denied.", "counter", snap.PermissionsDenied)
	write("kvelmo_tokens_consumed_total", "Total tokens consumed by agent executions.", "counter", snap.TokensConsumed)

	// Per-method RPC metrics
	if len(snap.Methods) > 0 {
		// Sort method names for stable output
		methods := make([]string, 0, len(snap.Methods))
		for name := range snap.Methods {
			methods = append(methods, name)
		}
		slices.Sort(methods)

		b = fmt.Appendf(b, "# HELP kvelmo_rpc_method_requests_total Total requests per RPC method.\n# TYPE kvelmo_rpc_method_requests_total counter\n")
		for _, name := range methods {
			ms := snap.Methods[name]
			label := fmt.Sprintf("method=%q", name)
			writeLabeled("kvelmo_rpc_method_requests_total", label, ms.Requests)
		}

		b = fmt.Appendf(b, "# HELP kvelmo_rpc_method_errors_total Total errors per RPC method.\n# TYPE kvelmo_rpc_method_errors_total counter\n")
		for _, name := range methods {
			ms := snap.Methods[name]
			label := fmt.Sprintf("method=%q", name)
			writeLabeled("kvelmo_rpc_method_errors_total", label, ms.Errors)
		}

		b = fmt.Appendf(b, "# HELP kvelmo_rpc_method_latency_avg_ms Average latency per RPC method.\n# TYPE kvelmo_rpc_method_latency_avg_ms gauge\n")
		for _, name := range methods {
			ms := snap.Methods[name]
			label := fmt.Sprintf("method=%q", name)
			writeLabeled("kvelmo_rpc_method_latency_avg_ms", label, ms.AvgLatencyMs)
		}
	}

	return strings.TrimRight(string(b), "\n") + "\n"
}
