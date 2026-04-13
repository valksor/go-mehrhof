package quality

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── readNodeScripts ────────────────────────────────────────────────────────

func TestReadNodeScripts_NoFile(t *testing.T) {
	_, err := readNodeScripts(t.TempDir())
	if err == nil {
		t.Error("readNodeScripts() with no file should return error")
	}
}

func TestReadNodeScripts_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{invalid}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readNodeScripts(dir)
	if err == nil {
		t.Error("readNodeScripts() with invalid JSON should return error")
	}
}

func TestReadNodeScripts_WithScripts(t *testing.T) {
	dir := t.TempDir()
	content := `{"name":"test","scripts":{"lint":"eslint .","test":"vitest","build":"tsc"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	scripts, err := readNodeScripts(dir)
	if err != nil {
		t.Fatalf("readNodeScripts() error = %v", err)
	}
	if scripts["lint"] != "eslint ." {
		t.Errorf("scripts[lint] = %q, want 'eslint .'", scripts["lint"])
	}
	if scripts["test"] != "vitest" {
		t.Errorf("scripts[test] = %q, want 'vitest'", scripts["test"])
	}
}

func TestReadNodeScripts_NoScripts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	scripts, err := readNodeScripts(dir)
	if err != nil {
		t.Fatalf("readNodeScripts() error = %v", err)
	}
	if len(scripts) != 0 {
		t.Errorf("readNodeScripts() with no scripts = %v, want empty", scripts)
	}
}

// ─── collectPythonFiles ─────────────────────────────────────────────────────

func TestCollectPythonFiles_Empty(t *testing.T) {
	files, err := collectPythonFiles(t.TempDir())
	if err != nil {
		t.Fatalf("collectPythonFiles() error = %v", err)
	}
	if len(files) != 0 {
		t.Errorf("collectPythonFiles() on empty dir = %v, want empty", files)
	}
}

func TestCollectPythonFiles_MixedFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"main.py", "helper.py", "main.go", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := collectPythonFiles(dir)
	if err != nil {
		t.Fatalf("collectPythonFiles() error = %v", err)
	}
	if len(files) != 2 {
		t.Errorf("collectPythonFiles() = %v (len=%d), want 2 .py files", files, len(files))
	}
	for _, f := range files {
		if filepath.Ext(f) != ".py" {
			t.Errorf("collectPythonFiles() returned non-.py file: %q", f)
		}
	}
}

func TestCollectPythonFiles_SkipsDirs(t *testing.T) {
	dir := t.TempDir()
	// Create files that should be found
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hi')"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "util.py"), []byte("pass"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create dirs that should be skipped
	for _, skipDir := range []string{".venv", "venv", "__pycache__", ".git", "node_modules"} {
		if err := os.MkdirAll(filepath.Join(dir, skipDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, skipDir, "skip.py"), []byte("pass"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := collectPythonFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("collectPythonFiles() found %d files, want 2: %v", len(files), files)
	}
}

func TestCollectPythonFiles_NestedSubdirs(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deep.py"), []byte("pass"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "top.py"), []byte("pass"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := collectPythonFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("collectPythonFiles() found %d files, want 2: %v", len(files), files)
	}
}

func TestCollectPythonFiles_NonExistentDir(t *testing.T) {
	_, err := collectPythonFiles(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Error("collectPythonFiles() with non-existent dir should return error")
	}
}

// ─── detectNodeRunner ───────────────────────────────────────────────────────

func TestDetectNodeRunner_Default(t *testing.T) {
	dir := t.TempDir()
	if got := detectNodeRunner(dir); got != "npm" {
		t.Errorf("detectNodeRunner() = %q, want npm", got)
	}
}

func TestDetectNodeRunner_BunLockb(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodeRunner(dir); got != "bun" {
		t.Errorf("detectNodeRunner() = %q, want bun", got)
	}
}

func TestDetectNodeRunner_BunLock(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bun.lock"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectNodeRunner(dir); got != "bun" {
		t.Errorf("detectNodeRunner() = %q, want bun", got)
	}
}

// ─── hasPythonProject ───────────────────────────────────────────────────────

func TestHasPythonProject_SetupPy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasPythonProject(dir) {
		t.Error("hasPythonProject() with setup.py should return true")
	}
}

func TestHasPythonProject_PyprojectToml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasPythonProject(dir) {
		t.Error("hasPythonProject() with pyproject.toml should return true")
	}
}

func TestHasPythonProject_RequirementsTxt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasPythonProject(dir) {
		t.Error("hasPythonProject() with requirements.txt should return true")
	}
}

func TestHasPythonProject_Empty(t *testing.T) {
	if hasPythonProject(t.TempDir()) {
		t.Error("hasPythonProject() on empty dir should return false")
	}
}

// ─── parseLineFindings ──────────────────────────────────────────────────────

func TestParseLineFindings_Empty(t *testing.T) {
	findings := parseLineFindings("test", "")
	if len(findings) != 0 {
		t.Errorf("parseLineFindings() with empty = %v, want empty", findings)
	}
}

func TestParseLineFindings_MultipleLines(t *testing.T) {
	output := "error on line 1\nerror on line 2\n\nerror on line 4"
	findings := parseLineFindings("ruff", output)
	if len(findings) != 3 {
		t.Errorf("parseLineFindings() found %d, want 3", len(findings))
	}
	for _, f := range findings {
		if f.Source != "ruff" {
			t.Errorf("finding.Source = %q, want ruff", f.Source)
		}
	}
}

func TestParseLineFindings_IDsAreUniqueAndStable(t *testing.T) {
	output := "error A\nerror B\nerror C"
	findings1 := parseLineFindings("test", output)
	findings2 := parseLineFindings("test", output)

	// IDs must be unique within a single call
	seen := make(map[string]bool)
	for _, f := range findings1 {
		if seen[f.ID] {
			t.Errorf("duplicate ID: %q", f.ID)
		}
		seen[f.ID] = true
	}

	// IDs must be stable across calls with the same input
	for i := range findings1 {
		if findings1[i].ID != findings2[i].ID {
			t.Errorf("unstable ID: call1=%q call2=%q", findings1[i].ID, findings2[i].ID)
		}
	}

	// IDs from different sources must differ
	findings3 := parseLineFindings("other", output)
	for i := range findings1 {
		if findings1[i].ID == findings3[i].ID {
			t.Errorf("same message different source should produce different IDs")
		}
	}
}

// ─── DetectCheckers ────────────────────────────────────────────────────────

func TestDetectCheckers_GoProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkers := DetectCheckers(dir)
	// Expect: GoVet linter checker + SlopChecker (always included)
	if len(checkers) < 2 {
		t.Errorf("DetectCheckers(go project) = %d checkers, want >= 2", len(checkers))
	}
	// Last checker should always be slop
	if checkers[len(checkers)-1].Name() != slopCheckerName {
		t.Errorf("last checker = %q, want slop-detector", checkers[len(checkers)-1].Name())
	}
}

func TestDetectCheckers_NodeProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	checkers := DetectCheckers(dir)
	if len(checkers) < 2 {
		t.Errorf("DetectCheckers(node project) = %d checkers, want >= 2", len(checkers))
	}
	hasNode := false
	for _, c := range checkers {
		if c.Name() == "node-scripts" {
			hasNode = true
		}
	}
	if !hasNode {
		t.Error("DetectCheckers(node project) missing node-scripts checker")
	}
}

func TestDetectCheckers_PolyglotProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	checkers := DetectCheckers(dir)
	// Should detect BOTH Go and Node checkers (not mutually exclusive)
	if len(checkers) < 3 {
		t.Errorf("DetectCheckers(polyglot) = %d checkers, want >= 3 (go + node + slop)", len(checkers))
	}
}

func TestDetectCheckers_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	checkers := DetectCheckers(dir)
	// Should still include slop detector even with no project markers
	if len(checkers) != 1 {
		t.Errorf("DetectCheckers(empty) = %d checkers, want 1 (slop only)", len(checkers))
	}
	if checkers[0].Name() != slopCheckerName {
		t.Errorf("sole checker = %q, want slop-detector", checkers[0].Name())
	}
}

func TestDetectCheckers_PythonProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.ruff]"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkers := DetectCheckers(dir)
	hasPython := false
	for _, c := range checkers {
		if c.Name() == "python" {
			hasPython = true
		}
	}
	if !hasPython {
		t.Error("DetectCheckers(python project) missing python checker")
	}
}
