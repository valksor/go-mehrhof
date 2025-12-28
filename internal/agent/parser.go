package agent

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// YAMLBlockParser parses YAML blocks from agent output
type YAMLBlockParser struct {
	fileBlockRe    *regexp.Regexp
	summaryBlockRe *regexp.Regexp
}

// NewYAMLBlockParser creates a new parser
func NewYAMLBlockParser() *YAMLBlockParser {
	return &YAMLBlockParser{
		fileBlockRe:    regexp.MustCompile("(?s)```yaml:file\\s*\\n(.+?)\\n```"),
		summaryBlockRe: regexp.MustCompile("(?s)```yaml:summary\\s*\\n(.+?)\\n```"),
	}
}

// ParseEvent parses a single line of JSON output from claude CLI
func (p *YAMLBlockParser) ParseEvent(line []byte) (Event, error) {
	event := Event{
		Timestamp: time.Now(),
		Data:      make(map[string]any),
		Raw:       line,
	}

	// Try to parse as JSON (claude --output-format stream-json)
	var jsonData map[string]any
	if err := json.Unmarshal(line, &jsonData); err == nil {
		event.Data = jsonData

		// Determine event type from JSON
		if typ, ok := jsonData["type"].(string); ok {
			switch typ {
			case "content_block_delta":
				event.Type = EventText
				// Extract text from delta
				if delta, ok := jsonData["delta"].(map[string]any); ok {
					if text, ok := delta["text"].(string); ok {
						event.Text = text
					}
				}
			case "tool_use":
				event.Type = EventToolUse
			case "tool_result":
				event.Type = EventToolResult
			case "message_stop":
				event.Type = EventComplete
			case "result":
				event.Type = EventComplete
				// Extract result text for easy access
				if result, ok := jsonData["result"].(string); ok {
					event.Text = result
					event.Data["text"] = result
				}
			case "assistant":
				// Extract text and tool calls from message.content[]
				p.parseAssistantMessage(&event, jsonData)
			case "error":
				event.Type = EventError
			default:
				event.Type = EventText
			}
		}

		// Check for usage stats
		if usage, ok := jsonData["usage"].(map[string]any); ok {
			event.Type = EventUsage
			event.Data = usage
		}

		return event, nil
	}

	// Plain text line
	event.Type = EventText
	event.Text = string(line)
	event.Data["text"] = string(line)

	return event, nil
}

// parseAssistantMessage extracts text and tool calls from assistant messages
func (p *YAMLBlockParser) parseAssistantMessage(event *Event, jsonData map[string]any) {
	event.Type = EventText

	msg, ok := jsonData["message"].(map[string]any)
	if !ok {
		return
	}

	content, ok := msg["content"].([]any)
	if !ok {
		return
	}

	var textBuilder strings.Builder
	var toolCalls []*ToolCall

	for _, c := range content {
		block, ok := c.(map[string]any)
		if !ok {
			continue
		}

		blockType, _ := block["type"].(string)
		switch blockType {
		case "text":
			if text, ok := block["text"].(string); ok {
				textBuilder.WriteString(text)
			}
		case "tool_use":
			tc := p.extractToolCall(block)
			if tc != nil {
				toolCalls = append(toolCalls, tc)
			}
		}
	}

	if textBuilder.Len() > 0 {
		event.Text = textBuilder.String()
		event.Data["text"] = event.Text
	}

	// Store tool calls in event data for later processing
	if len(toolCalls) > 0 {
		event.Type = EventToolUse
		event.ToolCall = toolCalls[0] // Primary tool call
		event.Data["tool_calls"] = toolCalls
	}
}

// extractToolCall extracts a standardized ToolCall from a Claude tool_use block
func (p *YAMLBlockParser) extractToolCall(block map[string]any) *ToolCall {
	name, _ := block["name"].(string)
	if name == "" {
		return nil
	}

	input, _ := block["input"].(map[string]any)
	tc := &ToolCall{
		Name:  name,
		Input: input,
	}

	// Generate human-readable description based on tool type
	tc.Description = p.describeToolCall(name, input)

	return tc
}

// extractQuestion extracts a Question from AskUserQuestion tool input
func (p *YAMLBlockParser) extractQuestion(input map[string]any) *Question {
	if input == nil {
		return nil
	}

	questions, ok := input["questions"].([]any)
	if !ok || len(questions) == 0 {
		return nil
	}

	firstQ, ok := questions[0].(map[string]any)
	if !ok {
		return nil
	}

	q := &Question{}
	q.Text, _ = firstQ["question"].(string)
	if q.Text == "" {
		return nil
	}

	// Extract options
	if opts, ok := firstQ["options"].([]any); ok {
		for _, opt := range opts {
			if optMap, ok := opt.(map[string]any); ok {
				label, _ := optMap["label"].(string)
				desc, _ := optMap["description"].(string)
				if label != "" {
					q.Options = append(q.Options, QuestionOption{
						Label:       label,
						Description: desc,
					})
				}
			}
		}
	}

	return q
}

// describeToolCall generates a human-readable description for a tool call
func (p *YAMLBlockParser) describeToolCall(name string, input map[string]any) string {
	switch name {
	case "Read":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Write":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Edit":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Glob":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "Grep":
		pattern, _ := input["pattern"].(string)
		path, _ := input["path"].(string)
		if path != "" {
			return pattern + " in " + path
		}
		return pattern
	case "Bash":
		if desc, ok := input["description"].(string); ok && desc != "" {
			return desc
		}
		if cmd, ok := input["command"].(string); ok {
			if len(cmd) > 60 {
				return cmd[:60] + "..."
			}
			return cmd
		}
	case "Task":
		subtype, _ := input["subagent_type"].(string)
		desc, _ := input["description"].(string)
		if subtype != "" {
			return "[" + subtype + "] " + desc
		}
		return desc
	case "AskUserQuestion":
		if questions, ok := input["questions"].([]any); ok && len(questions) > 0 {
			if q, ok := questions[0].(map[string]any); ok {
				if text, ok := q["question"].(string); ok {
					if len(text) > 60 {
						return text[:60] + "..."
					}
					return text
				}
			}
		}
		return "asking question"
	}
	return ""
}

// Parse aggregates events into a response
func (p *YAMLBlockParser) Parse(events []Event) (*Response, error) {
	response := &Response{
		Files:    make([]FileChange, 0),
		Messages: make([]string, 0),
	}

	// Collect all text content and check for questions
	var textBuilder strings.Builder
	for _, event := range events {
		// Check for AskUserQuestion tool call
		if event.ToolCall != nil && event.ToolCall.Name == "AskUserQuestion" {
			q := p.extractQuestion(event.ToolCall.Input)
			if q != nil {
				response.Question = q
			}
		}

		// Also check tool_calls array
		if toolCalls, ok := event.Data["tool_calls"].([]*ToolCall); ok {
			for _, tc := range toolCalls {
				if tc.Name == "AskUserQuestion" {
					q := p.extractQuestion(tc.Input)
					if q != nil {
						response.Question = q
					}
				}
			}
		}

		// Handle "result" event which contains the final text
		if result, ok := event.Data["result"].(string); ok {
			textBuilder.WriteString(result)
			continue
		}

		// Use pre-extracted text if available
		if event.Text != "" {
			textBuilder.WriteString(event.Text)
			continue
		}

		// Handle "assistant" message event with content array
		if msg, ok := event.Data["message"].(map[string]any); ok {
			if content, ok := msg["content"].([]any); ok {
				for _, c := range content {
					if block, ok := c.(map[string]any); ok {
						if text, ok := block["text"].(string); ok {
							textBuilder.WriteString(text)
						}
					}
				}
			}
		}

		switch event.Type {
		case EventText:
			if text, ok := event.Data["text"].(string); ok {
				textBuilder.WriteString(text)
			}
			// Handle claude stream-json delta format
			if delta, ok := event.Data["delta"].(map[string]any); ok {
				if text, ok := delta["text"].(string); ok {
					textBuilder.WriteString(text)
				}
			}
		case EventUsage:
			response.Usage = p.parseUsage(event.Data)
		}
	}

	fullText := textBuilder.String()

	// Extract file blocks
	fileMatches := p.fileBlockRe.FindAllStringSubmatch(fullText, -1)
	for _, match := range fileMatches {
		if len(match) > 1 {
			var fc FileChange
			if err := yaml.Unmarshal([]byte(match[1]), &fc); err == nil {
				response.Files = append(response.Files, fc)
			}
		}
	}

	// Extract summary
	summaryMatches := p.summaryBlockRe.FindAllStringSubmatch(fullText, -1)
	for _, match := range summaryMatches {
		if len(match) > 1 {
			var summary struct {
				Text string `yaml:"text"`
			}
			if err := yaml.Unmarshal([]byte(match[1]), &summary); err == nil {
				response.Summary = summary.Text
			}
		}
	}

	// Store non-yaml text as messages
	cleanText := p.fileBlockRe.ReplaceAllString(fullText, "")
	cleanText = p.summaryBlockRe.ReplaceAllString(cleanText, "")
	cleanText = strings.TrimSpace(cleanText)
	if cleanText != "" {
		response.Messages = append(response.Messages, cleanText)
	}

	return response, nil
}

func (p *YAMLBlockParser) parseUsage(data map[string]any) *UsageStats {
	stats := &UsageStats{}

	if v, ok := data["input_tokens"].(float64); ok {
		stats.InputTokens = int(v)
	}
	if v, ok := data["output_tokens"].(float64); ok {
		stats.OutputTokens = int(v)
	}
	if v, ok := data["cache_read_input_tokens"].(float64); ok {
		stats.CachedTokens = int(v)
	}

	return stats
}

// JSONLineParser parses JSON lines output
type JSONLineParser struct{}

// NewJSONLineParser creates a JSON line parser
func NewJSONLineParser() *JSONLineParser {
	return &JSONLineParser{}
}

// ParseEvent parses a JSON line
func (p *JSONLineParser) ParseEvent(line []byte) (Event, error) {
	event := Event{
		Timestamp: time.Now(),
		Data:      make(map[string]any),
		Raw:       line,
	}

	if err := json.Unmarshal(line, &event.Data); err != nil {
		event.Type = EventText
		event.Data["text"] = string(line)
		return event, nil
	}

	// Determine type from data
	if _, ok := event.Data["error"]; ok {
		event.Type = EventError
	} else if _, ok := event.Data["usage"]; ok {
		event.Type = EventUsage
	} else {
		event.Type = EventText
	}

	return event, nil
}

// Parse aggregates JSON events
func (p *JSONLineParser) Parse(events []Event) (*Response, error) {
	// Delegate to YAML parser for now - same logic
	yamlParser := NewYAMLBlockParser()
	return yamlParser.Parse(events)
}
