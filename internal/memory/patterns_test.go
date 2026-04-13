package memory

import (
	"testing"

	"github.com/valksor/kvelmo/internal/storage"
)

func TestDetectSkipPatterns_InsufficientData(t *testing.T) {
	tasks := []storage.ArchivedTask{
		{FinalState: "finished", PhasesRun: []string{"plan", "implement"}},
	}

	suggestions := DetectSkipPatterns(tasks)
	if len(suggestions) != 0 {
		t.Errorf("expected no suggestions with < %d tasks, got %d", MinSampleSize, len(suggestions))
	}
}

func TestDetectSkipPatterns_SimplifyAlwaysSkipped(t *testing.T) {
	// 6 tasks, none ran simplify
	tasks := make([]storage.ArchivedTask, 0, 6)
	for range 6 {
		tasks = append(tasks, storage.ArchivedTask{
			FinalState: "finished",
			PhasesRun:  []string{"plan", "implement", "review", "submit"},
		})
	}

	suggestions := DetectSkipPatterns(tasks)

	// Should suggest skipping simplify and optimize (both absent from all tasks)
	found := make(map[string]bool)
	for _, s := range suggestions {
		found[s.Phase] = true
		if s.SampleSize != 6 {
			t.Errorf("suggestion for %s: sample_size = %d, want 6", s.Phase, s.SampleSize)
		}
		if s.Confidence < 0.8 {
			t.Errorf("suggestion for %s: confidence = %f, want >= 0.8", s.Phase, s.Confidence)
		}
	}

	if !found["simplify"] {
		t.Error("expected suggestion to skip simplify")
	}
	if !found["optimize"] {
		t.Error("expected suggestion to skip optimize")
	}
}

func TestDetectSkipPatterns_NoSuggestionWhenPhaseUsed(t *testing.T) {
	tasks := make([]storage.ArchivedTask, 0, 6)
	for range 6 {
		tasks = append(tasks, storage.ArchivedTask{
			FinalState: "finished",
			PhasesRun:  []string{"plan", "implement", "simplify", "optimize", "review", "submit"},
		})
	}

	suggestions := DetectSkipPatterns(tasks)
	if len(suggestions) != 0 {
		t.Errorf("expected no suggestions when all phases used, got %d", len(suggestions))
	}
}

func TestDetectSkipPatterns_IgnoresOldFormatTasks(t *testing.T) {
	tasks := make([]storage.ArchivedTask, 0, 6)
	// 3 tasks with phase data (not enough)
	for range 3 {
		tasks = append(tasks, storage.ArchivedTask{
			FinalState: "finished",
			PhasesRun:  []string{"plan", "implement"},
		})
	}
	// 3 tasks without phase data (old format)
	for range 3 {
		tasks = append(tasks, storage.ArchivedTask{
			FinalState: "finished",
		})
	}

	suggestions := DetectSkipPatterns(tasks)
	// Only 3 tasks with data < MinSampleSize, should return nothing
	if len(suggestions) != 0 {
		t.Errorf("expected no suggestions with only 3 valid tasks, got %d", len(suggestions))
	}
}

func TestDetectAgentPatterns_DominantAgent(t *testing.T) {
	tasks := make([]storage.ArchivedTask, 0, 6)
	for range 5 {
		tasks = append(tasks, storage.ArchivedTask{
			FinalState: "finished",
			AgentUsed:  "anthropic",
		})
	}
	tasks = append(tasks, storage.ArchivedTask{
		FinalState: "finished",
		AgentUsed:  "openai",
	})

	suggestions := DetectAgentPatterns(tasks)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 agent suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Agent != "anthropic" {
		t.Errorf("expected anthropic, got %s", suggestions[0].Agent)
	}
}

func TestDetectAgentPatterns_NoDominant(t *testing.T) {
	tasks := make([]storage.ArchivedTask, 0, 6)
	for range 3 {
		tasks = append(tasks, storage.ArchivedTask{FinalState: "finished", AgentUsed: "anthropic"})
	}
	for range 3 {
		tasks = append(tasks, storage.ArchivedTask{FinalState: "finished", AgentUsed: "openai"})
	}

	suggestions := DetectAgentPatterns(tasks)
	if len(suggestions) != 0 {
		t.Errorf("expected no suggestions when agents split 50/50, got %d", len(suggestions))
	}
}
