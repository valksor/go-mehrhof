package git

import (
	"testing"
)

func TestParseLogOutputFull(t *testing.T) {
	// Format: SHA|message|author|date\x00body\x00 repeating.
	input := "abc123|Add feature|Alice|2026-05-01\x00Body line 1\nBody line 2\x00" +
		"def456|Fix bug|Bob|2026-05-02\x00Single line body\x00"

	entries := parseLogOutputFull(input)
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}

	if entries[0].SHA != "abc123" || entries[0].Message != "Add feature" {
		t.Errorf("entry 0 = %+v", entries[0])
	}
	if entries[0].Body != "Body line 1\nBody line 2" {
		t.Errorf("entry 0 body = %q", entries[0].Body)
	}
	if entries[1].SHA != "def456" || entries[1].Author != "Bob" {
		t.Errorf("entry 1 = %+v", entries[1])
	}
}

func TestParseLogOutputFull_Empty(t *testing.T) {
	if got := parseLogOutputFull(""); len(got) != 0 {
		t.Errorf("empty input: got %d entries", len(got))
	}
}

func TestParseLogOutputFull_SkipsBlankHeaders(t *testing.T) {
	input := "\x00ignored body\x00abc|m|a|d\x00body\x00"
	entries := parseLogOutputFull(input)
	if len(entries) != 1 {
		t.Errorf("expected 1 valid entry, got %d", len(entries))
	}
}

func TestParseLogOutputFull_SkipsMalformedHeader(t *testing.T) {
	input := "only|three|fields\x00body\x00abc|m|a|d\x00body\x00"
	entries := parseLogOutputFull(input)
	if len(entries) != 1 {
		t.Errorf("expected 1 valid entry, got %d", len(entries))
	}
}

func TestParseNumStat(t *testing.T) {
	input := `10	5	pkg/foo.go
2	0	pkg/bar.go
-	-	binary/image.png`

	got := parseNumStat(input)
	if got.Added != 12 {
		t.Errorf("Added = %d, want 12", got.Added)
	}
	if got.Removed != 5 {
		t.Errorf("Removed = %d, want 5", got.Removed)
	}
	if len(got.Files) != 3 {
		t.Errorf("Files = %v", got.Files)
	}
}

func TestParseNumStat_Empty(t *testing.T) {
	got := parseNumStat("")
	if got.Added != 0 || got.Removed != 0 || len(got.Files) != 0 {
		t.Errorf("empty input produced %+v", got)
	}
}

func TestParseNumStat_SkipsMalformed(t *testing.T) {
	input := "garbage\n10\t5\tpkg/x.go\n"
	got := parseNumStat(input)
	if got.Added != 10 || got.Removed != 5 || len(got.Files) != 1 {
		t.Errorf("expected one valid entry, got %+v", got)
	}
}

func TestParseNameStatusLine_Cases(t *testing.T) {
	cases := []struct {
		input      string
		wantPath   string
		wantStatus string
	}{
		{"A\tpkg/new.go", "pkg/new.go", "added"},
		{"M\tpkg/modified.go", "pkg/modified.go", "modified"},
		{"D\tpkg/deleted.go", "pkg/deleted.go", "deleted"},
		{"R100\told.go\tnew.go", "new.go", "renamed"},
		{"C50\tfrom.go\tto.go", "to.go", "renamed"},
		{"plain text", "plain text", "modified"}, // malformed
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			path, status := parseNameStatusLine(tc.input)
			if path != tc.wantPath {
				t.Errorf("path = %q, want %q", path, tc.wantPath)
			}
			if status != tc.wantStatus {
				t.Errorf("status = %q, want %q", status, tc.wantStatus)
			}
		})
	}
}
