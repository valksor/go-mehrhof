// Package openai implements the API agent Provider for OpenAI's chat completions API.
// The stream parser is exported for reuse by OpenAI-compatible backends (e.g., Ollama).
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/valksor/kvelmo/pkg/agent/apiagent"
)

const defaultBaseURL = "https://api.openai.com"

// Config holds OpenAI-specific configuration.
type Config struct {
	APIKey  string // API key (falls back to OPENAI_API_KEY env var)
	BaseURL string // API base URL (default: https://api.openai.com)
	Model   string // Model name (default: gpt-4o)
}

// Provider implements apiagent.Provider for OpenAI.
type Provider struct {
	config Config
}

// NewProvider creates a new OpenAI provider.
func NewProvider(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}

	return &Provider{config: cfg}
}

// New creates a fully configured API agent using the OpenAI provider.
func New(cfg Config, apiCfg apiagent.APIConfig) *apiagent.Base {
	provider := NewProvider(cfg)
	apiCfg.APIKey = provider.resolveAPIKey()
	apiCfg.BaseURL = provider.config.BaseURL
	apiCfg.Model = provider.config.Model

	return apiagent.NewBase(provider, apiCfg)
}

// Name returns "openai".
func (p *Provider) Name() string {
	return "openai"
}

// Available checks if the API key is configured.
func (p *Provider) Available() error {
	if p.resolveAPIKey() == "" {
		return errors.New("openai: OPENAI_API_KEY not set")
	}

	return nil
}

// BuildRequest constructs an OpenAI chat completions request.
func (p *Provider) BuildRequest(ctx context.Context, cfg *apiagent.APIConfig, messages []apiagent.Message, tools []apiagent.ToolDef) (*http.Request, error) {
	body := map[string]any{
		"model":    cfg.Model,
		"stream":   true,
		"messages": ConvertMessages(messages),
	}

	if len(tools) > 0 {
		body["tools"] = ConvertTools(tools)
	}

	if cfg.MaxTokens > 0 {
		body["max_tokens"] = cfg.MaxTokens
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = p.resolveAPIKey()
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	return req, nil
}

// ParseStream parses an OpenAI SSE stream into chunks.
func (p *Provider) ParseStream(ctx context.Context, body io.ReadCloser) (<-chan apiagent.Chunk, error) {
	return ParseOpenAIStream(ctx, body), nil
}

// ParseOpenAIStream parses an OpenAI-format SSE stream.
// Exported for reuse by OpenAI-compatible backends (Ollama, etc.).
func ParseOpenAIStream(ctx context.Context, body io.ReadCloser) <-chan apiagent.Chunk {
	chunks := make(chan apiagent.Chunk, 16)

	go func() {
		defer close(chunks)
		defer body.Close() //nolint:errcheck // Closed by caller or stream end

		// Accumulate tool calls across chunks (OpenAI sends them incrementally)
		pendingTools := make(map[int]*toolCallAccumulator)
		var usage apiagent.UsageData

		for sse := range apiagent.ParseSSE(ctx, body) {
			if sse.Data == "[DONE]" {
				// Flush any pending tool calls
				flushTools(pendingTools, chunks)
				chunk := apiagent.Chunk{Type: apiagent.ChunkDone}
				if usage.InputTokens > 0 || usage.OutputTokens > 0 {
					chunk.Usage = &usage
				}
				chunks <- chunk

				return
			}

			var resp streamResponse
			if err := json.Unmarshal([]byte(sse.Data), &resp); err != nil {
				continue // Skip malformed events
			}

			// Accumulate usage data (OpenAI sends it on the final chunk)
			if resp.Usage != nil {
				usage.InputTokens += resp.Usage.PromptTokens
				usage.OutputTokens += resp.Usage.CompletionTokens
			}

			if len(resp.Choices) == 0 {
				continue
			}

			delta := resp.Choices[0].Delta

			// Text content
			if delta.Content != "" {
				chunks <- apiagent.Chunk{
					Type: apiagent.ChunkText,
					Text: delta.Content,
				}
			}

			// Tool calls (accumulated across chunks)
			for _, tc := range delta.ToolCalls {
				acc, ok := pendingTools[tc.Index]
				if !ok {
					acc = &toolCallAccumulator{
						id:   tc.ID,
						name: tc.Function.Name,
					}
					pendingTools[tc.Index] = acc
				}

				if tc.ID != "" {
					acc.id = tc.ID
				}
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
				}

				acc.args += tc.Function.Arguments
			}

			// Check for stop reason
			if resp.Choices[0].FinishReason != "" {
				flushTools(pendingTools, chunks)
				chunk := apiagent.Chunk{Type: apiagent.ChunkDone}
				if usage.InputTokens > 0 || usage.OutputTokens > 0 {
					chunk.Usage = &usage
				}
				chunks <- chunk

				return
			}
		}

		// Stream ended without explicit DONE
		flushTools(pendingTools, chunks)
		chunk := apiagent.Chunk{Type: apiagent.ChunkDone}
		if usage.InputTokens > 0 || usage.OutputTokens > 0 {
			chunk.Usage = &usage
		}
		chunks <- chunk
	}()

	return chunks
}

func (p *Provider) resolveAPIKey() string {
	if p.config.APIKey != "" {
		return p.config.APIKey
	}

	return os.Getenv("OPENAI_API_KEY")
}

// ConvertMessages converts neutral messages to OpenAI format.
// Exported for reuse by OpenAI-compatible backends (Ollama, etc.).
func ConvertMessages(messages []apiagent.Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))

	for _, msg := range messages {
		m := map[string]any{
			"role": string(msg.Role),
		}

		if msg.Role == apiagent.RoleTool && msg.ToolResult != nil {
			m["tool_call_id"] = msg.ToolResult.ToolCallID
			m["content"] = msg.ToolResult.Content
		} else if msg.Role == apiagent.RoleAssistant && len(msg.ToolCalls) > 0 {
			if msg.Content != "" {
				m["content"] = msg.Content
			}

			toolCalls := make([]map[string]any, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Input) //nolint:errchkjson // Input is always serializable map[string]any
				toolCalls = append(toolCalls, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": string(argsJSON),
					},
				})
			}

			m["tool_calls"] = toolCalls
		} else {
			m["content"] = msg.Content
		}

		result = append(result, m)
	}

	return result
}

// ConvertTools converts neutral tool definitions to OpenAI format.
// Exported for reuse by OpenAI-compatible backends (Ollama, etc.).
func ConvertTools(tools []apiagent.ToolDef) []map[string]any {
	result := make([]map[string]any, 0, len(tools))

	for _, t := range tools {
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}

	return result
}

// streamResponse represents a chunk from the OpenAI streaming API.
type streamResponse struct {
	Choices []struct {
		Delta struct {
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *streamUsage `json:"usage,omitempty"`
}

type streamUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
}

type toolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// toolCallAccumulator accumulates incremental tool call data.
type toolCallAccumulator struct {
	id   string
	name string
	args string
}

// flushTools emits accumulated tool calls as ChunkToolUse events in index order.
func flushTools(tools map[int]*toolCallAccumulator, chunks chan<- apiagent.Chunk) {
	// Emit in index order for deterministic output.
	indices := make([]int, 0, len(tools))
	for idx := range tools {
		indices = append(indices, idx)
	}
	slices.Sort(indices)

	for _, idx := range indices {
		acc := tools[idx]
		var input map[string]any
		if acc.args != "" {
			_ = json.Unmarshal([]byte(acc.args), &input)
		}
		if input == nil {
			input = make(map[string]any)
		}

		chunks <- apiagent.Chunk{
			Type: apiagent.ChunkToolUse,
			ToolUse: &apiagent.ToolUseChunk{
				ID:    acc.id,
				Name:  acc.name,
				Input: input,
			},
		}
	}

	// Clear the map
	for k := range tools {
		delete(tools, k)
	}
}
