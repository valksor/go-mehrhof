package web

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valksor/kvelmo/pkg/socket"
)

// WorktreeCreatorClient implements WorktreeCreator using RPC to a global socket.
// Used by secondary instances to request worktree socket creation from the primary.
type WorktreeCreatorClient struct {
	globalSocketPath string
}

// NewWorktreeCreatorClient creates a client that communicates with the primary
// global socket to create worktree sockets on-demand.
func NewWorktreeCreatorClient(globalSocketPath string) *WorktreeCreatorClient {
	return &WorktreeCreatorClient{
		globalSocketPath: globalSocketPath,
	}
}

// GetOrCreateWorktreeSocket requests the primary instance to create a worktree socket.
func (c *WorktreeCreatorClient) GetOrCreateWorktreeSocket(projectPath string) (any, error) {
	client, err := socket.NewClient(c.globalSocketPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "worktrees.create", map[string]string{"path": projectPath})
	if err != nil {
		return nil, fmt.Errorf("create worktree socket: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("create worktree socket: %s", resp.Error.Message)
	}

	var result struct {
		SocketPath string `json:"socket_path"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result.SocketPath, nil
}
