package conductor

import (
	"fmt"
	"strings"
)

// KeyDecision represents a significant action taken by an AI agent
// during task implementation.
type KeyDecision struct {
	Tool      string `json:"tool"`
	Action    string `json:"action"`
	File      string `json:"file"`
	Reasoning string `json:"reasoning"`
}

// ExtractDecisions scans recorder records (each is a map from JSONL)
// and extracts tool_use events with their file targets and descriptions.
// Returns up to 20 decisions in encounter order.
func ExtractDecisions(records []map[string]any) []KeyDecision {
	var decisions []KeyDecision

	for _, record := range records {
		recType, ok := record["type"].(string)
		if !ok || recType != "tool_use" {
			continue
		}

		decision := KeyDecision{}

		if tool, ok := record["tool"].(string); ok {
			decision.Tool = tool
		}

		if action, ok := record["action"].(string); ok {
			decision.Action = action
		}

		if file, ok := record["file"].(string); ok {
			decision.File = file
		}

		if reasoning, ok := record["reasoning"].(string); ok {
			decision.Reasoning = reasoning
		}

		// Skip entries with no meaningful tool info.
		if decision.Tool == "" && decision.Action == "" {
			continue
		}

		decisions = append(decisions, decision)

		if len(decisions) >= 20 {
			break
		}
	}

	return decisions
}

// FormatDecisionsMarkdown formats decisions as a collapsible markdown section
// suitable for PR descriptions.
func FormatDecisionsMarkdown(decisions []KeyDecision, diffStat string) string {
	var b strings.Builder

	b.WriteString("<details>\n")
	b.WriteString("<summary>AI Agent Decisions</summary>\n\n")

	if len(decisions) == 0 {
		b.WriteString("No key decisions recorded.\n")
	} else {
		for i, d := range decisions {
			if d.Tool != "" {
				fmt.Fprintf(&b, "%d. **%s**", i+1, d.Tool)
				if d.Action != "" {
					fmt.Fprintf(&b, " - %s", d.Action)
				}
			} else {
				fmt.Fprintf(&b, "%d. **%s**", i+1, d.Action)
			}

			b.WriteString("\n")

			if d.File != "" {
				fmt.Fprintf(&b, "   - File: `%s`\n", d.File)
			}

			if d.Reasoning != "" {
				fmt.Fprintf(&b, "   - Reasoning: %s\n", d.Reasoning)
			}
		}
	}

	if diffStat != "" {
		b.WriteString("\n---\n\n")
		b.WriteString("**Diff stat:**\n```\n")
		b.WriteString(diffStat)
		b.WriteString("\n```\n")
	}

	b.WriteString("\n</details>\n")

	return b.String()
}
