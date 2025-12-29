package agent

import (
	"testing"
)

func TestNewYAMLBlockParser(t *testing.T) {
	p := NewYAMLBlockParser()
	if p == nil {
		t.Fatal("NewYAMLBlockParser returned nil")
	}
	if p.fileBlockRe == nil {
		t.Error("fileBlockRe should not be nil")
	}
	if p.summaryBlockRe == nil {
		t.Error("summaryBlockRe should not be nil")
	}
}

func TestParseEvent_JSON(t *testing.T) {
	p := NewYAMLBlockParser()

	tests := []struct {
		checkData func(t *testing.T, event Event)
		name      string
		input     string
		wantType  EventType
		wantText  string
	}{
		{
			name:     "content_block_delta",
			input:    `{"type":"content_block_delta","delta":{"text":"Hello"}}`,
			wantType: EventText,
			wantText: "Hello",
		},
		{
			name:     "tool_use",
			input:    `{"type":"tool_use","name":"Read"}`,
			wantType: EventToolUse,
		},
		{
			name:     "tool_result",
			input:    `{"type":"tool_result","result":"success"}`,
			wantType: EventToolResult,
		},
		{
			name:     "message_stop",
			input:    `{"type":"message_stop"}`,
			wantType: EventComplete,
		},
		{
			name:     "result",
			input:    `{"type":"result","result":"Final output"}`,
			wantType: EventComplete,
			wantText: "Final output",
		},
		{
			name:     "error",
			input:    `{"type":"error","error":"Something went wrong"}`,
			wantType: EventError,
		},
		{
			name:     "usage stats",
			input:    `{"usage":{"input_tokens":100,"output_tokens":50}}`,
			wantType: EventUsage,
			checkData: func(t *testing.T, event Event) {
				if event.Data["input_tokens"] != float64(100) {
					t.Errorf("input_tokens = %v, want 100", event.Data["input_tokens"])
				}
			},
		},
		{
			name:     "unknown type defaults to text",
			input:    `{"type":"unknown_event","data":"test"}`,
			wantType: EventText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := p.ParseEvent([]byte(tt.input))
			if err != nil {
				t.Fatalf("ParseEvent: %v", err)
			}
			if event.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", event.Type, tt.wantType)
			}
			if tt.wantText != "" && event.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", event.Text, tt.wantText)
			}
			if tt.checkData != nil {
				tt.checkData(t, event)
			}
		})
	}
}

func TestParseEvent_PlainText(t *testing.T) {
	p := NewYAMLBlockParser()

	// Non-JSON input should be treated as plain text
	event, err := p.ParseEvent([]byte("This is plain text output"))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if event.Type != EventText {
		t.Errorf("Type = %q, want %q", event.Type, EventText)
	}
	if event.Text != "This is plain text output" {
		t.Errorf("Text = %q, want %q", event.Text, "This is plain text output")
	}
}

func TestParseEvent_AssistantMessage(t *testing.T) {
	p := NewYAMLBlockParser()

	// Assistant message with text content
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}]}}`
	event, err := p.ParseEvent([]byte(input))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if event.Text != "Hello world" {
		t.Errorf("Text = %q, want %q", event.Text, "Hello world")
	}
}

func TestParseEvent_AssistantMessageWithToolUse(t *testing.T) {
	p := NewYAMLBlockParser()

	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"test.go"}}]}}`
	event, err := p.ParseEvent([]byte(input))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if event.Type != EventToolUse {
		t.Errorf("Type = %q, want %q", event.Type, EventToolUse)
	}
	if event.ToolCall == nil {
		t.Fatal("ToolCall should not be nil")
	}
	if event.ToolCall.Name != "Read" {
		t.Errorf("ToolCall.Name = %q, want %q", event.ToolCall.Name, "Read")
	}
}

func TestParseEvent_AssistantMessageMixed(t *testing.T) {
	p := NewYAMLBlockParser()

	// Mixed text and tool_use
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Let me read the file"},{"type":"tool_use","name":"Read","input":{"file_path":"test.go"}}]}}`
	event, err := p.ParseEvent([]byte(input))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if event.Text != "Let me read the file" {
		t.Errorf("Text = %q, want %q", event.Text, "Let me read the file")
	}
	if event.ToolCall == nil {
		t.Fatal("ToolCall should not be nil")
	}
}

func TestDescribeToolCall(t *testing.T) {
	p := NewYAMLBlockParser()

	tests := []struct {
		name  string
		tool  string
		input map[string]any
		want  string
	}{
		{"Read", "Read", map[string]any{"file_path": "test.go"}, "test.go"},
		{"Write", "Write", map[string]any{"file_path": "out.txt"}, "out.txt"},
		{"Edit", "Edit", map[string]any{"file_path": "edit.go"}, "edit.go"},
		{"Glob", "Glob", map[string]any{"pattern": "*.go"}, "*.go"},
		{"Grep with path", "Grep", map[string]any{"pattern": "TODO", "path": "src/"}, "TODO in src/"},
		{"Grep without path", "Grep", map[string]any{"pattern": "TODO"}, "TODO"},
		{"Bash with description", "Bash", map[string]any{"description": "Run tests", "command": "go test"}, "Run tests"},
		{"Bash without description", "Bash", map[string]any{"command": "go test ./..."}, "go test ./..."},
		{"Bash long command", "Bash", map[string]any{"command": "this is a very long command that should be truncated after sixty characters to prevent display issues"}, "this is a very long command that should be truncated after s..."},
		{"Task with subtype", "Task", map[string]any{"subagent_type": "explorer", "description": "Find files"}, "[explorer] Find files"},
		{"Task without subtype", "Task", map[string]any{"description": "Do something"}, "Do something"},
		{"AskUserQuestion", "AskUserQuestion", map[string]any{"questions": []any{map[string]any{"question": "What should I do?"}}}, "What should I do?"},
		{"AskUserQuestion long", "AskUserQuestion", map[string]any{"questions": []any{map[string]any{"question": "This is a very long question that needs to be truncated for proper display in the UI"}}}, "This is a very long question that needs to be truncated for ..."},
		{"Unknown tool", "Unknown", map[string]any{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.describeToolCall(tt.tool, tt.input)
			if got != tt.want {
				t.Errorf("describeToolCall(%q, %v) = %q, want %q", tt.tool, tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractQuestion(t *testing.T) {
	p := NewYAMLBlockParser()

	tests := []struct {
		input    map[string]any
		name     string
		wantText string
		wantOpts int
		wantNil  bool
	}{
		{
			name:    "nil input",
			input:   nil,
			wantNil: true,
		},
		{
			name:    "no questions",
			input:   map[string]any{},
			wantNil: true,
		},
		{
			name:    "empty questions array",
			input:   map[string]any{"questions": []any{}},
			wantNil: true,
		},
		{
			name: "valid question with options",
			input: map[string]any{
				"questions": []any{
					map[string]any{
						"question": "Which option?",
						"options": []any{
							map[string]any{"label": "A", "description": "Option A"},
							map[string]any{"label": "B", "description": "Option B"},
						},
					},
				},
			},
			wantNil:  false,
			wantText: "Which option?",
			wantOpts: 2,
		},
		{
			name: "question without options",
			input: map[string]any{
				"questions": []any{
					map[string]any{"question": "Simple question?"},
				},
			},
			wantNil:  false,
			wantText: "Simple question?",
			wantOpts: 0,
		},
		{
			name: "question with empty text",
			input: map[string]any{
				"questions": []any{
					map[string]any{"question": ""},
				},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.extractQuestion(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("extractQuestion() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("extractQuestion() returned nil, want non-nil")
			}
			if got.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", got.Text, tt.wantText)
			}
			if len(got.Options) != tt.wantOpts {
				t.Errorf("Options = %d, want %d", len(got.Options), tt.wantOpts)
			}
		})
	}
}

func TestExtractToolCall(t *testing.T) {
	p := NewYAMLBlockParser()

	tests := []struct {
		block    map[string]any
		name     string
		wantName string
		wantNil  bool
	}{
		{
			name:    "empty block",
			block:   map[string]any{},
			wantNil: true,
		},
		{
			name:    "no name",
			block:   map[string]any{"input": map[string]any{"path": "test"}},
			wantNil: true,
		},
		{
			name:     "valid tool call",
			block:    map[string]any{"name": "Read", "input": map[string]any{"file_path": "test.go"}},
			wantNil:  false,
			wantName: "Read",
		},
		{
			name:     "tool call without input",
			block:    map[string]any{"name": "Complete"},
			wantNil:  false,
			wantName: "Complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.extractToolCall(tt.block)
			if tt.wantNil {
				if got != nil {
					t.Errorf("extractToolCall() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("extractToolCall() returned nil, want non-nil")
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

func TestParse_EmptyEvents(t *testing.T) {
	p := NewYAMLBlockParser()

	resp, err := p.Parse([]Event{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp == nil {
		t.Fatal("Parse returned nil")
	}
	if len(resp.Files) != 0 {
		t.Errorf("Files = %d, want 0", len(resp.Files))
	}
}

func TestParse_TextEvents(t *testing.T) {
	p := NewYAMLBlockParser()

	events := []Event{
		{Type: EventText, Text: "Hello "},
		{Type: EventText, Text: "World"},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(resp.Messages) == 0 {
		t.Fatal("Messages should not be empty")
	}
	if resp.Messages[0] != "Hello World" {
		t.Errorf("Messages[0] = %q, want %q", resp.Messages[0], "Hello World")
	}
}

func TestParse_WithFileBlocks(t *testing.T) {
	p := NewYAMLBlockParser()

	fileBlock := "```yaml:file\npath: test.go\noperation: create\ncontent: package main\n```"
	events := []Event{
		{Type: EventText, Text: fileBlock},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("Files = %d, want 1", len(resp.Files))
	}
	if resp.Files[0].Path != "test.go" {
		t.Errorf("Files[0].Path = %q, want %q", resp.Files[0].Path, "test.go")
	}
	if resp.Files[0].Operation != FileOpCreate {
		t.Errorf("Files[0].Operation = %q, want %q", resp.Files[0].Operation, FileOpCreate)
	}
}

func TestParse_WithSummaryBlock(t *testing.T) {
	p := NewYAMLBlockParser()

	summaryBlock := "```yaml:summary\ntext: This is the summary\n```"
	events := []Event{
		{Type: EventText, Text: summaryBlock},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp.Summary != "This is the summary" {
		t.Errorf("Summary = %q, want %q", resp.Summary, "This is the summary")
	}
}

func TestParse_WithUsage(t *testing.T) {
	p := NewYAMLBlockParser()

	events := []Event{
		{Type: EventUsage, Data: map[string]any{
			"input_tokens":            float64(100),
			"output_tokens":           float64(50),
			"cache_read_input_tokens": float64(20),
		}},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if resp.Usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", resp.Usage.OutputTokens)
	}
	if resp.Usage.CachedTokens != 20 {
		t.Errorf("CachedTokens = %d, want 20", resp.Usage.CachedTokens)
	}
}

func TestParse_WithQuestion(t *testing.T) {
	p := NewYAMLBlockParser()

	events := []Event{
		{
			Type: EventToolUse,
			ToolCall: &ToolCall{
				Name: "AskUserQuestion",
				Input: map[string]any{
					"questions": []any{
						map[string]any{
							"question": "What option?",
							"options": []any{
								map[string]any{"label": "Yes", "description": "Proceed"},
								map[string]any{"label": "No", "description": "Cancel"},
							},
						},
					},
				},
			},
		},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp.Question == nil {
		t.Fatal("Question should not be nil")
	}
	if resp.Question.Text != "What option?" {
		t.Errorf("Question.Text = %q, want %q", resp.Question.Text, "What option?")
	}
	if len(resp.Question.Options) != 2 {
		t.Errorf("Question.Options = %d, want 2", len(resp.Question.Options))
	}
}

func TestParse_ResultEvent(t *testing.T) {
	p := NewYAMLBlockParser()

	events := []Event{
		{Type: EventComplete, Data: map[string]any{"result": "Final output text"}},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(resp.Messages) == 0 || resp.Messages[0] != "Final output text" {
		t.Errorf("Messages = %v, want ['Final output text']", resp.Messages)
	}
}

func TestParse_DeltaFormat(t *testing.T) {
	p := NewYAMLBlockParser()

	events := []Event{
		{Type: EventText, Data: map[string]any{"delta": map[string]any{"text": "Delta text"}}},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(resp.Messages) == 0 || resp.Messages[0] != "Delta text" {
		t.Errorf("Messages = %v, want ['Delta text']", resp.Messages)
	}
}

func TestParse_AssistantMessage(t *testing.T) {
	p := NewYAMLBlockParser()

	events := []Event{
		{Type: EventText, Data: map[string]any{
			"message": map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "Assistant text"},
				},
			},
		}},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(resp.Messages) == 0 || resp.Messages[0] != "Assistant text" {
		t.Errorf("Messages = %v, want ['Assistant text']", resp.Messages)
	}
}

func TestParseUsage(t *testing.T) {
	p := NewYAMLBlockParser()

	tests := []struct {
		data map[string]any
		want *UsageStats
		name string
	}{
		{
			name: "all fields",
			data: map[string]any{
				"input_tokens":            float64(100),
				"output_tokens":           float64(50),
				"cache_read_input_tokens": float64(20),
			},
			want: &UsageStats{InputTokens: 100, OutputTokens: 50, CachedTokens: 20},
		},
		{
			name: "partial fields",
			data: map[string]any{
				"input_tokens":  float64(100),
				"output_tokens": float64(50),
			},
			want: &UsageStats{InputTokens: 100, OutputTokens: 50, CachedTokens: 0},
		},
		{
			name: "empty data",
			data: map[string]any{},
			want: &UsageStats{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.parseUsage(tt.data)
			if got.InputTokens != tt.want.InputTokens {
				t.Errorf("InputTokens = %d, want %d", got.InputTokens, tt.want.InputTokens)
			}
			if got.OutputTokens != tt.want.OutputTokens {
				t.Errorf("OutputTokens = %d, want %d", got.OutputTokens, tt.want.OutputTokens)
			}
			if got.CachedTokens != tt.want.CachedTokens {
				t.Errorf("CachedTokens = %d, want %d", got.CachedTokens, tt.want.CachedTokens)
			}
		})
	}
}

func TestNewJSONLineParser(t *testing.T) {
	p := NewJSONLineParser()
	if p == nil {
		t.Fatal("NewJSONLineParser returned nil")
	}
}

func TestJSONLineParser_ParseEvent(t *testing.T) {
	p := NewJSONLineParser()

	tests := []struct {
		name     string
		input    string
		wantType EventType
	}{
		{"error event", `{"error":"something wrong"}`, EventError},
		{"usage event", `{"usage":{"tokens":100}}`, EventUsage},
		{"text event", `{"text":"hello"}`, EventText},
		{"plain text", "not json", EventText},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := p.ParseEvent([]byte(tt.input))
			if err != nil {
				t.Fatalf("ParseEvent: %v", err)
			}
			if event.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", event.Type, tt.wantType)
			}
		})
	}
}

func TestJSONLineParser_Parse(t *testing.T) {
	p := NewJSONLineParser()

	events := []Event{
		{Type: EventText, Text: "Test output"},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp == nil {
		t.Fatal("Parse returned nil")
	}
}

func TestParse_QuestionFromToolCallsArray(t *testing.T) {
	p := NewYAMLBlockParser()

	// Test extracting question from tool_calls array in event data
	events := []Event{
		{
			Type: EventToolUse,
			Data: map[string]any{
				"tool_calls": []*ToolCall{
					{
						Name: "AskUserQuestion",
						Input: map[string]any{
							"questions": []any{
								map[string]any{
									"question": "Array question?",
									"options":  []any{},
								},
							},
						},
					},
				},
			},
		},
	}

	resp, err := p.Parse(events)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp.Question == nil {
		t.Fatal("Question should not be nil")
	}
	if resp.Question.Text != "Array question?" {
		t.Errorf("Question.Text = %q, want %q", resp.Question.Text, "Array question?")
	}
}
