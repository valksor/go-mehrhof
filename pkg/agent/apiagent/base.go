package apiagent

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valksor/kvelmo/pkg/agent"
	"github.com/valksor/kvelmo/pkg/metrics"
)

// Base implements the agent.Agent interface for API-based agents.
// It drives the conversation loop, tool execution, and event streaming.
// Provider implementations only need to handle HTTP request building and stream parsing.
type Base struct {
	provider Provider
	config   APIConfig
	executor *ToolExecutor

	connected   atomic.Bool
	interrupted atomic.Bool
	cancelFn    context.CancelFunc

	mu sync.RWMutex
}

// NewBase creates a new API agent with the given provider and config.
func NewBase(provider Provider, config APIConfig) *Base {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Minute
	}
	if config.MaxTurns == 0 {
		config.MaxTurns = 50
	}
	if config.ExecTimeout == 0 {
		config.ExecTimeout = 2 * time.Minute
	}

	return &Base{
		provider: provider,
		config:   config,
		executor: &ToolExecutor{
			WorkDir:           config.WorkDir,
			PermissionHandler: agent.KvelmoPermissionHandler,
			ExecTimeout:       config.ExecTimeout,
		},
	}
}

// Name returns the provider name.
func (b *Base) Name() string {
	return b.provider.Name()
}

// Available checks if the provider is available.
func (b *Base) Available() error {
	return b.provider.Available()
}

// Connect marks the agent as connected.
// If the provider implements the Connector interface (e.g., Ollama model pull),
// it is called first.
func (b *Base) Connect(ctx context.Context) error {
	if connector, ok := b.provider.(Connector); ok {
		if err := connector.Connect(ctx, &b.config); err != nil {
			return err
		}
	}

	b.connected.Store(true)

	return nil
}

// Connected returns true if the agent is connected.
func (b *Base) Connected() bool {
	return b.connected.Load()
}

// SendPrompt sends a prompt and returns streaming events.
// This drives the conversation loop: prompt → stream → tool_use → execute → continue.
func (b *Base) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	if !b.connected.Load() {
		return nil, fmt.Errorf("%s: not connected", b.provider.Name())
	}

	b.interrupted.Store(false)

	events := make(chan agent.Event, 32)
	ctx, cancel := context.WithCancel(ctx)

	b.mu.Lock()
	b.cancelFn = cancel
	b.mu.Unlock()

	go b.conversationLoop(ctx, cancel, prompt, events)

	return events, nil
}

// HandlePermission is a no-op for API agents (permissions are handled inline by the executor).
func (b *Base) HandlePermission(_ string, _ bool) error {
	return nil
}

// Interrupt aborts the current conversation.
func (b *Base) Interrupt() error {
	b.interrupted.Store(true)

	b.mu.RLock()
	cancel := b.cancelFn
	b.mu.RUnlock()

	if cancel != nil {
		cancel()
	}

	return nil
}

// Close disconnects the agent.
func (b *Base) Close() error {
	b.connected.Store(false)

	if err := b.Interrupt(); err != nil {
		return err
	}

	return nil
}

// WithEnv returns a new agent with the environment variable set.
func (b *Base) WithEnv(key, value string) agent.Agent {
	clone := b.clone()
	// API agents don't use env vars for CLI, but store for potential subprocess tools
	if key == "OPENAI_API_KEY" || key == "ANTHROPIC_API_KEY" {
		clone.config.APIKey = value
	}

	return clone
}

// WithArgs is a no-op for API agents (no CLI subprocess).
func (b *Base) WithArgs(_ ...string) agent.Agent {
	return b.clone()
}

// WithWorkDir returns a new agent with the working directory set.
func (b *Base) WithWorkDir(dir string) agent.Agent {
	clone := b.clone()
	clone.config.WorkDir = dir
	clone.executor = &ToolExecutor{
		WorkDir:           dir,
		PermissionHandler: clone.executor.PermissionHandler,
		ExecTimeout:       clone.executor.ExecTimeout,
	}

	return clone
}

// WithTimeout returns a new agent with the timeout set.
func (b *Base) WithTimeout(d time.Duration) agent.Agent {
	clone := b.clone()
	clone.config.Timeout = d

	return clone
}

// conversationLoop drives the multi-turn conversation with tool execution.
func (b *Base) conversationLoop(ctx context.Context, cancel context.CancelFunc, prompt string, events chan<- agent.Event) {
	defer close(events)
	defer cancel()

	// Emit init event
	b.emit(events, agent.Event{
		Type:      agent.EventInit,
		Timestamp: time.Now(),
		Data:      map[string]any{"agent": b.provider.Name(), "model": b.config.Model},
	})

	// Build initial messages
	systemPrompt := b.config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt(b.config.WorkDir)
	}

	messages := []Message{
		{Role: RoleSystem, Content: systemPrompt},
		{Role: RoleUser, Content: prompt},
	}

	tools := KvelmoTools()

	var totalInputTokens, totalOutputTokens int64

	for turn := range b.config.MaxTurns {
		slog.Debug("apiagent: conversation turn", "agent", b.provider.Name(), "turn", turn+1, "messages", len(messages))

		// Check token budget before each turn.
		if b.config.TokenBudget > 0 && (totalInputTokens+totalOutputTokens) >= b.config.TokenBudget {
			b.emitError(events, fmt.Sprintf("token budget exceeded: used %d of %d tokens",
				totalInputTokens+totalOutputTokens, b.config.TokenBudget))

			return
		}

		if b.interrupted.Load() {
			b.emit(events, agent.Event{
				Type:      agent.EventInterrupted,
				Timestamp: time.Now(),
			})

			return
		}

		// Build and execute HTTP request
		req, err := b.provider.BuildRequest(ctx, &b.config, messages, tools)
		if err != nil {
			b.emitError(events, fmt.Sprintf("build request (turn %d): %v", turn+1, err))

			return
		}

		resp, err := b.config.httpClient().Do(req)
		if err != nil {
			b.emitError(events, fmt.Sprintf("API request: %v", err))

			return
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close() //nolint:errcheck // Best-effort cleanup on error path
			b.emitError(events, fmt.Sprintf("API returned status %d", resp.StatusCode))

			return
		}

		// Parse streaming response
		chunks, err := b.provider.ParseStream(ctx, resp.Body)
		if err != nil {
			resp.Body.Close() //nolint:errcheck // Best-effort cleanup on error path
			b.emitError(events, fmt.Sprintf("parse stream: %v", err))

			return
		}

		// Collect this turn's response
		var assistantTextBuilder strings.Builder
		var toolCalls []ToolCall
		hadToolUse := false
		var turnUsage *UsageData

		for chunk := range chunks {
			if b.interrupted.Load() {
				resp.Body.Close() //nolint:errcheck // Best-effort cleanup on error path
				b.emit(events, agent.Event{
					Type:      agent.EventInterrupted,
					Timestamp: time.Now(),
				})

				return
			}

			switch chunk.Type {
			case ChunkText:
				assistantTextBuilder.WriteString(chunk.Text)
				b.emit(events, agent.Event{
					Type:      agent.EventStream,
					Content:   chunk.Text,
					Timestamp: time.Now(),
				})

			case ChunkToolUse:
				hadToolUse = true
				if chunk.ToolUse != nil {
					toolCalls = append(toolCalls, ToolCall{
						ID:    chunk.ToolUse.ID,
						Name:  chunk.ToolUse.Name,
						Input: chunk.ToolUse.Input,
					})

					b.emit(events, agent.Event{
						Type:      agent.EventToolUse,
						Content:   chunk.ToolUse.Name,
						Timestamp: time.Now(),
						Data: map[string]any{
							"tool_call_id": chunk.ToolUse.ID,
							"tool":         chunk.ToolUse.Name,
							"input":        chunk.ToolUse.Input,
						},
					})
				}

			case ChunkError:
				resp.Body.Close() //nolint:errcheck // Best-effort cleanup on error path
				b.emitError(events, chunk.Error)

				return

			case ChunkDone:
				// Stream finished for this turn
				turnUsage = chunk.Usage
			}
		}

		resp.Body.Close() //nolint:errcheck // Best-effort cleanup on error path

		// Accumulate token usage for this turn.
		if turnUsage != nil {
			totalInputTokens += turnUsage.InputTokens
			totalOutputTokens += turnUsage.OutputTokens
			metrics.Global().RecordTokens(turnUsage.InputTokens + turnUsage.OutputTokens)
		}

		// Add assistant message to conversation
		assistantText := assistantTextBuilder.String()
		messages = append(messages, Message{
			Role:      RoleAssistant,
			Content:   assistantText,
			ToolCalls: toolCalls,
		})

		// If no tool calls, we're done
		slog.Debug("apiagent: turn complete", "agent", b.provider.Name(), "turn", turn+1, "tool_calls", len(toolCalls), "text_len", len(assistantText))
		if !hadToolUse {
			b.emit(events, agent.Event{
				Type:      agent.EventAssistant,
				Content:   assistantText,
				Timestamp: time.Now(),
			})
			b.emit(events, agent.Event{
				Type:      agent.EventComplete,
				Timestamp: time.Now(),
				Data: map[string]any{
					"input_tokens":  totalInputTokens,
					"output_tokens": totalOutputTokens,
					"total_tokens":  totalInputTokens + totalOutputTokens,
				},
			})

			return
		}

		// Execute tool calls and add results
		for _, tc := range toolCalls {
			slog.Debug("executing tool", "agent", b.provider.Name(), "tool", tc.Name, "turn", turn+1)

			output, execErr := b.executor.Execute(ctx, tc.Name, tc.Input)

			isError := execErr != nil
			content := output
			if isError {
				content = execErr.Error()
			}

			messages = append(messages, Message{
				Role: RoleTool,
				ToolResult: &ToolResult{
					ToolCallID: tc.ID,
					Content:    content,
					IsError:    isError,
				},
			})

			b.emit(events, agent.Event{
				Type:      agent.EventToolResult,
				Content:   tc.Name,
				Timestamp: time.Now(),
				Data: map[string]any{
					"tool_call_id": tc.ID,
					"tool":         tc.Name,
					"is_error":     isError,
					"output":       truncateOutput(content, 500),
				},
			})
		}

		// Continue the loop — send tool results back to the model
	}

	// Exceeded max turns
	b.emitError(events, fmt.Sprintf("exceeded maximum conversation turns (%d)", b.config.MaxTurns))
}

// emit sends an event on the channel.
// Control events (error, complete, interrupted) block to guarantee delivery.
// Stream events use non-blocking send to avoid stalling on slow consumers.
func (b *Base) emit(events chan<- agent.Event, event agent.Event) {
	switch event.Type { //nolint:exhaustive // Default handles all non-control event types
	case agent.EventError, agent.EventComplete, agent.EventInterrupted, agent.EventInit:
		// Control events must not be dropped — block until delivered
		events <- event
	default:
		// Stream/tool events: best-effort, skip if channel full
		select {
		case events <- event:
		default:
		}
	}
}

// emitError emits an error event.
func (b *Base) emitError(events chan<- agent.Event, msg string) {
	b.emit(events, agent.Event{
		Type:      agent.EventError,
		Error:     msg,
		Timestamp: time.Now(),
	})
}

// clone creates a shallow copy of the Base for With* builders.
func (b *Base) clone() *Base {
	return &Base{
		provider: b.provider,
		config:   b.config,
		executor: &ToolExecutor{
			WorkDir:           b.executor.WorkDir,
			PermissionHandler: b.executor.PermissionHandler,
			ExecTimeout:       b.executor.ExecTimeout,
		},
	}
}

// truncateOutput truncates output for event data to avoid huge payloads.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen] + "... (truncated)"
}
