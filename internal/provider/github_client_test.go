package provider

import "testing"

func TestNewGitHubClient(t *testing.T) {
	client := newGitHubClient("", "")
	if client == nil {
		t.Error("client should not be nil")
	}
}

func TestNewGitHubClient_WithHost(t *testing.T) {
	client := newGitHubClient("", "https://github.example.com")
	if client == nil {
		t.Error("client should not be nil for enterprise host")
	}
}

func TestParseGitHubIDFull(t *testing.T) {
	tests := []struct {
		id        string
		wantOwner string
		wantRepo  string
		wantNum   int
		wantErr   bool
	}{
		{"owner/repo#123", "owner", "repo", 123, false},
		{"my-org/my-repo#456", "my-org", "my-repo", 456, false},
		{"invalid", "", "", 0, true},
		{"owner/repo#abc", "", "", 0, true},
		{"norepo#123", "", "", 0, true},
		{"owner/repo", "", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			owner, repo, num, err := parseGitHubIDFull(tt.id)
			if tt.wantErr && err == nil {
				t.Errorf("%s: expected error", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("%s: %v", tt.id, err)
			}
			if owner != tt.wantOwner || repo != tt.wantRepo || num != tt.wantNum {
				t.Errorf("%s: got %s/%s#%d, want %s/%s#%d", tt.id, owner, repo, num, tt.wantOwner, tt.wantRepo, tt.wantNum)
			}
		})
	}
}
