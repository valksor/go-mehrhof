package meta_test

import (
	"strings"
	"testing"

	"github.com/valksor/kvelmo/meta"
)

func TestDocsURL(t *testing.T) {
	original := meta.Version
	t.Cleanup(func() { meta.Version = original })

	tests := []struct {
		version string
		want    string
	}{
		{"v1.0.0", meta.DocsBaseLatest},
		{"v2.3.5", meta.DocsBaseLatest},
		{"v0.1.0-rc1", meta.DocsBaseLatest},
		{"v", meta.DocsBaseLatest},
		{"dev", meta.DocsBaseNightly},
		{"nightly", meta.DocsBaseNightly},
		{"", meta.DocsBaseNightly},
		{"1.0.0", meta.DocsBaseNightly},
		{"main", meta.DocsBaseNightly},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			meta.Version = tt.version
			got := meta.DocsURL()
			if got != tt.want {
				t.Errorf("DocsURL() with Version=%q = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	if meta.Name == "" {
		t.Error("Name must not be empty")
	}
	if !strings.HasPrefix(meta.GlobalDir, ".") {
		t.Errorf("GlobalDir %q should start with a dot", meta.GlobalDir)
	}
	if !strings.Contains(meta.GlobalDir, meta.Name) {
		t.Errorf("GlobalDir %q should contain Name %q", meta.GlobalDir, meta.Name)
	}
	if !strings.HasSuffix(meta.BranchPrefix, "/") {
		t.Errorf("BranchPrefix %q should end with /", meta.BranchPrefix)
	}
	if !strings.Contains(meta.CommitTag, meta.Name) {
		t.Errorf("CommitTag %q should contain Name %q", meta.CommitTag, meta.Name)
	}
	if meta.EnvPrefix != strings.ToUpper(meta.Name) {
		t.Errorf("EnvPrefix %q should be uppercase of Name %q", meta.EnvPrefix, meta.Name)
	}
	if meta.ProjectWorkDir != "."+meta.Name {
		t.Errorf("ProjectWorkDir %q should be .%s", meta.ProjectWorkDir, meta.Name)
	}
}
