package gitlab

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/valksor/go-mehrhof/internal/provider"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *Ref
		wantErr   bool
		errString string
	}{
		{
			name:  "bare number",
			input: "5",
			want: &Ref{
				IssueIID:   5,
				IsExplicit: false,
			},
		},
		{
			name:  "hash number",
			input: "#123",
			want: &Ref{
				IssueIID:   123,
				IsExplicit: false,
			},
		},
		{
			name:  "gitlab scheme bare number",
			input: "gitlab:42",
			want: &Ref{
				IssueIID:   42,
				IsExplicit: false,
			},
		},
		{
			name:  "gl scheme bare number",
			input: "gl:99",
			want: &Ref{
				IssueIID:   99,
				IsExplicit: false,
			},
		},
		{
			name:  "explicit group/project#number",
			input: "group/project#5",
			want: &Ref{
				ProjectPath: "group/project",
				IssueIID:    5,
				IsExplicit:  true,
			},
		},
		{
			name:  "gitlab scheme with explicit project",
			input: "gitlab:myorg/myproject#100",
			want: &Ref{
				ProjectPath: "myorg/myproject",
				IssueIID:    100,
				IsExplicit:  true,
			},
		},
		{
			name:  "gl scheme with explicit project",
			input: "gl:acme/widgets#1",
			want: &Ref{
				ProjectPath: "acme/widgets",
				IssueIID:    1,
				IsExplicit:  true,
			},
		},
		{
			name:  "nested group path",
			input: "group/subgroup/project#42",
			want: &Ref{
				ProjectPath: "group/subgroup/project",
				IssueIID:    42,
				IsExplicit:  true,
			},
		},
		{
			name:  "hyphenated names",
			input: "gitlab:my-group/my-project#7",
			want: &Ref{
				ProjectPath: "my-group/my-project",
				IssueIID:    7,
				IsExplicit:  true,
			},
		},
		{
			name:  "underscored names",
			input: "my_org/my_project#10",
			want: &Ref{
				ProjectPath: "my_org/my_project",
				IssueIID:    10,
				IsExplicit:  true,
			},
		},
		{
			name:  "project ID format",
			input: "12345#678",
			want: &Ref{
				ProjectID:  12345,
				IssueIID:   678,
				IsExplicit: true,
			},
		},
		{
			name:  "gitlab scheme with project ID",
			input: "gitlab:12345#678",
			want: &Ref{
				ProjectID:  12345,
				IssueIID:   678,
				IsExplicit: true,
			},
		},
		{
			name:      "empty string",
			input:     "",
			wantErr:   true,
			errString: "empty reference",
		},
		{
			name:      "invalid format",
			input:     "not-a-reference",
			wantErr:   true,
			errString: "unrecognized format",
		},
		{
			name:  "zero issue number",
			input: "0",
			want: &Ref{
				IssueIID:   0,
				IsExplicit: false,
			},
		},
		{
			name:      "negative number",
			input:     "-5",
			wantErr:   true,
			errString: "unrecognized format",
		},
		{
			name:      "spaces before scheme cause error",
			input:     "  gitlab:group/project#5",
			wantErr:   true,
			errString: "unrecognized format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReference(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseReference(%q) expected error containing %q, got nil", tt.input, tt.errString)

					return
				}
				if tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("ParseReference(%q) error = %v, want error containing %q", tt.input, err, tt.errString)
				}

				return
			}

			if err != nil {
				t.Errorf("ParseReference(%q) unexpected error: %v", tt.input, err)

				return
			}

			if got.ProjectPath != tt.want.ProjectPath {
				t.Errorf("ParseReference(%q).ProjectPath = %q, want %q", tt.input, got.ProjectPath, tt.want.ProjectPath)
			}
			if got.ProjectID != tt.want.ProjectID {
				t.Errorf("ParseReference(%q).ProjectID = %d, want %d", tt.input, got.ProjectID, tt.want.ProjectID)
			}
			if got.IssueIID != tt.want.IssueIID {
				t.Errorf("ParseReference(%q).IssueIID = %d, want %d", tt.input, got.IssueIID, tt.want.IssueIID)
			}
			if got.IsExplicit != tt.want.IsExplicit {
				t.Errorf("ParseReference(%q).IsExplicit = %v, want %v", tt.input, got.IsExplicit, tt.want.IsExplicit)
			}
		})
	}
}

func TestRefString(t *testing.T) {
	tests := []struct {
		name string
		ref  *Ref
		want string
	}{
		{
			name: "explicit project path",
			ref: &Ref{
				ProjectPath: "group/project",
				IssueIID:    123,
				IsExplicit:  true,
			},
			want: "group/project#123",
		},
		{
			name: "simple reference",
			ref: &Ref{
				IssueIID:   42,
				IsExplicit: false,
			},
			want: "#42",
		},
		{
			name: "project ID format",
			ref: &Ref{
				ProjectID:  12345,
				IssueIID:   678,
				IsExplicit: true,
			},
			want: "12345#678",
		},
		{
			name: "project ID takes precedence over path",
			ref: &Ref{
				ProjectPath: "group/project",
				ProjectID:   12345,
				IssueIID:    5,
			},
			want: "group/project#5",
		},
		{
			name: "empty project with issue",
			ref: &Ref{
				IssueIID: 7,
			},
			want: "#7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.String()
			if got != tt.want {
				t.Errorf("Ref.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectProject(t *testing.T) {
	tests := []struct {
		name        string
		remoteURL   string
		host        string
		wantProject string
		wantErr     bool
		errString   string
	}{
		{
			name:        "ssh url gitlab.com",
			remoteURL:   "git@gitlab.com:group/project.git",
			wantProject: "group/project",
		},
		{
			name:        "https url gitlab.com",
			remoteURL:   "https://gitlab.com/group/project.git",
			wantProject: "group/project",
		},
		{
			name:        "https url without .git",
			remoteURL:   "https://gitlab.com/group/project",
			wantProject: "group/project",
		},
		{
			name:        "ssh url without .git",
			remoteURL:   "git@gitlab.com:group/project",
			wantProject: "group/project",
		},
		{
			name:        "nested group",
			remoteURL:   "git@gitlab.com:group/subgroup/project.git",
			wantProject: "group/subgroup/project",
		},
		{
			name:        "self-hosted ssh url",
			remoteURL:   "git@custom.gitlab.com:group/project.git",
			host:        "custom.gitlab.com",
			wantProject: "group/project",
		},
		{
			name:        "self-hosted https url",
			remoteURL:   "https://custom.gitlab.com/group/project.git",
			host:        "custom.gitlab.com",
			wantProject: "group/project",
		},
		{
			name:        "default host when empty",
			remoteURL:   "git@gitlab.com:group/project.git",
			host:        "",
			wantProject: "group/project",
		},
		{
			name:        "generic ssh format",
			remoteURL:   "git@github.com:owner/repo.git",
			wantProject: "owner/repo",
		},
		{
			name:        "https with trailing slash",
			remoteURL:   "https://gitlab.com/group/project/",
			wantProject: "group/project",
		},
		{
			name:      "empty url",
			remoteURL: "",
			wantErr:   true,
		},
		{
			name:      "not a url",
			remoteURL: "not-a-url",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectProject(tt.remoteURL, tt.host)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DetectProject(%q, %q) expected error, got nil", tt.remoteURL, tt.host)
				}
				if tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("DetectProject(%q, %q) error = %v, want containing %q", tt.remoteURL, tt.host, err, tt.errString)
				}

				return
			}

			if err != nil {
				t.Errorf("DetectProject(%q, %q) unexpected error: %v", tt.remoteURL, tt.host, err)

				return
			}

			if got != tt.wantProject {
				t.Errorf("DetectProject(%q, %q) = %q, want %q", tt.remoteURL, tt.host, got, tt.wantProject)
			}
		})
	}
}

func TestExtractLinkedIssues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []int64
	}{
		{
			name: "single reference",
			body: "This fixes #123",
			want: []int64{123},
		},
		{
			name: "multiple references",
			body: "Related to #1, #2, and #3",
			want: []int64{1, 2, 3},
		},
		{
			name: "no references",
			body: "No issue references here",
			want: []int64(nil),
		},
		{
			name: "mixed content",
			body: "Fixes #42 and closes #100. Also see issue #7.",
			want: []int64{42, 100, 7},
		},
		{
			name: "duplicate references",
			body: "See #5 and also #5 again",
			want: []int64{5},
		},
		{
			name: "references at line start",
			body: "#10 is the main issue\n#20 is related",
			want: []int64{10, 20},
		},
		{
			name: "large numbers",
			body: "Issue #123456789",
			want: []int64{123456789},
		},
		{
			name: "empty string",
			body: "",
			want: []int64(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractLinkedIssues(tt.body)

			if len(got) != len(tt.want) {
				t.Errorf("ExtractLinkedIssues() = %v, want %v", got, tt.want)

				return
			}

			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("ExtractLinkedIssues()[%d] = %d, want %d", i, got[i], w)
				}
			}
		})
	}
}

func TestExtractImageURLs(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "single image",
			body: "![alt](https://example.com/image.png)",
			want: []string{"https://example.com/image.png"},
		},
		{
			name: "multiple images",
			body: "![a](http://a.com/1.jpg) ![b](http://b.com/2.png)",
			want: []string{"http://a.com/1.jpg", "http://b.com/2.png"},
		},
		{
			name: "no images",
			body: "No images here",
			want: []string(nil),
		},
		{
			name: "duplicate URLs",
			body: "![a](https://example.com/img.png) ![b](https://example.com/img.png)",
			want: []string{"https://example.com/img.png"},
		},
		{
			name: "empty alt text",
			body: "![](https://example.com/img.gif)",
			want: []string{"https://example.com/img.gif"},
		},
		{
			name: "alt text with brackets not supported",
			body: "![alt [with] brackets](https://example.com/img.png)",
			want: []string(nil), // Regex doesn't support nested brackets in alt text
		},
		{
			name: "URL with query params",
			body: "![img](https://example.com/img.png?w=100&h=200)",
			want: []string{"https://example.com/img.png?w=100&h=200"},
		},
		{
			name: "empty string",
			body: "",
			want: []string(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractImageURLs(tt.body)

			if len(got) != len(tt.want) {
				t.Errorf("ExtractImageURLs() = %v, want %v", got, tt.want)

				return
			}

			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("ExtractImageURLs()[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

func TestParseTaskList(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []TaskItem
	}{
		{
			name: "empty body",
			body: "",
			want: nil,
		},
		{
			name: "no task list items",
			body: "Just some regular text\nNo checkboxes here",
			want: nil,
		},
		{
			name: "single unchecked task",
			body: "- [ ] Task one",
			want: []TaskItem{
				{Text: "Task one", Completed: false, Line: 1},
			},
		},
		{
			name: "single checked task",
			body: "- [x] Task completed",
			want: []TaskItem{
				{Text: "Task completed", Completed: true, Line: 1},
			},
		},
		{
			name: "checked with uppercase X",
			body: "- [X] Also completed",
			want: []TaskItem{
				{Text: "Also completed", Completed: true, Line: 1},
			},
		},
		{
			name: "multiple tasks",
			body: "- [ ] First task\n- [x] Second task done\n- [ ] Third task",
			want: []TaskItem{
				{Text: "First task", Completed: false, Line: 1},
				{Text: "Second task done", Completed: true, Line: 2},
				{Text: "Third task", Completed: false, Line: 3},
			},
		},
		{
			name: "asterisk bullets",
			body: "* [ ] Task with asterisk\n* [x] Completed with asterisk",
			want: []TaskItem{
				{Text: "Task with asterisk", Completed: false, Line: 1},
				{Text: "Completed with asterisk", Completed: true, Line: 2},
			},
		},
		{
			name: "mixed content",
			body: "# Header\n\nSome intro text.\n\n- [ ] First todo\n- Regular list item\n- [x] Done item\n\nMore text",
			want: []TaskItem{
				{Text: "First todo", Completed: false, Line: 5},
				{Text: "Done item", Completed: true, Line: 7},
			},
		},
		{
			name: "indented tasks",
			body: "  - [ ] Indented task\n    - [x] More indented",
			want: []TaskItem{
				{Text: "Indented task", Completed: false, Line: 1},
				{Text: "More indented", Completed: true, Line: 2},
			},
		},
		{
			name: "task with extra whitespace in text",
			body: "- [ ]   Task with spaces   ",
			want: []TaskItem{
				{Text: "Task with spaces", Completed: false, Line: 1},
			},
		},
		{
			name: "empty task text is skipped",
			body: "- [ ] \n- [ ] Valid task",
			want: []TaskItem{
				{Text: "Valid task", Completed: false, Line: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTaskList(tt.body)

			if len(got) != len(tt.want) {
				t.Errorf("ParseTaskList() = %d items, want %d items", len(got), len(tt.want))

				return
			}

			for i, w := range tt.want {
				if got[i].Text != w.Text {
					t.Errorf("ParseTaskList()[%d].Text = %q, want %q", i, got[i].Text, w.Text)
				}
				if got[i].Completed != w.Completed {
					t.Errorf("ParseTaskList()[%d].Completed = %v, want %v", i, got[i].Completed, w.Completed)
				}
				if got[i].Line != w.Line {
					t.Errorf("ParseTaskList()[%d].Line = %d, want %d", i, got[i].Line, w.Line)
				}
			}
		})
	}
}

func TestMapGitLabState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  provider.Status
	}{
		{
			name:  "opened state",
			state: "opened",
			want:  provider.StatusOpen,
		},
		{
			name:  "closed state",
			state: "closed",
			want:  provider.StatusClosed,
		},
		{
			name:  "unknown state defaults to open",
			state: "unknown",
			want:  provider.StatusOpen,
		},
		{
			name:  "empty state defaults to open",
			state: "",
			want:  provider.StatusOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGitLabState(tt.state)
			if got != tt.want {
				t.Errorf("mapGitLabState(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestInferTypeFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{
			name:   "no labels",
			labels: []string{},
			want:   "issue",
		},
		{
			name:   "bug label",
			labels: []string{"bug", "enhancement"},
			want:   "fix",
		},
		{
			name:   "bugfix label",
			labels: []string{"bugfix"},
			want:   "fix",
		},
		{
			name:   "fix label",
			labels: []string{"fix"},
			want:   "fix",
		},
		{
			name:   "feature label",
			labels: []string{"feature"},
			want:   "feature",
		},
		{
			name:   "enhancement label",
			labels: []string{"enhancement"},
			want:   "feature",
		},
		{
			name:   "docs label",
			labels: []string{"docs"},
			want:   "docs",
		},
		{
			name:   "documentation label",
			labels: []string{"documentation"},
			want:   "docs",
		},
		{
			name:   "refactor label",
			labels: []string{"refactor"},
			want:   "refactor",
		},
		{
			name:   "chore label",
			labels: []string{"chore"},
			want:   "chore",
		},
		{
			name:   "test label",
			labels: []string{"test"},
			want:   "test",
		},
		{
			name:   "ci label",
			labels: []string{"ci"},
			want:   "ci",
		},
		{
			name:   "case insensitive",
			labels: []string{"BUG", "Feature"},
			want:   "fix", // First match wins
		},
		{
			name:   "unknown label",
			labels: []string{"random-label"},
			want:   "issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTypeFromLabels(tt.labels)
			if got != tt.want {
				t.Errorf("inferTypeFromLabels(%v) = %q, want %q", tt.labels, got, tt.want)
			}
		})
	}
}

func TestInferPriorityFromLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   provider.Priority
	}{
		{
			name:   "no labels",
			labels: []string{},
			want:   provider.PriorityNormal,
		},
		{
			name:   "critical label",
			labels: []string{"critical"},
			want:   provider.PriorityCritical,
		},
		{
			name:   "urgent label",
			labels: []string{"urgent"},
			want:   provider.PriorityCritical,
		},
		{
			name:   "priority:high label",
			labels: []string{"priority:high"},
			want:   provider.PriorityHigh,
		},
		{
			name:   "high-priority label",
			labels: []string{"high-priority"},
			want:   provider.PriorityHigh,
		},
		{
			name:   "priority:low label",
			labels: []string{"priority:low"},
			want:   provider.PriorityLow,
		},
		{
			name:   "low-priority label",
			labels: []string{"low-priority"},
			want:   provider.PriorityLow,
		},
		{
			name:   "unknown label",
			labels: []string{"random-label"},
			want:   provider.PriorityNormal,
		},
		{
			name:   "case insensitive",
			labels: []string{"CRITICAL"},
			want:   provider.PriorityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferPriorityFromLabels(tt.labels)
			if got != tt.want {
				t.Errorf("inferPriorityFromLabels(%v) = %v, want %v", tt.labels, got, tt.want)
			}
		})
	}
}

func TestWrapAPIError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantNil     bool
		wantWrapped bool
		wantErr     error
	}{
		{
			name:    "nil error",
			err:     nil,
			wantNil: true,
		},
		{
			name:        "401 unauthorized",
			err:         errors.New("401 Unauthorized"),
			wantWrapped: true,
			wantErr:     ErrUnauthorized,
		},
		{
			name:        "403 rate limit",
			err:         errors.New("403 rate limit exceeded"),
			wantWrapped: true,
			wantErr:     ErrRateLimited,
		},
		{
			name:        "404 not found",
			err:         errors.New("404 Not Found"),
			wantWrapped: true,
			wantErr:     ErrIssueNotFound,
		},
		{
			name:        "403 forbidden (not rate limit)",
			err:         errors.New("403 Forbidden"),
			wantWrapped: true,
			wantErr:     ErrInsufficientScope,
		},
		{
			name:        "generic error passthrough",
			err:         errors.New("some other error"),
			wantWrapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapAPIError(tt.err)

			if tt.wantNil {
				if got != nil {
					t.Errorf("wrapAPIError() = %v, want nil", got)
				}

				return
			}

			if got == nil {
				t.Error("wrapAPIError() = nil, want non-nil error")

				return
			}

			if tt.wantWrapped {
				if !errors.Is(got, tt.wantErr) {
					t.Errorf("wrapAPIError() error = %v, want wrapped %v", got, tt.wantErr)
				}
			}
		})
	}
}

func TestResolveToken(t *testing.T) {
	// Save original env values
	oldMehrToken := os.Getenv("MEHR_GITLAB_TOKEN")
	oldGitlabToken := os.Getenv("GITLAB_TOKEN")
	defer func() {
		_ = os.Setenv("MEHR_GITLAB_TOKEN", oldMehrToken)
		_ = os.Setenv("GITLAB_TOKEN", oldGitlabToken)
	}()

	// Clear env vars for testing
	_ = os.Unsetenv("MEHR_GITLAB_TOKEN")
	_ = os.Unsetenv("GITLAB_TOKEN")

	tests := []struct {
		name         string
		configToken  string
		setMehrEnv   string
		setGitlabEnv string
		want         string
		wantErr      bool
	}{
		{
			name:    "no token available",
			wantErr: true,
		},
		{
			name:        "config token only",
			configToken: "config-token-123",
			want:        "config-token-123",
		},
		{
			name:        "MEHR_GITLAB_TOKEN overrides config",
			configToken: "config-token",
			setMehrEnv:  "mehr-token",
			want:        "mehr-token",
		},
		{
			name:         "GITLAB_TOKEN used when MEHR not set",
			configToken:  "config-token",
			setGitlabEnv: "gitlab-token",
			want:         "gitlab-token",
		},
		{
			name:         "MEHR_GITLAB_TOKEN overrides GITLAB_TOKEN",
			setMehrEnv:   "mehr-token",
			setGitlabEnv: "gitlab-token",
			want:         "mehr-token",
		},
		{
			name:         "GITLAB_TOKEN used as fallback",
			setGitlabEnv: "gitlab-token",
			want:         "gitlab-token",
		},
		{
			name:         "empty strings are ignored",
			setMehrEnv:   "",
			setGitlabEnv: "gitlab-token",
			want:         "gitlab-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			_ = os.Unsetenv("MEHR_GITLAB_TOKEN")
			_ = os.Unsetenv("GITLAB_TOKEN")

			// Set env vars as specified
			if tt.setMehrEnv != "" {
				_ = os.Setenv("MEHR_GITLAB_TOKEN", tt.setMehrEnv)
			}
			if tt.setGitlabEnv != "" {
				_ = os.Setenv("GITLAB_TOKEN", tt.setGitlabEnv)
			}

			got, err := ResolveToken(tt.configToken)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveToken() expected error, got nil")
				}
				if !errors.Is(err, ErrNoToken) {
					t.Errorf("ResolveToken() error = %v, want ErrNoToken", err)
				}

				return
			}

			if err != nil {
				t.Errorf("ResolveToken() unexpected error: %v", err)

				return
			}

			if got != tt.want {
				t.Errorf("ResolveToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInfo(t *testing.T) {
	info := Info()

	if info.Name != ProviderName {
		t.Errorf("Info().Name = %q, want %q", info.Name, ProviderName)
	}

	if len(info.Schemes) != 2 {
		t.Errorf("Info().Schemes length = %d, want 2", len(info.Schemes))
	}

	schemeMap := make(map[string]bool)
	for _, s := range info.Schemes {
		schemeMap[s] = true
	}

	if !schemeMap["gitlab"] {
		t.Error("Info().Schemes missing 'gitlab'")
	}
	if !schemeMap["gl"] {
		t.Error("Info().Schemes missing 'gl'")
	}
}

func TestProviderMatch(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "gitlab scheme",
			input: "gitlab:123",
			want:  true,
		},
		{
			name:  "gl scheme",
			input: "gl:123",
			want:  true,
		},
		{
			name:  "uppercase gitlab scheme",
			input: "GITLAB:123",
			want:  false,
		},
		{
			name:  "no scheme",
			input: "123",
			want:  false,
		},
		{
			name:  "different scheme",
			input: "github:123",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Match(tt.input)
			if got != tt.want {
				t.Errorf("Provider.Match(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestProviderNew(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setupConfig func() provider.Config
		wantErr     bool
		errContains string
	}{
		{
			name: "minimal config with token",
			setupConfig: func() provider.Config {
				return provider.NewConfig().Set("token", "test-token")
			},
		},
		{
			name: "full config",
			setupConfig: func() provider.Config {
				return provider.NewConfig().
					Set("token", "test-token").
					Set("host", "https://custom.gitlab.com").
					Set("project_path", "group/project").
					Set("branch_pattern", "custom/{key}").
					Set("commit_prefix", "[GL-{key}]")
			},
		},
		{
			name: "host without https",
			setupConfig: func() provider.Config {
				return provider.NewConfig().
					Set("token", "test-token").
					Set("host", "custom.gitlab.com/")
			},
		},
		{
			name: "empty token requires env",
			setupConfig: func() provider.Config {
				return provider.NewConfig()
			},
			wantErr:     true,
			errContains: "token not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			got, err := New(ctx, cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("New() expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("New() error = %v, want containing %q", err, tt.errContains)
				}

				return
			}

			if err != nil {
				t.Errorf("New() unexpected error: %v", err)

				return
			}

			p, ok := got.(*Provider)
			if !ok {
				t.Errorf("New() returned type %T, want *Provider", got)

				return
			}

			if p.config == nil {
				t.Error("New() provider config is nil")
			}

			if p.client == nil {
				t.Error("New() provider client is nil")
			}
		})
	}
}

func TestClientSettersGetters(t *testing.T) {
	c := &Client{
		projectPath: "original/path",
		projectID:   0,
		host:        "gitlab.com",
	}

	t.Run("SetProjectPath", func(t *testing.T) {
		c.SetProjectPath("new/path")
		if c.ProjectPath() != "new/path" {
			t.Errorf("SetProjectPath() didn't update, got %q", c.ProjectPath())
		}
		if c.ProjectID() != 0 {
			t.Errorf("SetProjectPath() should reset cached ID, got %d", c.ProjectID())
		}
	})

	t.Run("SetProjectID", func(t *testing.T) {
		c.SetProjectID(12345)
		if c.ProjectID() != 12345 {
			t.Errorf("SetProjectID() didn't update, got %d", c.ProjectID())
		}
	})

	t.Run("Host getter", func(t *testing.T) {
		if c.Host() != "gitlab.com" {
			t.Errorf("Host() = %q, want 'gitlab.com'", c.Host())
		}

		// Test with custom host
		c2 := &Client{host: "custom.host.com"}
		if c2.Host() != "custom.host.com" {
			t.Errorf("Host() with custom = %q, want 'custom.host.com'", c2.Host())
		}

		// Test default when empty
		c3 := &Client{host: ""}
		if c3.Host() != "gitlab.com" {
			t.Errorf("Host() with empty = %q, want 'gitlab.com'", c3.Host())
		}
	})
}

func TestProviderGetters(t *testing.T) {
	p := &Provider{
		config: &Config{
			Token:         "test-token",
			Host:          "https://gitlab.com",
			ProjectPath:   "group/project",
			BranchPattern: "issue/{key}",
			CommitPrefix:  "[#{key}]",
		},
		client: &Client{},
	}

	t.Run("GetConfig", func(t *testing.T) {
		cfg := p.GetConfig()
		if cfg == nil {
			t.Fatal("GetConfig() returned nil")
		}
		if cfg.Token != "test-token" {
			t.Errorf("GetConfig().Token = %q, want 'test-token'", cfg.Token)
		}
		if cfg.ProjectPath != "group/project" {
			t.Errorf("GetConfig().ProjectPath = %q, want 'group/project'", cfg.ProjectPath)
		}
	})

	t.Run("GetClient", func(t *testing.T) {
		c := p.GetClient()
		if c == nil {
			t.Error("GetClient() returned nil")
		}
	})
}

func TestProviderParse(t *testing.T) {
	tests := []struct {
		name        string
		projectPath string
		input       string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:        "explicit reference",
			projectPath: "group/project",
			input:       "group/project#123",
			want:        "group/project#123",
		},
		{
			name:        "simple reference with configured project",
			projectPath: "mygroup/myproject",
			input:       "42",
			want:        "mygroup/myproject#42",
		},
		{
			name:        "simple reference without configured project",
			input:       "42",
			wantErr:     true,
			errContains: "project not configured",
		},
		{
			name:  "project ID reference",
			input: "12345#42",
			want:  "12345#42",
		},
		{
			name:        "gitlab scheme with simple ref and configured project",
			projectPath: "configured/project",
			input:       "gitlab:10",
			want:        "configured/project#10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				config: &Config{
					ProjectPath: tt.projectPath,
				},
				projectPath: tt.projectPath,
			}

			got, err := p.Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Provider.Parse() expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Provider.Parse() error = %v, want containing %q", err, tt.errContains)
				}

				return
			}

			if err != nil {
				t.Errorf("Provider.Parse() unexpected error: %v", err)

				return
			}

			if got != tt.want {
				t.Errorf("Provider.Parse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderUpdateStatus(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		workUnitID  string
		status      provider.Status
		wantErr     bool
		errContains string
	}{
		{
			name:       "invalid reference",
			workUnitID: "invalid",
			wantErr:    true,
		},
		{
			name:       "valid reference but no client configured",
			workUnitID: "group/project#123",
			status:     provider.StatusClosed,
			wantErr:    true, // Will fail due to no project configured
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				config: &Config{
					ProjectPath: "group/project",
				},
				client: &Client{},
			}

			err := p.UpdateStatus(ctx, tt.workUnitID, tt.status)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Provider.UpdateStatus() expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("Provider.UpdateStatus() unexpected error: %v", err)
			}
		})
	}
}

func TestProviderAddLabels(t *testing.T) {
	ctx := context.Background()
	p := &Provider{
		config: &Config{
			ProjectPath: "group/project",
		},
		client: &Client{},
	}

	err := p.AddLabels(ctx, "invalid", []string{"label"})
	if err == nil {
		t.Error("Provider.AddLabels() with invalid reference expected error, got nil")
	}
}

func TestProviderRemoveLabels(t *testing.T) {
	ctx := context.Background()
	p := &Provider{
		config: &Config{
			ProjectPath: "group/project",
		},
		client: &Client{},
	}

	err := p.RemoveLabels(ctx, "invalid", []string{"label"})
	if err == nil {
		t.Error("Provider.RemoveLabels() with invalid reference expected error, got nil")
	}
}

func TestMapAssignees(t *testing.T) {
	tests := []struct {
		name      string
		assignees []*gitlab.IssueAssignee
		wantCount int
		wantIDs   map[string]bool
	}{
		{
			name:      "no assignees",
			assignees: []*gitlab.IssueAssignee{},
			wantCount: 0,
		},
		{
			name: "single assignee",
			assignees: []*gitlab.IssueAssignee{
				{ID: 123, Username: "user1"},
			},
			wantCount: 1,
			wantIDs:   map[string]bool{"123": true},
		},
		{
			name: "multiple assignees",
			assignees: []*gitlab.IssueAssignee{
				{ID: 1, Username: "user1"},
				{ID: 2, Username: "user2"},
			},
			wantCount: 2,
			wantIDs:   map[string]bool{"1": true, "2": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAssignees(tt.assignees)

			if len(got) != tt.wantCount {
				t.Errorf("mapAssignees() count = %d, want %d", len(got), tt.wantCount)
			}

			for _, p := range got {
				if !tt.wantIDs[p.ID] {
					t.Errorf("mapAssignees() unexpected ID %s", p.ID)
				}
			}
		})
	}
}

func TestMapNotes(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		notes []*gitlab.Note
		want  int
	}{
		{
			name:  "no notes",
			notes: []*gitlab.Note{},
			want:  0,
		},
		{
			name: "single note",
			notes: []*gitlab.Note{
				{
					ID:        1,
					Body:      "Test comment",
					CreatedAt: &now,
					UpdatedAt: &now,
					Author: gitlab.NoteAuthor{
						ID:       123,
						Username: "testuser",
					},
				},
			},
			want: 1,
		},
		{
			name: "multiple notes",
			notes: []*gitlab.Note{
				{
					ID:        1,
					Body:      "First",
					CreatedAt: &now,
					UpdatedAt: &now,
					Author: gitlab.NoteAuthor{
						ID:       123,
						Username: "user1",
					},
				},
				{
					ID:        2,
					Body:      "Second",
					CreatedAt: &now,
					UpdatedAt: &now,
					Author: gitlab.NoteAuthor{
						ID:       456,
						Username: "user2",
					},
				},
			},
			want: 2,
		},
		{
			name: "note without UpdatedAt uses CreatedAt",
			notes: []*gitlab.Note{
				{
					ID:        1,
					Body:      "Test",
					CreatedAt: &now,
					UpdatedAt: nil,
					Author: gitlab.NoteAuthor{
						ID:       1,
						Username: "user",
					},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapNotes(tt.notes)

			if len(got) != tt.want {
				t.Errorf("mapNotes() count = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestFormatIssueMarkdown(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		issue        *gitlab.Issue
		wantContains []string
	}{
		{
			name: "minimal issue",
			issue: &gitlab.Issue{
				IID:         123,
				Title:       "Test Issue",
				State:       "opened",
				CreatedAt:   &now,
				UpdatedAt:   &now,
				Description: "Test description",
			},
			wantContains: []string{"# #123: Test Issue", "**State:** opened", "Test description"},
		},
		{
			name: "issue with labels",
			issue: &gitlab.Issue{
				IID:       1,
				Title:     "Labelled Issue",
				State:     "closed",
				CreatedAt: &now,
				UpdatedAt: &now,
				Labels:    []string{"bug", "enhancement"},
			},
			wantContains: []string{"**Labels:** bug, enhancement"},
		},
		{
			name: "issue with author",
			issue: &gitlab.Issue{
				IID:       5,
				Title:     "Authored Issue",
				State:     "opened",
				CreatedAt: &now,
				UpdatedAt: &now,
				Author: &gitlab.IssueAuthor{
					ID:       123,
					Username: "testuser",
				},
			},
			wantContains: []string{"**Author:** @testuser"},
		},
		{
			name: "issue with URL",
			issue: &gitlab.Issue{
				IID:       10,
				Title:     "Issue with URL",
				State:     "opened",
				CreatedAt: &now,
				UpdatedAt: &now,
				WebURL:    "https://gitlab.com/group/project/issues/10",
			},
			wantContains: []string{"**URL:** https://gitlab.com/group/project/issues/10"},
		},
		{
			name: "issue with assignees",
			issue: &gitlab.Issue{
				IID:       7,
				Title:     "Assigned Issue",
				State:     "opened",
				CreatedAt: &now,
				UpdatedAt: &now,
				Assignees: []*gitlab.IssueAssignee{
					{Username: "user1"},
					{Username: "user2"},
				},
			},
			wantContains: []string{"**Assignees:** @user1, @user2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatIssueMarkdown(tt.issue)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatIssueMarkdown() missing %q", want)
				}
			}
		})
	}
}

func TestFormatNotesMarkdown(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		notes        []*gitlab.Note
		wantContains []string
	}{
		{
			name:         "no notes",
			notes:        []*gitlab.Note{},
			wantContains: []string{"# Notes"},
		},
		{
			name: "single note",
			notes: []*gitlab.Note{
				{
					Body:      "Test note content",
					CreatedAt: &now,
					Author: gitlab.NoteAuthor{
						Username: "testuser",
					},
				},
			},
			wantContains: []string{"# Notes", "## Note by @testuser", "Test note content"},
		},
		{
			name: "multiple notes",
			notes: []*gitlab.Note{
				{
					Body:      "First note",
					CreatedAt: &now,
					Author: gitlab.NoteAuthor{
						Username: "user1",
					},
				},
				{
					Body:      "Second note",
					CreatedAt: &now,
					Author: gitlab.NoteAuthor{
						Username: "user2",
					},
				},
			},
			wantContains: []string{"## Note by @user1", "First note", "## Note by @user2", "Second note"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatNotesMarkdown(tt.notes)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatNotesMarkdown() missing %q", want)
				}
			}
		})
	}
}

func TestRegister(t *testing.T) {
	registry := provider.NewRegistry()
	Register(registry)

	info, _, found := registry.Get("gitlab")
	if !found {
		t.Error("Register() failed: provider not found")
	}

	if info.Name != "gitlab" {
		t.Errorf("Register() name = %q, want 'gitlab'", info.Name)
	}
}
