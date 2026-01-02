package quality

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPHPCSFixerName(t *testing.T) {
	p := NewPHPCSFixer()
	if p.Name() != "php-cs-fixer" {
		t.Errorf("expected php-cs-fixer, got %s", p.Name())
	}
}

func TestPHPCSFixerParseOutput(t *testing.T) {
	p := NewPHPCSFixer()

	tests := []struct {
		name       string
		output     string
		wantIssues int
		wantPassed bool
	}{
		{
			name:       "empty",
			output:     "",
			wantIssues: 0,
			wantPassed: true,
		},
		{
			name:       "empty files array",
			output:     `{"files":[]}`,
			wantIssues: 0,
			wantPassed: true,
		},
		{
			name: "with one fixer",
			output: `{
				"files": [
					{
						"name": "src/Controller/HomeController.php",
						"appliedFixers": ["single_line_after_imports"]
					}
				]
			}`,
			wantIssues: 1,
			wantPassed: true, // style issues are warnings only
		},
		{
			name: "with multiple fixers",
			output: `{
				"files": [
					{
						"name": "src/Entity/User.php",
						"appliedFixers": ["blank_line_after_namespace", "no_unused_imports", "ordered_imports"]
					}
				]
			}`,
			wantIssues: 3,
			wantPassed: true,
		},
		{
			name: "with multiple files",
			output: `{
				"files": [
					{
						"name": "src/Controller/HomeController.php",
						"appliedFixers": ["single_line_after_imports"]
					},
					{
						"name": "src/Entity/User.php",
						"appliedFixers": ["blank_line_after_namespace", "ordered_imports"]
					}
				]
			}`,
			wantIssues: 3,
			wantPassed: true,
		},
		{
			name:       "success message in output",
			output:     `Fixed 0 of 10 files`,
			wantIssues: 0,
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.ParseOutput([]byte(tt.output))
			if err != nil {
				t.Fatalf("ParseOutput error: %v", err)
			}
			if len(result.Issues) != tt.wantIssues {
				t.Errorf("got %d issues, want %d", len(result.Issues), tt.wantIssues)
			}
			if result.Passed != tt.wantPassed {
				t.Errorf("got passed=%v, want %v", result.Passed, tt.wantPassed)
			}
		})
	}
}

func TestPHPCSFixerParseOutputInvalid(t *testing.T) {
	p := NewPHPCSFixer()

	// Invalid JSON that doesn't look like a success message
	_, err := p.ParseOutput([]byte(`{invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDetectForProjectPHP(t *testing.T) {
	tests := []struct {
		name       string
		createFile string
	}{
		{
			name:       "composer.json",
			createFile: "composer.json",
		},
		{
			name:       ".php-cs-fixer.php",
			createFile: ".php-cs-fixer.php",
		},
		{
			name:       ".php-cs-fixer.dist.php",
			createFile: ".php-cs-fixer.dist.php",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			markerFile := filepath.Join(tmpDir, tt.createFile)
			if err := os.WriteFile(markerFile, []byte(`{}`), 0o644); err != nil {
				t.Fatal(err)
			}

			r := NewRegistry()
			detected := r.DetectForProject(tmpDir)

			// Should detect php-cs-fixer if available
			var hasPHPCSFixer bool
			for _, l := range detected {
				if l.Name() == "php-cs-fixer" {
					hasPHPCSFixer = true
				}
			}

			// Only check if php-cs-fixer is available on system
			p := NewPHPCSFixer()
			if p.Available() && !hasPHPCSFixer {
				t.Errorf("expected php-cs-fixer to be detected for PHP project with %s", tt.createFile)
			}
		})
	}
}

func TestPHPCSFixerIssueDetails(t *testing.T) {
	p := NewPHPCSFixer()

	output := `{
		"files": [
			{
				"name": "src/Controller/HomeController.php",
				"appliedFixers": ["single_line_after_imports"]
			}
		]
	}`

	result, err := p.ParseOutput([]byte(output))
	if err != nil {
		t.Fatalf("ParseOutput error: %v", err)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.Path != "src/Controller/HomeController.php" {
		t.Errorf("expected path 'src/Controller/HomeController.php', got %q", issue.Path)
	}
	if issue.Rule != "single_line_after_imports" {
		t.Errorf("expected rule 'single_line_after_imports', got %q", issue.Rule)
	}
	if issue.Severity != SeverityWarning {
		t.Errorf("expected severity warning, got %s", issue.Severity)
	}
	if issue.Line != 0 {
		t.Errorf("expected line 0 (file-level), got %d", issue.Line)
	}
}
