// Package anthropic implements the API agent Provider for Anthropic's Messages API.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/valksor/kvelmo/pkg/agent/apiagent"
)

const defaultBaseURL = "https://api.anthropic.com"

// Config holds Anthropic-specific configuration.
type Config struct {
	APIKey  string // API key (falls back to ANTHROPIC_API_KEY env var)
	BaseURL string // API base URL (default: https://api.anthropic.com)
	Model   string // Model name (default: claude-sonnet-4-20250514)
}

// Provider implements apiagent.Provider for Anthropic.
type Provider struct {
	config Config
}

// NewProvider creates a new Anthropic provider.
func NewProvider(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}

	return &Provider{config: cfg}
}

// New creates a fully configured API agent using the Anthropic provider.
func New(cfg Config, apiCfg apiagent.APIConfig) *apiagent.Base {
	provider := NewProvider(cfg)
	apiCfg.APIKey = provider.resolveAPIKey()
	apiCfg.BaseURL = provider.config.BaseURL
	apiCfg.Model = provider.config.Model

	return apiagent.NewBase(provider, apiCfg)
}

// Name returns "anthropic".
func (p *Provider) Name() string {
	return "anthropic"
}

// Available checks if the API key is configured.
func (p *Provider) Available() error {
	if p.resolveAPIKey() == "" {
		return errors.New("anthropic: ANTHROPIC_API_KEY not set")
	}

	return nil
}

// BuildRequest constructs an Anthropic Messages API request.
func (p *Provider) BuildRequest(ctx context.Context, cfg *apiagent.APIConfig, messages []apiagent.Message, tools []apiagent.ToolDef) (*http.Request, error) {
	// Anthropic separates system prompt from messages
	var system string
	var apiMessages []map[string]any

	for _, msg := range messages {
		if msg.Role == apiagent.RoleSystem {
			system = msg.Content

			continue
		}

		apiMessages = append(apiMessages, convertMessage(msg))
	}

	body := map[string]any{
		"model":    cfg.Model,
		"stream":   true,
		"messages": apiMessages,
	}

	if system != "" {
		body["system"] = system
	}

	if len(tools) > 0 {
		body["tools"] = convertTools(tools)
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	body["max_tokens"] = maxTokens

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Anthropic-Version", "2023-06-01")

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = p.resolveAPIKey()
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}

	return req, nil
}

// ParseStream parses an Anthropic SSE stream into chunks.
func (p *Provider) ParseStream(ctx context.Context, body io.ReadCloser) (<-chan apiagent.Chunk, error) {
	chunks := make(chan apiagent.Chunk, 16)

	go func() {
		defer close(chunks)
		defer body.Close() //nolint:errcheck // Closed by caller or stream end

		// Track content blocks by index
		blocks := make(map[int]*contentBlock)

		for sse := range apiagent.ParseSSE(ctx, body) {
			switch sse.Event {
			case "content_block_start":
				var event contentBlockStart
				if err := json.Unmarshal([]byte(sse.Data), &event); err != nil {
					continue
				}

				blocks[event.Index] = &contentBlock{
					blockType: event.ContentBlock.Type,
					id:        event.ContentBlock.ID,
					name:      event.ContentBlock.Name,
				}

			case "content_block_delta":
				var event contentBlockDelta
				if err := json.Unmarshal([]byte(sse.Data), &event); err != nil {
					continue
				}

				block, ok := blocks[event.Index]
				if !ok {
					continue
				}

				switch event.Delta.Type {
				case "text_delta":
					chunks <- apiagent.Chunk{
						Type: apiagent.ChunkText,
						Text: event.Delta.Text,
					}

				case "input_json_delta":
					block.inputJSON += event.Delta.PartialJSON
				}

			case "content_block_stop":
				var event contentBlockStop
				if err := json.Unmarshal([]byte(sse.Data), &event); err != nil {
					continue
				}

				block, ok := blocks[event.Index]
				if !ok {
					continue
				}

				// Emit completed tool_use blocks
				if block.blockType == "tool_use" {
					var input map[string]any
					if block.inputJSON != "" {
						_ = json.Unmarshal([]byte(block.inputJSON), &input)
					}
					if input == nil {
						input = make(map[string]any)
					}

					chunks <- apiagent.Chunk{
						Type: apiagent.ChunkToolUse,
						ToolUse: &apiagent.ToolUseChunk{
							ID:    block.id,
							Name:  block.name,
							Input: input,
						},
					}
				}

				delete(blocks, event.Index)

			case "message_stop":
				chunks <- apiagent.Chunk{Type: apiagent.ChunkDone}

				return

			case "error":
				var event errorEvent
				if err := json.Unmarshal([]byte(sse.Data), &event); err != nil {
					chunks <- apiagent.Chunk{Type: apiagent.ChunkError, Error: sse.Data}
				} else {
					chunks <- apiagent.Chunk{Type: apiagent.ChunkError, Error: event.Error.Message}
				}

				return

			case "message_start", "message_delta", "ping":
				// Informational events, skip
			}
		}

		// Stream ended without message_stop
		chunks <- apiagent.Chunk{Type: apiagent.ChunkDone}
	}()

	return chunks, nil
}

func (p *Provider) resolveAPIKey() string {
	if p.config.APIKey != "" {
		return p.config.APIKey
	}

	return os.Getenv("ANTHROPIC_API_KEY")
}

// convertMessage converts a neutral message to Anthropic format.
func convertMessage(msg apiagent.Message) map[string]any {
	m := map[string]any{
		"role": string(msg.Role),
	}

	if msg.Role == apiagent.RoleAssistant && len(msg.ToolCalls) > 0 {
		// Assistant message with tool use: content is an array of blocks
		var content []map[string]any
		if msg.Content != "" {
			content = append(content, map[string]any{
				"type": "text",
				"text": msg.Content,
			})
		}

		for _, tc := range msg.ToolCalls {
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Name,
				"input": tc.Input,
			})
		}

		m["content"] = content
	} else if msg.Role == apiagent.RoleTool && msg.ToolResult != nil {
		// Tool result: Anthropic uses role "user" with tool_result content blocks
		m["role"] = "user"
		m["content"] = []map[string]any{
			{
				"type":        "tool_result",
				"tool_use_id": msg.ToolResult.ToolCallID,
				"content":     msg.ToolResult.Content,
				"is_error":    msg.ToolResult.IsError,
			},
		}
	} else {
		m["content"] = msg.Content
	}

	return m
}

// convertTools converts neutral tool definitions to Anthropic format.
func convertTools(tools []apiagent.ToolDef) []map[string]any {
	var result []map[string]any

	for _, t := range tools {
		result = append(result, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.Parameters,
		})
	}

	return result
}

// Anthropic SSE event types

type contentBlock struct {
	blockType string
	id        string
	name      string
	inputJSON string
}

type contentBlockStart struct {
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
		Text string `json:"text"`
	} `json:"content_block"`
}

type contentBlockDelta struct {
	Index int `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
}

type contentBlockStop struct {
	Index int `json:"index"`
}

type errorEvent struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
