package agent

import "testing"

func TestRunPreflight(t *testing.T) {
	result := RunPreflight()

	if result == nil {
		t.Fatal("RunPreflight returned nil")
	}

	// Should always have at least git, claude, codex checks
	if len(result.Checks) < 3 {
		t.Errorf("expected at least 3 checks, got %d", len(result.Checks))
	}

	// Git should always be available in CI/dev
	gitCheck := result.Checks[0]
	if gitCheck.Name != "git" {
		t.Errorf("first check name = %q, want git", gitCheck.Name)
	}
}

func TestPreflightResult_HasIssues(t *testing.T) {
	tests := []struct {
		name   string
		checks []CheckResult
		want   bool
	}{
		{
			name: "all passed",
			checks: []CheckResult{
				{Status: CheckPassed},
				{Status: CheckPassed},
			},
			want: false,
		},
		{
			name: "one failed",
			checks: []CheckResult{
				{Status: CheckPassed},
				{Status: CheckFailed},
			},
			want: true,
		},
		{
			name: "warning only",
			checks: []CheckResult{
				{Status: CheckPassed},
				{Status: CheckWarning},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &PreflightResult{Checks: tt.checks}
			if got := r.HasIssues(); got != tt.want {
				t.Errorf("HasIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"git version 2.43.0", "2.43.0"},
		{"claude 1.2.3", "1.2.3"},
		{"1.0.0", "1.0.0"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanVersion(tt.input)
			if got != tt.want {
				t.Errorf("cleanVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
