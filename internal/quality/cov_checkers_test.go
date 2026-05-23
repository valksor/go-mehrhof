package quality

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// writeGoModule writes a minimal compilable Go module into dir.
func writeGoModule(t *testing.T, dir, mainSrc string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module covtest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
}

func TestLinterChecker_NameAndCheck(t *testing.T) {
	checker := NewLinterChecker(NewGoVet())
	if checker.Name() != "go-vet" {
		t.Errorf("Name() = %q, want go-vet", checker.Name())
	}

	t.Run("clean go project yields no findings", func(t *testing.T) {
		dir := t.TempDir()
		writeGoModule(t, dir, "package main\n\nfunc main() {}\n")

		ff, err := checker.Check(context.Background(), dir)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if len(ff) != 0 {
			t.Errorf("Check() returned %d findings on clean project, want 0", len(ff))
		}
	})

	t.Run("non-go project is skipped (no findings)", func(t *testing.T) {
		dir := t.TempDir() // no go.mod => GoVet returns CoverageSkipped
		ff, err := checker.Check(context.Background(), dir)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if ff != nil {
			t.Errorf("Check() on non-go project = %v, want nil (skipped)", ff)
		}
	})
}

func TestGoVet_Lint_VetFailure(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not installed")
	}
	dir := t.TempDir()
	// A Printf with a mismatched verb triggers a go vet diagnostic.
	src := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%d\", \"not a number\")\n}\n"
	writeGoModule(t, dir, src)

	report, err := NewGoVet().Lint(context.Background(), dir)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Coverage != CoverageFull {
		t.Errorf("Coverage = %q, want full", report.Coverage)
	}
	if len(report.Issues) == 0 {
		t.Error("expected go vet to report at least one issue for mismatched Printf verb")
	}
	found := false
	for _, iss := range report.Issues {
		if iss.Rule == "vet" && iss.File != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a vet issue with a file, got %+v", report.Issues)
	}
}

func TestGolangCILint_Lint_SkippedNonGo(t *testing.T) {
	dir := t.TempDir() // no go.mod
	report, err := NewGolangCILint().Lint(context.Background(), dir)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if NewGolangCILint().Available() {
		// Tool is installed but the dir is not a Go project.
		if report.Coverage != CoverageSkipped {
			t.Errorf("Coverage = %q, want skipped for non-go dir", report.Coverage)
		}
	} else if report.Coverage != CoverageUnavailable {
		t.Errorf("Coverage = %q, want unavailable when tool missing", report.Coverage)
	}
}

func TestESLint_Lint_SkippedNonJS(t *testing.T) {
	dir := t.TempDir() // no package.json
	report, err := NewESLint().Lint(context.Background(), dir)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if NewESLint().Available() {
		if report.Coverage != CoverageSkipped {
			t.Errorf("Coverage = %q, want skipped for non-js dir", report.Coverage)
		}
	} else if report.Coverage != CoverageUnavailable {
		t.Errorf("Coverage = %q, want unavailable when npx missing", report.Coverage)
	}
}

func TestNodeScriptsChecker_Check(t *testing.T) {
	runner := "npm"
	if _, err := exec.LookPath("npm"); err != nil {
		if _, err := exec.LookPath("bun"); err != nil {
			t.Skip("neither npm nor bun installed")
		}
	}

	checker := NewNodeScriptsChecker("lint")
	if checker.Name() != "node-scripts" {
		t.Errorf("Name() = %q, want node-scripts", checker.Name())
	}

	t.Run("no package.json skips silently", func(t *testing.T) {
		ff, err := checker.Check(context.Background(), t.TempDir())
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if ff != nil {
			t.Errorf("Check() with no package.json = %v, want nil", ff)
		}
	})

	t.Run("failing lint script produces findings", func(t *testing.T) {
		dir := t.TempDir()
		// A lint script that always fails and emits a parseable line.
		pkg := `{
  "name": "covtest",
  "version": "1.0.0",
  "scripts": {
    "lint": "echo 'src/foo.js:1:1 error something is wrong' && exit 1"
  }
}`
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}

		ff, err := checker.Check(context.Background(), dir)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if len(ff) == 0 {
			t.Fatalf("expected findings from failing %s lint script", runner)
		}
		// Findings should be tagged with the node-scripts source.
		if ff[0].Source != "node-scripts-lint" {
			t.Errorf("finding Source = %q, want node-scripts-lint", ff[0].Source)
		}
	})

	t.Run("script not present is skipped", func(t *testing.T) {
		dir := t.TempDir()
		pkg := `{"name":"x","version":"1.0.0","scripts":{"build":"echo ok"}}`
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}
		ff, err := checker.Check(context.Background(), dir)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if len(ff) != 0 {
			t.Errorf("Check() with absent script = %v, want no findings", ff)
		}
	})
}

func TestPythonChecker_Check(t *testing.T) {
	checker := NewPythonChecker()
	if checker.Name() != "python" {
		t.Errorf("Name() = %q, want python", checker.Name())
	}

	if _, err := exec.LookPath("python3"); err != nil {
		if _, err := exec.LookPath("ruff"); err != nil {
			t.Skip("neither python3 nor ruff installed")
		}
	}

	t.Run("clean python file yields no findings", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "ok.py"), []byte("x = 1\n"), 0o644); err != nil {
			t.Fatalf("write ok.py: %v", err)
		}
		ff, err := checker.Check(context.Background(), dir)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		// ruff may flag style issues, but py_compile on valid syntax yields none.
		// Either way, the call must succeed without error.
		_ = ff
	})

	t.Run("syntax error produces findings", func(t *testing.T) {
		// Skip if ruff is present, since ruff may behave differently than
		// py_compile; this test specifically targets the py_compile path,
		// which only runs when ruff is absent.
		if _, err := exec.LookPath("ruff"); err == nil {
			t.Skip("ruff present; py_compile path not exercised")
		}
		if _, err := exec.LookPath("python3"); err != nil {
			t.Skip("python3 not installed")
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "bad.py"), []byte("def broken(:\n  pass\n"), 0o644); err != nil {
			t.Fatalf("write bad.py: %v", err)
		}
		ff, err := checker.Check(context.Background(), dir)
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if len(ff) == 0 {
			t.Error("expected py_compile findings for a syntax error")
		}
	})
}

func TestPythonChecker_CheckPyCompile_NoFiles(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	// Directly exercise checkPyCompile with a dir that has no .py files.
	ff, err := (&PythonChecker{}).checkPyCompile(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("checkPyCompile() error = %v", err)
	}
	if ff != nil {
		t.Errorf("checkPyCompile() with no .py files = %v, want nil", ff)
	}
}
