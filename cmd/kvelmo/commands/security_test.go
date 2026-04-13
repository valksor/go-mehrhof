package commands

import (
	"testing"

	"github.com/valksor/kvelmo/internal/security"
	"github.com/valksor/kvelmo/meta"
)

func TestSecurityCommand(t *testing.T) {
	if SecurityCmd.Use != "security" {
		t.Errorf("Use = %s, want security", SecurityCmd.Use)
	}
	if SecurityCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
	if SecurityCmd.Long == "" {
		t.Error("Long description should not be empty")
	}
}

func TestSecurityScanSubcommand(t *testing.T) {
	// Verify scan subcommand exists under SecurityCmd
	var found bool
	for _, sub := range SecurityCmd.Commands() {
		if sub.Use == "scan [dir]" {
			found = true

			break
		}
	}
	if !found {
		t.Error("missing 'scan [dir]' subcommand on security")
	}
}

func TestSecurityScanFlags(t *testing.T) {
	if f := securityScanCmd.Flags().Lookup("json"); f == nil {
		t.Error("--json flag should exist on security scan")
	}
}

func TestSecurityScanCommand_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runSecurityScan(securityScanCmd, nil)
	if err == nil {
		t.Fatal("runSecurityScan() expected error when no global socket running, got nil")
	}
}

func TestSecurityScanCommand_WithDir_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runSecurityScan(securityScanCmd, []string{t.TempDir()})
	if err == nil {
		t.Fatal("runSecurityScan() expected error when no global socket running, got nil")
	}
}

func TestSeverityLabel(t *testing.T) {
	tests := []struct {
		severity security.Severity
		want     string
	}{
		{security.SeverityCritical, "CRITICAL"},
		{security.SeverityHigh, "HIGH"},
		{security.SeverityMedium, "MEDIUM"},
		{security.SeverityLow, "LOW"},
		{security.SeverityInfo, "INFO"},
		{security.Severity("unknown"), "unknown"},
	}
	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := severityLabel(tt.severity)
			if got != tt.want {
				t.Errorf("severityLabel(%s) = %s, want %s", tt.severity, got, tt.want)
			}
		})
	}
}

func TestFormatScanners(t *testing.T) {
	tests := []struct {
		name     string
		scanners []string
		want     string
	}{
		{
			name:     "no scanners",
			scanners: nil,
			want:     "no scanners",
		},
		{
			name:     "empty slice",
			scanners: []string{},
			want:     "no scanners",
		},
		{
			name:     "single scanner",
			scanners: []string{"secrets"},
			want:     "secrets",
		},
		{
			name:     "two scanners",
			scanners: []string{"secrets", "deps"},
			want:     "secrets, deps",
		},
		{
			name:     "three scanners",
			scanners: []string{"secrets", "deps", "sast"},
			want:     "secrets, deps, sast",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatScanners(tt.scanners)
			if got != tt.want {
				t.Errorf("formatScanners(%v) = %q, want %q", tt.scanners, got, tt.want)
			}
		})
	}
}
