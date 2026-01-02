package youtrack

import (
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  string
		wantErr bool
	}{
		{
			name:    "short scheme",
			input:   "yt:ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "full scheme",
			input:   "youtrack:ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "bare ID",
			input:   "ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "URL - youtrack.cloud",
			input:   "https://youtrack.cloud/issue/ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "URL - myjetbrains.com",
			input:   "https://company.myjetbrains.com/youtrack/issue/ABC-123",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "URL with title",
			input:   "https://company.myjetbrains.com/youtrack/issue/ABC-123-some-title",
			wantID:  "ABC-123",
			wantErr: false,
		},
		{
			name:    "mixed case ID",
			input:   "aBc-123",
			wantID:  "ABC-123", // Normalized to uppercase
			wantErr: false,
		},
		{
			name:    "invalid format - no dash",
			input:   "ABC123",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "numeric only project",
			input:   "123-456",
			wantID:  "123-456",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseReference(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ref.ID != tt.wantID {
				t.Errorf("ParseReference() ID = %v, want %v", ref.ID, tt.wantID)
			}
		})
	}
}

func TestIsValidID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"ABC-123", true},
		{"PROJECT-1", true},
		{"123-456", true},
		{"A1-23", true},   // valid: number part is digits only
		{"A1-B2", false},  // invalid: letter in number part
		{"abc-123", true}, // lowercase is now valid
		{"ABC123", false}, // no dash
		{"ABC-", false},   // no number
		{"-123", false},   // no project
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := IsValidID(tt.id); got != tt.want {
				t.Errorf("IsValidID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestMapPriorityValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "critical",
			value:    map[string]interface{}{"name": "Critical"},
			expected: "critical", // Will map to PriorityCritical
		},
		{
			name:     "urgent",
			value:    map[string]interface{}{"name": "Urgent"},
			expected: "critical",
		},
		{
			name:     "high",
			value:    map[string]interface{}{"name": "High"},
			expected: "high",
		},
		{
			name:     "normal",
			value:    map[string]interface{}{"name": "Normal"},
			expected: "normal",
		},
		{
			name:     "low",
			value:    map[string]interface{}{"name": "Low"},
			expected: "low",
		},
		{
			name:     "unknown",
			value:    map[string]interface{}{"name": "Unknown"},
			expected: "normal",
		},
		{
			name:     "not a map",
			value:    "string",
			expected: "normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPriorityValue(tt.value)
			if got.String() != tt.expected {
				t.Errorf("mapPriorityValue() = %v, want %v", got.String(), tt.expected)
			}
		})
	}
}

func TestMapStatusValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "new",
			value:    map[string]interface{}{"name": "New"},
			expected: "open",
		},
		{
			name:     "in progress",
			value:    map[string]interface{}{"name": "In Progress"},
			expected: "in_progress",
		},
		{
			name:     "review",
			value:    map[string]interface{}{"name": "Review"},
			expected: "review",
		},
		{
			name:     "done",
			value:    map[string]interface{}{"name": "Done"},
			expected: "done",
		},
		{
			name:     "fixed",
			value:    map[string]interface{}{"name": "Fixed"},
			expected: "done",
		},
		{
			name:     "closed",
			value:    map[string]interface{}{"name": "Closed"},
			expected: "closed",
		},
		{
			name:     "obsolete",
			value:    map[string]interface{}{"name": "Obsolete"},
			expected: "closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapStatusValue(tt.value)
			if string(got) != tt.expected {
				t.Errorf("mapStatusValue() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractNameFromValue(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{
			name:  "map with name",
			value: map[string]interface{}{"name": "TestValue", "id": "123"},
			want:  "TestValue",
		},
		{
			name:  "map without name",
			value: map[string]interface{}{"id": "123"},
			want:  "",
		},
		{
			name:  "string value",
			value: "DirectString",
			want:  "DirectString",
		},
		{
			name:  "nil value",
			value: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractNameFromValue(tt.value); got != tt.want {
				t.Errorf("extractNameFromValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusToYouTrackState(t *testing.T) {
	tests := []struct {
		status provider.Status
		want   string
	}{
		{provider.StatusOpen, "New"},
		{provider.StatusInProgress, "In Progress"},
		{provider.StatusReview, "Review"},
		{provider.StatusDone, "Done"},
		{provider.StatusClosed, "Obsolete"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := statusToYouTrackState(tt.status); got != tt.want {
				t.Errorf("statusToYouTrackState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimeFromMillis(t *testing.T) {
	tests := []struct {
		name     string
		millis   int64
		wantZero bool
	}{
		{
			name:     "zero value",
			millis:   0,
			wantZero: true,
		},
		{
			name:     "positive timestamp",
			millis:   1609459200000, // 2021-01-01 00:00:00 UTC
			wantZero: false,
		},
		{
			name:     "negative timestamp",
			millis:   -1000,
			wantZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := timeFromMillis(tt.millis)
			isZero := got.IsZero()
			if isZero != tt.wantZero {
				t.Errorf("timeFromMillis(%d).IsZero() = %v, want %v", tt.millis, isZero, tt.wantZero)
			}
		})
	}

	// Test specific known value
	t.Run("known timestamp", func(t *testing.T) {
		// 2021-01-01 00:00:00 UTC = 1609459200000 ms
		got := timeFromMillis(1609459200000)
		want := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("timeFromMillis(1609459200000) = %v, want %v", got, want)
		}
	})

	// Test milliseconds precision
	t.Run("milliseconds precision", func(t *testing.T) {
		// 1609459200123 ms = 2021-01-01 00:00:00.123 UTC
		got := timeFromMillis(1609459200123)
		wantNanos := int64(123 * 1e6) // 123ms in nanoseconds
		if got.Nanosecond() != int(wantNanos) {
			t.Errorf("timeFromMillis(1609459200123).Nanosecond() = %v, want %v", got.Nanosecond(), wantNanos)
		}
	})
}

func TestHTTPError(t *testing.T) {
	tests := []struct {
		name         string
		code         int
		message      string
		wantContains string
	}{
		{
			name:         "with message",
			code:         404,
			message:      "not found",
			wantContains: "HTTP 404: not found",
		},
		{
			name:         "without message",
			code:         500,
			message:      "",
			wantContains: "HTTP 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newHTTPError(tt.code, tt.message)
			got := err.Error()
			if !containsStr(got, tt.wantContains) {
				t.Errorf("httpError.Error() = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}

func TestHTTPError_HTTPStatusCode(t *testing.T) {
	err := newHTTPError(404, "not found")
	if got := err.HTTPStatusCode(); got != 404 {
		t.Errorf("httpError.HTTPStatusCode() = %d, want 404", got)
	}
}

func TestWrapAPIError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		checkError    error
		wantUnwrapped bool
	}{
		{
			name:          "nil error",
			err:           nil,
			checkError:    nil,
			wantUnwrapped: true,
		},
		{
			name:          "already wrapped error",
			err:           providererrors.ErrNoToken,
			checkError:    providererrors.ErrNoToken,
			wantUnwrapped: true,
		},
		{
			name:          "issue not found",
			err:           providererrors.ErrNotFound,
			checkError:    providererrors.ErrNotFound,
			wantUnwrapped: true,
		},
		{
			name:          "rate limited",
			err:           providererrors.ErrRateLimited,
			checkError:    providererrors.ErrRateLimited,
			wantUnwrapped: true,
		},
		{
			name:          "network error",
			err:           providererrors.ErrNetworkError,
			checkError:    providererrors.ErrNetworkError,
			wantUnwrapped: true,
		},
		{
			name:          "unauthorized",
			err:           providererrors.ErrUnauthorized,
			checkError:    providererrors.ErrUnauthorized,
			wantUnwrapped: true,
		},
		{
			name:          "invalid reference",
			err:           providererrors.ErrInvalidReference,
			checkError:    providererrors.ErrInvalidReference,
			wantUnwrapped: true,
		},
		{
			name:          "401 unauthorized wraps correctly",
			err:           newHTTPError(http.StatusUnauthorized, "unauthorized"),
			checkError:    providererrors.ErrUnauthorized,
			wantUnwrapped: false,
		},
		{
			name:          "403 forbidden wraps as rate limited",
			err:           newHTTPError(http.StatusForbidden, "forbidden"),
			checkError:    providererrors.ErrRateLimited,
			wantUnwrapped: false,
		},
		{
			name:          "404 not found wraps correctly",
			err:           newHTTPError(http.StatusNotFound, "not found"),
			checkError:    providererrors.ErrNotFound,
			wantUnwrapped: false,
		},
		{
			name:          "429 too many requests wraps correctly",
			err:           newHTTPError(http.StatusTooManyRequests, "rate limit"),
			checkError:    providererrors.ErrRateLimited,
			wantUnwrapped: false,
		},
		{
			name:          "503 service unavailable wraps correctly",
			err:           newHTTPError(http.StatusServiceUnavailable, "unavailable"),
			checkError:    providererrors.ErrRateLimited,
			wantUnwrapped: false,
		},
		{
			name:          "500 internal server error unchanged",
			err:           newHTTPError(http.StatusInternalServerError, "server error"),
			checkError:    nil,
			wantUnwrapped: false,
		},
		{
			name:          "network error wraps correctly",
			err:           &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			checkError:    providererrors.ErrNetworkError,
			wantUnwrapped: false,
		},
		{
			name:          "generic error unchanged",
			err:           errors.New("generic error"),
			checkError:    nil,
			wantUnwrapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapAPIError(tt.err)
			if got == nil {
				if !tt.wantUnwrapped || tt.err != nil {
					t.Errorf("wrapAPIError() = nil, want non-nil")
				}
				return
			}
			if tt.wantUnwrapped {
				if !errors.Is(got, tt.err) {
					t.Errorf("wrapAPIError() should return same error, got %v, want %v", got, tt.err)
				}
			}
			if tt.checkError != nil {
				if !errors.Is(got, tt.checkError) {
					t.Errorf("wrapAPIError() should wrap %v, got %v", tt.checkError, got)
				}
			}
		})
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "youtrack cloud URL",
			rawURL: "https://youtrack.cloud/issue/ABC-123",
			want:   "youtrack.cloud",
		},
		{
			name:   "myjetbrains URL",
			rawURL: "https://company.myjetbrains.com/youtrack/issue/ABC-123",
			want:   "company.myjetbrains.com",
		},
		{
			name:   "URL with port",
			rawURL: "https://localhost:8080/youtrack/issue/ABC-123",
			want:   "localhost:8080",
		},
		{
			name:   "too short URL",
			rawURL: "https://",
			want:   "",
		},
		{
			name:   "empty string",
			rawURL: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHost(tt.rawURL)
			if got != tt.want {
				t.Errorf("extractHost(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findInStr(s, substr)))
}

func findInStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPriorityToYouTrack(t *testing.T) {
	tests := []struct {
		name     string
		priority provider.Priority
		want     string
	}{
		{"critical", provider.PriorityCritical, "Critical"},
		{"high", provider.PriorityHigh, "High"},
		{"normal", provider.PriorityNormal, "Normal"},
		{"low", provider.PriorityLow, "Low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := priorityToYouTrack(tt.priority)
			if got != tt.want {
				t.Errorf("priorityToYouTrack(%v) = %q, want %q", tt.priority, got, tt.want)
			}
		})
	}
}

func TestBuildQuery(t *testing.T) {
	tests := []struct {
		name string
		opts provider.ListOptions
		want string
	}{
		{
			name: "empty options",
			opts: provider.ListOptions{},
			want: "",
		},
		{
			name: "open status",
			opts: provider.ListOptions{Status: provider.StatusOpen},
			want: "Unresolved",
		},
		{
			name: "done status",
			opts: provider.ListOptions{Status: provider.StatusDone},
			want: "Resolved",
		},
		{
			name: "closed status",
			opts: provider.ListOptions{Status: provider.StatusClosed},
			want: "Resolved",
		},
		{
			name: "with labels",
			opts: provider.ListOptions{Labels: []string{"bug", "urgent"}},
			want: "tag: bug tag: urgent",
		},
		{
			name: "with order by asc",
			opts: provider.ListOptions{OrderBy: "created"},
			want: "sort by: created asc",
		},
		{
			name: "with order by desc",
			opts: provider.ListOptions{OrderBy: "updated", OrderDir: "desc"},
			want: "sort by: updated desc",
		},
		{
			name: "combined filters",
			opts: provider.ListOptions{
				Status:   provider.StatusOpen,
				Labels:   []string{"bug"},
				OrderBy:  "priority",
				OrderDir: "desc",
			},
			want: "Unresolved tag: bug sort by: priority desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildQuery(tt.opts)
			if got != tt.want {
				t.Errorf("buildQuery() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want string
	}{
		{
			name: "existing string key",
			m:    map[string]interface{}{"name": "test"},
			key:  "name",
			want: "test",
		},
		{
			name: "non-existent key",
			m:    map[string]interface{}{"other": "value"},
			key:  "name",
			want: "",
		},
		{
			name: "non-string value",
			m:    map[string]interface{}{"name": 123},
			key:  "name",
			want: "",
		},
		{
			name: "empty map",
			m:    map[string]interface{}{},
			key:  "name",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getValue(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("getValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapAssigneeValue(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantLen int
	}{
		{
			name: "single assignee map",
			value: map[string]interface{}{
				"id":   "user-1",
				"name": "John Doe",
			},
			wantLen: 1,
		},
		{
			name: "multiple assignees",
			value: []interface{}{
				map[string]interface{}{"id": "user-1", "name": "John"},
				map[string]interface{}{"id": "user-2", "name": "Jane"},
			},
			wantLen: 2,
		},
		{
			name:    "nil value",
			value:   nil,
			wantLen: 0,
		},
		{
			name:    "string value",
			value:   "just-a-string",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAssigneeValue(tt.value)
			if len(got) != tt.wantLen {
				t.Errorf("mapAssigneeValue() length = %d, want %d", len(got), tt.wantLen)
			}
		})
	}

	// Test specific assignee values
	t.Run("single assignee has correct values", func(t *testing.T) {
		got := mapAssigneeValue(map[string]interface{}{
			"id":   "user-123",
			"name": "Alice",
		})
		if len(got) != 1 {
			t.Fatalf("got %d assignees, want 1", len(got))
		}
		if got[0].ID != "user-123" {
			t.Errorf("assignee ID = %q, want \"user-123\"", got[0].ID)
		}
		if got[0].Name != "Alice" {
			t.Errorf("assignee Name = %q, want \"Alice\"", got[0].Name)
		}
	})
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		host        string
		wantBaseURL string
	}{
		{
			name:        "default host",
			token:       "test-token",
			host:        "",
			wantBaseURL: "https://youtrack.cloud/api",
		},
		{
			name:        "custom host without api",
			token:       "test-token",
			host:        "https://company.com",
			wantBaseURL: "https://company.com/api",
		},
		{
			name:        "custom host with trailing slash",
			token:       "test-token",
			host:        "https://company.com/",
			wantBaseURL: "https://company.com/api",
		},
		{
			name:        "custom host with youtrack path",
			token:       "test-token",
			host:        "https://company.com/youtrack",
			wantBaseURL: "https://company.com/api",
		},
		{
			name:        "custom host with youtrack and trailing slash",
			token:       "test-token",
			host:        "https://company.com/youtrack/",
			wantBaseURL: "https://company.com/api",
		},
		{
			name:        "custom host with api",
			token:       "test-token",
			host:        "https://company.com/api",
			wantBaseURL: "https://company.com/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.token, tt.host)
			if client.baseURL != tt.wantBaseURL {
				t.Errorf("NewClient().baseURL = %q, want %q", client.baseURL, tt.wantBaseURL)
			}
			if client.token != tt.token {
				t.Errorf("NewClient().token = %q, want %q", client.token, tt.token)
			}
		})
	}
}

func TestResolveToken(t *testing.T) {
	// Save and restore original env
	origMehr := os.Getenv("MEHR_YOUTRACK_TOKEN")
	origYt := os.Getenv("YOUTRACK_TOKEN")
	defer func() {
		if origMehr != "" {
			_ = os.Setenv("MEHR_YOUTRACK_TOKEN", origMehr)
		} else {
			_ = os.Unsetenv("MEHR_YOUTRACK_TOKEN")
		}
		if origYt != "" {
			_ = os.Setenv("YOUTRACK_TOKEN", origYt)
		} else {
			_ = os.Unsetenv("YOUTRACK_TOKEN")
		}
	}()

	tests := []struct {
		name         string
		configToken  string
		setMehrToken string
		setYtToken   string
		wantErr      bool
	}{
		{
			name:         "MEHR_YOUTRACK_TOKEN priority",
			configToken:  "config-token",
			setMehrToken: "mehr-token",
			setYtToken:   "yt-token",
			wantErr:      false,
		},
		{
			name:         "YOUTRACK_TOKEN fallback",
			configToken:  "config-token",
			setMehrToken: "",
			setYtToken:   "yt-token",
			wantErr:      false,
		},
		{
			name:         "config token fallback",
			configToken:  "config-token",
			setMehrToken: "",
			setYtToken:   "",
			wantErr:      false,
		},
		{
			name:         "no token available",
			configToken:  "",
			setMehrToken: "",
			setYtToken:   "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all tokens first
			_ = os.Unsetenv("MEHR_YOUTRACK_TOKEN")
			_ = os.Unsetenv("YOUTRACK_TOKEN")

			// Set test values
			if tt.setMehrToken != "" {
				_ = os.Setenv("MEHR_YOUTRACK_TOKEN", tt.setMehrToken)
			}
			if tt.setYtToken != "" {
				_ = os.Setenv("YOUTRACK_TOKEN", tt.setYtToken)
			}

			got, err := ResolveToken(tt.configToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify we got some non-empty token
				if got == "" {
					t.Error("ResolveToken() returned empty token, want non-empty")
				}
			}
		})
	}

	// Test token priority
	t.Run("priority order", func(t *testing.T) {
		// Clear all tokens
		_ = os.Unsetenv("MEHR_YOUTRACK_TOKEN")
		_ = os.Unsetenv("YOUTRACK_TOKEN")

		// Only config token
		got, err := ResolveToken("config-token")
		if err != nil {
			t.Errorf("ResolveToken() with config token error = %v", err)
		}
		if got != "config-token" {
			t.Errorf("ResolveToken() = %q, want \"config-token\"", got)
		}

		// Set YOUTRACK_TOKEN - should take priority
		_ = os.Setenv("YOUTRACK_TOKEN", "yt-token")
		got, err = ResolveToken("config-token")
		if err != nil {
			t.Errorf("ResolveToken() error = %v", err)
		}
		if got != "yt-token" {
			t.Errorf("ResolveToken() = %q, want \"yt-token\"", got)
		}

		// Set MEHR_YOUTRACK_TOKEN - should take highest priority
		_ = os.Setenv("MEHR_YOUTRACK_TOKEN", "mehr-token")
		got, err = ResolveToken("config-token")
		if err != nil {
			t.Errorf("ResolveToken() error = %v", err)
		}
		if got != "mehr-token" {
			t.Errorf("ResolveToken() = %q, want \"mehr-token\"", got)
		}
	})
}

func TestBytesReader(t *testing.T) {
	data := []byte("test data")
	reader := bytesReader(data)
	if reader == nil {
		t.Error("bytesReader() returned nil")
	}
	// Verify it's a strings.Reader
	if _, ok := reader.(*strings.Reader); !ok {
		t.Error("bytesReader() should return *strings.Reader")
	}
}
