package copilot

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
)

// AgentName is the canonical name for this agent.
const AgentName = "copilot"

// Mode defines the Copilot CLI operation mode.
type Mode string

const (
	// ModeSuggest uses "gh copilot suggest" for command suggestions.
	ModeSuggest Mode = "suggest"
	// ModeExplain uses "gh copilot explain" for explanations.
	ModeExplain Mode = "explain"
)

// TargetType for suggest mode.
type TargetType string

const (
	TargetShell TargetType = "shell"
	TargetGit   TargetType = "git"
	TargetGH    TargetType = "gh"
)

// Agent wraps the GitHub Copilot CLI (gh copilot).
type Agent struct {
	parser agent.Parser
	config agent.Config
	mode   Mode
	target TargetType
}

// New creates a Copilot agent with default config.
func New() *Agent {
	return &Agent{
		config: agent.Config{
			Command:     []string{"gh", "copilot"},
			Environment: make(map[string]string),
			Timeout:     5 * time.Minute, // Copilot is cloud-based, should be quick
			RetryCount:  3,
			RetryDelay:  time.Second,
		},
		mode:   ModeSuggest,
		target: TargetShell,
		parser: NewPlainTextParser(),
	}
}

// NewWithConfig creates a Copilot agent with custom config.
func NewWithConfig(cfg agent.Config) *Agent {
	if len(cfg.Command) == 0 {
		cfg.Command = []string{"gh", "copilot"}
	}
	return &Agent{
		config: cfg,
		mode:   ModeSuggest,
		target: TargetShell,
		parser: NewPlainTextParser(),
	}
}

// Name returns the agent identifier.
func (a *Agent) Name() string {
	return AgentName
}

// Available checks if the gh copilot extension is installed.
func (a *Agent) Available() error {
	// First check if gh is installed
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("gh CLI not found: %w", err)
	}

	// Check if copilot extension is installed
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ghPath, "copilot", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh copilot extension not installed or not working: %w", err)
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

	// Check for streaming errors
	if err := <-errCh; err != nil {
		return nil, err
	}

	return a.parser.Parse(collected)
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

	return a.parser.Parse(collected)
}

func (a *Agent) executeStream(ctx context.Context, prompt string, eventCh chan<- agent.Event) error {
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
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

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
			continue
		}

		eventCh <- event

		if event.Type == agent.EventComplete {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	// Read stderr
	stderrBytes, err := bufio.NewReader(stderr).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		slog.Debug("error reading stderr", "error", err)
	}

	// Wait for command
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() != 0 {
				if stderrBytes != "" {
					return fmt.Errorf("gh copilot exited with code %d: %s", exitErr.ExitCode(), stderrBytes)
				}
				return fmt.Errorf("gh copilot exited with code %d", exitErr.ExitCode())
			}
		}
		return fmt.Errorf("wait error: %w", err)
	}

	return nil
}

func (a *Agent) buildArgs(prompt string) []string {
	// Start with "copilot" subcommand (gh is the first element)
	args := []string{"copilot"}

	// Determine mode from config args or use default
	mode := a.mode
	target := a.target

	// Parse config args for mode/target overrides
	for i, arg := range a.config.Args {
		switch arg {
		case "--mode", "-m":
			if i+1 < len(a.config.Args) {
				switch a.config.Args[i+1] {
				case "suggest":
					mode = ModeSuggest
				case "explain":
					mode = ModeExplain
				}
			}
		case "--target", "-t":
			if i+1 < len(a.config.Args) {
				switch a.config.Args[i+1] {
				case "shell":
					target = TargetShell
				case "git":
					target = TargetGit
				case "gh":
					target = TargetGH
				}
			}
		}
	}

	// Add mode
	args = append(args, string(mode))

	// For suggest mode, add target type
	if mode == ModeSuggest {
		args = append(args, "-t", string(target))
	}

	// Add the prompt
	args = append(args, prompt)

	return args
}

// SetParser allows overriding the default parser.
func (a *Agent) SetParser(p agent.Parser) {
	a.parser = p
}

// SetMode sets the operation mode (suggest/explain).
func (a *Agent) SetMode(mode Mode) {
	a.mode = mode
}

// SetTarget sets the target type for suggest mode.
func (a *Agent) SetTarget(target TargetType) {
	a.target = target
}

// WithWorkDir sets the working directory.
func (a *Agent) WithWorkDir(dir string) *Agent {
	newConfig := a.config
	newConfig.WorkDir = dir
	return &Agent{
		config: newConfig,
		mode:   a.mode,
		target: a.target,
		parser: a.parser,
	}
}

// WithTimeout sets execution timeout.
func (a *Agent) WithTimeout(d time.Duration) *Agent {
	newConfig := a.config
	newConfig.Timeout = d
	return &Agent{
		config: newConfig,
		mode:   a.mode,
		target: a.target,
		parser: a.parser,
	}
}

// WithMode returns a new Agent with a different mode.
func (a *Agent) WithMode(mode Mode) *Agent {
	return &Agent{
		config: a.config,
		mode:   mode,
		target: a.target,
		parser: a.parser,
	}
}

// WithTarget returns a new Agent with a different target.
func (a *Agent) WithTarget(target TargetType) *Agent {
	return &Agent{
		config: a.config,
		mode:   a.mode,
		target: target,
		parser: a.parser,
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
	return &Agent{
		config: newConfig,
		mode:   a.mode,
		target: a.target,
		parser: a.parser,
	}
}

// WithArgs adds CLI arguments.
func (a *Agent) WithArgs(args ...string) agent.Agent {
	newConfig := a.config
	newArgs := make([]string, len(a.config.Args), len(a.config.Args)+len(args))
	copy(newArgs, a.config.Args)
	newConfig.Args = append(newArgs, args...)
	return &Agent{
		config: newConfig,
		mode:   a.mode,
		target: a.target,
		parser: a.parser,
	}
}

// Metadata returns agent capabilities.
func (a *Agent) Metadata() agent.AgentMetadata {
	return agent.AgentMetadata{
		Name:        "GitHub Copilot CLI",
		Description: "GitHub Copilot CLI for command suggestions and explanations",
		Capabilities: agent.AgentCapabilities{
			Streaming:      false, // Copilot CLI doesn't stream incrementally
			ToolUse:        false,
			FileOperations: false,
			CodeExecution:  false,
			MultiTurn:      false, // Single query per invocation
			SystemPrompt:   false,
		},
	}
}

// Register adds the Copilot agent to a registry.
func Register(r *agent.Registry) error {
	return r.Register(New())
}

// Ensure Agent implements agent.Agent and MetadataProvider.
var (
	_ agent.Agent            = (*Agent)(nil)
	_ agent.MetadataProvider = (*Agent)(nil)
)

// PlainTextParser parses plain text output from gh copilot.
type PlainTextParser struct{}

// NewPlainTextParser creates a plain text parser for copilot output.
func NewPlainTextParser() *PlainTextParser {
	return &PlainTextParser{}
}

// ParseEvent parses a single line of plain text output.
func (p *PlainTextParser) ParseEvent(line []byte) (agent.Event, error) {
	text := string(line)

	// Copilot outputs suggestions in a specific format
	// For suggest mode: it outputs the command directly
	// For explain mode: it outputs explanation text

	return agent.Event{
		Type:      agent.EventText,
		Timestamp: time.Now(),
		Text:      text,
		Data:      map[string]any{"text": text},
		Raw:       line,
	}, nil
}

// Parse aggregates events into a response.
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
		response.Summary = extractSummary(fullText)
	}

	return response, nil
}

// extractSummary extracts a summary from copilot output.
func extractSummary(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return ""
	}

	// Find the first non-empty line that looks like a command or explanation
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip header/prefix lines that copilot outputs
		if strings.HasPrefix(line, "Suggestion:") || strings.HasPrefix(line, "Command:") {
			continue
		}
		if len(line) > 200 {
			return line[:200] + "..."
		}
		return line
	}

	return ""
}
