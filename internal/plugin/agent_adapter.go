package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

// AgentAdapter wraps a plugin process to implement the agent.Agent interface.
type AgentAdapter struct {
	manifest *Manifest
	proc     *Process
	env      map[string]string
	parser   agent.Parser
}

// NewAgentAdapter creates a new agent adapter for a plugin.
func NewAgentAdapter(manifest *Manifest, proc *Process) *AgentAdapter {
	return &AgentAdapter{
		manifest: manifest,
		proc:     proc,
		env:      make(map[string]string),
		parser:   agent.NewYAMLBlockParser(),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent interface implementation
// ─────────────────────────────────────────────────────────────────────────────

// Name returns the agent's identifier.
func (a *AgentAdapter) Name() string {
	if a.manifest.Agent != nil {
		return a.manifest.Agent.Name
	}
	return a.manifest.Name
}

// Available checks if the agent plugin is available.
func (a *AgentAdapter) Available() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a.proc.Call(ctx, "agent.available", nil)
	if err != nil {
		return fmt.Errorf("check agent availability: %w", err)
	}

	var resp AgentAvailableResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return fmt.Errorf("parse availability response: %w", err)
	}

	if !resp.Available {
		if resp.Error != "" {
			return fmt.Errorf("agent error: %s", resp.Error)
		}
		return fmt.Errorf("agent not available")
	}
	return nil
}

// Run executes a prompt and returns the response.
func (a *AgentAdapter) Run(ctx context.Context, prompt string) (*agent.Response, error) {
	events, errCh := a.RunStream(ctx, prompt)

	var collected []agent.Event
	for event := range events {
		collected = append(collected, event)
	}

	if err := <-errCh; err != nil {
		return nil, err
	}

	return a.parser.Parse(collected)
}

// RunStream executes a prompt and streams events.
func (a *AgentAdapter) RunStream(ctx context.Context, prompt string) (<-chan agent.Event, <-chan error) {
	eventCh := make(chan agent.Event, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		// Send run request with streaming
		streamCh, err := a.proc.Stream(ctx, "agent.run", &AgentRunParams{
			Prompt: prompt,
			Env:    a.env,
		})
		if err != nil {
			errCh <- fmt.Errorf("start agent run: %w", err)
			return
		}

		// Process stream events
		for raw := range streamCh {
			var streamEvent StreamEvent
			if err := json.Unmarshal(raw, &streamEvent); err != nil {
				continue // Skip malformed events
			}

			event := convertStreamEvent(&streamEvent)
			eventCh <- event

			if event.Type == agent.EventComplete {
				return
			}
		}
	}()

	return eventCh, errCh
}

// RunWithCallback executes with a callback for each event.
func (a *AgentAdapter) RunWithCallback(ctx context.Context, prompt string, cb agent.StreamCallback) (*agent.Response, error) {
	events, errCh := a.RunStream(ctx, prompt)

	var collected []agent.Event
	for event := range events {
		if err := cb(event); err != nil {
			return nil, fmt.Errorf("callback error: %w", err)
		}
		collected = append(collected, event)
	}

	if err := <-errCh; err != nil {
		return nil, err
	}

	return a.parser.Parse(collected)
}

// WithEnv adds an environment variable and returns a new agent with that env set.
func (a *AgentAdapter) WithEnv(key, value string) agent.Agent {
	newEnv := make(map[string]string, len(a.env)+1)
	for k, v := range a.env {
		newEnv[k] = v
	}
	newEnv[key] = value

	return &AgentAdapter{
		manifest: a.manifest,
		proc:     a.proc,
		env:      newEnv,
		parser:   a.parser,
	}
}

// WithArgs adds CLI arguments. Plugin agents don't support args, so this is a no-op.
func (a *AgentAdapter) WithArgs(args ...string) agent.Agent {
	// Plugin agents don't support CLI args - they use their own protocol
	return a
}

// ─────────────────────────────────────────────────────────────────────────────
// MetadataProvider interface
// ─────────────────────────────────────────────────────────────────────────────

// Metadata returns information about the agent.
func (a *AgentAdapter) Metadata() agent.AgentMetadata {
	caps := agent.AgentCapabilities{
		Streaming: a.manifest.Agent != nil && a.manifest.Agent.Streaming,
	}

	// Check for specific capabilities
	if a.manifest.Agent != nil {
		for _, c := range a.manifest.Agent.Capabilities {
			switch c {
			case "streaming":
				caps.Streaming = true
			case "tool_use":
				caps.ToolUse = true
			case "file_operations":
				caps.FileOperations = true
			case "code_execution":
				caps.CodeExecution = true
			case "multi_turn":
				caps.MultiTurn = true
			case "system_prompt":
				caps.SystemPrompt = true
			}
		}
	}

	return agent.AgentMetadata{
		Name:         a.Name(),
		Description:  a.manifest.Description,
		Capabilities: caps,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Additional methods
// ─────────────────────────────────────────────────────────────────────────────

// Manifest returns the plugin manifest.
func (a *AgentAdapter) Manifest() *Manifest {
	return a.manifest
}

// SetParser sets a custom parser for agent output.
func (a *AgentAdapter) SetParser(p agent.Parser) {
	a.parser = p
}

// ─────────────────────────────────────────────────────────────────────────────
// Conversion helpers
// ─────────────────────────────────────────────────────────────────────────────

func convertStreamEvent(e *StreamEvent) agent.Event {
	event := agent.Event{
		Timestamp: time.Now(),
		Raw:       e.Data,
	}

	switch e.Type {
	case StreamEventText:
		event.Type = agent.EventText
		var text string
		if err := json.Unmarshal(e.Data, &text); err == nil {
			event.Text = text
		} else {
			// Try as object with text field
			var obj struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(e.Data, &obj); err == nil {
				event.Text = obj.Text
			}
		}

	case StreamEventToolUse:
		event.Type = agent.EventToolUse
		var tc struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			Input       map[string]any `json:"input"`
		}
		if err := json.Unmarshal(e.Data, &tc); err == nil {
			event.ToolCall = &agent.ToolCall{
				Name:        tc.Name,
				Description: tc.Description,
				Input:       tc.Input,
			}
		}

	case StreamEventToolResult:
		event.Type = agent.EventToolResult
		var data map[string]any
		if err := json.Unmarshal(e.Data, &data); err == nil {
			event.Data = data
		}

	case StreamEventFile:
		event.Type = agent.EventFile
		var data map[string]any
		if err := json.Unmarshal(e.Data, &data); err == nil {
			event.Data = data
		}

	case StreamEventUsage:
		event.Type = agent.EventUsage
		var data map[string]any
		if err := json.Unmarshal(e.Data, &data); err == nil {
			event.Data = data
		}

	case StreamEventComplete:
		event.Type = agent.EventComplete

	case StreamEventError:
		event.Type = agent.EventError
		var errMsg string
		if err := json.Unmarshal(e.Data, &errMsg); err == nil {
			event.Data = map[string]any{"error": errMsg}
		}

	default:
		// Unknown event type, pass through as-is
		event.Type = agent.EventType(e.Type)
		var data map[string]any
		if err := json.Unmarshal(e.Data, &data); err == nil {
			event.Data = data
		}
	}

	return event
}
