package strategy

import "strings"

// Direct is the default strategy — passes prompts through with minimal wrapping.
// This matches kvelmo's current behavior.
type Direct struct{}

func (d *Direct) Name() string { return "direct" }

func (d *Direct) BuildPrompt(input Input) string {
	var sb strings.Builder

	if input.Context != "" {
		sb.WriteString("## Context\n\n")
		sb.WriteString(input.Context)
		sb.WriteString("\n\n")
	}

	sb.WriteString(input.Task)

	if len(input.Constraints) > 0 {
		sb.WriteString("\n\n## Constraints\n\n")
		for _, c := range input.Constraints {
			sb.WriteString("- ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (d *Direct) EvaluateOutput(output string) Output {
	return Output{
		Content:  output,
		Metadata: make(map[string]string),
		Status:   statusComplete,
	}
}
