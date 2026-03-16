package report

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/activitylog"
)

// Report holds a structured compliance report.
type Report struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Period      string          `json:"period"`
	Tasks       TaskSummary     `json:"tasks"`
	AIUsage     []ModelUsage    `json:"ai_usage"`
	Activity    ActivitySummary `json:"activity"`
}

// TaskSummary aggregates task counts by state.
type TaskSummary struct {
	Total   int            `json:"total"`
	ByState map[string]int `json:"by_state"`
}

// ModelUsage tracks call counts per AI model.
type ModelUsage struct {
	Model     string `json:"model"`
	CallCount int    `json:"call_count"`
}

// ActivitySummary aggregates RPC call statistics.
type ActivitySummary struct {
	TotalCalls    int           `json:"total_calls"`
	ErrorCount    int           `json:"error_count"`
	ErrorRate     float64       `json:"error_rate"`
	AvgDurationMs float64       `json:"avg_duration_ms"`
	TopMethods    []MethodCount `json:"top_methods"`
}

// MethodCount pairs a method name with its invocation count.
type MethodCount struct {
	Method string `json:"method"`
	Count  int    `json:"count"`
}

// TaskInfo is the minimal task information needed for report generation.
type TaskInfo struct {
	ID    string
	Path  string
	State string
}

const topMethodsLimit = 10

// Generate creates a compliance report from activity log entries and task data.
func Generate(entries []activitylog.Entry, tasks []TaskInfo, period string) *Report {
	r := &Report{
		GeneratedAt: time.Now(),
		Period:      period,
		Tasks: TaskSummary{
			Total:   len(tasks),
			ByState: make(map[string]int),
		},
	}

	for _, t := range tasks {
		r.Tasks.ByState[t.State]++
	}

	r.Activity.TotalCalls = len(entries)
	methodCounts := make(map[string]int)
	modelCounts := make(map[string]int)
	var totalDuration int64

	for _, e := range entries {
		if e.Error != "" {
			r.Activity.ErrorCount++
		}
		totalDuration += e.DurationMs
		methodCounts[e.Method]++
		if e.AgentModel != "" {
			modelCounts[e.AgentModel]++
		}
	}

	if len(entries) > 0 {
		r.Activity.ErrorRate = float64(r.Activity.ErrorCount) / float64(len(entries)) * 100
		r.Activity.AvgDurationMs = float64(totalDuration) / float64(len(entries))
	}

	// Top methods sorted by count descending.
	type kv struct {
		key   string
		count int
	}
	sorted := make([]kv, 0, len(methodCounts))
	for k, v := range methodCounts {
		sorted = append(sorted, kv{k, v})
	}
	slices.SortFunc(sorted, func(a, b kv) int {
		if b.count != a.count {
			return b.count - a.count
		}

		return strings.Compare(a.key, b.key)
	})
	limit := min(topMethodsLimit, len(sorted))
	for _, s := range sorted[:limit] {
		r.Activity.TopMethods = append(r.Activity.TopMethods, MethodCount{Method: s.key, Count: s.count})
	}

	// AI model usage.
	for model, count := range modelCounts {
		r.AIUsage = append(r.AIUsage, ModelUsage{Model: model, CallCount: count})
	}
	slices.SortFunc(r.AIUsage, func(a, b ModelUsage) int {
		if b.CallCount != a.CallCount {
			return b.CallCount - a.CallCount
		}

		return strings.Compare(a.Model, b.Model)
	})

	return r
}

// FormatMarkdown renders a report as Markdown.
func FormatMarkdown(r *Report) string {
	var b strings.Builder
	b.WriteString("# Compliance Report\n\n")
	fmt.Fprintf(&b, "Generated: %s\n", r.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "Period: %s\n\n", r.Period)

	b.WriteString("## Tasks\n\n")
	fmt.Fprintf(&b, "Total: %d\n\n", r.Tasks.Total)
	if len(r.Tasks.ByState) > 0 {
		b.WriteString("| State | Count |\n|-------|-------|\n")
		// Sort states for deterministic output.
		states := make([]string, 0, len(r.Tasks.ByState))
		for s := range r.Tasks.ByState {
			states = append(states, s)
		}
		slices.Sort(states)
		for _, state := range states {
			fmt.Fprintf(&b, "| %s | %d |\n", state, r.Tasks.ByState[state])
		}
		b.WriteString("\n")
	}

	b.WriteString("## AI Model Usage\n\n")
	if len(r.AIUsage) > 0 {
		b.WriteString("| Model | Calls |\n|-------|-------|\n")
		for _, u := range r.AIUsage {
			fmt.Fprintf(&b, "| %s | %d |\n", u.Model, u.CallCount)
		}
		b.WriteString("\n")
	} else {
		b.WriteString("No AI model usage recorded.\n\n")
	}

	b.WriteString("## Activity\n\n")
	fmt.Fprintf(&b, "- Total RPC calls: %d\n", r.Activity.TotalCalls)
	fmt.Fprintf(&b, "- Errors: %d (%.1f%%)\n", r.Activity.ErrorCount, r.Activity.ErrorRate)
	fmt.Fprintf(&b, "- Avg duration: %.0fms\n\n", r.Activity.AvgDurationMs)

	if len(r.Activity.TopMethods) > 0 {
		b.WriteString("### Top Methods\n\n")
		b.WriteString("| Method | Count |\n|--------|-------|\n")
		for _, m := range r.Activity.TopMethods {
			fmt.Fprintf(&b, "| %s | %d |\n", m.Method, m.Count)
		}
	}

	return b.String()
}
