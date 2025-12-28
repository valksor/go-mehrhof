package github

import (
	"testing"
)

func TestExtractRepoFileLinks(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantLen   int
		wantIn    []string
		wantNotIn []string
	}{
		{
			name:    "empty body",
			body:    "",
			wantLen: 0,
		},
		{
			name:    "relative markdown link",
			body:    "See [documentation](./docs/README.md) for details.",
			wantLen: 1,
			wantIn:  []string{"./docs/README.md"},
		},
		{
			name:    "absolute path markdown link",
			body:    "Check [spec](/specs/feature.md) for requirements.",
			wantLen: 1,
			wantIn:  []string{"/specs/feature.md"},
		},
		{
			name:    "simple filename",
			body:    "See [notes](notes.md) for more info.",
			wantLen: 1,
			wantIn:  []string{"notes.md"},
		},
		{
			name:    "txt file link",
			body:    "Read the [changelog](./CHANGELOG.txt)",
			wantLen: 1,
			wantIn:  []string{"./CHANGELOG.txt"},
		},
		{
			name:    "yaml file link",
			body:    "Config in [config](./config.yaml)",
			wantLen: 1,
			wantIn:  []string{"./config.yaml"},
		},
		{
			name:    "yml file link",
			body:    "Config in [config](./config.yml)",
			wantLen: 1,
			wantIn:  []string{"./config.yml"},
		},
		{
			name:    "multiple links",
			body:    "See [doc1](./doc1.md) and [doc2](./doc2.md) and [doc3](/abs/doc3.md)",
			wantLen: 3,
			wantIn:  []string{"./doc1.md", "./doc2.md", "/abs/doc3.md"},
		},
		{
			name:      "ignores http URLs",
			body:      "See [external](https://example.com/doc.md) for reference.",
			wantLen:   0,
			wantNotIn: []string{"https://example.com/doc.md"},
		},
		{
			name:      "ignores http URLs mixed with local",
			body:      "See [local](./local.md) and [external](http://example.com/doc.md)",
			wantLen:   1,
			wantIn:    []string{"./local.md"},
			wantNotIn: []string{"http://example.com/doc.md"},
		},
		{
			name:    "deduplicates links",
			body:    "See [doc](./doc.md) and again [doc](./doc.md)",
			wantLen: 1,
			wantIn:  []string{"./doc.md"},
		},
		{
			name:    "ignores non-file extensions",
			body:    "See [code](./main.go) and [data](./data.json)",
			wantLen: 0,
		},
		{
			name:    "complex markdown body",
			body:    "# Title\n\nSome text\n\n[spec1](./docs/spec1.md)\n\n## Section\n\n[spec2](./docs/spec2.md)\n\nMore text with [inline](inline.md) link.",
			wantLen: 3,
			wantIn:  []string{"./docs/spec1.md", "./docs/spec2.md", "inline.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRepoFileLinks(tt.body)

			if len(got) != tt.wantLen {
				t.Errorf("ExtractRepoFileLinks() returned %d links, want %d: %v", len(got), tt.wantLen, got)
			}

			for _, want := range tt.wantIn {
				found := false
				for _, link := range got {
					if link == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ExtractRepoFileLinks() missing expected link %q", want)
				}
			}

			for _, notWant := range tt.wantNotIn {
				for _, link := range got {
					if link == notWant {
						t.Errorf("ExtractRepoFileLinks() should not contain %q", notWant)
					}
				}
			}
		})
	}
}
