package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

// defaultTimeout is the standard timeout for CLI socket operations.
const defaultTimeout = 30 * time.Second

// worktreeClient returns a connected client to the worktree socket in the cwd.
func worktreeClient(timeout time.Duration) (*socket.Client, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	path := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(path) {
		return nil, errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(path, socket.WithTimeout(timeout))
	if err != nil {
		return nil, fmt.Errorf("connect to worktree socket: %w", err)
	}

	return client, nil
}

// globalClient returns a connected client to the global socket.
func globalClient(timeout time.Duration) (*socket.Client, error) {
	path := socket.GlobalSocketPath()
	if !socket.SocketExists(path) {
		return nil, errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(path, socket.WithTimeout(timeout))
	if err != nil {
		return nil, fmt.Errorf("connect to global socket: %w", err)
	}

	return client, nil
}

// callWorktree connects, calls the method, and closes the client.
func callWorktree(ctx context.Context, method string, params any) (*socket.Response, error) {
	client, err := worktreeClient(defaultTimeout)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	return client.Call(ctx, method, params)
}

// callGlobal connects, calls the method, and closes the client.
func callGlobal(ctx context.Context, method string, params any) (*socket.Response, error) {
	client, err := globalClient(defaultTimeout)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	return client.Call(ctx, method, params)
}

// outputJSON pretty-prints raw JSON bytes to stdout.
// Falls back to raw output if the data isn't valid JSON.
func outputJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Println(string(data))

		return nil //nolint:nilerr // Fallback to raw output on parse failure
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(string(data))

		return nil //nolint:nilerr // Fallback to raw output on marshal failure
	}
	fmt.Println(string(out))

	return nil
}
