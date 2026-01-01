package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

const AgentName = "gemini"

// Agent wraps the Gemini CLI (https://github.com/google-gemini/gemini-cli)
type Agent struct {
	parser agent.Parser
	config agent.Config
}

// New creates a Gemini agent with default config
func New() *Agent {
	return &Agent{
		config: agent.Config{
			Command:     []string{"gemini"},
			Environment: make(map[string]string),
			Timeout:     30 * time.Minute,
			RetryCount:  3,
			RetryDelay:  time.Second,
		},
		parser: NewGeminiParser(),
	}
}

// NewWithConfig creates a Gemini agent with custom config
func NewWithConfig(cfg agent.Config) *Agent {
	if len(cfg.Command) == 0 {
		cfg.Command = []string{"gemini"}
	}
	return &Agent{
		config: cfg,
		parser: NewGeminiParser(),
	}
}

// Name returns the agent identifier
func (a *Agent) Name() string {
	return AgentName
}

// Available checks if the Gemini CLI is installed and configured
func (a *Agent) Available() error {
	binary := a.config.Command[0]
	path, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("gemini CLI not found: %w", err)
	}

	// Verify it runs
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gemini CLI not working: %w", err)
	}

	return nil
}

// Run executes a prompt and returns the aggregated response
func (a *Agent) Run(ctx context.Context, prompt string) (*agent.Response, error) {
	events, errCh := a.RunStream(ctx, prompt)

	var collected []agent.Event
	for event := range events {
		collected = append(collected, event)
	}

	// Check for streaming errors
	if err := <-errCh; err != nil {
		return nil, err
	}

	return a.parser.Parse(collected)
}

// RunStream executes a prompt and streams events
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

// RunWithCallback executes with a callback for each event
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

	return a.parser.Parse(collected)
}

func (a *Agent) executeStream(ctx context.Context, prompt string, eventCh chan<- agent.Event) error {
	// Build command with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()

	args := a.buildArgs(prompt)
	cmd := exec.CommandContext(timeoutCtx, a.config.Command[0], args...)

	// Set working directory
	if a.config.WorkDir != "" {
		cmd.Dir = a.config.WorkDir
	}

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range a.config.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// Get stderr pipe for error messages
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	// Read output line by line
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large responses

	for scanner.Scan() {
		select {
		case <-timeoutCtx.Done():
			_ = cmd.Process.Kill()
			return timeoutCtx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		event, err := a.parser.ParseEvent(line)
		if err != nil {
			// Log parse error but continue
			continue
		}

		eventCh <- event

		// Check for completion
		if event.Type == agent.EventComplete {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	// Read any stderr output
	stderrBytes, err := bufio.NewReader(stderr).ReadString('\n')
	if err != nil && err != io.EOF {
		// Log but don't fail - stderr may not have content
		slog.Debug("error reading stderr", "error", err)
	}

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		// Check if it's just a non-zero exit (which might be okay)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				if stderrBytes != "" {
					return fmt.Errorf("gemini exited with code %d: %s", exitErr.ExitCode(), stderrBytes)
				}
				return fmt.Errorf("gemini exited with code %d", exitErr.ExitCode())
			}
		}
		return fmt.Errorf("wait error: %w", err)
	}

	return nil
}

func (a *Agent) buildArgs(prompt string) []string {
	args := []string{}

	// Add base arguments from config
	if len(a.config.Command) > 1 {
		args = append(args, a.config.Command[1:]...)
	}

	// Add configured CLI arguments
	if len(a.config.Args) > 0 {
		args = append(args, a.config.Args...)
	}

	// Non-interactive mode with prompt
	args = append(args, "-p", prompt)

	// Use streaming JSON output for structured parsing
	args = append(args, "--output-format", "stream-json")

	return args
}

// SetParser allows overriding the default parser
func (a *Agent) SetParser(p agent.Parser) {
	a.parser = p
}

// WithWorkDir sets the working directory
// Returns a new Agent instance with the updated config to avoid data races.
func (a *Agent) WithWorkDir(dir string) *Agent {
	newConfig := a.config
	newConfig.WorkDir = dir
	return &Agent{
		config: newConfig,
		parser: a.parser,
	}
}

// WithTimeout sets execution timeout
// Returns a new Agent instance with the updated config to avoid data races.
func (a *Agent) WithTimeout(d time.Duration) *Agent {
	newConfig := a.config
	newConfig.Timeout = d
	return &Agent{
		config: newConfig,
		parser: a.parser,
	}
}

// WithEnv adds an environment variable.
// Returns a new Agent instance with the updated config to avoid data races.
func (a *Agent) WithEnv(key, value string) agent.Agent {
	newConfig := a.config
	newConfig.Environment = make(map[string]string, len(a.config.Environment)+1)
	for k, v := range a.config.Environment {
		newConfig.Environment[k] = v
	}
	newConfig.Environment[key] = value
	return &Agent{
		config: newConfig,
		parser: a.parser,
	}
}

// WithArgs adds CLI arguments to pass to the agent process
// Returns a new Agent instance with the updated config to avoid data races.
func (a *Agent) WithArgs(args ...string) agent.Agent {
	newConfig := a.config
	newArgs := make([]string, len(a.config.Args), len(a.config.Args)+len(args))
	copy(newArgs, a.config.Args)
	newConfig.Args = append(newArgs, args...)
	return &Agent{
		config: newConfig,
		parser: a.parser,
	}
}

// Metadata returns information about the Gemini agent
func (a *Agent) Metadata() agent.AgentMetadata {
	return agent.AgentMetadata{
		Name:        "Gemini CLI",
		Description: "Google Gemini AI via the official CLI",
		Models: []agent.ModelInfo{
			{
				ID:        "gemini-2.5-pro",
				Name:      "Gemini 2.5 Pro",
				Default:   true,
				MaxTokens: 1000000, // 1M token context
			},
			{
				ID:        "gemini-2.5-flash",
				Name:      "Gemini 2.5 Flash",
				Default:   false,
				MaxTokens: 1000000,
			},
			{
				ID:        "gemini-3-pro",
				Name:      "Gemini 3 Pro",
				Default:   false,
				MaxTokens: 1000000,
			},
		},
		Capabilities: agent.AgentCapabilities{
			Streaming:      true,
			ToolUse:        true,
			FileOperations: true,
			CodeExecution:  true,
			MultiTurn:      true,
			SystemPrompt:   true,
		},
	}
}

// Register adds the Gemini agent to a registry
func Register(r *agent.Registry) error {
	return r.Register(New())
}

// Ensure Agent implements agent.Agent
var _ agent.Agent = (*Agent)(nil)

// Ensure Agent implements agent.MetadataProvider
var _ agent.MetadataProvider = (*Agent)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// Gemini-specific parser for stream-json output
// ─────────────────────────────────────────────────────────────────────────────

// GeminiParser parses Gemini CLI's streaming JSON output
type GeminiParser struct{}

// NewGeminiParser creates a new parser for Gemini output
func NewGeminiParser() *GeminiParser {
	return &GeminiParser{}
}

// ParseEvent parses a single line of JSON output from Gemini CLI
func (p *GeminiParser) ParseEvent(line []byte) (agent.Event, error) {
	event := agent.Event{
		Timestamp: time.Now(),
		Data:      make(map[string]any),
		Raw:       line,
	}

	// Try to parse as JSON
	var jsonData map[string]any
	if err := json.Unmarshal(line, &jsonData); err == nil {
		event.Data = jsonData

		// Determine event type from JSON structure
		// Gemini CLI stream-json format may vary; handle common patterns
		if typ, ok := jsonData["type"].(string); ok {
			switch typ {
			case "text", "content":
				event.Type = agent.EventText
				if text, ok := jsonData["text"].(string); ok {
					event.Text = text
				} else if content, ok := jsonData["content"].(string); ok {
					event.Text = content
				}
			case "tool_call", "function_call":
				event.Type = agent.EventToolUse
				p.parseToolCall(&event, jsonData)
			case "tool_result", "function_result":
				event.Type = agent.EventToolResult
			case "done", "complete", "end":
				event.Type = agent.EventComplete
			case "error":
				event.Type = agent.EventError
			default:
				event.Type = agent.EventText
			}
		}

		// Handle text content in various formats
		if event.Text == "" {
			if text, ok := jsonData["text"].(string); ok {
				event.Text = text
				event.Type = agent.EventText
			} else if content, ok := jsonData["content"].(string); ok {
				event.Text = content
				event.Type = agent.EventText
			} else if parts, ok := jsonData["parts"].([]any); ok {
				// Gemini often returns content in parts array
				var textParts []string
				for _, part := range parts {
					if partMap, ok := part.(map[string]any); ok {
						if text, ok := partMap["text"].(string); ok {
							textParts = append(textParts, text)
						}
					}
				}
				if len(textParts) > 0 {
					event.Text = strings.Join(textParts, "")
					event.Type = agent.EventText
				}
			}
		}

		// Check for usage/metadata
		if usage, ok := jsonData["usageMetadata"].(map[string]any); ok {
			event.Type = agent.EventUsage
			event.Data = usage
		}

		// Check for finish reason (indicates completion)
		if _, ok := jsonData["finishReason"]; ok {
			event.Type = agent.EventComplete
		}

		return event, nil
	}

	// Plain text line fallback
	event.Type = agent.EventText
	event.Text = string(line)
	event.Data["text"] = string(line)

	return event, nil
}

// parseToolCall extracts tool call information from Gemini output
func (p *GeminiParser) parseToolCall(event *agent.Event, jsonData map[string]any) {
	// Handle Gemini's function call format
	if functionCall, ok := jsonData["functionCall"].(map[string]any); ok {
		name, _ := functionCall["name"].(string)
		args, _ := functionCall["args"].(map[string]any)
		event.ToolCall = &agent.ToolCall{
			Name:  name,
			Input: args,
		}
	}
}

// Parse aggregates events into a response
func (p *GeminiParser) Parse(events []agent.Event) (*agent.Response, error) {
	response := &agent.Response{
		Files:    make([]agent.FileChange, 0),
		Messages: make([]string, 0),
	}

	var textBuilder strings.Builder
	for _, event := range events {
		switch event.Type {
		case agent.EventText:
			if event.Text != "" {
				textBuilder.WriteString(event.Text)
			}
		case agent.EventUsage:
			response.Usage = p.parseUsage(event.Data)
		}
	}

	fullText := strings.TrimSpace(textBuilder.String())
	if fullText != "" {
		response.Messages = append(response.Messages, fullText)
		response.Summary = summarizeOutput(fullText)
	}

	return response, nil
}

// parseUsage extracts usage statistics from Gemini metadata
func (p *GeminiParser) parseUsage(data map[string]any) *agent.UsageStats {
	stats := &agent.UsageStats{}

	// Gemini uses different field names
	if v, ok := data["promptTokenCount"].(float64); ok {
		stats.InputTokens = int(v)
	}
	if v, ok := data["candidatesTokenCount"].(float64); ok {
		stats.OutputTokens = int(v)
	}
	if v, ok := data["cachedContentTokenCount"].(float64); ok {
		stats.CachedTokens = int(v)
	}

	return stats
}

// summarizeOutput extracts a summary from the response
func summarizeOutput(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return ""
	}

	// Return first non-empty line, truncated if needed
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
