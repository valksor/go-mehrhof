package conductor

import (
	"testing"
)

func TestDefaultAutoOptions(t *testing.T) {
	opts := DefaultAutoOptions()

	if opts.QualityTarget != "quality" {
		t.Errorf("QualityTarget = %q, want %q", opts.QualityTarget, "quality")
	}
	if opts.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", opts.MaxRetries)
	}
	if opts.SquashMerge != true {
		t.Errorf("SquashMerge = %v, want true", opts.SquashMerge)
	}
	if opts.DeleteBranch != true {
		t.Errorf("DeleteBranch = %v, want true", opts.DeleteBranch)
	}
	if opts.TargetBranch != "" {
		t.Errorf("TargetBranch = %q, want empty (auto-detect)", opts.TargetBranch)
	}
	if opts.Push != false {
		t.Errorf("Push = %v, want false", opts.Push)
	}
}

func TestAutoOptionsStruct(t *testing.T) {
	opts := AutoOptions{
		QualityTarget: "lint",
		MaxRetries:    5,
		SquashMerge:   false,
		DeleteBranch:  false,
		TargetBranch:  "main",
		Push:          true,
	}

	if opts.QualityTarget != "lint" {
		t.Errorf("QualityTarget = %q, want %q", opts.QualityTarget, "lint")
	}
	if opts.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", opts.MaxRetries)
	}
	if opts.SquashMerge != false {
		t.Errorf("SquashMerge = %v, want false", opts.SquashMerge)
	}
	if opts.DeleteBranch != false {
		t.Errorf("DeleteBranch = %v, want false", opts.DeleteBranch)
	}
	if opts.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want %q", opts.TargetBranch, "main")
	}
	if opts.Push != true {
		t.Errorf("Push = %v, want true", opts.Push)
	}
}

func TestAutoResultStruct(t *testing.T) {
	result := AutoResult{
		PlanningDone:    true,
		ImplementDone:   true,
		QualityAttempts: 2,
		QualityPassed:   true,
		FinishDone:      true,
		FailedAt:        "",
	}

	if result.PlanningDone != true {
		t.Errorf("PlanningDone = %v, want true", result.PlanningDone)
	}
	if result.ImplementDone != true {
		t.Errorf("ImplementDone = %v, want true", result.ImplementDone)
	}
	if result.QualityAttempts != 2 {
		t.Errorf("QualityAttempts = %d, want 2", result.QualityAttempts)
	}
	if result.QualityPassed != true {
		t.Errorf("QualityPassed = %v, want true", result.QualityPassed)
	}
	if result.FinishDone != true {
		t.Errorf("FinishDone = %v, want true", result.FinishDone)
	}
	if result.FailedAt != "" {
		t.Errorf("FailedAt = %q, want empty", result.FailedAt)
	}
}

func TestAutoResultStruct_Failed(t *testing.T) {
	result := AutoResult{
		PlanningDone:    true,
		ImplementDone:   true,
		QualityAttempts: 3,
		QualityPassed:   false,
		FinishDone:      false,
		FailedAt:        "quality",
	}

	if result.QualityPassed != false {
		t.Errorf("QualityPassed = %v, want false", result.QualityPassed)
	}
	if result.FinishDone != false {
		t.Errorf("FinishDone = %v, want false", result.FinishDone)
	}
	if result.FailedAt != "quality" {
		t.Errorf("FailedAt = %q, want %q", result.FailedAt, "quality")
	}
}

func TestWithAutoMode(t *testing.T) {
	opts := DefaultOptions()
	WithAutoMode(true)(&opts)

	if opts.AutoMode != true {
		t.Errorf("AutoMode = %v, want true", opts.AutoMode)
	}
	// AutoMode should auto-enable SkipAgentQuestions
	if opts.SkipAgentQuestions != true {
		t.Errorf("SkipAgentQuestions = %v, want true (auto-set by AutoMode)", opts.SkipAgentQuestions)
	}
}

func TestWithAutoMode_Disabled(t *testing.T) {
	opts := DefaultOptions()
	// First enable
	WithAutoMode(true)(&opts)
	// Then disable
	WithAutoMode(false)(&opts)

	if opts.AutoMode != false {
		t.Errorf("AutoMode = %v, want false", opts.AutoMode)
	}
	// SkipAgentQuestions should remain true (was set when AutoMode was true)
	if opts.SkipAgentQuestions != true {
		t.Errorf("SkipAgentQuestions = %v, should remain true from previous setting", opts.SkipAgentQuestions)
	}
}

func TestWithSkipAgentQuestions(t *testing.T) {
	opts := DefaultOptions()
	WithSkipAgentQuestions(true)(&opts)

	if opts.SkipAgentQuestions != true {
		t.Errorf("SkipAgentQuestions = %v, want true", opts.SkipAgentQuestions)
	}
}

func TestWithSkipAgentQuestions_False(t *testing.T) {
	opts := DefaultOptions()
	// First enable
	WithSkipAgentQuestions(true)(&opts)
	// Then disable
	WithSkipAgentQuestions(false)(&opts)

	if opts.SkipAgentQuestions != false {
		t.Errorf("SkipAgentQuestions = %v, want false", opts.SkipAgentQuestions)
	}
}

func TestWithMaxQualityRetries(t *testing.T) {
	opts := DefaultOptions()
	WithMaxQualityRetries(5)(&opts)

	if opts.MaxQualityRetries != 5 {
		t.Errorf("MaxQualityRetries = %d, want 5", opts.MaxQualityRetries)
	}
}

func TestWithMaxQualityRetries_Zero(t *testing.T) {
	opts := DefaultOptions()
	WithMaxQualityRetries(0)(&opts)

	if opts.MaxQualityRetries != 0 {
		t.Errorf("MaxQualityRetries = %d, want 0 (skip quality)", opts.MaxQualityRetries)
	}
}

func TestDefaultOptions_IncludesAutoDefaults(t *testing.T) {
	opts := DefaultOptions()

	// Verify auto-related defaults
	if opts.AutoMode != false {
		t.Errorf("AutoMode = %v, want false", opts.AutoMode)
	}
	if opts.SkipAgentQuestions != false {
		t.Errorf("SkipAgentQuestions = %v, want false", opts.SkipAgentQuestions)
	}
	if opts.MaxQualityRetries != 3 {
		t.Errorf("MaxQualityRetries = %d, want 3", opts.MaxQualityRetries)
	}
}

func TestOptionsApply_AutoOptions(t *testing.T) {
	opts := DefaultOptions()
	opts.Apply(
		WithAutoMode(true),
		WithMaxQualityRetries(10),
	)

	if opts.AutoMode != true {
		t.Errorf("AutoMode = %v, want true", opts.AutoMode)
	}
	if opts.SkipAgentQuestions != true {
		t.Errorf("SkipAgentQuestions = %v, want true", opts.SkipAgentQuestions)
	}
	if opts.MaxQualityRetries != 10 {
		t.Errorf("MaxQualityRetries = %d, want 10", opts.MaxQualityRetries)
	}
}
