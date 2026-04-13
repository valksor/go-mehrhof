package commands

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/storage"
)

func TestComputeStats(t *testing.T) {
	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		tasks       []storage.ArchivedTask
		wantTotal   int
		wantByState map[string]int
		wantRate    float64
		wantAvgDur  bool
		wantRecent  int
	}{
		{
			name:        "empty tasks",
			tasks:       nil,
			wantTotal:   0,
			wantByState: map[string]int{},
			wantRate:    0,
			wantRecent:  0,
		},
		{
			name: "single finished task",
			tasks: []storage.ArchivedTask{
				{ID: "t1", Title: "Fix bug", FinalState: "finished", StartedAt: now.Add(-10 * time.Minute), CompletedAt: now},
			},
			wantTotal:   1,
			wantByState: map[string]int{"finished": 1},
			wantRate:    100,
			wantAvgDur:  true,
			wantRecent:  1,
		},
		{
			name: "mix of states",
			tasks: []storage.ArchivedTask{
				{ID: "t1", Title: "Task 1", FinalState: "finished", StartedAt: now.Add(-5 * time.Minute), CompletedAt: now},
				{ID: "t2", Title: "Task 2", FinalState: "finished", StartedAt: now.Add(-10 * time.Minute), CompletedAt: now},
				{ID: "t3", Title: "Task 3", FinalState: "abandoned", StartedAt: now.Add(-3 * time.Minute), CompletedAt: now},
				{ID: "t4", Title: "Task 4", FinalState: "failed", StartedAt: now.Add(-7 * time.Minute), CompletedAt: now},
			},
			wantTotal:   4,
			wantByState: map[string]int{"finished": 2, "abandoned": 1, "failed": 1},
			wantRate:    50,
			wantAvgDur:  true,
			wantRecent:  4,
		},
		{
			name: "tasks with durations",
			tasks: []storage.ArchivedTask{
				{ID: "t1", Title: "Task 1", FinalState: "submitted", StartedAt: now.Add(-20 * time.Minute), CompletedAt: now},
				{ID: "t2", Title: "Task 2", FinalState: "submitted", StartedAt: now.Add(-10 * time.Minute), CompletedAt: now},
			},
			wantTotal:   2,
			wantByState: map[string]int{"submitted": 2},
			wantRate:    100,
			wantAvgDur:  true,
			wantRecent:  2,
		},
		{
			name: "empty title uses ID",
			tasks: []storage.ArchivedTask{
				{ID: "task-abc-123", Title: "", FinalState: "finished", StartedAt: now.Add(-5 * time.Minute), CompletedAt: now},
			},
			wantTotal:  1,
			wantRate:   100,
			wantAvgDur: true,
			wantRecent: 1,
		},
		{
			name: "more than 5 tasks limits recent to 5",
			tasks: []storage.ArchivedTask{
				{ID: "t1", Title: "Task 1", FinalState: "finished", StartedAt: now.Add(-1 * time.Minute), CompletedAt: now},
				{ID: "t2", Title: "Task 2", FinalState: "finished", StartedAt: now.Add(-2 * time.Minute), CompletedAt: now},
				{ID: "t3", Title: "Task 3", FinalState: "finished", StartedAt: now.Add(-3 * time.Minute), CompletedAt: now},
				{ID: "t4", Title: "Task 4", FinalState: "finished", StartedAt: now.Add(-4 * time.Minute), CompletedAt: now},
				{ID: "t5", Title: "Task 5", FinalState: "finished", StartedAt: now.Add(-5 * time.Minute), CompletedAt: now},
				{ID: "t6", Title: "Task 6", FinalState: "finished", StartedAt: now.Add(-6 * time.Minute), CompletedAt: now},
				{ID: "t7", Title: "Task 7", FinalState: "abandoned", StartedAt: now.Add(-7 * time.Minute), CompletedAt: now},
			},
			wantTotal:  7,
			wantRate:   85.7,
			wantAvgDur: true,
			wantRecent: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeStats(tt.tasks)

			if got.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", got.Total, tt.wantTotal)
			}

			if tt.wantByState != nil {
				for state, count := range tt.wantByState {
					if got.ByState[state] != count {
						t.Errorf("ByState[%s] = %d, want %d", state, got.ByState[state], count)
					}
				}
			}

			if got.SuccessRate != tt.wantRate {
				t.Errorf("SuccessRate = %v, want %v", got.SuccessRate, tt.wantRate)
			}

			if tt.wantAvgDur && got.AvgDuration == "" {
				t.Error("AvgDuration should be populated")
			}
			if !tt.wantAvgDur && got.AvgDuration != "" {
				t.Errorf("AvgDuration = %s, want empty", got.AvgDuration)
			}

			if len(got.Recent) != tt.wantRecent {
				t.Errorf("Recent count = %d, want %d", len(got.Recent), tt.wantRecent)
			}
		})
	}
}

func TestComputeStats_EmptyTitleUsesID(t *testing.T) {
	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	tasks := []storage.ArchivedTask{
		{ID: "task-abc-123", Title: "", FinalState: "finished", StartedAt: now.Add(-5 * time.Minute), CompletedAt: now},
	}

	got := computeStats(tasks)

	if len(got.Recent) != 1 {
		t.Fatalf("Recent count = %d, want 1", len(got.Recent))
	}
	if got.Recent[0].Title != "task-abc-123" {
		t.Errorf("Recent[0].Title = %s, want task-abc-123", got.Recent[0].Title)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "30 seconds", d: 30 * time.Second, want: "30s"},
		{name: "90 seconds", d: 90 * time.Second, want: "1m30s"},
		{name: "65 minutes", d: 65 * time.Minute, want: "1h5m"},
		{name: "0 seconds", d: 0, want: "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.d, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		limit int
		want  string
	}{
		{name: "short string within limit", s: "hello", limit: 10, want: "hello"},
		{name: "string exactly at limit", s: "hello", limit: 5, want: "hello"},
		{name: "string exceeding limit", s: "hello world, this is a long string", limit: 15, want: "hello world,..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.limit)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.limit, got, tt.want)
			}
		})
	}
}

func TestPrintStats_Empty(t *testing.T) {
	// Reset the global JSON flag
	statsJSON = false
	if err := printStats(statsOutput{ByState: map[string]int{}}); err != nil {
		t.Errorf("printStats(empty) error = %v", err)
	}
}

func TestPrintStats_JSON(t *testing.T) {
	statsJSON = true
	defer func() { statsJSON = false }()
	out := statsOutput{
		Total:       5,
		ByState:     map[string]int{"finished": 3, "failed": 2},
		SuccessRate: 60.0,
	}
	if err := printStats(out); err != nil {
		t.Errorf("printStats(JSON) error = %v", err)
	}
}

func TestPrintStats_WithRecent(t *testing.T) {
	statsJSON = false
	out := statsOutput{
		Total:       2,
		ByState:     map[string]int{"finished": 2},
		SuccessRate: 100.0,
		AvgDuration: "5m30s",
		Recent: []recentTask{
			{Title: "Task 1", FinalState: "finished", CompletedAt: "2026-01-01 12:00", Duration: "3m"},
		},
	}
	if err := printStats(out); err != nil {
		t.Errorf("printStats(with recent) error = %v", err)
	}
}

func TestPrintStats_WithoutAvgDuration(t *testing.T) {
	statsJSON = false
	out := statsOutput{
		Total:       1,
		ByState:     map[string]int{"finished": 1},
		SuccessRate: 100.0,
	}
	if err := printStats(out); err != nil {
		t.Errorf("printStats(no avg duration) error = %v", err)
	}
}

func TestIsClosedConnErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "regular error", err: errors.New("something went wrong"), want: false},
		{
			name: "net.OpError with closed connection",
			err: &net.OpError{
				Op:  "read",
				Net: "unix",
				Err: errors.New("use of closed network connection"),
			},
			want: true,
		},
		{
			name: "net.OpError with different message",
			err: &net.OpError{
				Op:  "read",
				Net: "unix",
				Err: errors.New("connection refused"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isClosedConnErr(tt.err)
			if got != tt.want {
				t.Errorf("isClosedConnErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
