package main

import (
	"errors"
	"testing"
)

func TestIsUnknownCommandError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"unknown command error", errors.New(`unknown command "foo" for "kvelmo"`), true},
		{"unknown command bare", errors.New("unknown command"), true},
		{"flag error", errors.New("flag needs an argument"), false},
		{"unrelated", errors.New("connection refused"), false},
		{"empty", errors.New(""), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUnknownCommandError(tc.err); got != tc.want {
				t.Errorf("isUnknownCommandError(%q) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestRootCommandRegistered confirms init() has populated rootCmd with the
// expected major subcommands. This is a smoke test ensuring importing the
// package doesn't panic and the command surface is wired up.
func TestRootCommandRegistered(t *testing.T) {
	subs := rootCmd.Commands()
	if len(subs) == 0 {
		t.Fatal("rootCmd has no subcommands after init()")
	}

	wantNames := []string{"version", "serve", "start", "status", "plan", "implement", "submit"}
	have := make(map[string]bool, len(subs))
	for _, s := range subs {
		have[s.Name()] = true
	}
	for _, name := range wantNames {
		if !have[name] {
			t.Errorf("rootCmd missing subcommand %q", name)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	// versionCmd is registered with a Run func; calling it should not panic
	// and should output a non-empty version string. Just verify it has the
	// Use field set as expected.
	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %q, want version", versionCmd.Use)
	}
	if versionCmd.Run == nil {
		t.Error("versionCmd.Run is nil")
	}
}

func TestLicenseCommand(t *testing.T) {
	if licenseCmd.Use != "license" {
		t.Errorf("licenseCmd.Use = %q, want license", licenseCmd.Use)
	}
	if licenseCmd.Run == nil {
		t.Error("licenseCmd.Run is nil")
	}
}

func TestGenManPagesCommand(t *testing.T) {
	if genManPagesCmd.Use != "gen-man-pages [directory]" {
		t.Errorf("genManPagesCmd.Use = %q", genManPagesCmd.Use)
	}
	if !genManPagesCmd.Hidden {
		t.Error("genManPagesCmd should be Hidden")
	}
}

// TestGenManPages exercises the RunE for the man-pages generator.
func TestGenManPages(t *testing.T) {
	dir := t.TempDir()
	if err := genManPagesCmd.RunE(genManPagesCmd, []string{dir}); err != nil {
		t.Errorf("gen-man-pages RunE: %v", err)
	}
}
