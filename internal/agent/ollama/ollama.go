package ollama

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

const AgentName = "ollama"

// DefaultModel is the default model to use when none is specified
const DefaultModel = "codellama"

// Agent wraps the ollama CLI for local model inference
type Agent struct {
	parser agent.Parser
	config agent.Config
	model  string
}

// New creates an Ollama agent with default config
func New() *Agent {
	return &Agent{
		config: agent.Config{
			Command:     []string{"ollama"},
			Environment: make(map[string]string),
			Timeout:     30 * time.Minute,
			RetryCount:  3,
			RetryDelay:  time.Second,
		},
		model:  DefaultModel,
		parser: NewPlainTextParser(),
	}
}

// NewWithConfig creates an Ollama agent with custom config
func NewWithConfig(cfg agent.Config) *Agent {
	if len(cfg.Command) == 0 {
		cfg.Command = []string{"ollama"}
	}
	return &Agent{
		config: cfg,
		model:  DefaultModel,
		parser: NewPlainTextParser(),
	}
}

// NewWithModel creates an Ollama agent with a specific model
func NewWithModel(model string) *Agent {
	a := New()
	a.model = model
	return a
}

// Name returns the agent identifier
func (a *Agent) Name() string {
	return AgentName
}

// Available checks if the ollama CLI is installed and running
func (a *Agent) Available() error {
	binary := a.config.Command[0]
	path, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("ollama CLI not found: %w", err)
	}

	// Verify it runs with --version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ollama CLI not working: %w", err)
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
					return fmt.Errorf("ollama exited with code %d: %s", exitErr.ExitCode(), stderrBytes)
				}
				return fmt.Errorf("ollama exited with code %d", exitErr.ExitCode())
			}
		}
		return fmt.Errorf("wait error: %w", err)
	}

	return nil
}

func (a *Agent) buildArgs(prompt string) []string {
	args := []string{"run"}

	// Determine the model to use
	model := a.model

	// Check if model is specified in config args (e.g., --model llama3)
	for i, arg := range a.config.Args {
		if arg == "--model" && i+1 < len(a.config.Args) {
			model = a.config.Args[i+1]
			break
		}
	}

	// Add model
	args = append(args, model)

	// Add any additional config args (excluding --model since we handled it)
	for i := 0; i < len(a.config.Args); i++ {
		if a.config.Args[i] == "--model" && i+1 < len(a.config.Args) {
			i++ // Skip the model value
			continue
		}
		args = append(args, a.config.Args[i])
	}

	// Add the prompt as the final argument
	args = append(args, prompt)

	return args
}

// SetParser allows overriding the default parser
func (a *Agent) SetParser(p agent.Parser) {
	a.parser = p
}

// SetModel sets the model to use
func (a *Agent) SetModel(model string) {
	a.model = model
}

// GetModel returns the current model
func (a *Agent) GetModel() string {
	return a.model
}

// WithWorkDir sets the working directory
// Returns a new Agent instance with the updated config to avoid data races.
func (a *Agent) WithWorkDir(dir string) *Agent {
	newConfig := a.config
	newConfig.WorkDir = dir
	return &Agent{
		config: newConfig,
		model:  a.model,
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
		model:  a.model,
		parser: a.parser,
	}
}

// WithModel returns a new Agent instance with a different model.
func (a *Agent) WithModel(model string) *Agent {
	newConfig := a.config
	return &Agent{
		config: newConfig,
		model:  model,
		parser: a.parser,
	}
}

// WithEnv adds an environment variable.
// Returns a new Agent instance with the updated config to avoid data races.
//
// Thread safety: This method is safe for concurrent use as it returns a new
// Agent instance rather than modifying the receiver. The returned Agent shares
// the same parser reference with the original; if the parser is not thread-safe,
// avoid calling Run/RunStream on multiple Agent instances concurrently.
func (a *Agent) WithEnv(key, value string) agent.Agent {
	newConfig := a.config
	newConfig.Environment = make(map[string]string, len(a.config.Environment)+1)
	for k, v := range a.config.Environment {
		newConfig.Environment[k] = v
	}
	newConfig.Environment[key] = value
	return &Agent{
		config: newConfig,
		model:  a.model,
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
		model:  a.model,
		parser: a.parser,
	}
}

// Register adds the Ollama agent to a registry
func Register(r *agent.Registry) error {
	return r.Register(New())
}

// Ensure Agent implements agent.Agent
var _ agent.Agent = (*Agent)(nil)

// PlainTextParser parses plain text output from ollama
type PlainTextParser struct{}

// NewPlainTextParser creates a plain text parser for ollama output
func NewPlainTextParser() *PlainTextParser {
	return &PlainTextParser{}
}

// ParseEvent parses a single line of plain text output
func (p *PlainTextParser) ParseEvent(line []byte) (agent.Event, error) {
	return agent.Event{
		Type:      agent.EventText,
		Timestamp: time.Now(),
		Text:      string(line),
		Data:      map[string]any{"text": string(line)},
		Raw:       line,
	}, nil
}

// Parse aggregates events into a response
func (p *PlainTextParser) Parse(events []agent.Event) (*agent.Response, error) {
	response := &agent.Response{
		Files:    make([]agent.FileChange, 0),
		Messages: make([]string, 0),
	}

	var textBuilder strings.Builder
	for _, event := range events {
		if event.Text != "" {
			textBuilder.WriteString(event.Text)
			textBuilder.WriteString("\n")
		}
	}

	fullText := strings.TrimSpace(textBuilder.String())
	if fullText != "" {
		response.Messages = append(response.Messages, fullText)
		response.Summary = summarizeOllamaOutput(fullText)
	}

	return response, nil
}

// summarizeOllamaOutput extracts a summary from ollama's output
func summarizeOllamaOutput(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return ""
	}

	// For ollama, just take the first meaningful line as summary
	// since it outputs plain text responses
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
