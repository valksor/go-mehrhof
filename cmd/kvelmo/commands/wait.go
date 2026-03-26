package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/valksor/kvelmo/pkg/cli"
	"github.com/valksor/kvelmo/pkg/conductor"
	"github.com/valksor/kvelmo/pkg/socket"
)

// waitForJob connects to the worktree socket, subscribes to the event stream,
// and blocks until the specified job completes or fails.
// Returns nil on success, error on failure.
func waitForJob(socketPath, jobID string) error {
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect for streaming: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Send stream.subscribe request.
	req := socket.Request{
		JSONRPC: "2.0",
		ID:      "wait-1",
		Method:  "stream.subscribe",
		Params:  json.RawMessage(`{"last_seq":0}`),
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	reqBytes = append(reqBytes, '\n')
	if _, err := conn.Write(reqBytes); err != nil {
		return fmt.Errorf("send subscribe: %w", err)
	}

	// Read initial JSON-RPC response.
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if scanErr := scanner.Err(); scanErr != nil {
			return fmt.Errorf("read response: %w", scanErr)
		}

		return errors.New("connection closed before response")
	}

	var resp socket.Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("subscribe failed: %s", resp.Error.Message)
	}

	// Handle Ctrl+C: exit cleanly without stopping the running job.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		_ = conn.Close()
	}()

	// Stream events until job completes or fails.
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event conductor.ConductorEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		// Filter events by job ID when specified.
		if jobID != "" && event.JobID != "" && event.JobID != jobID {
			continue
		}

		switch event.Type {
		case "job_output":
			fmt.Print(event.Message)
		case "state_changed":
			fmt.Printf("\n[State] %s\n", event.Message)
		case "job_completed":
			fmt.Fprint(os.Stderr, "\a") // Terminal bell on completion

			return nil
		case "job_failed":
			fmt.Fprint(os.Stderr, "\a") // Terminal bell on failure
			_, _ = cli.Red.Fprintf(os.Stderr, "\n[Failed] %s\n", event.Error)

			return fmt.Errorf("job failed: %s", event.Error)
		case "error":
			_, _ = cli.Red.Fprintf(os.Stderr, "\n[Error] %s\n", event.Error)
			if event.Message != "" {
				fmt.Fprintf(os.Stderr, "  %s\n", event.Message)
			}
		case "heartbeat":
			// Keepalive, ignore.
		}
	}

	if err := scanner.Err(); err != nil {
		// Closed by signal handler — this is expected.
		if isClosedConnErr(err) {
			return nil
		}

		return fmt.Errorf("read stream: %w", err)
	}

	return nil
}
