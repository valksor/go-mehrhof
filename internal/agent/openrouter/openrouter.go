package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

// AgentName is the canonical name for this agent.
const AgentName = "openrouter"

// DefaultModel is the default model to use.
const DefaultModel = "anthropic/claude-3.5-sonnet"

// BaseURL is the OpenRouter API endpoint.
const BaseURL = "https://openrouter.ai/api/v1/chat/completions"

// Agent wraps the OpenRouter API for multi-model inference.
type Agent struct {
	httpClient *http.Client
	config     agent.Config
	model      string
	apiKey     string
}

// New creates an OpenRouter agent with default config.
func New() *Agent {
	return &Agent{
		httpClient: &http.Client{Timeout: 5 * time.Minute},
		config: agent.Config{
			Environment: make(map[string]string),
			Timeout:     5 * time.Minute,
			RetryCount:  3,
			RetryDelay:  time.Second,
		},
		model:  DefaultModel,
		apiKey: resolveAPIKey(""),
	}
}

// NewWithConfig creates an OpenRouter agent with custom config.
func NewWithConfig(cfg agent.Config) *Agent {
	return &Agent{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		config:     cfg,
		model:      DefaultModel,
		apiKey:     resolveAPIKey(""),
	}
}

// NewWithModel creates an OpenRouter agent with a specific model.
func NewWithModel(model string) *Agent {
	a := New()
	a.model = model

	return a
}

// resolveAPIKey finds API key from multiple sources.
func resolveAPIKey(configKey string) string {
	if key := os.Getenv("MEHR_OPENROUTER_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		return key
	}

	return configKey
}

// Name returns the agent identifier.
func (a *Agent) Name() string {
	return AgentName
}

// Available checks if the OpenRouter API is accessible.
func (a *Agent) Available() error {
	if a.apiKey == "" {
		return errors.New("OpenRouter API key not configured. Set OPENROUTER_API_KEY environment variable")
	}

	return nil
}

// Run executes a prompt and returns the aggregated response.
func (a *Agent) Run(ctx context.Context, prompt string) (*agent.Response, error) {
	events, errCh := a.RunStream(ctx, prompt)

	var collected []agent.Event
	for event := range events {
		collected = append(collected, event)
	}

	if err := <-errCh; err != nil {
		return nil, err
	}

	return parseEvents(collected)
}

// RunStream executes a prompt and streams events.
func (a *Agent) RunStream(ctx context.Context, prompt string) (<-chan agent.Event, <-chan error) {
	eventCh := make(chan agent.Event, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		err := a.executeStream(ctx, prompt, eventCh)
		if err != nil {
			errCh <- err
		}
	}()

	return eventCh, errCh
}

// RunWithCallback executes with a callback for each event.
func (a *Agent) RunWithCallback(ctx context.Context, prompt string, cb agent.StreamCallback) (*agent.Response, error) {
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

	return parseEvents(collected)
}

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request format for OpenRouter API.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// ChatResponse is the response format for non-streaming.
type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// StreamChunk is the format of streaming chunks.
type StreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

func (a *Agent) executeStream(ctx context.Context, prompt string, eventCh chan<- agent.Event) error {
	// Determine model from config args
	model := a.model
	for i, arg := range a.config.Args {
		if arg == "--model" && i+1 < len(a.config.Args) {
			model = a.config.Args[i+1]

			break
		}
	}

	// Build request
	reqBody := ChatRequest{
		Model: model,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
		Stream: true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Resolve API key (check config env first)
	apiKey := a.apiKey
	if key, ok := a.config.Environment["OPENROUTER_API_KEY"]; ok && key != "" {
		apiKey = key
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Http-Referer", "https://github.com/valksor/go-mehrhof")
	req.Header.Set("X-Title", "Mehrhof CLI")

	// Execute request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse SSE stream
	reader := bufio.NewReader(resp.Body)
	var usage *agent.UsageStats

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" || line == "data: [DONE]" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		// Extract text content
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				eventCh <- agent.Event{
					Type:      agent.EventText,
					Timestamp: time.Now(),
					Text:      choice.Delta.Content,
					Data:      map[string]any{"text": choice.Delta.Content},
				}
			}

			if choice.FinishReason != "" {
				// Check for usage in final chunk
				if chunk.Usage != nil {
					usage = &agent.UsageStats{
						InputTokens:  chunk.Usage.PromptTokens,
						OutputTokens: chunk.Usage.CompletionTokens,
					}
				}
			}
		}
	}

	// Send completion event
	completeData := map[string]any{"model": model}
	if usage != nil {
		completeData["usage"] = usage
	}

	eventCh <- agent.Event{
		Type:      agent.EventComplete,
		Timestamp: time.Now(),
		Data:      completeData,
	}

	return nil
}

// parseEvents aggregates events into a response.
func parseEvents(events []agent.Event) (*agent.Response, error) {
	response := &agent.Response{
		Files:    make([]agent.FileChange, 0),
		Messages: make([]string, 0),
	}

	var textBuilder strings.Builder
	for _, event := range events {
		switch event.Type {
		case agent.EventText:
			textBuilder.WriteString(event.Text)
		case agent.EventComplete:
			if usage, ok := event.Data["usage"].(*agent.UsageStats); ok {
				response.Usage = usage
			}
		case agent.EventToolUse, agent.EventToolResult, agent.EventFile, agent.EventError, agent.EventUsage:
			// Ignore other event types
		}
	}

	fullText := strings.TrimSpace(textBuilder.String())
	if fullText != "" {
		response.Messages = append(response.Messages, fullText)
		response.Summary = extractSummary(fullText)
	}

	return response, nil
}

// extractSummary gets a summary from the response.
func extractSummary(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 200 {
				return line[:200] + "..."
			}

			return line
		}
	}

	return ""
}

// SetModel sets the model to use.
func (a *Agent) SetModel(model string) {
	a.model = model
}

// GetModel returns the current model.
func (a *Agent) GetModel() string {
	return a.model
}

// WithWorkDir sets the working directory (not used for API agent).
func (a *Agent) WithWorkDir(dir string) *Agent {
	newConfig := a.config
	newConfig.WorkDir = dir

	return &Agent{
		httpClient: a.httpClient,
		config:     newConfig,
		model:      a.model,
		apiKey:     a.apiKey,
	}
}

// WithTimeout sets execution timeout.
func (a *Agent) WithTimeout(d time.Duration) *Agent {
	newConfig := a.config
	newConfig.Timeout = d

	return &Agent{
		httpClient: &http.Client{Timeout: d},
		config:     newConfig,
		model:      a.model,
		apiKey:     a.apiKey,
	}
}

// WithModel returns a new Agent with a different model.
func (a *Agent) WithModel(model string) *Agent {
	return &Agent{
		httpClient: a.httpClient,
		config:     a.config,
		model:      model,
		apiKey:     a.apiKey,
	}
}

// WithEnv adds an environment variable.
func (a *Agent) WithEnv(key, value string) agent.Agent {
	newConfig := a.config
	newConfig.Environment = make(map[string]string, len(a.config.Environment)+1)
	for k, v := range a.config.Environment {
		newConfig.Environment[k] = v
	}
	newConfig.Environment[key] = value

	// Update API key if it's being set
	apiKey := a.apiKey
	if key == "OPENROUTER_API_KEY" || key == "MEHR_OPENROUTER_API_KEY" {
		apiKey = value
	}

	return &Agent{
		httpClient: a.httpClient,
		config:     newConfig,
		model:      a.model,
		apiKey:     apiKey,
	}
}

// WithArgs adds CLI arguments.
func (a *Agent) WithArgs(args ...string) agent.Agent {
	newConfig := a.config
	newArgs := make([]string, len(a.config.Args), len(a.config.Args)+len(args))
	copy(newArgs, a.config.Args)
	newConfig.Args = append(newArgs, args...)

	return &Agent{
		httpClient: a.httpClient,
		config:     newConfig,
		model:      a.model,
		apiKey:     a.apiKey,
	}
}

// Metadata returns agent capabilities.
func (a *Agent) Metadata() agent.AgentMetadata {
	return agent.AgentMetadata{
		Name:        "OpenRouter",
		Description: "Unified API for 100+ AI models (Claude, GPT, Llama, etc.)",
		Models: []agent.ModelInfo{
			{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Default: true},
			{ID: "anthropic/claude-3-opus", Name: "Claude 3 Opus"},
			{ID: "openai/gpt-4-turbo", Name: "GPT-4 Turbo"},
			{ID: "openai/gpt-4o", Name: "GPT-4o"},
			{ID: "meta-llama/llama-3.1-405b-instruct", Name: "Llama 3.1 405B"},
			{ID: "google/gemini-pro-1.5", Name: "Gemini 1.5 Pro"},
		},
		Capabilities: agent.AgentCapabilities{
			Streaming:      true,
			ToolUse:        false, // Depends on model
			FileOperations: false,
			CodeExecution:  false,
			MultiTurn:      true,
			SystemPrompt:   true,
		},
	}
}

// Register adds the OpenRouter agent to a registry.
func Register(r *agent.Registry) error {
	return r.Register(New())
}

// Ensure Agent implements agent.Agent and MetadataProvider.
var (
	_ agent.Agent            = (*Agent)(nil)
	_ agent.MetadataProvider = (*Agent)(nil)
)
