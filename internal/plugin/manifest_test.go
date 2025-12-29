package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginTypeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value PluginType
		want  string
	}{
		{"PluginTypeProvider", PluginTypeProvider, "provider"},
		{"PluginTypeAgent", PluginTypeAgent, "agent"},
		{"PluginTypeWorkflow", PluginTypeWorkflow, "workflow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestManifestValidate(t *testing.T) {
	tests := []struct {
		manifest  Manifest
		name      string
		errSubstr string
		wantErr   bool
	}{
		{
			name: "valid provider manifest",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test-provider",
				Type:     PluginTypeProvider,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
				Provider: &ProviderConfig{
					Name:    "test",
					Schemes: []string{"test:"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid agent manifest",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test-agent",
				Type:     PluginTypeAgent,
				Protocol: "1",
				Executable: ExecutableConfig{
					Command: []string{"python", "agent.py"},
				},
				Agent: &AgentConfig{
					Name: "test-agent",
				},
			},
			wantErr: false,
		},
		{
			name: "valid workflow manifest",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test-workflow",
				Type:     PluginTypeWorkflow,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./workflow",
				},
				Workflow: &WorkflowConfig{},
			},
			wantErr: false,
		},
		{
			name: "missing version",
			manifest: Manifest{
				Name:     "test",
				Type:     PluginTypeProvider,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
				Provider: &ProviderConfig{
					Name:    "test",
					Schemes: []string{"test:"},
				},
			},
			wantErr:   true,
			errSubstr: "version is required",
		},
		{
			name: "missing name",
			manifest: Manifest{
				Version:  "1.0.0",
				Type:     PluginTypeProvider,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
				Provider: &ProviderConfig{
					Name:    "test",
					Schemes: []string{"test:"},
				},
			},
			wantErr:   true,
			errSubstr: "name is required",
		},
		{
			name: "missing type",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test",
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
			},
			wantErr:   true,
			errSubstr: "type is required",
		},
		{
			name: "missing protocol",
			manifest: Manifest{
				Version: "1.0.0",
				Name:    "test",
				Type:    PluginTypeProvider,
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
				Provider: &ProviderConfig{
					Name:    "test",
					Schemes: []string{"test:"},
				},
			},
			wantErr:   true,
			errSubstr: "protocol is required",
		},
		{
			name: "missing executable",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test",
				Type:     PluginTypeProvider,
				Protocol: "1",
				Provider: &ProviderConfig{
					Name:    "test",
					Schemes: []string{"test:"},
				},
			},
			wantErr:   true,
			errSubstr: "executable.path or executable.command is required",
		},
		{
			name: "provider missing config",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test",
				Type:     PluginTypeProvider,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
			},
			wantErr:   true,
			errSubstr: "provider configuration required",
		},
		{
			name: "provider missing name",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test",
				Type:     PluginTypeProvider,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
				Provider: &ProviderConfig{
					Schemes: []string{"test:"},
				},
			},
			wantErr:   true,
			errSubstr: "provider.name is required",
		},
		{
			name: "provider missing schemes",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test",
				Type:     PluginTypeProvider,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
				Provider: &ProviderConfig{
					Name: "test",
				},
			},
			wantErr:   true,
			errSubstr: "provider.schemes is required",
		},
		{
			name: "agent missing config",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test",
				Type:     PluginTypeAgent,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
			},
			wantErr:   true,
			errSubstr: "agent configuration required",
		},
		{
			name: "agent missing name",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test",
				Type:     PluginTypeAgent,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
				Agent: &AgentConfig{},
			},
			wantErr:   true,
			errSubstr: "agent.name is required",
		},
		{
			name: "workflow missing config",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test",
				Type:     PluginTypeWorkflow,
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
			},
			wantErr:   true,
			errSubstr: "workflow configuration required",
		},
		{
			name: "invalid type",
			manifest: Manifest{
				Version:  "1.0.0",
				Name:     "test",
				Type:     PluginType("invalid"),
				Protocol: "1",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
			},
			wantErr:   true,
			errSubstr: "invalid plugin type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("Validate() error = %q, want substring %q", err.Error(), tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestManifestExecutablePath(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		want     string
	}{
		{
			name: "empty path",
			manifest: Manifest{
				Executable: ExecutableConfig{},
			},
			want: "",
		},
		{
			name: "absolute path",
			manifest: Manifest{
				Dir: "/plugins/test",
				Executable: ExecutableConfig{
					Path: "/usr/bin/plugin",
				},
			},
			want: "/usr/bin/plugin",
		},
		{
			name: "relative path",
			manifest: Manifest{
				Dir: "/plugins/test",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
			},
			want: "/plugins/test/plugin",
		},
		{
			name: "relative path without dot",
			manifest: Manifest{
				Dir: "/plugins/test",
				Executable: ExecutableConfig{
					Path: "bin/plugin",
				},
			},
			want: "/plugins/test/bin/plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.manifest.ExecutablePath()
			if got != tt.want {
				t.Errorf("ExecutablePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestManifestExecutableCommand(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		want     []string
	}{
		{
			name: "command specified",
			manifest: Manifest{
				Dir: "/plugins/test",
				Executable: ExecutableConfig{
					Command: []string{"python", "-m", "plugin"},
				},
			},
			want: []string{"python", "-m", "plugin"},
		},
		{
			name: "path only",
			manifest: Manifest{
				Dir: "/plugins/test",
				Executable: ExecutableConfig{
					Path: "./plugin",
				},
			},
			want: []string{"/plugins/test/plugin"},
		},
		{
			name: "empty executable",
			manifest: Manifest{
				Dir:        "/plugins/test",
				Executable: ExecutableConfig{},
			},
			want: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.manifest.ExecutableCommand()
			if len(got) != len(tt.want) {
				t.Errorf("ExecutableCommand() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExecutableCommand()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestManifestHasCapability(t *testing.T) {
	tests := []struct {
		name       string
		manifest   Manifest
		capability string
		want       bool
	}{
		{
			name: "has capability",
			manifest: Manifest{
				Provider: &ProviderConfig{
					Capabilities: []string{"read", "write", "list"},
				},
			},
			capability: "write",
			want:       true,
		},
		{
			name: "does not have capability",
			manifest: Manifest{
				Provider: &ProviderConfig{
					Capabilities: []string{"read", "list"},
				},
			},
			capability: "write",
			want:       false,
		},
		{
			name: "empty capabilities",
			manifest: Manifest{
				Provider: &ProviderConfig{
					Capabilities: []string{},
				},
			},
			capability: "read",
			want:       false,
		},
		{
			name:       "nil provider",
			manifest:   Manifest{},
			capability: "read",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.manifest.HasCapability(tt.capability)
			if got != tt.want {
				t.Errorf("HasCapability(%q) = %v, want %v", tt.capability, got, tt.want)
			}
		})
	}
}

func TestLoadManifest(t *testing.T) {
	t.Run("valid manifest file", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifestPath := filepath.Join(tmpDir, "manifest.yaml")

		content := `version: "1.0.0"
name: test-provider
type: provider
protocol: "1"
executable:
  path: ./plugin
provider:
  name: test
  schemes:
    - "test:"
`
		if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write manifest: %v", err)
		}

		m, err := LoadManifest(manifestPath)
		if err != nil {
			t.Fatalf("LoadManifest() error: %v", err)
		}

		if m.Name != "test-provider" {
			t.Errorf("Name = %q, want %q", m.Name, "test-provider")
		}
		if m.Dir != tmpDir {
			t.Errorf("Dir = %q, want %q", m.Dir, tmpDir)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadManifest("/nonexistent/manifest.yaml")
		if err == nil {
			t.Error("LoadManifest() expected error for nonexistent file")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifestPath := filepath.Join(tmpDir, "manifest.yaml")

		if err := os.WriteFile(manifestPath, []byte("invalid: yaml: content:"), 0o644); err != nil {
			t.Fatalf("failed to write manifest: %v", err)
		}

		_, err := LoadManifest(manifestPath)
		if err == nil {
			t.Error("LoadManifest() expected error for invalid yaml")
		}
	})

	t.Run("invalid manifest", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifestPath := filepath.Join(tmpDir, "manifest.yaml")

		content := `version: "1.0.0"
name: test
# missing required fields
`
		if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write manifest: %v", err)
		}

		_, err := LoadManifest(manifestPath)
		if err == nil {
			t.Error("LoadManifest() expected error for invalid manifest")
		}
	})
}
