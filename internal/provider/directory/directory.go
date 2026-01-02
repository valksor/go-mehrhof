package directory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/provider/file"
)

// ProviderName is the registered name for this provider.
const ProviderName = "directory"

// Provider handles directory-based tasks.
type Provider struct {
	basePath string
}

// Info returns provider metadata.
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Local directory task source",
		Schemes:     []string{"dir"},
		Priority:    15, // Higher than file provider
		Capabilities: provider.CapabilitySet{
			provider.CapRead: true,
			provider.CapList: true,
		},
	}
}

// New creates a directory provider.
func New(ctx context.Context, cfg provider.Config) (any, error) {
	basePath := cfg.GetString("base_path")
	if basePath == "" {
		basePath = "."
	}
	return &Provider{basePath: basePath}, nil
}

// Match checks if input has the dir: scheme prefix.
func (p *Provider) Match(input string) bool {
	return strings.HasPrefix(input, "dir:")
}

// Parse extracts the directory path from input.
func (p *Provider) Parse(input string) (string, error) {
	// Remove dir: prefix if present
	path := strings.TrimPrefix(input, "dir:")
	path = strings.TrimSuffix(path, "/")

	// Resolve to absolute path
	resolved := p.resolvePath(path)

	// Verify directory exists
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("directory not found: %s", resolved)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", resolved)
	}

	return resolved, nil
}

// Fetch reads the directory and creates a WorkUnit.
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	// Look for README or similar
	readmePath, title, description, frontmatter := p.findReadme(id)

	if title == "" {
		title = filepath.Base(id)
	}

	info, _ := os.Stat(id)
	modTime := time.Now()
	if info != nil {
		modTime = info.ModTime()
	}

	// Extract naming info from directory name
	dirName := filepath.Base(id)
	externalKey := naming.KeyFromDirectory(id)
	taskType := naming.TaskTypeFromFilename(dirName) // Works for directory names too

	wu := &provider.WorkUnit{
		ID:          p.generateID(id),
		ExternalID:  id,
		Provider:    ProviderName,
		Title:       title,
		Description: description,
		Status:      provider.StatusOpen,
		Priority:    provider.PriorityNormal,
		Labels:      []string{},
		CreatedAt:   modTime,
		UpdatedAt:   modTime,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: id,
			SyncedAt:  time.Now(),
		},
		Metadata: map[string]any{
			"readme_path": readmePath,
		},
		// Naming fields
		ExternalKey: externalKey,
		TaskType:    taskType,
		Slug:        naming.Slugify(title, 50),
	}

	// Apply frontmatter overrides from README if present
	if frontmatter != nil {
		if frontmatter.Key != "" {
			wu.ExternalKey = frontmatter.Key
		}
		if frontmatter.Type != "" {
			wu.TaskType = frontmatter.Type
		}
		// Agent configuration from frontmatter
		if frontmatter.Agent != "" || len(frontmatter.AgentEnv) > 0 || len(frontmatter.AgentSteps) > 0 {
			wu.AgentConfig = &provider.AgentConfig{
				Name: frontmatter.Agent,
				Env:  frontmatter.AgentEnv,
			}
			// Map per-step agent configuration
			if len(frontmatter.AgentSteps) > 0 {
				wu.AgentConfig.Steps = make(map[string]provider.StepAgentConfig)
				for step, stepCfg := range frontmatter.AgentSteps {
					wu.AgentConfig.Steps[step] = provider.StepAgentConfig{
						Name: stepCfg.Agent,
						Env:  stepCfg.Env,
					}
				}
			}
		}
	}

	// Find subtasks (other files in directory)
	subtasks := p.findSubtasks(id)
	wu.Subtasks = subtasks

	return wu, nil
}

// List returns all files in directory as WorkUnits.
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	var units []*provider.WorkUnit

	entries, err := os.ReadDir(p.basePath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	// Files to skip when listing
	skipFiles := []string{"readme.md", "task.md", "index.md"}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip README-like files
		if slices.Contains(skipFiles, strings.ToLower(name)) {
			continue
		}

		path := filepath.Join(p.basePath, name)
		wu, err := p.fetchFile(ctx, path)
		if err != nil {
			continue // Skip files that fail to parse
		}
		units = append(units, wu)
	}

	return units, nil
}

func (p *Provider) fetchFile(ctx context.Context, path string) (*provider.WorkUnit, error) {
	// Extract filename without extension for fallback title
	filename := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	parsed, err := file.ParseMarkdownFile(path, filename)
	if err != nil {
		return nil, err
	}

	info, _ := os.Stat(path)
	modTime := time.Now()
	if info != nil {
		modTime = info.ModTime()
	}

	return &provider.WorkUnit{
		ID:          p.generateID(path),
		ExternalID:  path,
		Provider:    ProviderName,
		Title:       parsed.Title,
		Description: parsed.Body,
		Status:      provider.StatusOpen,
		Priority:    provider.PriorityNormal,
		Labels:      []string{},
		CreatedAt:   modTime,
		UpdatedAt:   modTime,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: path,
			SyncedAt:  time.Now(),
		},
	}, nil
}

func (p *Provider) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(p.basePath, path)
}

func (p *Provider) generateID(path string) string {
	return filepath.Base(path)
}

func (p *Provider) findReadme(dir string) (path, title, description string, frontmatter *file.Frontmatter) {
	// Check for README files in order of preference
	candidates := []string{
		"README.md",
		"readme.md",
		"Readme.md",
		"TASK.md",
		"task.md",
		"index.md",
	}

	for _, name := range candidates {
		checkPath := filepath.Join(dir, name)
		if _, err := os.Stat(checkPath); err == nil {
			// Use filename without extension as fallback
			fallbackTitle := strings.TrimSuffix(name, filepath.Ext(name))
			parsed, err := file.ParseMarkdownFile(checkPath, fallbackTitle)
			if err == nil {
				return checkPath, parsed.Title, parsed.Body, parsed.Frontmatter
			}
		}
	}

	return "", "", "", nil
}

func (p *Provider) findSubtasks(dir string) []string {
	var subtasks []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return subtasks
	}

	// Files to skip when finding subtasks
	skipFiles := []string{"readme.md", "task.md", "index.md"}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip README-like files
		if slices.Contains(skipFiles, strings.ToLower(name)) {
			continue
		}

		subtasks = append(subtasks, filepath.Join(dir, name))
	}

	return subtasks
}

// Snapshot captures all files in the directory for storage.
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	snapshot := &provider.Snapshot{
		Type:  "directory",
		Ref:   id,
		Files: []provider.SnapshotFile{},
	}

	// Walk directory and capture all markdown files
	err := filepath.WalkDir(id, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only capture markdown and text files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".txt" && ext != ".yaml" && ext != ".yml" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			//nolint:nilerr // Skip unreadable files in WalkDir
			return nil // Skip files we can't read
		}

		// Store relative path
		relPath, _ := filepath.Rel(id, path)
		snapshot.Files = append(snapshot.Files, provider.SnapshotFile{
			Path:    relPath,
			Content: string(content),
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return snapshot, nil
}

// Register adds directory provider to registry.
func Register(r *provider.Registry) {
	_ = r.Register(Info(), New)
}
