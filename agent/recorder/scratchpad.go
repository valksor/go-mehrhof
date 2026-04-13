package recorder

import (
	"encoding/json"
	"fmt"
	"time"
)

// Thought represents a single structured reasoning step from an agent session.
type Thought struct {
	ID         string    `json:"id"`
	Sequence   int       `json:"sequence"`
	Type       string    `json:"type"` // reasoning, tool_call, observation, decision
	Content    string    `json:"content"`
	ToolName   string    `json:"tool_name,omitempty"`
	TokensUsed int       `json:"tokens_used,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// Scratchpad is a structured index of agent reasoning for a single job.
type Scratchpad struct {
	JobID    string    `json:"job_id"`
	Phase    string    `json:"phase"`
	Thoughts []Thought `json:"thoughts"`
}

// BuildScratchpadFromFile reads a recording file and builds a scratchpad.
func BuildScratchpadFromFile(path string) (*Scratchpad, error) {
	records, err := ReadAll(path)
	if err != nil {
		return nil, fmt.Errorf("read recording: %w", err)
	}
	if len(records) == 0 {
		return &Scratchpad{}, nil
	}
	// Use the first record's job ID.
	jobID := records[0].JobID

	return BuildScratchpad(jobID, "", records), nil
}

// BuildScratchpad parses raw recording records into a structured scratchpad.
func BuildScratchpad(jobID, phase string, records []Record) *Scratchpad {
	pad := &Scratchpad{
		JobID: jobID,
		Phase: phase,
	}

	seq := 0
	for _, rec := range records {
		if rec.Direction != Outbound {
			continue
		}

		thought := recordToThought(rec, &seq)
		if thought != nil {
			pad.Thoughts = append(pad.Thoughts, *thought)
		}
	}

	return pad
}

func recordToThought(rec Record, seq *int) *Thought {
	*seq++
	base := Thought{
		ID:        fmt.Sprintf("thought-%d", *seq),
		Sequence:  *seq,
		Timestamp: rec.Timestamp,
	}

	switch rec.Type {
	case "assistant":
		base.Type = "reasoning"
		base.Content = extractContent(rec.Event)
		if base.Content == "" {
			*seq--

			return nil
		}
	case "tool_use":
		base.Type = "tool_call"
		base.ToolName = extractField(rec.Event, "name")
		base.Content = extractField(rec.Event, "input")
		if base.Content == "" {
			base.Content = base.ToolName
		}
	case "tool_result":
		base.Type = "observation"
		base.Content = extractContent(rec.Event)
		if base.Content == "" {
			*seq--

			return nil
		}
	case "complete":
		base.Type = "decision"
		base.Content = extractContent(rec.Event)
		if base.Content == "" {
			base.Content = "Agent completed"
		}
	default:
		*seq--

		return nil
	}

	return &base
}

func extractContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try as string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try as object with "content" or "message" field.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		if content, ok := obj["content"]; ok {
			var cs string
			if err := json.Unmarshal(content, &cs); err == nil {
				return cs
			}
		}
		if message, ok := obj["message"]; ok {
			var ms string
			if err := json.Unmarshal(message, &ms); err == nil {
				return ms
			}
		}
	}

	// Truncate raw JSON as fallback (rune-safe to avoid splitting UTF-8).
	sr := string(raw)
	runes := []rune(sr)
	if len(runes) > 200 {
		return string(runes[:200]) + "..."
	}

	return sr
}

func extractField(raw json.RawMessage, field string) string {
	if len(raw) == 0 {
		return ""
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}

	val, ok := obj[field]
	if !ok {
		return ""
	}

	var s string
	if err := json.Unmarshal(val, &s); err == nil {
		return s
	}

	// Non-string field: return raw JSON (rune-safe to avoid splitting UTF-8).
	sv := string(val)
	runes := []rune(sv)
	if len(runes) > 200 {
		return string(runes[:200]) + "..."
	}

	return sv
}
