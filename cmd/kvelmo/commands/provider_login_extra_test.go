package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestConfirmOverride(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"\n", false},
		{"anything else\n", false},
	}

	for _, tc := range cases {
		t.Run(strings.TrimSpace(tc.input), func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetIn(strings.NewReader(tc.input))
			cmd.SetOut(&bytes.Buffer{})

			if got := confirmOverride(cmd); got != tc.want {
				t.Errorf("confirmOverride(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestPrintTokenHelp_RealConfig(t *testing.T) {
	var buf bytes.Buffer
	// Use a real provider config so we hit the optional fields.
	cfg, ok := providerLoginConfigs["github"]
	if !ok {
		t.Fatal("providerLoginConfigs[github] missing — test fixture broken")
	}

	printTokenHelp(&buf, cfg)

	out := buf.String()
	if !strings.Contains(out, "GitHub") {
		t.Errorf("expected provider name in help output:\n%s", out)
	}
	if !strings.Contains(out, "Get token:") {
		t.Errorf("expected 'Get token:' label in help output:\n%s", out)
	}
}

func TestRunProviderLogin_UnknownProvider(t *testing.T) {
	cmd := &cobra.Command{}
	fn := runProviderLogin("not-a-real-provider")
	err := fn(cmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error = %q, want 'unknown provider'", err.Error())
	}
}
