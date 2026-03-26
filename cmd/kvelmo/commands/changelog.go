package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/agent"
	"github.com/valksor/kvelmo/pkg/agent/anthropic"
	"github.com/valksor/kvelmo/pkg/agent/apiagent"
	"github.com/valksor/kvelmo/pkg/agent/claude"
	"github.com/valksor/kvelmo/pkg/agent/codex"
	"github.com/valksor/kvelmo/pkg/agent/ollama"
	"github.com/valksor/kvelmo/pkg/agent/openai"
	"github.com/valksor/kvelmo/pkg/changelog"
	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/socket"
)

// errNoSocket signals that the worktree socket is unavailable, triggering standalone fallback.
var errNoSocket = errors.New("no worktree socket")

var (
	changelogFull  bool
	changelogJSON  bool
	changelogAgent string
)

// ChangelogCmd generates a changelog between two git refs.
var ChangelogCmd = &cobra.Command{
	Use:   "changelog <source> <target> [note]",
	Short: "Generate changelog between two refs",
	Long: `Generate a changelog from commits between two git refs (tags, branches, SHAs).

An AI agent analyzes the actual diff to produce meaningful, human-readable
release notes grouped by category (Added, Changed, Fixed, Removed).

Works with or without a running kvelmo server — when no server is running,
the agent is invoked directly (like 'pipe').

An optional note is rendered as additional context for the AI.

Examples:
  kvelmo changelog v1.0.0 v1.1.0
  kvelmo changelog main release/2.0
  kvelmo changelog abc1234 def5678 --full
  kvelmo changelog dev main "only frontend changes"
  kvelmo changelog v1.0.0 HEAD --agent claude`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runChangelog,
}

func init() {
	ChangelogCmd.Flags().BoolVar(&changelogFull, "full", false, "Include commit body text for richer AI context")
	ChangelogCmd.Flags().BoolVar(&changelogJSON, "json", false, "Output raw JSON response")
	ChangelogCmd.Flags().StringVarP(&changelogAgent, "agent", "a", "", "AI agent to use for generation")
}

func runChangelog(_ *cobra.Command, args []string) error {
	source, target := args[0], args[1]
	var note string
	if len(args) > 2 {
		note = args[2]
	}

	// Try socket path first.
	if err := changelogViaSocket(source, target, note); err != nil {
		if !errors.Is(err, errNoSocket) {
			return err
		}
		// Standalone: invoke agent directly.
		return changelogStandalone(source, target, note)
	}

	return nil
}

// changelogViaSocket attempts to generate a changelog through the worktree socket.
// Returns errNoSocket when no socket is available, triggering standalone fallback.
// Real server errors (bad refs, internal errors) are returned as-is to surface to the user.
func changelogViaSocket(source, target, note string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errNoSocket
	}
	if !socket.SocketExists(socket.WorktreeSocketPath(cwd)) {
		return errNoSocket
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	params := map[string]any{
		"source": source,
		"target": target,
		"full":   changelogFull,
	}
	if note != "" {
		params["note"] = note
	}

	resp, err := callWorktree(ctx, "changelog.generate", params)
	if err != nil {
		return fmt.Errorf("changelog: %w", err)
	}

	var result struct {
		Markdown string `json:"markdown"`
		JobID    string `json:"job_id"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		fmt.Println(string(resp.Result))

		return nil
	}

	// AI path: got a job_id, stream the output (--json not supported here).
	if result.JobID != "" {
		return waitForJob(socket.WorktreeSocketPath(cwd), result.JobID)
	}

	// Fallback path: got complete result.
	if changelogJSON {
		return outputJSON(resp.Result)
	}

	if result.Markdown == "" {
		fmt.Printf("No commits between %s and %s\n", source, target)
	} else {
		fmt.Print(result.Markdown)
	}

	return nil
}

// changelogStandalone generates a changelog by invoking an agent directly.
func changelogStandalone(source, target, note string) error {
	if changelogJSON {
		return errors.New("--json is not supported in standalone mode (requires running server)")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	repo, err := git.Open(cwd)
	if err != nil {
		return fmt.Errorf("open git repository: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Gather git context.
	commits, err := gatherChangelogCommits(ctx, repo, source, target, changelogFull)
	if err != nil {
		return fmt.Errorf("list commits: %w", err)
	}

	if len(commits) == 0 {
		fmt.Printf("No commits between %s and %s\n", source, target)

		return nil
	}

	diff, err := repo.DiffBetween(ctx, source, target)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}

	diffStat, err := repo.DiffStatBetween(ctx, source, target)
	if err != nil {
		diffStat = "(stat unavailable)"
	}

	prompt := changelog.GeneratePrompt(commits, diff, diffStat, note)

	fmt.Fprintf(os.Stderr, "Generating changelog (%d commits)...\n", len(commits))

	// Resolve and connect agent.
	ag, err := resolveChangelogAgent(cwd)
	if err != nil {
		return err
	}

	ag = ag.WithWorkDir(cwd).WithTimeout(5 * time.Minute)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := ag.Connect(ctx); err != nil {
		return fmt.Errorf("connect agent: %w", err)
	}
	defer func() { _ = ag.Close() }()

	events, err := ag.SendPrompt(ctx, prompt)
	if err != nil {
		return fmt.Errorf("send prompt: %w", err)
	}

	for event := range events {
		switch event.Type {
		case agent.EventStream:
			fmt.Print(event.Content)
		case agent.EventPermission:
			if event.PermissionRequest != nil {
				_ = ag.HandlePermission(event.PermissionRequest.ID, false)
			}
		case agent.EventComplete:
			fmt.Println()

			return nil
		case agent.EventError:
			return fmt.Errorf("agent error: %s", event.Error)
		case agent.EventAssistant, agent.EventToolUse, agent.EventToolResult,
			agent.EventInit, agent.EventKeepAlive, agent.EventSubagent, agent.EventProgress,
			agent.EventToolProgress, agent.EventInterrupted:
			// Not relevant; silently ignored.
		}
	}

	return nil
}

// gatherChangelogCommits collects commit info from the git repository.
func gatherChangelogCommits(ctx context.Context, repo *git.Repository, source, target string, full bool) ([]changelog.CommitInfo, error) {
	if full {
		raw, err := repo.CommitsBetweenFull(ctx, source, target)
		if err != nil {
			return nil, err
		}

		return changelog.GatherCommitsFull(raw), nil
	}

	raw, err := repo.CommitsBetween(ctx, source, target)
	if err != nil {
		return nil, err
	}

	return changelog.GatherCommits(raw), nil
}

// resolveChangelogAgent sets up an agent for standalone changelog generation.
func resolveChangelogAgent(cwd string) (agent.Agent, error) {
	effective, _, _, err := settings.LoadEffective(cwd)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	agentName := changelogAgent
	if agentName == "" {
		agentName = effective.Agent.Default
	}

	reg := agent.NewRegistry()
	if err := claude.RegisterWithPermissionHandler(reg, agent.KvelmoPermissionHandler); err != nil {
		slog.Debug("claude agent not available", "error", err)
	}
	if err := codex.RegisterWithPermissionHandler(reg, agent.KvelmoPermissionHandler); err != nil {
		slog.Debug("codex agent not available", "error", err)
	}

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

	if customCfg, ok := effective.CustomAgents[agentName]; ok {
		base, err := reg.Get(customCfg.Extends)
		if err != nil {
			return nil, fmt.Errorf("custom agent %q extends unknown base %q: %w", agentName, customCfg.Extends, err)
		}
		ag := base
		for k, v := range customCfg.Env {
			ag = ag.WithEnv(k, v)
		}
		if len(customCfg.Args) > 0 {
			ag = ag.WithArgs(customCfg.Args...)
		}

		return ag, nil
	}

	ag, err := reg.GetOrDetect(agentName)
	if err != nil {
		return nil, fmt.Errorf("resolve agent %q (available: %v): %w", agentName, reg.List(), err)
	}

	return ag, nil
}
