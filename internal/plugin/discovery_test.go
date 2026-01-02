package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Test helper functions
// ──────────────────────────────────────────────────────────────────────────────

// createTestPlugin creates a valid plugin directory with manifest.
func createTestPlugin(t *testing.T, baseDir, name string, pluginType PluginType) string {
	t.Helper()

	pluginDir := filepath.Join(baseDir, name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}

	manifest := buildManifest(name, pluginType)
	manifestPath := filepath.Join(pluginDir, ManifestFileName)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	return pluginDir
}

// buildManifest creates a valid manifest YAML for the given type.
func buildManifest(name string, pluginType PluginType) string {
	base := `version: "1.0"
name: ` + name + `
type: ` + string(pluginType) + `
description: Test plugin
protocol: "1.0"
executable:
  path: plugin
`
	switch pluginType {
	case PluginTypeProvider:
		return base + `provider:
  name: ` + name + `
  schemes: ["test"]
  capabilities: ["read"]
`
	case PluginTypeAgent:
		return base + `agent:
  name: ` + name + `
  streaming: true
`
	case PluginTypeWorkflow:
		return base + `workflow:
  phases: []
`
	}
	return base
}

// ──────────────────────────────────────────────────────────────────────────────
// NewDiscovery tests
// ──────────────────────────────────────────────────────────────────────────────

func TestNewDiscovery(t *testing.T) {
	tests := []struct {
		name       string
		globalDir  string
		projectDir string
	}{
		{
			name:       "both directories specified",
			globalDir:  "/home/user/.mehrhof/plugins",
			projectDir: "/project/.mehrhof/plugins",
		},
		{
			name:       "only global directory",
			globalDir:  "/home/user/.mehrhof/plugins",
			projectDir: "",
		},
		{
			name:       "only project directory",
			globalDir:  "",
			projectDir: "/project/.mehrhof/plugins",
		},
		{
			name:       "empty directories",
			globalDir:  "",
			projectDir: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDiscovery(tt.globalDir, tt.projectDir)
			if d == nil {
				t.Fatal("NewDiscovery returned nil")
			}
			if d.globalDir != tt.globalDir {
				t.Errorf("globalDir = %q, want %q", d.globalDir, tt.globalDir)
			}
			if d.projectDir != tt.projectDir {
				t.Errorf("projectDir = %q, want %q", d.projectDir, tt.projectDir)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// DefaultGlobalDir tests
// ──────────────────────────────────────────────────────────────────────────────

func TestDefaultGlobalDir(t *testing.T) {
	dir, err := DefaultGlobalDir()
	if err != nil {
		t.Fatalf("DefaultGlobalDir() error = %v", err)
	}

	if dir == "" {
		t.Error("DefaultGlobalDir() returned empty string")
	}

	// Should end with .mehrhof/plugins
	if !filepath.IsAbs(dir) {
		t.Error("DefaultGlobalDir() should return absolute path")
	}

	suffix := filepath.Join(".mehrhof", "plugins")
	if filepath.Base(filepath.Dir(dir)) != ".mehrhof" || filepath.Base(dir) != "plugins" {
		t.Errorf("DefaultGlobalDir() = %q, should end with %q", dir, suffix)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// DefaultProjectDir tests
// ──────────────────────────────────────────────────────────────────────────────

func TestDefaultProjectDir(t *testing.T) {
	tests := []struct {
		name          string
		workspaceRoot string
		want          string
	}{
		{
			name:          "simple path",
			workspaceRoot: "/project",
			want:          "/project/.mehrhof/plugins",
		},
		{
			name:          "nested path",
			workspaceRoot: "/home/user/workspace/myproject",
			want:          "/home/user/workspace/myproject/.mehrhof/plugins",
		},
		{
			name:          "empty workspace",
			workspaceRoot: "",
			want:          ".mehrhof/plugins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultProjectDir(tt.workspaceRoot)
			// Use filepath.Clean to normalize paths for comparison
			if filepath.Clean(got) != filepath.Clean(tt.want) {
				t.Errorf("DefaultProjectDir(%q) = %q, want %q", tt.workspaceRoot, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Discover tests
// ──────────────────────────────────────────────────────────────────────────────

func TestDiscover(t *testing.T) {
	t.Run("empty directories", func(t *testing.T) {
		globalDir := t.TempDir()
		projectDir := t.TempDir()

		d := NewDiscovery(globalDir, projectDir)
		plugins, err := d.Discover()
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		if len(plugins) != 0 {
			t.Errorf("Discover() found %d plugins, want 0", len(plugins))
		}
	})

	t.Run("non-existent directories", func(t *testing.T) {
		d := NewDiscovery("/nonexistent/global", "/nonexistent/project")
		plugins, err := d.Discover()
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		if len(plugins) != 0 {
			t.Errorf("Discover() found %d plugins, want 0", len(plugins))
		}
	})

	t.Run("plugins in global dir only", func(t *testing.T) {
		globalDir := t.TempDir()
		projectDir := t.TempDir()

		createTestPlugin(t, globalDir, "plugin1", PluginTypeProvider)
		createTestPlugin(t, globalDir, "plugin2", PluginTypeAgent)

		d := NewDiscovery(globalDir, projectDir)
		plugins, err := d.Discover()
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		if len(plugins) != 2 {
			t.Errorf("Discover() found %d plugins, want 2", len(plugins))
		}

		// Verify scope is set correctly
		for _, p := range plugins {
			if p.Scope != ScopeGlobal {
				t.Errorf("plugin %s has scope %q, want %q", p.Name, p.Scope, ScopeGlobal)
			}
		}
	})

	t.Run("plugins in project dir only", func(t *testing.T) {
		globalDir := t.TempDir()
		projectDir := t.TempDir()

		createTestPlugin(t, projectDir, "myplugin", PluginTypeWorkflow)

		d := NewDiscovery(globalDir, projectDir)
		plugins, err := d.Discover()
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		if len(plugins) != 1 {
			t.Errorf("Discover() found %d plugins, want 1", len(plugins))
		}
		if plugins[0].Scope != ScopeProject {
			t.Errorf("plugin scope = %q, want %q", plugins[0].Scope, ScopeProject)
		}
	})

	t.Run("project plugins override global", func(t *testing.T) {
		globalDir := t.TempDir()
		projectDir := t.TempDir()

		// Create same-named plugin in both dirs
		createTestPlugin(t, globalDir, "shared-plugin", PluginTypeProvider)
		createTestPlugin(t, projectDir, "shared-plugin", PluginTypeProvider)

		d := NewDiscovery(globalDir, projectDir)
		plugins, err := d.Discover()
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		if len(plugins) != 1 {
			t.Errorf("Discover() found %d plugins, want 1 (deduplicated)", len(plugins))
		}
		if plugins[0].Scope != ScopeProject {
			t.Errorf("plugin scope = %q, want %q (project should override)", plugins[0].Scope, ScopeProject)
		}
	})

	t.Run("invalid manifest skipped", func(t *testing.T) {
		globalDir := t.TempDir()
		projectDir := t.TempDir()

		// Create valid plugin
		createTestPlugin(t, globalDir, "valid-plugin", PluginTypeProvider)

		// Create invalid plugin (missing required fields)
		invalidDir := filepath.Join(globalDir, "invalid-plugin")
		if err := os.MkdirAll(invalidDir, 0o755); err != nil {
			t.Fatalf("create invalid plugin dir: %v", err)
		}
		invalidManifest := `version: "1.0"
name: invalid
# missing type, protocol, executable
`
		if err := os.WriteFile(filepath.Join(invalidDir, ManifestFileName), []byte(invalidManifest), 0o644); err != nil {
			t.Fatalf("write invalid manifest: %v", err)
		}

		d := NewDiscovery(globalDir, projectDir)
		plugins, err := d.Discover()
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		// Should only find the valid plugin
		if len(plugins) != 1 {
			t.Errorf("Discover() found %d plugins, want 1 (invalid should be skipped)", len(plugins))
		}
	})

	t.Run("non-directory entries skipped", func(t *testing.T) {
		globalDir := t.TempDir()
		projectDir := t.TempDir()

		// Create valid plugin
		createTestPlugin(t, globalDir, "valid-plugin", PluginTypeProvider)

		// Create a file (not directory) in plugins dir
		filePath := filepath.Join(globalDir, "not-a-plugin.txt")
		if err := os.WriteFile(filePath, []byte("not a plugin"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		d := NewDiscovery(globalDir, projectDir)
		plugins, err := d.Discover()
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		if len(plugins) != 1 {
			t.Errorf("Discover() found %d plugins, want 1", len(plugins))
		}
	})

	t.Run("empty global dir config", func(t *testing.T) {
		projectDir := t.TempDir()
		createTestPlugin(t, projectDir, "project-plugin", PluginTypeAgent)

		d := NewDiscovery("", projectDir) // Empty global dir
		plugins, err := d.Discover()
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		if len(plugins) != 1 {
			t.Errorf("Discover() found %d plugins, want 1", len(plugins))
		}
	})

	t.Run("empty project dir config", func(t *testing.T) {
		globalDir := t.TempDir()
		createTestPlugin(t, globalDir, "global-plugin", PluginTypeAgent)

		d := NewDiscovery(globalDir, "") // Empty project dir
		plugins, err := d.Discover()
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		if len(plugins) != 1 {
			t.Errorf("Discover() found %d plugins, want 1", len(plugins))
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// DiscoverByType tests
// ──────────────────────────────────────────────────────────────────────────────

func TestDiscoverByType(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()

	// Create plugins of each type
	createTestPlugin(t, globalDir, "provider1", PluginTypeProvider)
	createTestPlugin(t, globalDir, "provider2", PluginTypeProvider)
	createTestPlugin(t, globalDir, "agent1", PluginTypeAgent)
	createTestPlugin(t, globalDir, "workflow1", PluginTypeWorkflow)

	d := NewDiscovery(globalDir, projectDir)

	tests := []struct {
		name       string
		pluginType PluginType
		wantCount  int
	}{
		{
			name:       "find providers",
			pluginType: PluginTypeProvider,
			wantCount:  2,
		},
		{
			name:       "find agents",
			pluginType: PluginTypeAgent,
			wantCount:  1,
		},
		{
			name:       "find workflows",
			pluginType: PluginTypeWorkflow,
			wantCount:  1,
		},
		{
			name:       "find non-existent type",
			pluginType: PluginType("nonexistent"),
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugins, err := d.DiscoverByType(tt.pluginType)
			if err != nil {
				t.Fatalf("DiscoverByType() error = %v", err)
			}
			if len(plugins) != tt.wantCount {
				t.Errorf("DiscoverByType(%q) found %d plugins, want %d", tt.pluginType, len(plugins), tt.wantCount)
			}
			// Verify all returned plugins have correct type
			for _, p := range plugins {
				if p.Type != tt.pluginType {
					t.Errorf("plugin %s has type %q, want %q", p.Name, p.Type, tt.pluginType)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// DiscoverByName tests
// ──────────────────────────────────────────────────────────────────────────────

func TestDiscoverByName(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()

	createTestPlugin(t, globalDir, "my-provider", PluginTypeProvider)
	createTestPlugin(t, projectDir, "my-agent", PluginTypeAgent)

	d := NewDiscovery(globalDir, projectDir)

	tests := []struct {
		name       string
		pluginName string
		wantFound  bool
	}{
		{
			name:       "find existing plugin in global",
			pluginName: "my-provider",
			wantFound:  true,
		},
		{
			name:       "find existing plugin in project",
			pluginName: "my-agent",
			wantFound:  true,
		},
		{
			name:       "plugin not found",
			pluginName: "nonexistent",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin, err := d.DiscoverByName(tt.pluginName)
			if err != nil {
				t.Fatalf("DiscoverByName() error = %v", err)
			}
			if tt.wantFound && plugin == nil {
				t.Errorf("DiscoverByName(%q) = nil, want non-nil", tt.pluginName)
			}
			if !tt.wantFound && plugin != nil {
				t.Errorf("DiscoverByName(%q) = %v, want nil", tt.pluginName, plugin)
			}
			if plugin != nil && plugin.Name != tt.pluginName {
				t.Errorf("plugin.Name = %q, want %q", plugin.Name, tt.pluginName)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GlobalDir and ProjectDir tests
// ──────────────────────────────────────────────────────────────────────────────

func TestDiscoveryAccessors(t *testing.T) {
	globalDir := "/global/plugins"
	projectDir := "/project/plugins"

	d := NewDiscovery(globalDir, projectDir)

	if got := d.GlobalDir(); got != globalDir {
		t.Errorf("GlobalDir() = %q, want %q", got, globalDir)
	}
	if got := d.ProjectDir(); got != projectDir {
		t.Errorf("ProjectDir() = %q, want %q", got, projectDir)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// EnsureDir tests
// ──────────────────────────────────────────────────────────────────────────────

func TestEnsureDir(t *testing.T) {
	t.Run("create new directory", func(t *testing.T) {
		baseDir := t.TempDir()
		newDir := filepath.Join(baseDir, "plugins", "subdir")

		err := EnsureDir(newDir)
		if err != nil {
			t.Fatalf("EnsureDir() error = %v", err)
		}

		info, err := os.Stat(newDir)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("created path is not a directory")
		}
	})

	t.Run("directory already exists", func(t *testing.T) {
		existingDir := t.TempDir()

		err := EnsureDir(existingDir)
		if err != nil {
			t.Fatalf("EnsureDir() error = %v", err)
		}
		// Should not error when dir already exists
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// PluginDir tests
// ──────────────────────────────────────────────────────────────────────────────

func TestPluginDir(t *testing.T) {
	tests := []struct {
		name       string
		baseDir    string
		pluginName string
		want       string
	}{
		{
			name:       "simple paths",
			baseDir:    "/home/user/.mehrhof/plugins",
			pluginName: "my-plugin",
			want:       "/home/user/.mehrhof/plugins/my-plugin",
		},
		{
			name:       "empty base dir",
			baseDir:    "",
			pluginName: "plugin",
			want:       "plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PluginDir(tt.baseDir, tt.pluginName)
			if got != tt.want {
				t.Errorf("PluginDir(%q, %q) = %q, want %q", tt.baseDir, tt.pluginName, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// InstallPath tests
// ──────────────────────────────────────────────────────────────────────────────

func TestInstallPath(t *testing.T) {
	tests := []struct {
		name       string
		baseDir    string
		pluginName string
		want       string
	}{
		{
			name:       "standard path",
			baseDir:    "/home/user/.mehrhof/plugins",
			pluginName: "my-plugin",
			want:       "/home/user/.mehrhof/plugins/my-plugin/plugin.yaml",
		},
		{
			name:       "empty base dir",
			baseDir:    "",
			pluginName: "plugin",
			want:       "plugin/plugin.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InstallPath(tt.baseDir, tt.pluginName)
			if got != tt.want {
				t.Errorf("InstallPath(%q, %q) = %q, want %q", tt.baseDir, tt.pluginName, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Constants tests
// ──────────────────────────────────────────────────────────────────────────────

func TestConstants(t *testing.T) {
	if ManifestFileName != "plugin.yaml" {
		t.Errorf("ManifestFileName = %q, want %q", ManifestFileName, "plugin.yaml")
	}
	if ScopeGlobal != "global" {
		t.Errorf("ScopeGlobal = %q, want %q", ScopeGlobal, "global")
	}
	if ScopeProject != "project" {
		t.Errorf("ScopeProject = %q, want %q", ScopeProject, "project")
	}
}
