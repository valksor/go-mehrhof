package conductor

import (
	"testing"

	"github.com/valksor/kvelmo/internal/findings"
	"github.com/valksor/kvelmo/settings"
)

func TestGetConsensusConfig_Nil(t *testing.T) {
	c := &Conductor{}

	// No settings loaded — should return nil.
	cfg := c.getConsensusConfig()
	if cfg != nil {
		t.Fatal("expected nil consensus config when no settings loaded")
	}
}

func TestGetConsensusConfig_Present(t *testing.T) {
	s := settings.DefaultSettings()
	s.Agent.Consensus = &settings.ConsensusConfig{
		Enabled:      true,
		Agents:       []string{"claude", "codex"},
		MinAgreement: 2,
	}

	c := &Conductor{}
	c.cachedSettings.Store(s)

	cfg := c.getConsensusConfig()
	if cfg == nil {
		t.Fatal("expected non-nil consensus config")
	}

	if !cfg.Enabled {
		t.Error("expected consensus to be enabled")
	}

	if len(cfg.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(cfg.Agents))
	}

	if cfg.MinAgreement != 2 {
		t.Errorf("expected min agreement 2, got %d", cfg.MinAgreement)
	}
}

func TestGetConsensusConfig_NotConfigured(t *testing.T) {
	s := settings.DefaultSettings()

	c := &Conductor{}
	c.cachedSettings.Store(s)

	cfg := c.getConsensusConfig()
	if cfg != nil {
		t.Error("expected nil consensus config when not configured")
	}
}

func TestParseConsensusFindings_Empty(t *testing.T) {
	result := parseConsensusFindings("test-agent", "")
	if len(result) != 0 {
		t.Errorf("expected 0 findings from empty output, got %d", len(result))
	}
}

func TestParseConsensusFindings_WithFileRef(t *testing.T) {
	output := "main.go:42: unused variable foo\nutils.go:10:5: missing return"
	result := parseConsensusFindings("claude", output)

	if len(result) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(result))
	}

	if result[0].File != "main.go" {
		t.Errorf("expected file main.go, got %s", result[0].File)
	}

	if result[0].Line != 42 {
		t.Errorf("expected line 42, got %d", result[0].Line)
	}

	if result[1].File != "utils.go" {
		t.Errorf("expected file utils.go, got %s", result[1].File)
	}
}

func TestParseConsensusFindings_PlainText(t *testing.T) {
	output := "The code has some issues\nNeed to fix error handling"
	result := parseConsensusFindings("codex", output)

	if len(result) != 2 {
		t.Fatalf("expected 2 findings from plain text, got %d", len(result))
	}

	for _, f := range result {
		if f.Source != "codex" {
			t.Errorf("expected source codex, got %s", f.Source)
		}

		if f.Category != findings.CategoryQuality {
			t.Errorf("expected category quality, got %s", f.Category)
		}
	}
}

func TestParseFileLineRule(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		wantOK bool
		wantF  string
		wantL  int
		wantR  string
	}{
		{
			name:   "standard file:line:msg",
			line:   "main.go:42: unchecked error return",
			wantOK: true,
			wantF:  "main.go",
			wantL:  42,
			wantR:  "unchecked error return",
		},
		{
			name:   "file:line:col:msg",
			line:   "pkg/foo.go:10:5: missing nil check",
			wantOK: true,
			wantF:  "pkg/foo.go",
			wantL:  10,
			wantR:  "missing nil check",
		},
		{
			name:   "no colon",
			line:   "just some text",
			wantOK: false,
		},
		{
			name:   "not a number",
			line:   "file.go:abc: msg",
			wantOK: false,
		},
		{
			name:   "empty file",
			line:   ":42: msg",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, l, r, ok := parseFileLineRule(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if !ok {
				return
			}

			if f != tt.wantF {
				t.Errorf("file = %q, want %q", f, tt.wantF)
			}

			if l != tt.wantL {
				t.Errorf("line = %d, want %d", l, tt.wantL)
			}

			if r != tt.wantR {
				t.Errorf("rule = %q, want %q", r, tt.wantR)
			}
		})
	}
}
