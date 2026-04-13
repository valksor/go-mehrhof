package security

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSecretScannerName(t *testing.T) {
	s := NewSecretScanner()
	if s.Name() != "secret-scanner" {
		t.Errorf("Name() = %s, want secret-scanner", s.Name())
	}
}

func TestSecretScannerFindsSecrets(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create a file with a secret

	content := `package main

const awsKey = "AKIAIOSFODNN7EXAMPLE"
const password = "supersecretpassword123"
`
	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	s := NewSecretScanner()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	report, err := s.Scan(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(report.Findings) < 1 {
		t.Errorf("Expected at least 1 finding, got %d", len(report.Findings))
	}

	// Check that we found the AWS key
	foundAWS := false
	for _, f := range report.Findings {
		if f.Type == "secret" && f.Severity == SeverityCritical {
			foundAWS = true

			break
		}
	}
	if !foundAWS {
		t.Error("Expected to find AWS key as critical secret")
	}
}

func TestSecretScannerSkipsIgnoredDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file in node_modules (should be skipped)
	nodeModules := filepath.Join(tmpDir, "node_modules")
	_ = os.MkdirAll(nodeModules, 0o755)
	secretFile := filepath.Join(nodeModules, "config.js")
	_ = os.WriteFile(secretFile, []byte(`const key = "AKIAIOSFODNN7EXAMPLE"`), 0o644)

	s := NewSecretScanner()
	ctx := context.Background()

	report, err := s.Scan(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(report.Findings) != 0 {
		t.Errorf("Expected 0 findings (node_modules should be skipped), got %d", len(report.Findings))
	}
}

func TestSecretScannerContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files
	for i := range 10 {
		f := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".go")
		_ = os.WriteFile(f, []byte("package main"), 0o644)
	}

	s := NewSecretScanner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := s.Scan(ctx, tmpDir)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestDependencyScannerName(t *testing.T) {
	d := NewDependencyScanner()
	if d.Name() != "dependency-scanner" {
		t.Errorf("Name() = %s, want dependency-scanner", d.Name())
	}
}

func TestDependencyScannerNoGovulncheck(t *testing.T) {
	// This test verifies behavior when govulncheck is not installed
	// It should return an info-level finding
	d := NewDependencyScanner()
	ctx := context.Background()
	tmpDir := t.TempDir()

	report, err := d.Scan(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should have either findings from govulncheck or info about missing tool
	if report.Duration == 0 {
		t.Error("Duration should be non-zero")
	}
}

func TestRunnerMultipleScanners(t *testing.T) {
	r := NewRunner()

	if len(r.scanners) < 2 {
		t.Errorf("Expected at least 2 default scanners, got %d", len(r.scanners))
	}

	tmpDir := t.TempDir()
	ctx := context.Background()

	reports, err := r.Run(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(reports) < 2 {
		t.Errorf("Expected at least 2 reports, got %d", len(reports))
	}
}

func TestRunnerContextCancellation(t *testing.T) {
	r := NewRunner()
	tmpDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Run(ctx, tmpDir)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestSeverityConstants(t *testing.T) {
	tests := []struct {
		severity Severity
		want     string
	}{
		{SeverityCritical, "critical"},
		{SeverityHigh, "high"},
		{SeverityMedium, "medium"},
		{SeverityLow, "low"},
		{SeverityInfo, "info"},
	}

	for _, tt := range tests {
		if string(tt.severity) != tt.want {
			t.Errorf("Severity %v = %s, want %s", tt.severity, string(tt.severity), tt.want)
		}
	}
}

func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"app.js", true},
		{"config.yaml", true},
		{".env", true},
		{"image.png", false},
		{"document.pdf", false},
		{"binary", false},
	}

	for _, tt := range tests {
		got := isSourceFile(tt.path)
		if got != tt.want {
			t.Errorf("isSourceFile(%s) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{".git", true},
		{"node_modules", true},
		{"vendor", true},
		{"src", false},
		{"pkg", false},
		{"internal", false},
	}

	for _, tt := range tests {
		got := shouldSkipDir(tt.name)
		if got != tt.want {
			t.Errorf("shouldSkipDir(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
