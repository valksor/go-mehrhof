package commands

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/testutil"
)

func TestParseFlexibleTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      time.Time
		wantError string
	}{
		{
			name:  "parses RFC3339 timestamp",
			input: "2026-03-24T15:04:05Z",
			want:  time.Date(2026, time.March, 24, 15, 4, 5, 0, time.UTC),
		},
		{
			name:  "parses date only",
			input: "2026-03-24",
			want:  time.Date(2026, time.March, 24, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "rejects invalid format",
			input:     "03/24/2026",
			wantError: "expected RFC3339 or YYYY-MM-DD format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseFlexibleTime(tt.input)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("parseFlexibleTime(%q) error = nil, want %q", tt.input, tt.wantError)
				}
				if got != (time.Time{}) {
					t.Fatalf("parseFlexibleTime(%q) time = %v, want zero time on error", tt.input, got)
				}
				if err.Error() == "" || err.Error()[:len(tt.wantError)] != tt.wantError {
					t.Fatalf("parseFlexibleTime(%q) error = %q, want prefix %q", tt.input, err.Error(), tt.wantError)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseFlexibleTime(%q) error = %v, want nil", tt.input, err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("parseFlexibleTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFileStatusLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status string
		want   string
	}{
		{status: "added", want: "A"},
		{status: "modified", want: "M"},
		{status: "deleted", want: "D"},
		{status: "renamed", want: "R"},
		{status: "copied", want: "copied"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()

			if got := fileStatusLabel(tt.status); got != tt.want {
				t.Fatalf("fileStatusLabel(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestInferFailedPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lastError string
		want      string
	}{
		{name: "defaults to plan when empty", lastError: "", want: "plan"},
		{name: "detects implement case-insensitively", lastError: "IMPLEMENT phase failed", want: "implement"},
		{name: "detects simplify", lastError: "simplify job crashed", want: "simplify"},
		{name: "detects optimize", lastError: "optimize timed out", want: "optimize"},
		{name: "prefers first matching phase order", lastError: "plan failed before implement retried", want: "implement"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := inferFailedPhase(tt.lastError); got != tt.want {
				t.Fatalf("inferFailedPhase(%q) = %q, want %q", tt.lastError, got, tt.want)
			}
		})
	}
}

func TestValidatePhase(t *testing.T) {
	t.Parallel()

	validPhases := []string{"plan", "implement", "simplify", "optimize"}
	for _, phase := range validPhases {
		t.Run("valid_"+phase, func(t *testing.T) {
			t.Parallel()

			if err := validatePhase(phase); err != nil {
				t.Fatalf("validatePhase(%q) error = %v, want nil", phase, err)
			}
		})
	}

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		err := validatePhase("review")
		if err == nil {
			t.Fatal("validatePhase(review) error = nil, want error")
		}
		const want = "invalid phase \"review\": must be one of plan, implement, simplify, optimize"
		if err.Error() != want {
			t.Fatalf("validatePhase(review) error = %q, want %q", err.Error(), want)
		}
	})
}

func TestCompleteJobIDsWithArgsSkipsLookup(t *testing.T) {
	t.Parallel()

	got, directive := completeJobIDs(&cobra.Command{}, []string{"already-present"}, "")
	if got != nil {
		t.Fatalf("completeJobIDs(...) ids = %v, want nil when args already provided", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completeJobIDs(...) directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
	}
}

func TestIsStaleSocket(t *testing.T) {
	t.Parallel()

	t.Run("missing path is not stale", func(t *testing.T) {
		t.Parallel()

		if isStaleSocket(filepath.Join(testutil.TempDir(t), "missing.sock")) {
			t.Fatal("isStaleSocket(missing) = true, want false")
		}
	})

	t.Run("responsive socket is not stale", func(t *testing.T) {
		socketPath := filepath.Join(testutil.TempDir(t), "live.sock")
		listener, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
		if err != nil {
			t.Fatalf("net.Listen(unix) error = %v", err)
		}
		defer func() {
			_ = listener.Close()
			_ = os.Remove(socketPath)
		}()

		acceptDone := make(chan struct{})
		go func() {
			defer close(acceptDone)
			conn, acceptErr := listener.Accept()
			if acceptErr == nil {
				_ = conn.Close()
			}
		}()

		if isStaleSocket(socketPath) {
			t.Fatal("isStaleSocket(live socket) = true, want false")
		}

		<-acceptDone
	})
}

func TestPrintConfigValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "string", value: "hello", want: "hello\n"},
		{name: "bool", value: true, want: "true\n"},
		{name: "object marshals to json", value: map[string]any{"max": 3}, want: "{\"max\":3}\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				printConfigValue(tt.value)
			})
			if output != tt.want {
				t.Fatalf("printConfigValue(%#v) = %q, want %q", tt.value, output, tt.want)
			}
		})
	}
}

func TestConfigOfflineCommands(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	globalPath := filepath.Join(homeDir, meta.GlobalDir, meta.ConfigFile)
	projectPath := settings.ProjectPath(projectDir)

	globalConfig := []byte("workers:\n  max: 2\nagent:\n  default: codex\n")
	projectConfig := []byte("workers:\n  max: 5\nagent:\n  default: claude\n")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(global config dir) error = %v", err)
	}
	if err := os.WriteFile(globalPath, globalConfig, 0o644); err != nil {
		t.Fatalf("WriteFile(global config) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(project config dir) error = %v", err)
	}
	if err := os.WriteFile(projectPath, projectConfig, 0o644); err != nil {
		t.Fatalf("WriteFile(project config) error = %v", err)
	}

	t.Chdir(projectDir)

	effective, err := loadEffectiveOffline()
	if err != nil {
		t.Fatalf("loadEffectiveOffline() error = %v", err)
	}
	if effective.Workers.Max != 5 {
		t.Fatalf("effective.Workers.Max = %d, want 5", effective.Workers.Max)
	}
	if effective.Agent.Default != "claude" {
		t.Fatalf("effective.Agent.Default = %q, want %q", effective.Agent.Default, "claude")
	}

	showOutput := captureStdout(t, func() {
		if err := configShowOffline(); err != nil {
			t.Fatalf("configShowOffline() error = %v", err)
		}
	})
	if !strings.Contains(showOutput, "\"max\": 5") {
		t.Fatalf("configShowOffline() output = %q, want merged workers.max", showOutput)
	}
	if !strings.Contains(showOutput, "\"default\": \"claude\"") {
		t.Fatalf("configShowOffline() output = %q, want merged agent.default value", showOutput)
	}

	getOutput := captureStdout(t, func() {
		if err := configGetOffline("workers.max"); err != nil {
			t.Fatalf("configGetOffline(workers.max) error = %v", err)
		}
	})
	if getOutput != "5\n" {
		t.Fatalf("configGetOffline(workers.max) output = %q, want %q", getOutput, "5\\n")
	}

	if err := configGetOffline("workers.min"); err == nil {
		t.Fatal("configGetOffline(workers.min) error = nil, want error")
	} else if err.Error() != "unknown configuration path: min" {
		t.Fatalf("configGetOffline(workers.min) error = %q, want %q", err.Error(), "unknown configuration path: min")
	}
}

func TestRunConfigDiff(t *testing.T) {
	projectDir := t.TempDir()
	t.Chdir(projectDir)

	t.Run("missing project config", func(t *testing.T) {
		output := captureStdout(t, func() {
			if err := runConfigDiff(&cobra.Command{}, nil); err != nil {
				t.Fatalf("runConfigDiff() error = %v, want nil", err)
			}
		})
		if !strings.Contains(output, "No project config found") {
			t.Fatalf("runConfigDiff() output = %q, want missing-project message", output)
		}
	})

	t.Run("prints project overrides", func(t *testing.T) {
		projectPath := settings.ProjectPath(projectDir)
		if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(project config dir) error = %v", err)
		}
		if err := os.WriteFile(projectPath, []byte("debug: true\nworkers:\n  max: 9\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(project config) error = %v", err)
		}

		output := captureStdout(t, func() {
			if err := runConfigDiff(&cobra.Command{}, nil); err != nil {
				t.Fatalf("runConfigDiff() error = %v, want nil", err)
			}
		})
		if !strings.Contains(output, "Project overrides:") {
			t.Fatalf("runConfigDiff() output = %q, want header", output)
		}
		if !strings.Contains(output, "\"max\": 9") {
			t.Fatalf("runConfigDiff() output = %q, want workers.max override", output)
		}
	})
}

func TestOutputJSON(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "pretty prints valid json",
			data: []byte(`{"workers":{"max":3}}`),
			want: "{\n  \"workers\": {\n    \"max\": 3\n  }\n}\n",
		},
		{
			name: "falls back to raw bytes for invalid json",
			data: []byte("not-json"),
			want: "not-json\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				if err := outputJSON(tt.data); err != nil {
					t.Fatalf("outputJSON() error = %v, want nil", err)
				}
			})
			if output != tt.want {
				t.Fatalf("outputJSON() output = %q, want %q", output, tt.want)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	outputCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outputCh <- buf.String()
	}()

	fn()
	_ = w.Close()

	output := <-outputCh
	_ = r.Close()

	return output
}
