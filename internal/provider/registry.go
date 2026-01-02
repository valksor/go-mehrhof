package provider

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// ProviderInfo describes a registered provider.
type ProviderInfo struct {
	Name         string
	Description  string
	Schemes      []string
	Capabilities CapabilitySet
	Priority     int // Higher priority = checked first for auto-detection
}

// Factory creates a provider instance.
type Factory func(ctx context.Context, cfg Config) (any, error)

type registeredProvider struct {
	info    ProviderInfo
	factory Factory
}

// Registry manages provider registration and lookup.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]registeredProvider
	schemes   map[string]string // scheme -> provider name
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]registeredProvider),
		schemes:   make(map[string]string),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(info ProviderInfo, factory Factory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[info.Name]; exists {
		return fmt.Errorf("provider %s already registered", info.Name)
	}

	r.providers[info.Name] = registeredProvider{
		info:    info,
		factory: factory,
	}

	// Register schemes
	for _, scheme := range info.Schemes {
		r.schemes[scheme] = info.Name
	}

	return nil
}

// Get returns provider info and factory by name.
func (r *Registry) Get(name string) (ProviderInfo, Factory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rp, ok := r.providers[name]
	if !ok {
		return ProviderInfo{}, nil, false
	}

	return rp.info, rp.factory, true
}

// GetByScheme returns provider info and factory by scheme.
func (r *Registry) GetByScheme(scheme string) (ProviderInfo, Factory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	name, ok := r.schemes[scheme]
	if !ok {
		return ProviderInfo{}, nil, false
	}

	return r.Get(name)
}

// List returns all registered providers sorted by priority (highest first).
func (r *Registry) List() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]ProviderInfo, 0, len(r.providers))
	for _, rp := range r.providers {
		infos = append(infos, rp.info)
	}

	// Sort by priority descending
	slices.SortFunc(infos, func(a, b ProviderInfo) int {
		return cmp.Compare(b.Priority, a.Priority)
	})

	return infos
}

// ResolveOptions configures reference resolution.
type ResolveOptions struct {
	DefaultProvider string // Fallback provider for bare references (without scheme)
}

// Resolve parses a reference with explicit scheme or default provider fallback.
// Resolution order:
//  1. Explicit scheme prefix (e.g., "file:task.md") - uses that provider
//  2. Default provider set in options - applies default scheme
//  3. Error with helpful message listing available schemes
func (r *Registry) Resolve(ctx context.Context, input string, cfg Config, opts ResolveOptions) (any, string, error) {
	scheme, identifier := parseScheme(input)

	if scheme != "" {
		return r.resolveWithScheme(ctx, scheme, identifier, cfg)
	}

	// No explicit scheme - try default provider
	if opts.DefaultProvider != "" {
		return r.resolveWithScheme(ctx, opts.DefaultProvider, input, cfg)
	}

	// No scheme and no default - return helpful error
	return nil, "", r.noSchemeError(input)
}

// parseScheme extracts scheme prefix from input.
// Returns ("file", "task.md") for "file:task.md"
// Returns ("", "task.md") for "task.md" (no scheme)
// Handles Windows paths like "C:\path" correctly (returns no scheme).
func parseScheme(input string) (string, string) {
	idx := strings.Index(input, ":")
	if idx == -1 {
		return "", input
	}
	// Check for Windows absolute path (e.g., "C:\path" or "C:/path")
	if idx == 1 && len(input) > 2 && (input[2] == '\\' || input[2] == '/') {
		return "", input
	}

	return input[:idx], input[idx+1:]
}

// resolveWithScheme creates provider instance and parses identifier.
func (r *Registry) resolveWithScheme(ctx context.Context, scheme, identifier string, cfg Config) (any, string, error) {
	info, factory, ok := r.GetByScheme(scheme)
	if !ok {
		return nil, "", fmt.Errorf("unknown provider scheme: %s\nAvailable schemes: %s",
			scheme, strings.Join(r.listSchemes(), ", "))
	}

	instance, err := factory(ctx, cfg)
	if err != nil {
		return nil, "", fmt.Errorf("create provider %s: %w", info.Name, err)
	}

	ident, ok := instance.(Identifier)
	if !ok {
		return nil, "", fmt.Errorf("provider %s does not support parsing", info.Name)
	}

	// Pass full input with scheme to Parse() for consistency
	id, err := ident.Parse(scheme + ":" + identifier)
	if err != nil {
		return nil, "", err
	}

	return instance, id, nil
}

// noSchemeError creates a helpful error message when no scheme is provided.
func (r *Registry) noSchemeError(input string) error {
	schemes := r.listSchemes()

	return fmt.Errorf(
		"no scheme provided for reference: %s\n\n"+
			"Use format 'scheme:identifier' where scheme is one of:\n"+
			"  %s\n\n"+
			"Examples:\n"+
			"  file:task.md     - Single markdown file\n"+
			"  dir:tasks/       - Directory of tasks\n\n"+
			"Or set a default provider in .mehrhof/config.yaml:\n"+
			"  providers:\n"+
			"      default: file",
		input, strings.Join(schemes, ", "))
}

// listSchemes returns all registered scheme names sorted alphabetically.
func (r *Registry) listSchemes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schemes := make([]string, 0, len(r.schemes))
	for scheme := range r.schemes {
		schemes = append(schemes, scheme)
	}
	slices.Sort(schemes)

	return schemes
}

// Create creates a provider instance by name.
func (r *Registry) Create(ctx context.Context, name string, cfg Config) (any, error) {
	_, factory, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}

	return factory(ctx, cfg)
}
