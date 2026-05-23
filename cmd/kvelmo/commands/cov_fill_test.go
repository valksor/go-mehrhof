package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/settings"
)

// --- config: printConfigValue default (non-string/bool) branch ---

func TestPrintConfigValue_Number(t *testing.T) {
	out := captureStdout(t, func() {
		printConfigValue(float64(42))
	})
	if strings.TrimSpace(out) != "42" {
		t.Errorf("printConfigValue(number) = %q, want 42", strings.TrimSpace(out))
	}
}

func TestPrintConfigValue_Map(t *testing.T) {
	out := captureStdout(t, func() {
		printConfigValue(map[string]any{"max": float64(4)})
	})
	if !strings.Contains(out, "\"max\":4") {
		t.Errorf("printConfigValue(map) = %q", out)
	}
}

// --- config: configGetOffline unknown-path error branch ---

func TestConfigGetOffline_UnknownPath(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	if err := configGetOffline("no.such.path"); err == nil {
		t.Fatal("expected error for unknown configuration path")
	}
}

// --- config: offline load error when the global config YAML is malformed ---

func TestConfigShowOffline_LoadError(t *testing.T) {
	home := shortKvelmoHome(t)
	chdirToShortTemp(t)
	// Write malformed YAML at the global config path so LoadGlobal errors.
	cfgPath := filepath.Join(home, "kvelmo.yaml")
	if err := os.WriteFile(cfgPath, []byte("workers: [unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := configShowOffline(); err == nil {
		t.Fatal("expected error from malformed global config")
	}
}

func TestConfigGetOffline_LoadError(t *testing.T) {
	home := shortKvelmoHome(t)
	chdirToShortTemp(t)
	cfgPath := filepath.Join(home, "kvelmo.yaml")
	if err := os.WriteFile(cfgPath, []byte(": : not valid yaml :\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := configGetOffline("workers.max"); err == nil {
		t.Fatal("expected error from malformed global config")
	}
}

func TestLoadEffectiveOffline_LoadError(t *testing.T) {
	home := shortKvelmoHome(t)
	chdirToShortTemp(t)
	cfgPath := filepath.Join(home, "kvelmo.yaml")
	if err := os.WriteFile(cfgPath, []byte("\t- broken: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := loadEffectiveOffline(); err == nil {
		t.Fatal("expected loadEffectiveOffline error from malformed global config")
	}
}

// --- config diff: populated project-overrides branch ---

func TestRunConfigDiff_WithProject(t *testing.T) {
	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)

	// Write a valid project config so the marshal + "Project overrides:" branch runs.
	s := settings.DefaultSettings()
	if err := settings.SaveProject(dir, s); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := runConfigDiff(configDiffCmd, nil); err != nil {
			t.Errorf("runConfigDiff with project: %v", err)
		}
	})
	if !strings.Contains(out, "Project overrides:") {
		t.Errorf("config diff (with project) output:\n%s", out)
	}
}

func TestRunConfigDiff_LoadError(t *testing.T) {
	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)
	// Malformed project config -> LoadProject returns an error (not nil).
	projPath := settings.ProjectPath(dir)
	if err := os.MkdirAll(filepath.Dir(projPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projPath, []byte("git: [oops\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := runConfigDiff(configDiffCmd, nil); err == nil {
			t.Error("expected error from malformed project config")
		}
	})
	if !strings.Contains(out, "No project config found") {
		t.Errorf("config diff (load error) output:\n%s", out)
	}
}

// --- config edit: editor not found in PATH ---

func TestRunConfigEdit_EditorNotInPath(t *testing.T) {
	t.Setenv("EDITOR", "definitely-not-a-real-editor-binary-xyz")
	shortKvelmoHome(t)
	configEditCmd.SetContext(t.Context())

	if err := runConfigEdit(configEditCmd, nil); err == nil {
		t.Fatal("expected error when editor is not in PATH")
	}
}

// --- config path command ---

func TestConfigPathCmd_Run(t *testing.T) {
	shortKvelmoHome(t)
	out := captureStdout(t, func() {
		configPathCmd.Run(configPathCmd, nil)
	})
	if !strings.Contains(out, "kvelmo.yaml") {
		t.Errorf("config path output = %q", out)
	}
}

// --- config nestedGet error branches ---

func TestNestedGet_Errors(t *testing.T) {
	m := map[string]any{"a": map[string]any{"b": "v"}, "leaf": "x"}

	if _, err := nestedGet(m, "missing"); err == nil {
		t.Error("expected error for missing top-level key")
	}
	if _, err := nestedGet(m, "a.missing"); err == nil {
		t.Error("expected error for missing nested key")
	}
	// "leaf" is a string, so descending into "leaf.x" must fail.
	if _, err := nestedGet(m, "leaf.x"); err == nil {
		t.Error("expected error when descending into non-map value")
	}
}
