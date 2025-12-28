package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/go-mehrhof/internal/provider"
)

func TestInfo(t *testing.T) {
	info := Info()

	if info.Name != ProviderName {
		t.Errorf("Name = %q, want %q", info.Name, ProviderName)
	}
	if info.Description == "" {
		t.Error("Description should not be empty")
	}
	if len(info.Schemes) == 0 {
		t.Error("Schemes should not be empty")
	}
	if info.Schemes[0] != "file" {
		t.Errorf("Schemes[0] = %q, want %q", info.Schemes[0], "file")
	}
	if !info.Capabilities[provider.CapRead] {
		t.Error("should have read capability")
	}
}

func TestNew(t *testing.T) {
	ctx := context.Background()
	cfg := provider.Config{}

	p, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	prov, ok := p.(*Provider)
	if !ok {
		t.Fatal("New should return *Provider")
	}

	if prov.basePath != "." {
		t.Errorf("basePath = %q, want %q", prov.basePath, ".")
	}
}

func TestNewWithBasePath(t *testing.T) {
	ctx := context.Background()
	cfg := provider.NewConfig().Set("base_path", "/custom/path")

	p, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	prov := p.(*Provider)
	if prov.basePath != "/custom/path" {
		t.Errorf("basePath = %q, want %q", prov.basePath, "/custom/path")
	}
}

func TestMatchWithPrefix(t *testing.T) {
	p := &Provider{basePath: "."}

	if !p.Match("file:task.md") {
		t.Error("should match file: prefix")
	}
	if !p.Match("file:/absolute/path.md") {
		t.Error("should match file: with absolute path")
	}
}

func TestMatchExplicitOnly(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# Task"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	// Only explicit prefix should match
	if !p.Match("file:task.md") {
		t.Error("should match file: prefix")
	}

	// Bare paths should NOT match (explicit scheme required)
	if p.Match("task.md") {
		t.Error("should NOT match bare file path (explicit scheme required)")
	}
	if p.Match("nonexistent.md") {
		t.Error("should NOT match bare non-existent file")
	}
}

func TestMatchDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	if p.Match("subdir") {
		t.Error("should not match directory")
	}
}

func TestParse(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# Task"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	path, err := p.Parse("task.md")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if path != taskFile {
		t.Errorf("Parse = %q, want %q", path, taskFile)
	}
}

func TestParseWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "task.md")
	if err := os.WriteFile(taskFile, []byte("# Task"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	path, err := p.Parse("file:task.md")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if path != taskFile {
		t.Errorf("Parse = %q, want %q", path, taskFile)
	}
}

func TestParseNotFound(t *testing.T) {
	p := &Provider{basePath: t.TempDir()}

	_, err := p.Parse("nonexistent.md")
	if err == nil {
		t.Error("Parse should fail for non-existent file")
	}
}

func TestParseDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	p := &Provider{basePath: tmpDir}

	_, err := p.Parse("subdir")
	if err == nil {
		t.Error("Parse should fail for directory")
	}
}

func TestFetch(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "my-task.md")
	content := `---
title: Test Task
priority: high
labels:
  - bug
  - urgent
---

This is the task description.
`
	if err := os.WriteFile(taskFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	wu, err := p.Fetch(ctx, taskFile)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if wu.ID != "my-task" {
		t.Errorf("ID = %q, want %q", wu.ID, "my-task")
	}
	if wu.Title != "Test Task" {
		t.Errorf("Title = %q, want %q", wu.Title, "Test Task")
	}
	if wu.Provider != ProviderName {
		t.Errorf("Provider = %q, want %q", wu.Provider, ProviderName)
	}
	if wu.Priority != provider.PriorityHigh {
		t.Errorf("Priority = %d, want %d (high)", wu.Priority, provider.PriorityHigh)
	}
	if len(wu.Labels) != 2 {
		t.Errorf("Labels = %v, want 2 labels", wu.Labels)
	}
}

func TestFetchNoFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "simple.md")
	content := `# Simple Task

Just a simple task without frontmatter.
`
	if err := os.WriteFile(taskFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	wu, err := p.Fetch(ctx, taskFile)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if wu.Title != "Simple Task" {
		t.Errorf("Title = %q, want %q", wu.Title, "Simple Task")
	}
	if wu.Priority != provider.PriorityNormal {
		t.Errorf("Priority should be normal without frontmatter")
	}
}

func TestSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "snapshot.md")
	content := "# Snapshot Test\n\nContent here."
	if err := os.WriteFile(taskFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	snap, err := p.Snapshot(ctx, taskFile)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snap.Type != "file" {
		t.Errorf("Type = %q, want %q", snap.Type, "file")
	}
	if snap.Ref != taskFile {
		t.Errorf("Ref = %q, want %q", snap.Ref, taskFile)
	}
	if snap.Content != content {
		t.Errorf("Content mismatch")
	}
}

func TestSnapshotNotFound(t *testing.T) {
	p := &Provider{basePath: t.TempDir()}
	ctx := context.Background()

	_, err := p.Snapshot(ctx, "/nonexistent/file.md")
	if err == nil {
		t.Error("Snapshot should fail for non-existent file")
	}
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input string
		want  provider.Priority
	}{
		{"critical", provider.PriorityCritical},
		{"CRITICAL", provider.PriorityCritical},
		{"urgent", provider.PriorityCritical},
		{"high", provider.PriorityHigh},
		{"HIGH", provider.PriorityHigh},
		{"low", provider.PriorityLow},
		{"LOW", provider.PriorityLow},
		{"normal", provider.PriorityNormal},
		{"medium", provider.PriorityNormal},
		{"unknown", provider.PriorityNormal},
		{"", provider.PriorityNormal},
	}

	for _, tt := range tests {
		got := parsePriority(tt.input)
		if got != tt.want {
			t.Errorf("parsePriority(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestGenerateID(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		path string
		want string
	}{
		{"/path/to/task.md", "task"},
		{"simple.md", "simple"},
		{"/complex/path/my-feature.md", "my-feature"},
		{"no-extension", "no-extension"},
	}

	for _, tt := range tests {
		got := p.generateID(tt.path)
		if got != tt.want {
			t.Errorf("generateID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestResolvePath(t *testing.T) {
	p := &Provider{basePath: "/base"}

	// Absolute path should be unchanged
	if got := p.resolvePath("/absolute/path.md"); got != "/absolute/path.md" {
		t.Errorf("resolvePath for absolute = %q, want %q", got, "/absolute/path.md")
	}

	// Relative path should be joined with basePath
	if got := p.resolvePath("relative.md"); got != "/base/relative.md" {
		t.Errorf("resolvePath for relative = %q, want %q", got, "/base/relative.md")
	}
}

func TestProviderNameConstant(t *testing.T) {
	if ProviderName != "file" {
		t.Errorf("ProviderName = %q, want %q", ProviderName, "file")
	}
}

func TestFetchWithAgentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "agent-task.md")
	content := `---
title: Task with Agent
agent: glm
---

Task description here.
`
	if err := os.WriteFile(taskFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	wu, err := p.Fetch(ctx, taskFile)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if wu.AgentConfig == nil {
		t.Fatal("AgentConfig should not be nil when agent is specified in frontmatter")
	}
	if wu.AgentConfig.Name != "glm" {
		t.Errorf("AgentConfig.Name = %q, want %q", wu.AgentConfig.Name, "glm")
	}
}

func TestFetchWithAgentEnv(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "agent-env-task.md")
	content := `---
title: Task with Agent Env
agent: claude
agent_env:
  ANTHROPIC_API_KEY: "${CUSTOM_KEY}"
  MAX_TOKENS: "8192"
---

Task description here.
`
	if err := os.WriteFile(taskFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	wu, err := p.Fetch(ctx, taskFile)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if wu.AgentConfig == nil {
		t.Fatal("AgentConfig should not be nil when agent_env is specified in frontmatter")
	}
	if wu.AgentConfig.Name != "claude" {
		t.Errorf("AgentConfig.Name = %q, want %q", wu.AgentConfig.Name, "claude")
	}
	if len(wu.AgentConfig.Env) != 2 {
		t.Errorf("AgentConfig.Env should have 2 entries, got %d", len(wu.AgentConfig.Env))
	}
	if wu.AgentConfig.Env["ANTHROPIC_API_KEY"] != "${CUSTOM_KEY}" {
		t.Errorf("AgentConfig.Env[ANTHROPIC_API_KEY] = %q, want %q", wu.AgentConfig.Env["ANTHROPIC_API_KEY"], "${CUSTOM_KEY}")
	}
	if wu.AgentConfig.Env["MAX_TOKENS"] != "8192" {
		t.Errorf("AgentConfig.Env[MAX_TOKENS] = %q, want %q", wu.AgentConfig.Env["MAX_TOKENS"], "8192")
	}
}

func TestFetchWithAgentEnvOnly(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "env-only-task.md")
	content := `---
title: Task with Agent Env Only
agent_env:
  CUSTOM_VAR: "value"
---

Task description here.
`
	if err := os.WriteFile(taskFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	wu, err := p.Fetch(ctx, taskFile)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if wu.AgentConfig == nil {
		t.Fatal("AgentConfig should not be nil when agent_env is specified in frontmatter")
	}
	if wu.AgentConfig.Name != "" {
		t.Errorf("AgentConfig.Name should be empty, got %q", wu.AgentConfig.Name)
	}
	if len(wu.AgentConfig.Env) != 1 {
		t.Errorf("AgentConfig.Env should have 1 entry, got %d", len(wu.AgentConfig.Env))
	}
}

func TestFetchNoAgentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "no-agent-task.md")
	content := `---
title: Task without Agent
priority: low
---

Task description here.
`
	if err := os.WriteFile(taskFile, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p := &Provider{basePath: tmpDir}
	ctx := context.Background()

	wu, err := p.Fetch(ctx, taskFile)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if wu.AgentConfig != nil {
		t.Error("AgentConfig should be nil when agent is not specified in frontmatter")
	}
}
