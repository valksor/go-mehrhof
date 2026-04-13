package browser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
)

// ExecOptions configures browser command execution.
type ExecOptions struct {
	// WorktreePath is the path to the current worktree (for state isolation)
	WorktreePath string

	// SessionName is the playwright-cli session name (for session persistence)
	SessionName string

	// StateFile overrides the auto-generated state file path
	StateFile string

	// Env adds additional environment variables
	Env map[string]string
}

// Exec runs a playwright-cli command and returns the output.
func Exec(ctx context.Context, opts *ExecOptions, args ...string) ([]byte, error) {
	if err := EnsureInstalled(ctx); err != nil {
		return nil, fmt.Errorf("ensure installed: %w", err)
	}

	cmdArgs, env, cleanup, err := buildCommand(opts, args)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	slog.Debug("browser: executing command", "args", args)
	cmd := exec.CommandContext(ctx, BinaryPath(), cmdArgs...)
	cmd.Env = append(os.Environ(), env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Error("browser: command failed", "args", args, "error", err)

		return nil, fmt.Errorf("playwright-cli: %w\nstderr: %s", err, stderr.String())
	}

	slog.Debug("browser: command completed", "args", args)

	return stdout.Bytes(), nil
}

// ExecStream runs a playwright-cli command and returns a reader for streaming output.
func ExecStream(ctx context.Context, opts *ExecOptions, args ...string) (io.ReadCloser, error) {
	if err := EnsureInstalled(ctx); err != nil {
		return nil, fmt.Errorf("ensure installed: %w", err)
	}

	cmdArgs, env, cleanup, err := buildCommand(opts, args)
	if err != nil {
		cleanup()

		return nil, err
	}

	cmd := exec.CommandContext(ctx, BinaryPath(), cmdArgs...)
	cmd.Env = append(os.Environ(), env...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cleanup()

		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cleanup()

		return nil, fmt.Errorf("start: %w", err)
	}

	// Return a wrapper that cleans up when closed
	return &streamReader{
		ReadCloser: stdout,
		cmd:        cmd,
		cleanup:    cleanup,
	}, nil
}

type streamReader struct {
	io.ReadCloser

	cmd     *exec.Cmd
	cleanup func()
}

func (r *streamReader) Close() error {
	defer r.cleanup()
	_ = r.ReadCloser.Close()

	return r.cmd.Wait()
}

// ExecInteractive runs playwright-cli with stdin/stdout/stderr connected.
// Use this for commands that require user interaction.
func ExecInteractive(ctx context.Context, opts *ExecOptions, args ...string) error {
	if err := EnsureInstalled(ctx); err != nil {
		return fmt.Errorf("ensure installed: %w", err)
	}

	cmdArgs, env, cleanup, err := buildCommand(opts, args)
	if err != nil {
		return err
	}
	defer cleanup()

	cmd := exec.CommandContext(ctx, BinaryPath(), cmdArgs...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// buildCommand prepares the command arguments and environment.
// Returns cleanup function to call after execution.
//
//nolint:nonamedreturns // Named returns document the multiple return values
func buildCommand(opts *ExecOptions, args []string) (cmdArgs []string, env []string, cleanup func(), err error) {
	cleanup = func() {} // no-op by default

	cfg, err := LoadConfig()
	if err != nil {
		return nil, nil, cleanup, fmt.Errorf("load config: %w", err)
	}

	// Write playwright config
	if err := cfg.WritePlaywrightConfig(); err != nil {
		return nil, nil, cleanup, fmt.Errorf("write playwright config: %w", err)
	}

	// Build command arguments
	cmdArgs = make([]string, 0, len(args)+4)

	// Add session name if specified (for persistent sessions)
	if opts != nil && opts.SessionName != "" {
		cmdArgs = append(cmdArgs, "-s="+opts.SessionName)
	}

	// Add config file
	cmdArgs = append(cmdArgs, "--config="+PlaywrightConfigPath())

	// Handle state file
	var stateFile string
	if opts != nil && opts.StateFile != "" {
		stateFile = opts.StateFile
	} else if opts != nil && opts.WorktreePath != "" {
		// Create merged state for worktree
		wtHash := WorktreeHash(opts.WorktreePath)
		merged, err := MergeState(cfg.Profile, wtHash)
		if err != nil {
			return nil, nil, cleanup, fmt.Errorf("merge state: %w", err)
		}
		stateFile = merged

		// Set cleanup to extract worktree state after execution
		cleanup = func() {
			// Best effort - don't fail on cleanup errors
			_ = ExtractWorktreeState(merged, cfg.Profile, wtHash)
			_ = os.Remove(merged)
		}
	}

	// Add user command arguments
	cmdArgs = append(cmdArgs, args...)

	// Build environment
	env = make([]string, 0)

	// Set state file via environment if specified
	if stateFile != "" {
		env = append(env, "PLAYWRIGHT_CLI_STATE_FILE="+stateFile)
	}

	// Set headless mode
	if cfg.Headless {
		env = append(env, "PLAYWRIGHT_CLI_HEADLESS=true")
	}

	// Add custom env vars
	if opts != nil && opts.Env != nil {
		for k, v := range opts.Env {
			env = append(env, k+"="+v)
		}
	}

	return cmdArgs, env, cleanup, nil
}

// Run is a convenience function for running a browser command in the current directory.
func Run(ctx context.Context, args ...string) ([]byte, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get cwd: %w", err)
	}

	return Exec(ctx, &ExecOptions{WorktreePath: cwd}, args...)
}

// RunInteractive is a convenience function for running an interactive browser command.
func RunInteractive(ctx context.Context, args ...string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	return ExecInteractive(ctx, &ExecOptions{WorktreePath: cwd}, args...)
}
