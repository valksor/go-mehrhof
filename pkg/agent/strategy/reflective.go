package strategy

import (
	"context"
	"fmt"
	"strings"
)

// Reflective implements a multi-pass reasoning strategy:
// 1. Initial execution with the original prompt
// 2. Reflection pass asking the agent to review its work
// 3. Optional correction pass if the reflection identifies issues.
type Reflective struct{}

// NOTE: Reflective is not auto-registered because RunLoop integration with the
// conductor is not yet wired. Calling applyStrategy with "reflective" will fall
// back to the default (direct) strategy until conductor-side Runner dispatch is
// implemented. Register manually after that:
//
//	strategy.Register(&Reflective{})

func (r *Reflective) Name() string { return "reflective" }

func (r *Reflective) BuildPrompt(input Input) string {
	// For non-Runner callers, just pass through like direct.
	var sb strings.Builder

	if input.Context != "" {
		sb.WriteString("## Context\n\n")
		sb.WriteString(input.Context)
		sb.WriteString("\n\n")
	}

	sb.WriteString(input.Task)

	return sb.String()
}

func (r *Reflective) EvaluateOutput(output string) Output {
	return Output{
		Content:  output,
		Metadata: make(map[string]string),
		Status:   "complete",
	}
}

// RunLoop implements the Runner interface with a 3-pass execution model.
func (r *Reflective) RunLoop(ctx context.Context, exec ExecFunc, input Input, emit func(Event)) (Output, error) {
	var allOutput strings.Builder

	// Pass 1: Initial execution
	emit(Event{Type: "pass_started", Message: "Initial execution", Pass: 1})

	prompt := r.BuildPrompt(input)

	result1, err := exec(ctx, prompt)
	if err != nil {
		return Output{Content: err.Error(), Status: "error"}, err
	}

	allOutput.WriteString(result1)
	emit(Event{Type: "pass_completed", Message: "Initial pass complete", Pass: 1})

	// Pass 2: Reflection
	emit(Event{Type: "pass_started", Message: "Reflection", Pass: 2})

	reflectionPrompt := fmt.Sprintf(
		"Review the work you just completed. What did you miss? Are there any bugs, "+
			"edge cases, or improvements?\n\nYour previous output:\n%s",
		truncateForReflection(result1),
	)

	result2, reflectErr := exec(ctx, reflectionPrompt)
	if reflectErr != nil {
		// Reflection failure is non-fatal — return initial result without error.
		emit(Event{Type: "pass_completed", Message: "Reflection failed, using initial result", Pass: 2})

		return Output{Content: allOutput.String(), Status: "complete"}, nil //nolint:nilerr // intentionally non-fatal
	}

	emit(Event{Type: "reflection", Message: result2, Pass: 2})
	emit(Event{Type: "pass_completed", Message: "Reflection complete", Pass: 2})

	// Check if reflection found actionable items.
	if !hasActionableItems(result2) {
		return Output{Content: allOutput.String(), Status: "complete"}, nil
	}

	// Pass 3: Corrections
	emit(Event{Type: "pass_started", Message: "Applying corrections", Pass: 3})

	correctionPrompt := "Based on your review, apply the following corrections:\n\n" + result2

	result3, correctErr := exec(ctx, correctionPrompt)
	if correctErr != nil {
		// Correction failure is non-fatal — return what we have.
		emit(Event{Type: "pass_completed", Message: "Corrections failed, using reflection result", Pass: 3})

		return Output{Content: allOutput.String(), Status: "complete"}, nil //nolint:nilerr // intentionally non-fatal
	}

	allOutput.WriteString("\n\n")
	allOutput.WriteString(result3)
	emit(Event{Type: "pass_completed", Message: "Corrections applied", Pass: 3})

	return Output{Content: allOutput.String(), Status: "complete"}, nil
}

// truncateForReflection limits output length for the reflection prompt.
func truncateForReflection(s string) string {
	const maxLen = 4000
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen] + "\n\n[truncated for review]"
}

// hasActionableItems checks if reflection output suggests changes are needed.
// Action phrases take priority over approval phrases, so "looks good, but missing X"
// is treated as actionable.
func hasActionableItems(reflection string) bool {
	lower := strings.ToLower(reflection)

	// Check for action words first — presence of issues overrides generic approval.
	actionPhrases := []string{
		"should", "could", "fix", "bug", "missing", "incorrect",
		"improve", "change", "update", "error", "issue",
	}

	for _, phrase := range actionPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}

	// If the reflection says everything is fine and no action words, skip corrections.
	noActionPhrases := []string{
		"looks good", "no issues", "nothing to fix", "no changes needed",
		"everything is correct", "no bugs found", "all good",
	}

	for _, phrase := range noActionPhrases {
		if strings.Contains(lower, phrase) {
			return false
		}
	}

	return false
}
