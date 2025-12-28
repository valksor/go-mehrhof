package plugin

import (
	"testing"
)

func TestNewRegistry(t *testing.T) {
	discovery := &Discovery{}
	r := NewRegistry(discovery)

	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.discovery != discovery {
		t.Error("discovery not set correctly")
	}
	if r.loader == nil {
		t.Error("loader should not be nil")
	}
	if r.plugins == nil {
		t.Error("plugins map should be initialized")
	}
	if r.enabled == nil {
		t.Error("enabled map should be initialized")
	}
	if r.config == nil {
		t.Error("config map should be initialized")
	}
}

func TestRegistrySetEnabled(t *testing.T) {
	r := NewRegistry(&Discovery{})

	// Set enabled plugins
	r.SetEnabled([]string{"plugin1", "plugin2", "plugin3"})

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.enabled) != 3 {
		t.Errorf("enabled count = %d, want 3", len(r.enabled))
	}
	if !r.enabled["plugin1"] {
		t.Error("plugin1 should be enabled")
	}
	if !r.enabled["plugin2"] {
		t.Error("plugin2 should be enabled")
	}
	if !r.enabled["plugin3"] {
		t.Error("plugin3 should be enabled")
	}
	if r.enabled["plugin4"] {
		t.Error("plugin4 should not be enabled")
	}
}

func TestRegistrySetEnabledOverwrite(t *testing.T) {
	r := NewRegistry(&Discovery{})

	// Set initial enabled plugins
	r.SetEnabled([]string{"plugin1", "plugin2"})

	// Overwrite with new list
	r.SetEnabled([]string{"plugin3"})

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.enabled) != 1 {
		t.Errorf("enabled count = %d, want 1", len(r.enabled))
	}
	if r.enabled["plugin1"] {
		t.Error("plugin1 should no longer be enabled")
	}
	if !r.enabled["plugin3"] {
		t.Error("plugin3 should be enabled")
	}
}

func TestRegistrySetEnabledEmpty(t *testing.T) {
	r := NewRegistry(&Discovery{})

	// Set enabled plugins
	r.SetEnabled([]string{"plugin1"})

	// Clear with empty list
	r.SetEnabled([]string{})

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.enabled) != 0 {
		t.Errorf("enabled count = %d, want 0", len(r.enabled))
	}
}

func TestRegistrySetConfig(t *testing.T) {
	r := NewRegistry(&Discovery{})

	config := map[string]map[string]any{
		"plugin1": {
			"key1": "value1",
			"key2": 42,
		},
		"plugin2": {
			"enabled": true,
		},
	}

	r.SetConfig(config)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.config) != 2 {
		t.Errorf("config count = %d, want 2", len(r.config))
	}
	if r.config["plugin1"]["key1"] != "value1" {
		t.Error("plugin1.key1 not set correctly")
	}
	if r.config["plugin1"]["key2"] != 42 {
		t.Error("plugin1.key2 not set correctly")
	}
	if r.config["plugin2"]["enabled"] != true {
		t.Error("plugin2.enabled not set correctly")
	}
}

func TestRegistryGetConfig(t *testing.T) {
	r := NewRegistry(&Discovery{})

	config := map[string]map[string]any{
		"plugin1": {
			"key1": "value1",
		},
	}
	r.SetConfig(config)

	// Get existing config
	cfg := r.GetConfig("plugin1")
	if cfg == nil {
		t.Fatal("GetConfig returned nil for existing plugin")
	}
	if cfg["key1"] != "value1" {
		t.Errorf("key1 = %v, want %q", cfg["key1"], "value1")
	}

	// Get non-existing config
	cfg = r.GetConfig("nonexistent")
	if cfg != nil {
		t.Errorf("GetConfig for nonexistent = %v, want nil", cfg)
	}
}

func TestPluginInfoStruct(t *testing.T) {
	manifest := &Manifest{
		Name:    "test-plugin",
		Version: "1.0.0",
	}

	info := &PluginInfo{
		Manifest: manifest,
		Process:  nil,
		Enabled:  true,
	}

	if info.Manifest.Name != "test-plugin" {
		t.Errorf("Manifest.Name = %q, want %q", info.Manifest.Name, "test-plugin")
	}
	if !info.Enabled {
		t.Error("Enabled should be true")
	}
	if info.Process != nil {
		t.Error("Process should be nil")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Get tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRegistryGet(t *testing.T) {
	r := NewRegistry(&Discovery{})

	// Manually add a plugin to the registry
	r.plugins["test-plugin"] = &PluginInfo{
		Manifest: &Manifest{Name: "test-plugin", Version: "1.0"},
		Enabled:  true,
	}

	t.Run("plugin exists", func(t *testing.T) {
		info, ok := r.Get("test-plugin")
		if !ok {
			t.Error("Get returned false for existing plugin")
		}
		if info == nil {
			t.Fatal("Get returned nil info for existing plugin")
		}
		if info.Manifest.Name != "test-plugin" {
			t.Errorf("Manifest.Name = %q, want %q", info.Manifest.Name, "test-plugin")
		}
	})

	t.Run("plugin does not exist", func(t *testing.T) {
		info, ok := r.Get("nonexistent")
		if ok {
			t.Error("Get returned true for non-existing plugin")
		}
		if info != nil {
			t.Error("Get returned non-nil info for non-existing plugin")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// GetProcess tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRegistryGetProcess(t *testing.T) {
	r := NewRegistry(&Discovery{})

	// Plugin with process
	r.plugins["with-process"] = &PluginInfo{
		Manifest: &Manifest{Name: "with-process"},
		Process:  &Process{started: true}, // Mock process
		Enabled:  true,
	}

	// Plugin without process
	r.plugins["without-process"] = &PluginInfo{
		Manifest: &Manifest{Name: "without-process"},
		Process:  nil,
		Enabled:  false,
	}

	t.Run("plugin with process", func(t *testing.T) {
		proc, ok := r.GetProcess("with-process")
		if !ok {
			t.Error("GetProcess returned false for plugin with process")
		}
		if proc == nil {
			t.Error("GetProcess returned nil process")
		}
	})

	t.Run("plugin without process", func(t *testing.T) {
		proc, ok := r.GetProcess("without-process")
		if ok {
			t.Error("GetProcess returned true for plugin without process")
		}
		if proc != nil {
			t.Error("GetProcess returned non-nil for plugin without process")
		}
	})

	t.Run("plugin does not exist", func(t *testing.T) {
		proc, ok := r.GetProcess("nonexistent")
		if ok {
			t.Error("GetProcess returned true for non-existing plugin")
		}
		if proc != nil {
			t.Error("GetProcess returned non-nil for non-existing plugin")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// List tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRegistryList(t *testing.T) {
	r := NewRegistry(&Discovery{})

	t.Run("empty registry", func(t *testing.T) {
		list := r.List()
		if len(list) != 0 {
			t.Errorf("List() len = %d, want 0", len(list))
		}
	})

	t.Run("registry with plugins", func(t *testing.T) {
		r.plugins["plugin1"] = &PluginInfo{Manifest: &Manifest{Name: "plugin1"}}
		r.plugins["plugin2"] = &PluginInfo{Manifest: &Manifest{Name: "plugin2"}}
		r.plugins["plugin3"] = &PluginInfo{Manifest: &Manifest{Name: "plugin3"}}

		list := r.List()
		if len(list) != 3 {
			t.Errorf("List() len = %d, want 3", len(list))
		}

		// Verify all plugins are returned (order may vary)
		names := make(map[string]bool)
		for _, info := range list {
			names[info.Manifest.Name] = true
		}
		for _, name := range []string{"plugin1", "plugin2", "plugin3"} {
			if !names[name] {
				t.Errorf("List() missing plugin %q", name)
			}
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// ListEnabled tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRegistryListEnabled(t *testing.T) {
	r := NewRegistry(&Discovery{})

	// Mix of enabled/disabled with/without process
	r.plugins["enabled-with-process"] = &PluginInfo{
		Manifest: &Manifest{Name: "enabled-with-process"},
		Process:  &Process{started: true},
		Enabled:  true,
	}
	r.plugins["enabled-no-process"] = &PluginInfo{
		Manifest: &Manifest{Name: "enabled-no-process"},
		Process:  nil,
		Enabled:  true,
	}
	r.plugins["disabled-with-process"] = &PluginInfo{
		Manifest: &Manifest{Name: "disabled-with-process"},
		Process:  &Process{started: true},
		Enabled:  false,
	}
	r.plugins["disabled-no-process"] = &PluginInfo{
		Manifest: &Manifest{Name: "disabled-no-process"},
		Process:  nil,
		Enabled:  false,
	}

	list := r.ListEnabled()

	// Should only return enabled plugins with process
	if len(list) != 1 {
		t.Errorf("ListEnabled() len = %d, want 1", len(list))
	}
	if len(list) > 0 && list[0].Manifest.Name != "enabled-with-process" {
		t.Errorf("ListEnabled()[0].Name = %q, want %q", list[0].Manifest.Name, "enabled-with-process")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ListByType tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRegistryListByType(t *testing.T) {
	r := NewRegistry(&Discovery{})

	r.plugins["provider1"] = &PluginInfo{Manifest: &Manifest{Name: "provider1", Type: PluginTypeProvider}}
	r.plugins["provider2"] = &PluginInfo{Manifest: &Manifest{Name: "provider2", Type: PluginTypeProvider}}
	r.plugins["agent1"] = &PluginInfo{Manifest: &Manifest{Name: "agent1", Type: PluginTypeAgent}}
	r.plugins["workflow1"] = &PluginInfo{Manifest: &Manifest{Name: "workflow1", Type: PluginTypeWorkflow}}

	tests := []struct {
		name       string
		pluginType PluginType
		wantCount  int
	}{
		{"providers", PluginTypeProvider, 2},
		{"agents", PluginTypeAgent, 1},
		{"workflows", PluginTypeWorkflow, 1},
		{"nonexistent type", PluginType("other"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := r.ListByType(tt.pluginType)
			if len(list) != tt.wantCount {
				t.Errorf("ListByType(%q) len = %d, want %d", tt.pluginType, len(list), tt.wantCount)
			}
			for _, info := range list {
				if info.Manifest.Type != tt.pluginType {
					t.Errorf("plugin %q has type %q, want %q", info.Manifest.Name, info.Manifest.Type, tt.pluginType)
				}
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// ListEnabledByType tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRegistryListEnabledByType(t *testing.T) {
	r := NewRegistry(&Discovery{})

	// Enabled providers with process
	r.plugins["provider1"] = &PluginInfo{
		Manifest: &Manifest{Name: "provider1", Type: PluginTypeProvider},
		Process:  &Process{started: true},
		Enabled:  true,
	}
	// Disabled provider
	r.plugins["provider2"] = &PluginInfo{
		Manifest: &Manifest{Name: "provider2", Type: PluginTypeProvider},
		Enabled:  false,
	}
	// Enabled agent with process
	r.plugins["agent1"] = &PluginInfo{
		Manifest: &Manifest{Name: "agent1", Type: PluginTypeAgent},
		Process:  &Process{started: true},
		Enabled:  true,
	}
	// Enabled workflow without process (not fully loaded)
	r.plugins["workflow1"] = &PluginInfo{
		Manifest: &Manifest{Name: "workflow1", Type: PluginTypeWorkflow},
		Process:  nil,
		Enabled:  true,
	}

	tests := []struct {
		name       string
		pluginType PluginType
		wantCount  int
	}{
		{"enabled providers", PluginTypeProvider, 1},
		{"enabled agents", PluginTypeAgent, 1},
		{"enabled workflows (none with process)", PluginTypeWorkflow, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := r.ListEnabledByType(tt.pluginType)
			if len(list) != tt.wantCount {
				t.Errorf("ListEnabledByType(%q) len = %d, want %d", tt.pluginType, len(list), tt.wantCount)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Type-specific helper tests
// ──────────────────────────────────────────────────────────────────────────────

func TestRegistryProviders(t *testing.T) {
	r := NewRegistry(&Discovery{})
	r.plugins["provider1"] = &PluginInfo{
		Manifest: &Manifest{Name: "provider1", Type: PluginTypeProvider},
		Process:  &Process{started: true},
		Enabled:  true,
	}

	providers := r.Providers()
	if len(providers) != 1 {
		t.Errorf("Providers() len = %d, want 1", len(providers))
	}
}

func TestRegistryAgents(t *testing.T) {
	r := NewRegistry(&Discovery{})
	r.plugins["agent1"] = &PluginInfo{
		Manifest: &Manifest{Name: "agent1", Type: PluginTypeAgent},
		Process:  &Process{started: true},
		Enabled:  true,
	}

	agents := r.Agents()
	if len(agents) != 1 {
		t.Errorf("Agents() len = %d, want 1", len(agents))
	}
}

func TestRegistryWorkflows(t *testing.T) {
	r := NewRegistry(&Discovery{})
	r.plugins["workflow1"] = &PluginInfo{
		Manifest: &Manifest{Name: "workflow1", Type: PluginTypeWorkflow},
		Process:  &Process{started: true},
		Enabled:  true,
	}

	workflows := r.Workflows()
	if len(workflows) != 1 {
		t.Errorf("Workflows() len = %d, want 1", len(workflows))
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// initMethod tests
// ──────────────────────────────────────────────────────────────────────────────

func TestInitMethod(t *testing.T) {
	tests := []struct {
		name       string
		pluginType PluginType
		want       string
	}{
		{"provider", PluginTypeProvider, "provider.init"},
		{"agent", PluginTypeAgent, "agent.init"},
		{"workflow", PluginTypeWorkflow, "workflow.init"},
		{"unknown", PluginType("unknown"), "init"},
		{"empty", PluginType(""), "init"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := initMethod(tt.pluginType)
			if got != tt.want {
				t.Errorf("initMethod(%q) = %q, want %q", tt.pluginType, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Discovery accessor test
// ──────────────────────────────────────────────────────────────────────────────

func TestRegistryDiscovery(t *testing.T) {
	discovery := &Discovery{globalDir: "/global", projectDir: "/project"}
	r := NewRegistry(discovery)

	got := r.Discovery()
	if got != discovery {
		t.Error("Discovery() did not return the same instance")
	}
	if got.GlobalDir() != "/global" {
		t.Errorf("Discovery().GlobalDir() = %q, want %q", got.GlobalDir(), "/global")
	}
}
