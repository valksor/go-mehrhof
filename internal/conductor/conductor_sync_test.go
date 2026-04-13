package conductor

import (
	"testing"

	"github.com/valksor/kvelmo/settings"
)

func TestGetMappedStatus(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		state    string
		mapping  map[string]string
		want     string
	}{
		{
			name:     "no mapping returns raw state",
			provider: "github",
			state:    "implementing",
			want:     "implementing",
		},
		{
			name:     "github with mapping",
			provider: "github",
			state:    "implementing",
			mapping:  map[string]string{"implementing": "in-progress", "submitted": "done"},
			want:     "in-progress",
		},
		{
			name:     "github unmapped state falls through",
			provider: "github",
			state:    "planning",
			mapping:  map[string]string{"implementing": "in-progress"},
			want:     "planning",
		},
		{
			name:     "nil settings returns raw state",
			provider: "github",
			state:    "planning",
			want:     "planning",
		},
		{
			name:     "unknown provider returns raw state",
			provider: "wrike",
			state:    "implementing",
			want:     "implementing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := settings.DefaultSettings()
			if tt.mapping != nil {
				switch tt.provider {
				case "github":
					s.Providers.GitHub.StatusMapping = tt.mapping
				case "linear":
					s.Providers.Linear.StatusMapping = tt.mapping
				case "jira":
					s.Providers.Jira.StatusMapping = tt.mapping
				}
			}

			got := getMappedStatus(s, tt.provider, tt.state)
			if got != tt.want {
				t.Errorf("getMappedStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetMappedStatusNilSettings(t *testing.T) {
	got := getMappedStatus(nil, "github", "implementing")
	if got != "implementing" {
		t.Errorf("getMappedStatus(nil) = %q, want %q", got, "implementing")
	}
}

func TestIsStatusSyncEnabled(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		setup    func(*settings.Settings)
		want     bool
	}{
		{
			name:     "disabled by default for github",
			provider: "github",
			setup:    func(_ *settings.Settings) {},
			want:     false,
		},
		{
			name:     "enabled for github",
			provider: "github",
			setup:    func(s *settings.Settings) { s.Providers.GitHub.StatusSync = true },
			want:     true,
		},
		{
			name:     "enabled for linear",
			provider: "linear",
			setup:    func(s *settings.Settings) { s.Providers.Linear.StatusSync = true },
			want:     true,
		},
		{
			name:     "enabled for jira",
			provider: "jira",
			setup:    func(s *settings.Settings) { s.Providers.Jira.StatusSync = true },
			want:     true,
		},
		{
			name:     "unsupported provider",
			provider: "wrike",
			setup:    func(_ *settings.Settings) {},
			want:     false,
		},
		{
			name:     "nil settings",
			provider: "github",
			setup:    nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s *settings.Settings
			if tt.setup != nil {
				s = settings.DefaultSettings()
				tt.setup(s)
			}

			got := isStatusSyncEnabled(s, tt.provider)
			if got != tt.want {
				t.Errorf("isStatusSyncEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinearStatusMapping(t *testing.T) {
	s := settings.DefaultSettings()
	s.Providers.Linear.StatusMapping = map[string]string{
		"implementing": "In Progress",
		"submitted":    "Done",
		"planning":     "Backlog",
	}

	got := getMappedStatus(s, "linear", "implementing")
	if got != "In Progress" {
		t.Errorf("getMappedStatus(linear, implementing) = %q, want %q", got, "In Progress")
	}

	got = getMappedStatus(s, "linear", "reviewing")
	if got != "reviewing" {
		t.Errorf("getMappedStatus(linear, reviewing) = %q, want %q", got, "reviewing")
	}
}

func TestJiraStatusMapping(t *testing.T) {
	s := settings.DefaultSettings()
	s.Providers.Jira.StatusMapping = map[string]string{
		"implementing": "In Development",
		"submitted":    "Done",
	}

	got := getMappedStatus(s, "jira", "implementing")
	if got != "In Development" {
		t.Errorf("getMappedStatus(jira, implementing) = %q, want %q", got, "In Development")
	}
}
