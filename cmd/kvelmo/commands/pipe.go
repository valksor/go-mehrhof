package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/anthropic"
	"github.com/valksor/kvelmo/agent/apiagent"
	"github.com/valksor/kvelmo/agent/claude"
	"github.com/valksor/kvelmo/agent/claudesdk"
	"github.com/valksor/kvelmo/agent/codex"
	"github.com/valksor/kvelmo/agent/ollama"
	"github.com/valksor/kvelmo/agent/openai"
	"github.com/valksor/kvelmo/agent/replay"
	"github.com/valksor/kvelmo/meta"
	"github.com/valksor/kvelmo/settings"
)

var PipeCmd = &cobra.Command{
	Use:   "pipe [prompt]",
	Short: "Run a one-shot prompt through the configured AI agent",
	Long: fmt.Sprintf(`Run a single prompt through the configured AI agent and stream the response to stdout.

No running '%[1]s serve' instance is required — the agent is invoked directly.

The prompt can be provided as arguments or piped via stdin:

  %[1]s pipe "summarize the README"
  echo "what files are here?" | %[1]s pipe

The agent is resolved from settings (global then project), and can be
overridden per-invocation with flags.`, meta.Name),
	Args: cobra.ArbitraryArgs,
	RunE: runPipe,
}

var (
	pipeAgent   string
	pipeTimeout time.Duration
	pipeReplay  string
)

func init() {
	PipeCmd.Flags().StringVarP(&pipeAgent, "agent", "a", "", "Agent to use (claude, codex, or custom agent name)")
	PipeCmd.Flags().DurationVar(&pipeTimeout, "timeout", 10*time.Minute, "Maximum execution time")
	PipeCmd.Flags().StringVar(&pipeReplay, "replay", "", "Replay a recording file instead of calling a live agent")
}

func runPipe(cmd *cobra.Command, args []string) error {
	// Resolve prompt: args first, then stdin.
	prompt := strings.Join(args, " ")
	if prompt == "" {
		data, err := io.ReadAll(bufio.NewReader(os.Stdin))
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		prompt = strings.TrimSpace(string(data))
	}
	if prompt == "" {
		return fmt.Errorf("no prompt provided\n\nUsage: %s pipe \"your prompt here\"\n   or: echo \"prompt\" | %s pipe", meta.Name, meta.Name)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Load effective settings (global merged with project).
	effective, _, _, err := settings.LoadEffective(cwd)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	// Determine agent name.
	agentName := pipeAgent
	if agentName == "" {
		agentName = effective.Agent.Default
	}

	// Build registry with all available agents.
	// Use KvelmoPermissionHandler to allow Write/Edit/Bash for planning/implementation.
	reg := agent.NewRegistry()
	if err := claude.RegisterWithPermissionHandler(reg, agent.KvelmoPermissionHandler); err != nil {
		slog.Debug("claude agent not available", "error", err)
	}
	// claude-sdk: WebSocket Agent SDK variant, broken on the official
	// Anthropic CLI (--sdk-url rejected). See agent/claudesdk/.
	if err := claudesdk.RegisterWithPermissionHandler(reg, agent.KvelmoPermissionHandler); err != nil {
		slog.Debug("claude-sdk agent not available", "error", err)
	}
	if err := codex.RegisterWithPermissionHandler(reg, agent.KvelmoPermissionHandler); err != nil {
		slog.Debug("codex agent not available", "error", err)
	}
	// claude-mcp is the default agent — must be registered here too.
	if err := registerClaudeMCP(reg, effective); err != nil {
		slog.Debug("claude-mcp agent not available", "error", err)
	}

	// Register API-based agents from settings.
	apiCfg := apiagent.DefaultAPIConfig()
	apiCfg.TokenBudget = effective.Agent.TokenBudget

	oaiCfg := effective.Agent.OpenAI
	if err := reg.Register(openai.New(
		openai.Config{APIKey: oaiCfg.APIKey, BaseURL: oaiCfg.BaseURL, Model: oaiCfg.Model}, apiCfg,
	)); err != nil {
		slog.Debug("openai agent registration failed", "error", err)
	}

	antCfg := effective.Agent.Anthropic
	if err := reg.Register(anthropic.New(
		anthropic.Config{APIKey: antCfg.APIKey, BaseURL: antCfg.BaseURL, Model: antCfg.Model}, apiCfg,
	)); err != nil {
		slog.Debug("anthropic agent registration failed", "error", err)
	}

	olCfg := effective.Agent.Ollama
	if err := reg.Register(ollama.New(
		ollama.Config{BaseURL: olCfg.BaseURL, Model: olCfg.Model}, apiCfg,
	)); err != nil {
		slog.Debug("ollama agent registration failed", "error", err)
	}

	// Resolve the agent instance.
	var ag agent.Agent

	if pipeReplay != "" {
		// Use replay agent instead of live agent.
		ra, err := replay.New(pipeReplay)
		if err != nil {
			return fmt.Errorf("load replay recording: %w", err)
		}
		ag = ra
	} else if customCfg, ok := effective.CustomAgents[agentName]; ok {
		// Custom agent: extend a base agent with extra args/env.
		base, err := reg.Get(customCfg.Extends)
		if err != nil {
			return fmt.Errorf("custom agent %q extends unknown base %q: %w", agentName, customCfg.Extends, err)
		}
		ag = base
		for k, v := range customCfg.Env {
			ag = ag.WithEnv(k, v)
		}
		if len(customCfg.Args) > 0 {
			ag = ag.WithArgs(customCfg.Args...)
		}
	} else {
		ag, err = reg.GetOrDetect(agentName)
		if err != nil {
			return fmt.Errorf("resolve agent: %w", err)
		}
	}

	ag = ag.WithWorkDir(cwd).WithTimeout(pipeTimeout)

	// Set up context with timeout and signal handling.
	ctx, cancel := context.WithTimeout(context.Background(), pipeTimeout)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Connect the agent.
	if err := ag.Connect(ctx); err != nil {
		return fmt.Errorf("connect agent: %w", err)
	}
	defer func() { _ = ag.Close() }()

	// Send the prompt and stream output.
	events, err := ag.SendPrompt(ctx, prompt)
	if err != nil {
		return fmt.Errorf("send prompt: %w", err)
	}

	// Streams are preferred (token-by-token). If no stream content arrives,
	// fall back to printing the full assistant message so short or cached
	// responses still reach stdout.
	var streamPrinted bool
	var assistantFallback string

	for event := range events {
		switch event.Type {
		case agent.EventStream:
			if event.Content != "" {
				streamPrinted = true
				fmt.Print(event.Content)
			}
		case agent.EventAssistant:
			if event.Content != "" {
				assistantFallback = event.Content
			}
		case agent.EventPermission:
			if event.PermissionRequest != nil {
				approved := agent.DefaultPermissionHandler(*event.PermissionRequest)
				_ = ag.HandlePermission(event.PermissionRequest.ID, approved)
			}
		case agent.EventComplete:
			if !streamPrinted && assistantFallback != "" {
				fmt.Print(assistantFallback)
			}
			fmt.Println()

			return nil
		case agent.EventError:
			return fmt.Errorf("agent error: %s", event.Error)
		case agent.EventToolUse, agent.EventToolResult,
			agent.EventInit, agent.EventKeepAlive, agent.EventSubagent, agent.EventProgress,
			agent.EventToolProgress, agent.EventInterrupted, agent.EventPromptSuggestion:
			// Not relevant to the pipe command; silently ignored.
		}
	}

	if !streamPrinted && assistantFallback != "" {
		fmt.Print(assistantFallback)
		fmt.Println()
	}

	return nil
}
