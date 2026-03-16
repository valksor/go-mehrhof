package commands

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/conductor"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var (
	watchJSON     bool
	watchProgress bool
)

// WatchCmd streams live output from a running task to the terminal.
var WatchCmd = &cobra.Command{
	Use:     "watch",
	Aliases: []string{"w"},
	Short:   "Stream live task output to the terminal",
	Long: fmt.Sprintf(`Stream live events from a running task to the terminal.

Connects to the worktree socket and subscribes to the event stream,
displaying agent output, state changes, and errors in real time.

  %[1]s watch          # formatted output
  %[1]s watch --json   # raw JSON events (NDJSON)

Press Ctrl+C to stop watching without affecting the running task.`, meta.Name),
	RunE: runWatch,
}

func init() {
	WatchCmd.Flags().BoolVar(&watchJSON, "json", false, "Output raw JSON events (NDJSON)")
	WatchCmd.Flags().BoolVar(&watchProgress, "progress", false, "Show progress summary instead of full output")
}

func runWatch(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		return fmt.Errorf("no worktree socket running for %s\nRun '%s start' first", cwd, meta.Name)
	}

	// Connect directly to the unix socket so we can read the streaming
	// NDJSON events that stream.subscribe writes after the initial response.
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(cmd.Context(), "unix", wtPath)
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Send JSON-RPC request for stream.subscribe.
	req := socket.Request{
		JSONRPC: "2.0",
		ID:      "watch-1",
		Method:  "stream.subscribe",
		Params:  json.RawMessage(`{"last_seq":0}`),
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	reqBytes = append(reqBytes, '\n')
	if _, err := conn.Write(reqBytes); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	// Read the initial JSON-RPC response.
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
	go func() {
		<-sigCh
		_ = conn.Close()
	}()

	// Read NDJSON event stream.
	var outputLines int

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		if watchJSON {
			fmt.Println(string(line))

			continue
		}

		var event conductor.ConductorEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		if watchProgress {
			switch event.Type {
			case "job_output":
				outputLines++
				if outputLines%50 == 0 {
					fmt.Printf("\r  ▸ %d lines of output...", outputLines)
				}
			case "state_changed":
				fmt.Printf("\n● %s\n", event.Message)
			case "progress":
				if event.Message != "" {
					fmt.Printf("\r  ▸ %s", event.Message)
				}
			case "job_failed":
				fmt.Fprintf(os.Stderr, "\n\033[31m[Failed] %s\033[0m\n", event.Error)

				return fmt.Errorf("job failed: %s", event.Error)
			case "error":
				fmt.Fprintf(os.Stderr, "\n\033[31m[Error] %s\033[0m\n", event.Error)
			case "heartbeat":
				// Keepalive, ignore.
			}

			continue
		}

		switch event.Type {
		case "job_output":
			fmt.Print(event.Message)
		case "state_changed":
			fmt.Printf("\n[State] %s\n", event.Message)
		case "job_failed":
			fmt.Fprintf(os.Stderr, "\n\033[31m[Failed] %s\033[0m\n", event.Error)

			return fmt.Errorf("job failed: %s", event.Error)
		case "error":
			fmt.Fprintf(os.Stderr, "\n\033[31m[Error] %s\033[0m\n", event.Error)
			if event.Message != "" {
				fmt.Fprintf(os.Stderr, "  %s\n", event.Message)
			}
		case "heartbeat":
			// Keepalive, ignore.
		default:
			// Other event types: show if they have a message.
			if event.Message != "" {
				fmt.Printf("[%s] %s\n", event.Type, event.Message)
			}
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

// isClosedConnErr returns true if the error indicates the connection was
// closed, which is expected when the user presses Ctrl+C.
func isClosedConnErr(err error) bool {
	if err == nil {
		return false
	}
	// net.ErrClosed or "use of closed network connection"
	opErr := &net.OpError{}
	if errors.As(err, &opErr) {
		return opErr.Err.Error() == "use of closed network connection"
	}

	return false
}
