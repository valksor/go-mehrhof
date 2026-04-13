package strategy

import (
	"strings"
)

// Iterative builds prompts that instruct the agent to implement, self-review,
// and fix issues in a single pass. Output is checked for unresolved markers.
type Iterative struct{}

func (it *Iterative) Name() string { return "iterative" }

func (it *Iterative) BuildPrompt(input Input) string {
	var sb strings.Builder

	if input.Context != "" {
		sb.WriteString("## Previous Attempt\n\n")
		sb.WriteString("The following is output from a prior iteration. Review it, identify issues, and improve.\n\n")
		sb.WriteString(input.Context)
		sb.WriteString("\n\n---\n\n")
	}

	sb.WriteString(input.Task)

	sb.WriteString("\n\n## Self-Review Process\n\n")
	sb.WriteString("After completing the implementation:\n")
	sb.WriteString("1. Review your own work for correctness, edge cases, and style\n")
	sb.WriteString("2. Identify any issues or improvements\n")
	sb.WriteString("3. Fix any issues you find\n")
	sb.WriteString("4. If something cannot be resolved, mark it with TODO or FIXME\n")

	if len(input.Constraints) > 0 {
		sb.WriteString("\n## Constraints\n\n")
		for _, c := range input.Constraints {
			sb.WriteString("- ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// unresolvedMarkers are strings that indicate the output needs another iteration.
var unresolvedMarkers = []string{"TODO", "FIXME", "HACK", "NEEDS_REVIEW", "XXX"}

func (it *Iterative) EvaluateOutput(output string) Output {
	upper := strings.ToUpper(output)

	for _, marker := range unresolvedMarkers {
		if strings.Contains(upper, marker) {
			return Output{
				Content:  output,
				Metadata: map[string]string{"marker": marker},
				Status:   "needs_iteration",
			}
		}
	}

	return Output{
		Content:  output,
		Metadata: make(map[string]string),
		Status:   "complete",
	}
}
