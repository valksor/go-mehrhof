package conductor

import (
	"testing"
)

func TestDefaultYoloOptions(t *testing.T) {
	opts := DefaultYoloOptions()

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

func TestYoloOptionsStruct(t *testing.T) {
	opts := YoloOptions{
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

func TestYoloResultStruct(t *testing.T) {
	result := YoloResult{
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

func TestYoloResultStruct_Failed(t *testing.T) {
	result := YoloResult{
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

func TestWithYoloMode(t *testing.T) {
	opts := DefaultOptions()
	WithYoloMode(true)(&opts)

	if opts.YoloMode != true {
		t.Errorf("YoloMode = %v, want true", opts.YoloMode)
	}
	// YoloMode should auto-enable SkipAgentQuestions
	if opts.SkipAgentQuestions != true {
		t.Errorf("SkipAgentQuestions = %v, want true (auto-set by YoloMode)", opts.SkipAgentQuestions)
	}
}

func TestWithYoloMode_Disabled(t *testing.T) {
	opts := DefaultOptions()
	// First enable
	WithYoloMode(true)(&opts)
	// Then disable
	WithYoloMode(false)(&opts)

	if opts.YoloMode != false {
		t.Errorf("YoloMode = %v, want false", opts.YoloMode)
	}
	// SkipAgentQuestions should remain true (was set when YoloMode was true)
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

func TestDefaultOptions_IncludesYoloDefaults(t *testing.T) {
	opts := DefaultOptions()

	// Verify yolo-related defaults
	if opts.YoloMode != false {
		t.Errorf("YoloMode = %v, want false", opts.YoloMode)
	}
	if opts.SkipAgentQuestions != false {
		t.Errorf("SkipAgentQuestions = %v, want false", opts.SkipAgentQuestions)
	}
	if opts.MaxQualityRetries != 3 {
		t.Errorf("MaxQualityRetries = %d, want 3", opts.MaxQualityRetries)
	}
}

func TestOptionsApply_YoloOptions(t *testing.T) {
	opts := DefaultOptions()
	opts.Apply(
		WithYoloMode(true),
		WithMaxQualityRetries(10),
	)

	if opts.YoloMode != true {
		t.Errorf("YoloMode = %v, want true", opts.YoloMode)
	}
	if opts.SkipAgentQuestions != true {
		t.Errorf("SkipAgentQuestions = %v, want true", opts.SkipAgentQuestions)
	}
	if opts.MaxQualityRetries != 10 {
		t.Errorf("MaxQualityRetries = %d, want 10", opts.MaxQualityRetries)
	}
}
