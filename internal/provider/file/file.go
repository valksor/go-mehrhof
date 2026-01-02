package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the registered name for this provider.
const ProviderName = "file"

// Provider handles markdown file tasks.
type Provider struct {
	basePath string
}

// Info returns provider metadata.
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Local file task source",
		Schemes:     []string{"file"},
		Priority:    10,
		Capabilities: provider.CapabilitySet{
			provider.CapRead: true,
		},
	}
}

// New creates a file provider.
func New(ctx context.Context, cfg provider.Config) (any, error) {
	basePath := cfg.GetString("base_path")
	if basePath == "" {
		basePath = "."
	}
	return &Provider{basePath: basePath}, nil
}

// Match checks if input has the file: scheme prefix.
func (p *Provider) Match(input string) bool {
	return strings.HasPrefix(input, "file:")
}

// Parse extracts the file path from input.
func (p *Provider) Parse(input string) (string, error) {
	// Remove file: prefix if present
	path := strings.TrimPrefix(input, "file:")

	// Resolve to absolute path
	resolved := p.resolvePath(path)

	// Verify file exists
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", resolved)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", resolved)
	}

	return resolved, nil
}

// Fetch reads the file and creates a WorkUnit.
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	// Extract filename without extension for fallback title
	filename := strings.TrimSuffix(filepath.Base(id), filepath.Ext(id))

	parsed, err := ParseMarkdownFile(id, filename)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	info, _ := os.Stat(id)
	modTime := time.Now()
	if info != nil {
		modTime = info.ModTime()
	}

	// Extract naming info from filename
	externalKey := naming.KeyFromFilename(filename)
	taskType := naming.TaskTypeFromFilename(filename)

	wu := &provider.WorkUnit{
		ID:          p.generateID(id),
		ExternalID:  id,
		Provider:    ProviderName,
		Title:       parsed.Title,
		Description: parsed.Body,
		Status:      provider.StatusOpen,
		Priority:    provider.PriorityNormal,
		Labels:      []string{},
		Metadata:    make(map[string]any),
		CreatedAt:   modTime,
		UpdatedAt:   modTime,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: id,
			SyncedAt:  time.Now(),
		},
		// Naming fields
		ExternalKey: externalKey,
		TaskType:    taskType,
		Slug:        naming.Slugify(parsed.Title, 50),
	}

	// Apply frontmatter metadata if present
	if parsed.Frontmatter != nil {
		if parsed.Frontmatter.Priority != "" {
			wu.Priority = parsePriority(parsed.Frontmatter.Priority)
		}
		if len(parsed.Frontmatter.Labels) > 0 {
			wu.Labels = parsed.Frontmatter.Labels
		}
		// Frontmatter overrides filename-derived naming
		if parsed.Frontmatter.Key != "" {
			wu.ExternalKey = parsed.Frontmatter.Key
		}
		if parsed.Frontmatter.Type != "" {
			wu.TaskType = parsed.Frontmatter.Type
		}
		if parsed.Frontmatter.Slug != "" {
			wu.Slug = parsed.Frontmatter.Slug
		}
		// Agent configuration from frontmatter
		if parsed.Frontmatter.Agent != "" || len(parsed.Frontmatter.AgentEnv) > 0 || len(parsed.Frontmatter.AgentArgs) > 0 || len(parsed.Frontmatter.AgentSteps) > 0 {
			wu.AgentConfig = &provider.AgentConfig{
				Name: parsed.Frontmatter.Agent,
				Env:  parsed.Frontmatter.AgentEnv,
				Args: parsed.Frontmatter.AgentArgs,
			}
			// Map per-step agent configuration
			if len(parsed.Frontmatter.AgentSteps) > 0 {
				wu.AgentConfig.Steps = make(map[string]provider.StepAgentConfig)
				for step, stepCfg := range parsed.Frontmatter.AgentSteps {
					wu.AgentConfig.Steps[step] = provider.StepAgentConfig{
						Name: stepCfg.Agent,
						Env:  stepCfg.Env,
						Args: stepCfg.Args,
					}
				}
			}
		}
	}

	return wu, nil
}

func (p *Provider) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(p.basePath, path)
}

func (p *Provider) generateID(path string) string {
	// Use filename without extension as ID
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func parsePriority(s string) provider.Priority {
	switch strings.ToLower(s) {
	case "critical", "urgent":
		return provider.PriorityCritical
	case "high":
		return provider.PriorityHigh
	case "low":
		return provider.PriorityLow
	default:
		return provider.PriorityNormal
	}
}

// Snapshot captures the file content for storage.
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	content, err := os.ReadFile(id)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return &provider.Snapshot{
		Type:    "file",
		Ref:     id,
		Content: string(content),
	}, nil
}

// Register adds file provider to registry.
func Register(r *provider.Registry) {
	_ = r.Register(Info(), New)
}
