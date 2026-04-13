package provider

import "testing"

func TestNewGitLabClient(t *testing.T) {
	client, err := newGitLabClient("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("client should not be nil")
	}
}

func TestNewGitLabClient_CustomHost(t *testing.T) {
	client, err := newGitLabClient("", "https://gitlab.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("client should not be nil with custom host")
	}
}

func TestParseGitLabID(t *testing.T) {
	tests := []struct {
		id          string
		wantProject string
		wantNum     int
		wantIsMR    bool
		wantErr     bool
	}{
		{"group/project#123", "group/project", 123, false, false},
		{"group/sub/project#456", "group/sub/project", 456, false, false},
		{"group/project!789", "group/project", 789, true, false},
		{"group/sub/project!101", "group/sub/project", 101, true, false},
		{"invalid", "", 0, false, true},
		{"no-separator", "", 0, false, true},
		{"group/project#abc", "", 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			project, num, isMR, err := parseGitLabID(tt.id)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %s", tt.id)
				}

				return
			}
			if err != nil {
				t.Errorf("unexpected error for %s: %v", tt.id, err)

				return
			}
			if project != tt.wantProject {
				t.Errorf("project: got %s, want %s", project, tt.wantProject)
			}
			if num != tt.wantNum {
				t.Errorf("number: got %d, want %d", num, tt.wantNum)
			}
			if isMR != tt.wantIsMR {
				t.Errorf("isMR: got %v, want %v", isMR, tt.wantIsMR)
			}
		})
	}
}
