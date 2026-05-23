package socket

import (
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/conductor"
)

func TestSuggestNextAction(t *testing.T) {
	cases := []struct {
		state    conductor.State
		wu       *conductor.WorkUnit
		contains string
	}{
		{conductor.StateNone, nil, "kvelmo start"},
		{conductor.StateLoaded, nil, "kvelmo plan"},
		{conductor.StatePlanning, nil, "Planning in progress"},
		{conductor.StatePlanned, nil, "kvelmo implement"},
		{conductor.StateImplementing, nil, "Implementation in progress"},
		{conductor.StateImplemented, nil, "kvelmo review"},
		{conductor.StateSimplifying, nil, "Simplification in progress"},
		{conductor.StateOptimizing, nil, "Optimization in progress"},
		{conductor.StateReviewing, nil, "Review in progress"},
		{conductor.StateSubmitted, nil, "PR submitted"},
		{conductor.StateSubmitted, &conductor.WorkUnit{PRID: "PR-7"}, "PR-7"},
		{conductor.StateFailed, nil, "kvelmo retry"},
		{conductor.StateWaiting, nil, "waiting for input"},
		{conductor.StatePaused, nil, "paused"},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			got := suggestNextAction(tc.state, tc.wu)
			if !strings.Contains(got, tc.contains) {
				t.Errorf("suggestNextAction(%s) = %q, want substring %q", tc.state, got, tc.contains)
			}
		})
	}
}

func TestFormatTimeSince(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		t    time.Time
		want string
	}{
		{"future is just now", now.Add(time.Hour), "just now"},
		{"seconds is just now", now.Add(-30 * time.Second), "just now"},
		{"one minute", now.Add(-1 * time.Minute), "1 minute ago"},
		{"several minutes", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"one hour", now.Add(-1 * time.Hour), "1 hour ago"},
		{"several hours", now.Add(-3 * time.Hour), "3 hours ago"},
		{"one day", now.Add(-25 * time.Hour), "1 day ago"},
		{"several days", now.Add(-72 * time.Hour), "3 days ago"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatTimeSince(tc.t); got != tc.want {
				t.Errorf("formatTimeSince() = %q, want %q", got, tc.want)
			}
		})
	}
}
