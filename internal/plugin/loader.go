package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Process represents a running plugin process.
type Process struct {
	manifest *Manifest
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	stderr   io.ReadCloser

	mu       sync.Mutex
	reqID    atomic.Int64
	pending  map[int64]chan *Response
	streamCh chan json.RawMessage

	started  bool
	stopping bool
	done     chan struct{}
	err      error

	// ctx and cancel allow graceful shutdown of goroutines
	ctx    context.Context
	cancel context.CancelFunc
}

// Loader manages plugin process lifecycle.
type Loader struct {
	mu        sync.RWMutex
	processes map[string]*Process
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
		return fmt.Errorf("errors stopping plugins: %v", errs)
	}
	return nil
}

// startProcess spawns the plugin executable and sets up communication.
func startProcess(ctx context.Context, manifest *Manifest) (*Process, error) {
	cmdArgs := manifest.ExecutableCommand()
	if len(cmdArgs) == 0 {
		return nil, fmt.Errorf("no executable configured for plugin %s", manifest.Name)
	}

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
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
		stdout:   bufio.NewReaderSize(stdout, 1024*1024), // 1MB buffer
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

	// Try to send shutdown request (ignore errors)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = p.Call(ctx, "shutdown", nil)

	// Close stdin to signal EOF
	_ = p.stdin.Close()

	// Explicitly close stdout and stderr pipes
	if p.stdout != nil {
		p.stdout.Reset(nil) // Reset underlying reader to release resources
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
	}

	// Wait for process with timeout
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case err := <-done:
		return err
	case <-timer.C:
		// Force kill
		_ = p.cmd.Process.Kill()
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
