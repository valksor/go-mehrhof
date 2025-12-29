package plugin

import (
	"context"
	"fmt"
	"sync"
)

// PluginInfo holds information about a registered plugin.
type PluginInfo struct {
	Manifest *Manifest
	Process  *Process
	Enabled  bool
}

// Registry manages all discovered and loaded plugins.
type Registry struct {
	discovery *Discovery
	loader    *Loader
	plugins   map[string]*PluginInfo
	enabled   map[string]bool
	config    map[string]map[string]any
	mu        sync.RWMutex
}

// NewRegistry creates a new plugin registry.
func NewRegistry(discovery *Discovery) *Registry {
	return &Registry{
		discovery: discovery,
		loader:    NewLoader(),
		plugins:   make(map[string]*PluginInfo),
		enabled:   make(map[string]bool),
		config:    make(map[string]map[string]any),
	}
}

// SetEnabled sets the list of explicitly enabled plugins.
// Only plugins in this list will be loaded.
func (r *Registry) SetEnabled(names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.enabled = make(map[string]bool, len(names))
	for _, name := range names {
		r.enabled[name] = true
	}
}

// SetConfig sets plugin-specific configuration.
func (r *Registry) SetConfig(config map[string]map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.config = config
}

// GetConfig returns configuration for a specific plugin.
func (r *Registry) GetConfig(name string) map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.config[name]
}

// DiscoverAndLoad discovers all plugins and loads enabled ones.
func (r *Registry) DiscoverAndLoad(ctx context.Context) error {
	manifests, err := r.discovery.Discover()
	if err != nil {
		return fmt.Errorf("discover plugins: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, manifest := range manifests {
		// Check if explicitly enabled
		enabled := r.enabled[manifest.Name]

		info := &PluginInfo{
			Manifest: manifest,
			Enabled:  enabled,
		}

		if enabled {
			// Load the plugin process
			proc, err := r.loader.Load(ctx, manifest)
			if err != nil {
				// Log error but continue with other plugins
				// In production, use proper logging
				continue
			}
			info.Process = proc

			// Initialize the plugin with config
			cfg := r.config[manifest.Name]
			if cfg == nil {
				cfg = make(map[string]any)
			}
			if _, err := proc.Call(ctx, initMethod(manifest.Type), &InitParams{Config: cfg}); err != nil {
				// Initialization failed, unload
				_ = proc.Stop()
				info.Process = nil
				continue
			}
		}

		r.plugins[manifest.Name] = info
	}

	return nil
}

// initMethod returns the init method name based on plugin type.
func initMethod(t PluginType) string {
	switch t {
	case PluginTypeProvider:
		return "provider.init"
	case PluginTypeAgent:
		return "agent.init"
	case PluginTypeWorkflow:
		return "workflow.init"
	default:
		return "init"
	}
}

// Get returns a plugin by name.
func (r *Registry) Get(name string) (*PluginInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.plugins[name]
	return info, ok
}

// GetProcess returns the running process for a plugin.
func (r *Registry) GetProcess(name string) (*Process, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.plugins[name]
	if !ok || info.Process == nil {
		return nil, false
	}
	return info.Process, true
}

// List returns all discovered plugins.
func (r *Registry) List() []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*PluginInfo, 0, len(r.plugins))
	for _, info := range r.plugins {
		result = append(result, info)
	}
	return result
}

// ListEnabled returns all enabled and loaded plugins.
func (r *Registry) ListEnabled() []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*PluginInfo
	for _, info := range r.plugins {
		if info.Enabled && info.Process != nil {
			result = append(result, info)
		}
	}
	return result
}

// ListByType returns plugins of a specific type.
func (r *Registry) ListByType(t PluginType) []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*PluginInfo
	for _, info := range r.plugins {
		if info.Manifest.Type == t {
			result = append(result, info)
		}
	}
	return result
}

// ListEnabledByType returns enabled plugins of a specific type.
func (r *Registry) ListEnabledByType(t PluginType) []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*PluginInfo
	for _, info := range r.plugins {
		if info.Manifest.Type == t && info.Enabled && info.Process != nil {
			result = append(result, info)
		}
	}
	return result
}

// Providers returns all enabled provider plugins.
func (r *Registry) Providers() []*PluginInfo {
	return r.ListEnabledByType(PluginTypeProvider)
}

// Agents returns all enabled agent plugins.
func (r *Registry) Agents() []*PluginInfo {
	return r.ListEnabledByType(PluginTypeAgent)
}

// Workflows returns all enabled workflow plugins.
func (r *Registry) Workflows() []*PluginInfo {
	return r.ListEnabledByType(PluginTypeWorkflow)
}

// Enable enables a specific plugin and loads it.
func (r *Registry) Enable(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if info.Enabled {
		return nil // Already enabled
	}

	// Load the plugin
	proc, err := r.loader.Load(ctx, info.Manifest)
	if err != nil {
		return fmt.Errorf("load plugin: %w", err)
	}

	// Initialize
	cfg := r.config[name]
	if cfg == nil {
		cfg = make(map[string]any)
	}
	if _, err := proc.Call(ctx, initMethod(info.Manifest.Type), &InitParams{Config: cfg}); err != nil {
		_ = proc.Stop()
		return fmt.Errorf("initialize plugin: %w", err)
	}

	info.Process = proc
	info.Enabled = true
	r.enabled[name] = true

	return nil
}

// Disable disables a specific plugin and unloads it.
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if !info.Enabled {
		return nil // Already disabled
	}

	if info.Process != nil {
		// Stop the process - errors are expected if already stopped
		_ = info.Process.Stop()
		info.Process = nil
	}

	info.Enabled = false
	delete(r.enabled, name)

	return nil
}

// Reload rediscovers and reloads all plugins.
func (r *Registry) Reload(ctx context.Context) error {
	// Stop all running plugins
	r.mu.Lock()
	for _, info := range r.plugins {
		if info.Process != nil {
			_ = info.Process.Stop()
		}
	}
	r.plugins = make(map[string]*PluginInfo)
	r.mu.Unlock()

	// Rediscover and load
	return r.DiscoverAndLoad(ctx)
}

// Shutdown stops all running plugins.
func (r *Registry) Shutdown() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, info := range r.plugins {
		if info.Process != nil {
			if err := info.Process.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("stop %s: %w", info.Manifest.Name, err))
			}
			info.Process = nil
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// Discovery returns the plugin discovery instance.
func (r *Registry) Discovery() *Discovery {
	return r.discovery
}
