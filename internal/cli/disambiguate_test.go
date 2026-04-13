package cli

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "app"}
	root.AddCommand(
		&cobra.Command{Use: "start", Short: "Start a task"},
		&cobra.Command{Use: "status", Short: "Show status"},
		&cobra.Command{Use: "stop", Short: "Stop a task"},
		&cobra.Command{Use: "stats", Short: "Show statistics"},
		&cobra.Command{Use: "implement", Short: "Implement code", Aliases: []string{"impl"}},
		&cobra.Command{Use: "plan", Short: "Plan a task"},
		&cobra.Command{Use: "submit", Short: "Submit PR"},
		&cobra.Command{Use: "version", Short: "Print version"},
	)

	return root
}

func TestDisambiguateCommand(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		wantCmd    string
		wantNames  []string
		wantNilCmd bool
	}{
		{
			name:    "exact match",
			prefix:  "start",
			wantCmd: "start",
		},
		{
			name:    "exact alias match",
			prefix:  "impl",
			wantCmd: "implement",
		},
		{
			name:    "unique prefix returns command",
			prefix:  "imp",
			wantCmd: "implement",
		},
		{
			name:    "unique prefix plan",
			prefix:  "pl",
			wantCmd: "plan",
		},
		{
			name:    "unique prefix version",
			prefix:  "v",
			wantCmd: "version",
		},
		{
			name:       "ambiguous prefix returns suggestions",
			prefix:     "st",
			wantNilCmd: true,
			wantNames:  []string{"start", "stats", "status", "stop"},
		},
		{
			name:       "ambiguous prefix sta",
			prefix:     "sta",
			wantNilCmd: true,
			wantNames:  []string{"start", "stats", "status"},
		},
		{
			name:       "no match returns nil and empty",
			prefix:     "xyz",
			wantNilCmd: true,
			wantNames:  nil,
		},
		{
			name:       "empty prefix returns nil",
			prefix:     "",
			wantNilCmd: true,
			wantNames:  nil,
		},
		{
			name:    "unique prefix sub",
			prefix:  "sub",
			wantCmd: "submit",
		},
	}

	root := newTestRoot()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, names := DisambiguateCommand(root, tt.prefix)

			if tt.wantNilCmd {
				if cmd != nil {
					t.Errorf("expected nil command, got %q", cmd.Name())
				}
			} else {
				if cmd == nil {
					t.Fatalf("expected command %q, got nil", tt.wantCmd)
				}
				if cmd.Name() != tt.wantCmd {
					t.Errorf("expected command %q, got %q", tt.wantCmd, cmd.Name())
				}
			}

			if tt.wantNames == nil && names != nil {
				t.Errorf("expected nil names, got %v", names)
			}

			if tt.wantNames != nil {
				if len(names) != len(tt.wantNames) {
					t.Fatalf("expected %d suggestions, got %d: %v", len(tt.wantNames), len(names), names)
				}
				sorted := slices.Clone(names)
				slices.Sort(sorted)
				for i, want := range tt.wantNames {
					if sorted[i] != want {
						t.Errorf("suggestion[%d]: expected %q, got %q", i, want, sorted[i])
					}
				}
			}
		})
	}
}

func TestFindPrefixMatches(t *testing.T) {
	root := newTestRoot()

	tests := []struct {
		name      string
		prefix    string
		wantCount int
	}{
		{"exact match returns one", "start", 1},
		{"unique prefix", "imp", 1},
		{"ambiguous prefix", "st", 4},
		{"no match", "xyz", 0},
		{"empty prefix", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := FindPrefixMatches(root, tt.prefix)
			if len(matches) != tt.wantCount {
				t.Errorf("expected %d matches, got %d", tt.wantCount, len(matches))
			}
		})
	}
}

func TestFormatAmbiguousError(t *testing.T) {
	result := FormatAmbiguousError("st", []string{"start", "status", "stop"})

	if result == "" {
		t.Fatal("expected non-empty error string")
	}

	for _, cmd := range []string{"start", "status", "stop"} {
		if !contains(result, cmd) {
			t.Errorf("expected error to contain %q", cmd)
		}
	}

	if !contains(result, `"st"`) {
		t.Error("expected error to contain the prefix")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

func TestDisambiguateCommandAliasPrefix(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	root.AddCommand(
		&cobra.Command{Use: "implement", Aliases: []string{"impl"}},
		&cobra.Command{Use: "import", Short: "Import data"},
	)

	// "imp" is ambiguous between implement and import
	cmd, names := DisambiguateCommand(root, "imp")
	if cmd != nil {
		t.Errorf("expected nil command for ambiguous prefix, got %q", cmd.Name())
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 suggestions, got %d: %v", len(names), names)
	}
}
