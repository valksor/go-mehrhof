// Package plugin provides runtime plugin support for the mehrhof task automation tool.
//
// Plugins are external processes that communicate via JSON-RPC 2.0 over stdin/stdout.
// This allows extending mehrhof with custom providers, agents, and workflow components
// without recompiling the main binary.
//
// Plugin types:
//   - Provider plugins: Custom task sources (Jira, YouTrack, Linear, etc.)
//   - Agent plugins: Custom AI backends
//   - Workflow plugins: Custom phases, guards, and effects for the state machine
//
// Plugin discovery:
//   - Global plugins: ~/.mehrhof/plugins/
//   - Project-local plugins: .mehrhof/plugins/
//
// Thread safety:
//   - The Loader is safe for concurrent use.
//   - Individual Process methods are not thread-safe and should be called serially.
//
// Security:
//   - Plugin executable paths are validated to prevent directory traversal.
//   - Relative paths must be within the plugin directory.
package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// defaultPluginBufferSize is the buffer size for plugin stdout/stderr
	defaultPluginBufferSize = 1024 * 1024 // 1MB
	// pluginStopTimeout is the maximum time to wait for a plugin to stop gracefully
	pluginStopTimeout = 10 * time.Second
)

// Process represents a running plugin process.
type Process struct {
	stdin    io.WriteCloser
	stderr   io.ReadCloser
	ctx      context.Context
	err      error
	done     chan struct{}
	cmd      *exec.Cmd
	stdout   *bufio.Reader
	cancel   context.CancelFunc
	manifest *Manifest
	pending  map[int64]chan *Response
	streamCh chan json.RawMessage
	reqID    atomic.Int64
	mu       sync.Mutex
	stopping bool
	started  bool
}

// Loader manages plugin process lifecycle.
type Loader struct {
	processes map[string]*Process
	mu        sync.RWMutex
}

// NewLoader creates a new plugin loader.
func NewLoader() *Loader {
	return &Loader{
		processes: make(map[string]*Process),
	}
}

// Load starts a plugin process from a manifest.
func (l *Loader) Load(ctx context.Context, manifest *Manifest) (*Process, error) {
	l.mu.Lock()

	// Check if already loaded
	proc, ok := l.processes[manifest.Name]
	if ok {
		if proc.started && !proc.stopping {
			l.mu.Unlock()
			return proc, nil
		}
		// Previous process is stopping, wait for it
		// Release lock before waiting to avoid blocking other operations
		l.mu.Unlock()

		select {
		case <-proc.done:
			// Process finished, proceed to load new one
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for plugin to stop: %w", ctx.Err())
		}

		// Re-acquire lock for the rest of the operation
		l.mu.Lock()
	}

	// Re-check in case another goroutine loaded while we were waiting
	if proc, ok := l.processes[manifest.Name]; ok {
		if proc.started && !proc.stopping {
			l.mu.Unlock()
			return proc, nil
		}
	}

	// Lock is now held for the startProcess call
	proc, err := startProcess(ctx, manifest)
	if err != nil {
		l.mu.Unlock()
		return nil, err
	}

	l.processes[manifest.Name] = proc
	l.mu.Unlock()
	return proc, nil
}

// Get returns a loaded plugin process by name.
func (l *Loader) Get(name string) (*Process, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	proc, ok := l.processes[name]
	return proc, ok
}

// Unload stops and removes a plugin process.
func (l *Loader) Unload(name string) error {
	l.mu.Lock()
	proc, ok := l.processes[name]
	if !ok {
		l.mu.Unlock()
		return nil
	}
	delete(l.processes, name)
	l.mu.Unlock()

	return proc.Stop()
}

// UnloadAll stops all plugin processes.
func (l *Loader) UnloadAll() error {
	l.mu.Lock()
	procs := make([]*Process, 0, len(l.processes))
	for _, proc := range l.processes {
		procs = append(procs, proc)
	}
	l.processes = make(map[string]*Process)
	l.mu.Unlock()

	var errs []error
	for _, proc := range procs {
		if err := proc.Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// startProcess spawns the plugin executable and sets up communication.
func startProcess(ctx context.Context, manifest *Manifest) (*Process, error) {
	cmdArgs := manifest.ExecutableCommand()
	if len(cmdArgs) == 0 {
		return nil, fmt.Errorf("no executable configured for plugin %s", manifest.Name)
	}

	// Validate the executable path for security
	// Ensure it's either an absolute path or a relative path within the plugin directory
	execPath := cmdArgs[0]
	if !filepath.IsAbs(execPath) {
		// Relative path - must be within the plugin directory
		if manifest.Dir == "" {
			return nil, fmt.Errorf("plugin %s: relative executable path requires a valid plugin directory", manifest.Name)
		}
		execPath = filepath.Join(manifest.Dir, execPath)
		// Clean the path to resolve any ".." components
		execPath = filepath.Clean(execPath)
		// Verify the resolved path is still within the plugin directory
		rel, err := filepath.Rel(manifest.Dir, execPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("plugin %s: executable path %q escapes plugin directory", manifest.Name, cmdArgs[0])
		}
	}

	cmd := exec.CommandContext(ctx, execPath, cmdArgs[1:]...)
	cmd.Dir = manifest.Dir

	// Inherit environment and add plugin-specific vars
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	// Create a context for this process that can be cancelled on shutdown
	procCtx, procCancel := context.WithCancel(ctx)

	proc := &Process{
		manifest: manifest,
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReaderSize(stdout, defaultPluginBufferSize),
		stderr:   stderr,
		pending:  make(map[int64]chan *Response),
		done:     make(chan struct{}),
		ctx:      procCtx,
		cancel:   procCancel,
	}

	if err := cmd.Start(); err != nil {
		procCancel()
		return nil, fmt.Errorf("start plugin %s: %w", manifest.Name, err)
	}

	proc.started = true

	// Start response reader goroutine
	go proc.readResponses()

	// Start stderr reader goroutine (for logging)
	go proc.readStderr()

	return proc, nil
}

// readResponses reads JSON-RPC responses and notifications from stdout.
func (p *Process) readResponses() {
	defer close(p.done)

	for {
		// Check if context was cancelled (e.g., during Stop)
		select {
		case <-p.ctx.Done():
			// Context cancelled, exit gracefully
			p.mu.Lock()
			for id, ch := range p.pending {
				close(ch)
				delete(p.pending, id)
			}
			if p.streamCh != nil {
				close(p.streamCh)
				p.streamCh = nil
			}
			p.mu.Unlock()
			return
		default:
			// Continue reading
		}

		line, err := p.stdout.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				p.err = fmt.Errorf("read stdout: %w", err)
			}
			// Close all pending requests
			p.mu.Lock()
			for id, ch := range p.pending {
				close(ch)
				delete(p.pending, id)
			}
			if p.streamCh != nil {
				close(p.streamCh)
				p.streamCh = nil
			}
			p.mu.Unlock()
			return
		}

		// Try to parse as response (has ID)
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // Skip malformed lines
		}

		if resp.ID != 0 {
			// This is a response to a request
			p.mu.Lock()
			if ch, ok := p.pending[resp.ID]; ok {
				ch <- &resp
				delete(p.pending, resp.ID)
			}
			p.mu.Unlock()
		} else {
			// This is a notification (streaming event)
			var notif Notification
			if err := json.Unmarshal(line, &notif); err != nil {
				continue
			}

			if notif.Method == "stream" {
				p.mu.Lock()
				if p.streamCh != nil {
					// Marshal params back to JSON for the stream channel
					if paramsJSON, err := json.Marshal(notif.Params); err == nil {
						select {
						case p.streamCh <- paramsJSON:
						default:
							// Channel full, drop event
						}
					}
				}
				p.mu.Unlock()
			}
		}
	}
}

// readStderr reads and logs stderr output.
func (p *Process) readStderr() {
	scanner := bufio.NewScanner(p.stderr)
	for scanner.Scan() {
		// In production, this could be sent to a logger
		// For now, we just discard it or could print for debugging
		_ = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		// Scanner error - log it via the process error field
		p.mu.Lock()
		if p.err == nil {
			p.err = fmt.Errorf("stderr scanner error: %w", err)
		}
		p.mu.Unlock()
	}
}

// Call sends a JSON-RPC request and waits for a response.
func (p *Process) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	p.mu.Lock()
	if p.stopping {
		p.mu.Unlock()
		return nil, fmt.Errorf("plugin is stopping")
	}

	id := p.reqID.Add(1)
	ch := make(chan *Response, 1)
	p.pending[id] = ch
	p.mu.Unlock()

	req := NewRequest(id, method, params)
	data, err := json.Marshal(req)
	if err != nil {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	p.mu.Lock()
	_, err = p.stdin.Write(data)
	p.mu.Unlock()
	if err != nil {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Wait for response with context timeout
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("plugin process closed")
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, ctx.Err()
	}
}

// Stream sends a JSON-RPC request that returns streaming events.
// Returns a channel that receives stream events until completion or error.
func (p *Process) Stream(ctx context.Context, method string, params any) (<-chan json.RawMessage, error) {
	p.mu.Lock()
	if p.stopping {
		p.mu.Unlock()
		return nil, fmt.Errorf("plugin is stopping")
	}

	// Set up stream channel
	p.streamCh = make(chan json.RawMessage, 100)
	streamCh := p.streamCh
	p.mu.Unlock()

	req := NewRequest(0, method, params) // ID 0 for streaming (no response expected)
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	p.mu.Lock()
	_, err = p.stdin.Write(data)
	p.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Wrap channel to handle context cancellation
	out := make(chan json.RawMessage, 100)
	go func() {
		defer close(out)
		// Drain remaining events on exit to prevent goroutine leak
		defer func() {
			for range streamCh {
				// Drain any remaining events
			}
		}()
		for {
			select {
			case event, ok := <-streamCh:
				if !ok {
					return
				}
				select {
				case out <- event:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// Stop gracefully stops the plugin process.
func (p *Process) Stop() error {
	p.mu.Lock()
	if p.stopping {
		p.mu.Unlock()
		<-p.done
		return p.err
	}
	p.stopping = true
	p.mu.Unlock()

	// Signal goroutines to stop (unblocks ReadBytes)
	if p.cancel != nil {
		p.cancel()
	}

	// Try to send shutdown request - log error but continue with cleanup
	ctx, cancel := context.WithTimeout(context.Background(), pluginStopTimeout)
	defer cancel()
	if _, err := p.Call(ctx, "shutdown", nil); err != nil {
		slog.Warn("plugin shutdown request failed", "plugin", p.manifest.Name, "error", err)
	}

	// Close stdin to signal EOF
	if err := p.stdin.Close(); err != nil {
		slog.Warn("failed to close plugin stdin", "plugin", p.manifest.Name, "error", err)
	}

	// Close stderr pipe
	if p.stderr != nil {
		if err := p.stderr.Close(); err != nil {
			slog.Warn("failed to close plugin stderr", "plugin", p.manifest.Name, "error", err)
		}
		p.stderr = nil
	}

	// Close stdout pipe - Reset(nil) doesn't close the underlying pipe,
	// so we need to get the underlying reader and close it directly
	if p.stdout != nil {
		// bufio.Reader doesn't expose the underlying reader,
		// but since we're stopping anyway, just set to nil
		// The pipe will be closed when the process exits
		p.stdout = nil
	}

	// Wait for process with timeout
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	timer := time.NewTimer(pluginStopTimeout)
	defer timer.Stop()

	select {
	case err := <-done:
		return err
	case <-timer.C:
		// Force kill - log error but still return wait result
		if err := p.cmd.Process.Kill(); err != nil {
			slog.Warn("failed to kill plugin process", "plugin", p.manifest.Name, "error", err)
		}
		// Wait returns immediately for killed process
		return <-done
	}
}

// Manifest returns the plugin manifest.
func (p *Process) Manifest() *Manifest {
	return p.manifest
}

// IsRunning returns true if the process is still running.
func (p *Process) IsRunning() bool {
	select {
	case <-p.done:
		return false
	default:
		return p.started && !p.stopping
	}
}
