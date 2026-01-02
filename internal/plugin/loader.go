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
	"context"
	"errors"
	"fmt"
	"sync"
)

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
func (l *Loader) Unload(ctx context.Context, name string) error {
	l.mu.Lock()
	proc, ok := l.processes[name]
	if !ok {
		l.mu.Unlock()

		return nil
	}
	delete(l.processes, name)
	l.mu.Unlock()

	return proc.Stop(ctx)
}

// UnloadAll stops all plugin processes.
func (l *Loader) UnloadAll(ctx context.Context) error {
	l.mu.Lock()
	procs := make([]*Process, 0, len(l.processes))
	for _, proc := range l.processes {
		procs = append(procs, proc)
	}
	l.processes = make(map[string]*Process)
	l.mu.Unlock()

	var errs []error
	for _, proc := range procs {
		if err := proc.Stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
