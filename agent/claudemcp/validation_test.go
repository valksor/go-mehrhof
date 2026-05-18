package claudemcp

import "testing"

func TestValidatePermissionMode(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"acceptEdits", PermissionModeAcceptEdits, false},
		{"auto", PermissionModeAuto, false},
		{"bypassPermissions", PermissionModeBypassPermission, false},
		{"dontAsk", PermissionModeDontAsk, false},
		{"default", PermissionModeDefault, false},
		{"empty defaults", "", false},
		{"typo", "acceptedits", true},
		{"unknown", "doWhatever", true},
		{"shell injection attempt", "; rm -rf /", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePermissionMode(tc.mode)
			if (err != nil) != tc.wantErr {
				t.Errorf("validatePermissionMode(%q) err=%v wantErr=%v", tc.mode, err, tc.wantErr)
			}
		})
	}
}

func TestIsAnthropicCredEnv(t *testing.T) {
	cases := []struct {
		kv   string
		want bool
	}{
		{"ANTHROPIC_API_KEY=sk-test", true},
		{"ANTHROPIC_AUTH_TOKEN=tok", true},
		{"ANTHROPIC_BEARER_TOKEN=bearer", true},
		{"PATH=/usr/bin", false},
		{"OPENAI_API_KEY=sk-other", false},
		{"ANTHROPIC_BASE_URL=https://example.com", false},
		{"anthropic_api_key=lowercase-not-matched", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.kv, func(t *testing.T) {
			if got := isAnthropicCredEnv(tc.kv); got != tc.want {
				t.Errorf("isAnthropicCredEnv(%q) = %v, want %v", tc.kv, got, tc.want)
			}
		})
	}
}
