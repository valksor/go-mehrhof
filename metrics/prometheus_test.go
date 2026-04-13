package metrics

import (
	"strings"
	"testing"
)

func TestRenderPrometheus_ZeroSnapshot(t *testing.T) {
	out := RenderPrometheus(Snapshot{})

	// Every metric should have HELP and TYPE lines
	expectedMetrics := []string{
		"kvelmo_jobs_submitted_total",
		"kvelmo_jobs_completed_total",
		"kvelmo_jobs_failed_total",
		"kvelmo_jobs_in_progress",
		"kvelmo_rpc_requests_total",
		"kvelmo_rpc_errors_total",
		"kvelmo_rpc_latency_avg_ms",
		"kvelmo_rpc_latency_p99_ms",
		"kvelmo_agent_connects_total",
		"kvelmo_agent_disconnects_total",
		"kvelmo_events_dropped_total",
		"kvelmo_permissions_approved_total",
		"kvelmo_permissions_denied_total",
	}

	for _, name := range expectedMetrics {
		if !strings.Contains(out, "# HELP "+name+" ") {
			t.Errorf("missing HELP line for %s", name)
		}
		if !strings.Contains(out, "# TYPE "+name+" ") {
			t.Errorf("missing TYPE line for %s", name)
		}
		// Value line: metric name followed by a space and value
		if !strings.Contains(out, name+" 0") {
			t.Errorf("missing zero value line for %s", name)
		}
	}
}

func TestRenderPrometheus_NonZeroValues(t *testing.T) {
	snap := Snapshot{
		JobsSubmitted:       10,
		JobsCompleted:       7,
		JobsFailed:          1,
		JobsInProgress:      2,
		RPCRequests:         100,
		RPCErrors:           3,
		AvgLatencyMs:        12.5,
		P99LatencyMs:        45,
		AgentConnects:       5,
		AgentDisconnects:    2,
		EventsDropped:       1,
		PermissionsApproved: 20,
		PermissionsDenied:   4,
	}

	out := RenderPrometheus(snap)

	checks := []struct {
		substr string
	}{
		{"kvelmo_jobs_submitted_total 10"},
		{"kvelmo_jobs_completed_total 7"},
		{"kvelmo_jobs_failed_total 1"},
		{"kvelmo_jobs_in_progress 2"},
		{"kvelmo_rpc_requests_total 100"},
		{"kvelmo_rpc_errors_total 3"},
		{"kvelmo_rpc_latency_avg_ms 12.5"},
		{"kvelmo_rpc_latency_p99_ms 45"},
		{"kvelmo_agent_connects_total 5"},
		{"kvelmo_agent_disconnects_total 2"},
		{"kvelmo_events_dropped_total 1"},
		{"kvelmo_permissions_approved_total 20"},
		{"kvelmo_permissions_denied_total 4"},
	}

	for _, tc := range checks {
		if !strings.Contains(out, tc.substr) {
			t.Errorf("output missing %q", tc.substr)
		}
	}
}

func TestRenderPrometheus_TypeAnnotations(t *testing.T) {
	out := RenderPrometheus(Snapshot{})

	// Verify counter vs gauge types
	counters := []string{
		"kvelmo_jobs_submitted_total",
		"kvelmo_jobs_completed_total",
		"kvelmo_jobs_failed_total",
		"kvelmo_rpc_requests_total",
		"kvelmo_rpc_errors_total",
		"kvelmo_agent_connects_total",
		"kvelmo_agent_disconnects_total",
		"kvelmo_events_dropped_total",
		"kvelmo_permissions_approved_total",
		"kvelmo_permissions_denied_total",
	}
	gauges := []string{
		"kvelmo_jobs_in_progress",
		"kvelmo_rpc_latency_avg_ms",
		"kvelmo_rpc_latency_p99_ms",
	}

	for _, name := range counters {
		if !strings.Contains(out, "# TYPE "+name+" counter") {
			t.Errorf("expected counter type for %s", name)
		}
	}
	for _, name := range gauges {
		if !strings.Contains(out, "# TYPE "+name+" gauge") {
			t.Errorf("expected gauge type for %s", name)
		}
	}
}
