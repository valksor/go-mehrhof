package report

import (
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/pkg/activitylog"
)

func TestGenerate_Empty(t *testing.T) {
	r := Generate(nil, nil, "7d")
	if r.Tasks.Total != 0 {
		t.Errorf("expected 0 tasks, got %d", r.Tasks.Total)
	}
	if r.Activity.TotalCalls != 0 {
		t.Errorf("expected 0 calls, got %d", r.Activity.TotalCalls)
	}
	if r.Activity.ErrorRate != 0 {
		t.Errorf("expected 0 error rate, got %f", r.Activity.ErrorRate)
	}
	if r.Period != "7d" {
		t.Errorf("expected period 7d, got %s", r.Period)
	}
}

func TestGenerate_WithData(t *testing.T) {
	entries := []activitylog.Entry{
		{Timestamp: time.Now(), Method: "start", DurationMs: 100, AgentModel: "claude"},
		{Timestamp: time.Now(), Method: "plan", DurationMs: 200, AgentModel: "claude"},
		{Timestamp: time.Now(), Method: "implement", DurationMs: 300, Error: "timeout"},
	}
	tasks := []TaskInfo{
		{ID: "t1", State: "submitted"},
		{ID: "t2", State: "implementing"},
	}

	r := Generate(entries, tasks, "7d")

	if r.Tasks.Total != 2 {
		t.Errorf("expected 2 tasks, got %d", r.Tasks.Total)
	}
	if r.Tasks.ByState["submitted"] != 1 {
		t.Errorf("expected 1 submitted task, got %d", r.Tasks.ByState["submitted"])
	}
	if r.Tasks.ByState["implementing"] != 1 {
		t.Errorf("expected 1 implementing task, got %d", r.Tasks.ByState["implementing"])
	}
	if r.Activity.TotalCalls != 3 {
		t.Errorf("expected 3 calls, got %d", r.Activity.TotalCalls)
	}
	if r.Activity.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", r.Activity.ErrorCount)
	}
	expectedRate := float64(1) / float64(3) * 100
	if r.Activity.ErrorRate != expectedRate {
		t.Errorf("expected error rate %f, got %f", expectedRate, r.Activity.ErrorRate)
	}
	expectedAvg := float64(100+200+300) / 3
	if r.Activity.AvgDurationMs != expectedAvg {
		t.Errorf("expected avg duration %f, got %f", expectedAvg, r.Activity.AvgDurationMs)
	}
	if len(r.AIUsage) != 1 || r.AIUsage[0].Model != "claude" {
		t.Errorf("expected claude model usage, got %v", r.AIUsage)
	}
	if r.AIUsage[0].CallCount != 2 {
		t.Errorf("expected 2 claude calls, got %d", r.AIUsage[0].CallCount)
	}
	if len(r.Activity.TopMethods) != 3 {
		t.Errorf("expected 3 top methods, got %d", len(r.Activity.TopMethods))
	}
}

func TestGenerate_TopMethodsLimit(t *testing.T) {
	var entries []activitylog.Entry
	for i := range 15 {
		method := string(rune('a' + i))
		for range i + 1 {
			entries = append(entries, activitylog.Entry{
				Timestamp: time.Now(),
				Method:    method,
			})
		}
	}

	r := Generate(entries, nil, "30d")

	if len(r.Activity.TopMethods) != 10 {
		t.Errorf("expected 10 top methods, got %d", len(r.Activity.TopMethods))
	}
	// Most frequent method should be first.
	if r.Activity.TopMethods[0].Count < r.Activity.TopMethods[len(r.Activity.TopMethods)-1].Count {
		t.Error("top methods not sorted by count descending")
	}
}

func TestFormatMarkdown(t *testing.T) {
	r := &Report{
		GeneratedAt: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
		Period:      "7d",
		Tasks:       TaskSummary{Total: 2, ByState: map[string]int{"submitted": 1, "implementing": 1}},
		AIUsage:     []ModelUsage{{Model: "claude", CallCount: 5}},
		Activity: ActivitySummary{
			TotalCalls:    10,
			ErrorCount:    1,
			ErrorRate:     10.0,
			AvgDurationMs: 150,
			TopMethods:    []MethodCount{{Method: "start", Count: 5}},
		},
	}

	md := FormatMarkdown(r)

	if md == "" {
		t.Fatal("expected non-empty markdown")
	}
	if !strings.Contains(md, "# Compliance Report") {
		t.Error("missing report header")
	}
	if !strings.Contains(md, "Total: 2") {
		t.Error("missing task total")
	}
	if !strings.Contains(md, "claude") {
		t.Error("missing AI model")
	}
	if !strings.Contains(md, "10.0%") {
		t.Error("missing error rate")
	}
	if !strings.Contains(md, "start") {
		t.Error("missing top method")
	}
}

func TestFormatMarkdown_NoAIUsage(t *testing.T) {
	r := &Report{
		GeneratedAt: time.Now(),
		Period:      "7d",
		Tasks:       TaskSummary{Total: 0, ByState: make(map[string]int)},
		Activity:    ActivitySummary{},
	}

	md := FormatMarkdown(r)

	if !strings.Contains(md, "No AI model usage recorded.") {
		t.Error("expected no AI usage message")
	}
}
