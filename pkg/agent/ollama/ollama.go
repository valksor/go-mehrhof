// Package ollama implements the API agent Provider for Ollama.
// Uses Ollama's native /api/chat endpoint which supports proper tool calling.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/agent/apiagent"
)

const defaultBaseURL = "http://localhost:11434"

// Config holds Ollama-specific configuration.
type Config struct {
	BaseURL string // Ollama server URL (default: http://localhost:11434)
	Model   string // Model name (default: llama3.1)
}

// Provider implements apiagent.Provider for Ollama.
type Provider struct {
	config Config
}

// NewProvider creates a new Ollama provider.
func NewProvider(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = "llama3.1"
	}

	return &Provider{config: cfg}
}

// New creates a fully configured API agent using the Ollama provider.
func New(cfg Config, apiCfg apiagent.APIConfig) *apiagent.Base {
	provider := NewProvider(cfg)
	apiCfg.BaseURL = provider.config.BaseURL
	apiCfg.Model = provider.config.Model

	return apiagent.NewBase(provider, apiCfg)
}

// Name returns "ollama".
func (p *Provider) Name() string {
	return "ollama"
}

// Available checks if the Ollama server is reachable.
func (p *Provider) Available() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := strings.TrimRight(p.config.BaseURL, "/") + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("ollama: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama: server not reachable at %s: %w", p.config.BaseURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck // Read-only availability check

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama: server returned status %d", resp.StatusCode)
	}

	return nil
}

// Connect checks if the configured model exists locally and pulls it if missing.
// Implements apiagent.Connector.
func (p *Provider) Connect(ctx context.Context, cfg *apiagent.APIConfig) error {
	model := cfg.Model
	if model == "" {
		model = p.config.Model
	}

	return p.ensureModel(ctx, model)
}

// ensureModel checks if a model is available locally and pulls it if not.
func (p *Provider) ensureModel(ctx context.Context, model string) error {
	exists, err := p.modelExists(ctx, model)
	if err != nil {
		return fmt.Errorf("ollama: check model: %w", err)
	}
	if exists {
		return nil
	}

	return p.pullModel(ctx, model)
}

// modelExists checks if a model is available locally via /api/show.
func (p *Provider) modelExists(ctx context.Context, model string) (bool, error) {
	url := strings.TrimRight(p.config.BaseURL, "/") + "/api/show"
	body, err := json.Marshal(map[string]string{"name": model})
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close() //nolint:errcheck // Read-only check

	return resp.StatusCode == http.StatusOK, nil
}

// pullModel downloads a model from the Ollama registry via /api/pull.
func (p *Provider) pullModel(ctx context.Context, model string) error {
	fmt.Printf("Ollama: model %q not found locally, downloading...\n", model)
	slog.Info("ollama: model not found locally, pulling", "model", model)

	url := strings.TrimRight(p.config.BaseURL, "/") + "/api/pull"
	body, err := json.Marshal(map[string]any{"name": model, "stream": true})
	if err != nil {
		return fmt.Errorf("ollama: marshal pull request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ollama: create pull request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 0}).Do(req) // No timeout — downloads can be large
	if err != nil {
		return fmt.Errorf("ollama: pull model: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Streaming response cleanup

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama: pull returned status %d", resp.StatusCode)
	}

	var lastStatus string
	for msg := range apiagent.ParseNDJSON(ctx, resp.Body) {
		var progress pullProgress
		if jsonErr := json.Unmarshal(msg, &progress); jsonErr != nil {
			continue
		}

		if progress.Error != "" {
			return fmt.Errorf("ollama: pull failed: %s", progress.Error)
		}

		if progress.Status != lastStatus {
			fmt.Printf("Ollama: %s\n", progress.Status)
			slog.Info("ollama: pull progress", "status", progress.Status)
			lastStatus = progress.Status
		}
	}

	fmt.Printf("Ollama: model %q ready\n", model)
	slog.Info("ollama: model ready", "model", model)

	return nil
}

type pullProgress struct {
	Status    string `json:"status"`
	Digest    string `json:"digest"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Error     string `json:"error"`
}

// BuildRequest constructs a request using Ollama's native /api/chat endpoint.
// The native endpoint supports proper structured tool calling (unlike /v1/chat/completions).
func (p *Provider) BuildRequest(ctx context.Context, cfg *apiagent.APIConfig, messages []apiagent.Message, tools []apiagent.ToolDef) (*http.Request, error) {
	body := map[string]any{
		"model":    cfg.Model,
		"stream":   true,
		"messages": convertMessages(messages),
	}

	if len(tools) > 0 {
		body["tools"] = convertTools(tools)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// ParseStream parses Ollama's native NDJSON streaming format.
func (p *Provider) ParseStream(ctx context.Context, body io.ReadCloser) (<-chan apiagent.Chunk, error) {
	chunks := make(chan apiagent.Chunk, 16)

	go func() {
		defer close(chunks)
		defer body.Close() //nolint:errcheck // Stream cleanup

		for msg := range apiagent.ParseNDJSON(ctx, body) {
			var resp chatResponse
			if err := json.Unmarshal(msg, &resp); err != nil {
				continue
			}

			// Text content
			if resp.Message.Content != "" {
				chunks <- apiagent.Chunk{
					Type: apiagent.ChunkText,
					Text: resp.Message.Content,
				}
			}

			// Tool calls
			for _, tc := range resp.Message.ToolCalls {
				// Convert arguments from map to map[string]any
				input := make(map[string]any)
				for k, v := range tc.Function.Arguments {
					input[k] = v
				}

				id := tc.ID
				if id == "" {
					id = fmt.Sprintf("call_%d", time.Now().UnixNano())
				}

				chunks <- apiagent.Chunk{
					Type: apiagent.ChunkToolUse,
					ToolUse: &apiagent.ToolUseChunk{
						ID:    id,
						Name:  tc.Function.Name,
						Input: input,
					},
				}
			}

			// Check if done
			if resp.Done {
				chunks <- apiagent.Chunk{Type: apiagent.ChunkDone}

				return
			}
		}

		// Stream ended without done flag
		chunks <- apiagent.Chunk{Type: apiagent.ChunkDone}
	}()

	return chunks, nil
}

// Ollama native /api/chat response format.

type chatResponse struct {
	Message    chatMessage `json:"message"`
	Done       bool        `json:"done"`
	DoneReason string      `json:"done_reason"`
}

type chatMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []toolCall `json:"tool_calls"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// convertMessages converts neutral messages to Ollama native format.
func convertMessages(messages []apiagent.Message) []map[string]any {
	var result []map[string]any

	for _, msg := range messages {
		m := map[string]any{
			"role": string(msg.Role),
		}

		if msg.Role == apiagent.RoleTool && msg.ToolResult != nil {
			m["content"] = msg.ToolResult.Content
		} else if msg.Role == apiagent.RoleAssistant && len(msg.ToolCalls) > 0 {
			if msg.Content != "" {
				m["content"] = msg.Content
			}

			var toolCalls []map[string]any
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, map[string]any{
					"id": tc.ID,
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": tc.Input,
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

// convertTools converts neutral tool definitions to Ollama format.
func convertTools(tools []apiagent.ToolDef) []map[string]any {
	var result []map[string]any

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
