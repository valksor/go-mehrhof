package commands

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/meta"
)

// === Command metadata tests ===

func TestAbandonCommand(t *testing.T) {
	if AbandonCmd.Use != "abandon" {
		t.Errorf("Use = %s, want abandon", AbandonCmd.Use)
	}
	if f := AbandonCmd.Flags().Lookup("keep-branch"); f == nil {
		t.Error("--keep-branch flag should exist")
	}
}

func TestAbortCommand(t *testing.T) {
	if AbortCmd.Use != "abort" {
		t.Errorf("Use = %s, want abort", AbortCmd.Use)
	}
}

func TestBrowseCommand(t *testing.T) {
	if BrowseCmd.Use != "browse [path]" {
		t.Errorf("Use = %s, want browse [path]", BrowseCmd.Use)
	}
	if f := BrowseCmd.Flags().Lookup("files"); f == nil {
		t.Error("--files flag should exist")
	}
}

func TestBrowserCommand(t *testing.T) {
	if BrowserCmd.Use != "browser" {
		t.Errorf("Use = %s, want browser", BrowserCmd.Use)
	}
}

func TestChatCommand(t *testing.T) {
	if ChatCmd.Use != "chat" {
		t.Errorf("Use = %s, want chat", ChatCmd.Use)
	}
}

func TestCheckpointsCommand(t *testing.T) {
	if CheckpointsCmd.Use != "checkpoints" {
		t.Errorf("Use = %s, want checkpoints", CheckpointsCmd.Use)
	}
}

func TestCompletionCommand(t *testing.T) {
	if CompletionCmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("Use = %s, want completion [bash|zsh|fish|powershell]", CompletionCmd.Use)
	}
}

func TestConfigCommand(t *testing.T) {
	if ConfigCmd.Use != "config" {
		t.Errorf("Use = %s, want config", ConfigCmd.Use)
	}
}

func TestDeleteCommand(t *testing.T) {
	if DeleteCmd.Use != "delete" {
		t.Errorf("Use = %s, want delete", DeleteCmd.Use)
	}
	if f := DeleteCmd.Flags().Lookup("delete-branch"); f == nil {
		t.Error("--delete-branch flag should exist")
	}
}

func TestFilesCommand(t *testing.T) {
	if FilesCmd.Use != "files" {
		t.Errorf("Use = %s, want files", FilesCmd.Use)
	}
}

func TestGitCommand(t *testing.T) {
	if GitCmd.Use != "git" {
		t.Errorf("Use = %s, want git", GitCmd.Use)
	}
}

func TestJobsCommand(t *testing.T) {
	if JobsCmd.Use != "jobs" {
		t.Errorf("Use = %s, want jobs", JobsCmd.Use)
	}
}

func TestListCommand(t *testing.T) {
	if ListCmd.Use != "list" {
		t.Errorf("Use = %s, want list", ListCmd.Use)
	}
}

func TestMemoryCommand(t *testing.T) {
	if MemoryCmd.Use != "memory" {
		t.Errorf("Use = %s, want memory", MemoryCmd.Use)
	}
}

func TestOptimizeCommand(t *testing.T) {
	if OptimizeCmd.Use != "optimize" {
		t.Errorf("Use = %s, want optimize", OptimizeCmd.Use)
	}
}

func TestPipeCommand(t *testing.T) {
	if PipeCmd.Use != "pipe [prompt]" {
		t.Errorf("Use = %s, want pipe [prompt]", PipeCmd.Use)
	}
	if f := PipeCmd.Flags().Lookup("agent"); f == nil {
		t.Error("--agent flag should exist")
	}
	if f := PipeCmd.Flags().Lookup("timeout"); f == nil {
		t.Error("--timeout flag should exist")
	}
}

func TestResetCommand(t *testing.T) {
	if ResetCmd.Use != "reset" {
		t.Errorf("Use = %s, want reset", ResetCmd.Use)
	}
}

func TestScreenshotsCommand(t *testing.T) {
	if ScreenshotsCmd.Use != "screenshots" {
		t.Errorf("Use = %s, want screenshots", ScreenshotsCmd.Use)
	}
}

func TestSimplifyCommand(t *testing.T) {
	if SimplifyCmd.Use != "simplify" {
		t.Errorf("Use = %s, want simplify", SimplifyCmd.Use)
	}
}

func TestUpdateCommand(t *testing.T) {
	if UpdateCmd.Use != "update" {
		t.Errorf("Use = %s, want update", UpdateCmd.Use)
	}
}

// === No-socket error path tests ===

func TestRunBrowse_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowse(BrowseCmd, nil); err == nil {
		t.Error("runBrowse() expected error (no socket), got nil")
	}
}

func TestRunChatSend_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runChatSend(chatSendCmd, []string{"hello"}); err == nil {
		t.Error("runChatSend() expected error (no socket), got nil")
	}
}

func TestRunChatStop_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runChatStop(chatStopCmd, nil); err == nil {
		t.Error("runChatStop() expected error (no socket), got nil")
	}
}

func TestRunChatHistory_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runChatHistory(chatHistoryCmd, nil); err == nil {
		t.Error("runChatHistory() expected error (no socket), got nil")
	}
}

func TestRunChatClear_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runChatClear(chatClearCmd, nil); err == nil {
		t.Error("runChatClear() expected error (no socket), got nil")
	}
}

func TestConfigShowCmd_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	// config show works offline via direct file read fallback.
	if err := configShowCmd.RunE(configShowCmd, nil); err != nil {
		t.Errorf("configShowCmd offline fallback failed: %v", err)
	}
}

func TestConfigGetCmd_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	// config get works offline via direct file read fallback.
	if err := configGetCmd.RunE(configGetCmd, []string{"workers.max"}); err != nil {
		t.Errorf("configGetCmd offline fallback failed: %v", err)
	}
}

func TestConfigSetCmd_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := configSetCmd.RunE(configSetCmd, []string{"workers.max", "5"}); err == nil {
		t.Error("configSetCmd expected error (no socket), got nil")
	}
}

func TestConfigInitCmd_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := configInitCmd.RunE(configInitCmd, nil); err == nil {
		t.Error("configInitCmd expected error (no socket), got nil")
	}
}

func TestRunFilesSearch_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runFilesSearch(filesSearchCmd, []string{"query"}); err == nil {
		t.Error("runFilesSearch() expected error (no socket), got nil")
	}
}

func TestRunFilesList_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runFilesList(filesListCmd, nil); err == nil {
		t.Error("runFilesList() expected error (no socket), got nil")
	}
}

func TestRunGitStatus_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runGitStatus(gitStatusCmd, nil); err == nil {
		t.Error("runGitStatus() expected error (no socket), got nil")
	}
}

func TestRunGitDiff_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runGitDiff(gitDiffCmd, nil); err == nil {
		t.Error("runGitDiff() expected error (no socket), got nil")
	}
}

func TestRunGitLog_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runGitLog(gitLogCmd, nil); err == nil {
		t.Error("runGitLog() expected error (no socket), got nil")
	}
}

func TestRunJobsGet_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runJobsGet(jobsGetCmd, []string{"job-123"}); err == nil {
		t.Error("runJobsGet() expected error (no socket), got nil")
	}
}

func TestRunMemorySearch_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runMemorySearch(memorySearchCmd, []string{"query"}); err == nil {
		t.Error("runMemorySearch() expected error (no socket), got nil")
	}
}

func TestRunMemoryStats_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runMemoryStats(memoryStatsCmd, nil); err == nil {
		t.Error("runMemoryStats() expected error (no socket), got nil")
	}
}

func TestRunMemoryClear_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runMemoryClear(memoryClearCmd, nil); err == nil {
		t.Error("runMemoryClear() expected error (no socket), got nil")
	}
}

func TestRunScreenshotsList_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runScreenshotsList(screenshotsListCmd, nil); err == nil {
		t.Error("runScreenshotsList() expected error (no socket), got nil")
	}
}

func TestRunScreenshotsDelete_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runScreenshotsDelete(screenshotsDeleteCmd, []string{"screenshot-123"}); err == nil {
		t.Error("runScreenshotsDelete() expected error (no socket), got nil")
	}
}

func TestRunScreenshotsGet_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runScreenshotsGet(screenshotsGetCmd, []string{"screenshot-123"}); err == nil {
		t.Error("runScreenshotsGet() expected error (no socket), got nil")
	}
}

func TestRunWorkersAdd_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runWorkersAdd(workersAddCmd, nil); err == nil {
		t.Error("runWorkersAdd() expected error (no socket), got nil")
	}
}

func TestRunWorkersRemove_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runWorkersRemove(workersRemoveCmd, []string{"worker-1"}); err == nil {
		t.Error("runWorkersRemove() expected error (no socket), got nil")
	}
}

func TestRunWorkersStats_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runWorkersStats(workersStatsCmd, nil); err == nil {
		t.Error("runWorkersStats() expected error (no socket), got nil")
	}
}

func TestStatsCommand(t *testing.T) {
	if StatsCmd.Use != "stats" {
		t.Errorf("Use = %s, want stats", StatsCmd.Use)
	}
	if f := StatsCmd.Flags().Lookup("json"); f == nil {
		t.Error("--json flag should exist")
	}
	if f := StatsCmd.Flags().Lookup("all"); f == nil {
		t.Error("--all flag should exist")
	}
}

func TestWatchCommand(t *testing.T) {
	if WatchCmd.Use != "watch" {
		t.Errorf("Use = %s, want watch", WatchCmd.Use)
	}
	if f := WatchCmd.Flags().Lookup("json"); f == nil {
		t.Error("--json flag should exist")
	}
}

func TestDiffCommand(t *testing.T) {
	if DiffCmd.Use != "diff" {
		t.Errorf("Use = %s, want diff", DiffCmd.Use)
	}
	if f := DiffCmd.Flags().Lookup("stat"); f == nil {
		t.Error("--stat flag should exist")
	}
}

func TestShowCommand(t *testing.T) {
	if ShowCmd.Use != "show" {
		t.Errorf("Use = %s, want show", ShowCmd.Use)
	}
	if f := ShowCmd.PersistentFlags().Lookup("json"); f == nil {
		t.Error("--json persistent flag should exist")
	}
}

func TestExplainCommand(t *testing.T) {
	if ExplainCmd.Use != "explain" {
		t.Errorf("Use = %s, want explain", ExplainCmd.Use)
	}
	if f := ExplainCmd.Flags().Lookup("prompt"); f == nil {
		t.Error("--prompt flag should exist")
	}
}

func TestLogsCommand(t *testing.T) {
	if LogsCmd.Use != "logs" {
		t.Errorf("Use = %s, want logs", LogsCmd.Use)
	}
	if f := LogsCmd.Flags().Lookup("limit"); f == nil {
		t.Error("--limit flag should exist")
	}
	if f := LogsCmd.Flags().Lookup("full"); f == nil {
		t.Error("--full flag should exist")
	}
	if f := LogsCmd.Flags().Lookup("json"); f == nil {
		t.Error("--json flag should exist")
	}
}

func TestCleanupCommand(t *testing.T) {
	if CleanupCmd.Use != "cleanup" {
		t.Errorf("Use = %s, want cleanup", CleanupCmd.Use)
	}
	if f := CleanupCmd.Flags().Lookup("force"); f == nil {
		t.Error("--force flag should exist")
	}
	if f := CleanupCmd.Flags().Lookup("dry-run"); f == nil {
		t.Error("--dry-run flag should exist")
	}
}

func TestDiagnoseCommand(t *testing.T) {
	if DiagnoseCmd.Use != "diagnose" {
		t.Errorf("Use = %s, want diagnose", DiagnoseCmd.Use)
	}
}

func TestRemoteCommand(t *testing.T) {
	if RemoteCmd.Use != "remote" {
		t.Errorf("Use = %s, want remote", RemoteCmd.Use)
	}
	// Verify subcommands
	subs := RemoteCmd.Commands()
	subNames := make(map[string]bool)
	for _, sub := range subs {
		subNames[sub.Use] = true
	}
	if !subNames["approve"] {
		t.Error("missing subcommand 'approve'")
	}
	if !subNames["merge"] {
		t.Error("missing subcommand 'merge'")
	}
}

func TestRemoteApproveFlags(t *testing.T) {
	if f := RemoteApproveCmd.Flags().Lookup("comment"); f == nil {
		t.Error("--comment flag should exist on approve")
	}
}

func TestRemoteMergeFlags(t *testing.T) {
	if f := RemoteMergeCmd.Flags().Lookup("method"); f == nil {
		t.Error("--method flag should exist on merge")
	}
}

func TestFinishCommandFlags(t *testing.T) {
	if FinishCmd.Use != "finish" {
		t.Errorf("Use = %s, want finish", FinishCmd.Use)
	}
	if f := FinishCmd.Flags().Lookup("delete-remote"); f == nil {
		t.Error("--delete-remote flag should exist")
	}
	if f := FinishCmd.Flags().Lookup("force"); f == nil {
		t.Error("--force flag should exist")
	}
}

func TestRefreshCommand(t *testing.T) {
	if RefreshCmd.Use != "refresh" {
		t.Errorf("Use = %s, want refresh", RefreshCmd.Use)
	}
}

func TestQualityCommand(t *testing.T) {
	if QualityCmd.Use != "quality" {
		t.Errorf("Use = %s, want quality", QualityCmd.Use)
	}
}

func TestGitHubProviderCommand(t *testing.T) {
	if GitHubCmd.Use != "github" {
		t.Errorf("Use = %s, want github", GitHubCmd.Use)
	}
	// Verify login subcommand exists
	loginCmd := findProviderLogin(GitHubCmd)
	if loginCmd == nil {
		t.Error("missing login subcommand for github")
	}
}

func TestGitLabProviderCommand(t *testing.T) {
	if GitLabCmd.Use != "gitlab" {
		t.Errorf("Use = %s, want gitlab", GitLabCmd.Use)
	}
	loginCmd := findProviderLogin(GitLabCmd)
	if loginCmd == nil {
		t.Error("missing login subcommand for gitlab")
	}
}

func TestLinearProviderCommand(t *testing.T) {
	if LinearCmd.Use != "linear" {
		t.Errorf("Use = %s, want linear", LinearCmd.Use)
	}
	loginCmd := findProviderLogin(LinearCmd)
	if loginCmd == nil {
		t.Error("missing login subcommand for linear")
	}
}

func TestWrikeProviderCommand(t *testing.T) {
	if WrikeCmd.Use != "wrike" {
		t.Errorf("Use = %s, want wrike", WrikeCmd.Use)
	}
	loginCmd := findProviderLogin(WrikeCmd)
	if loginCmd == nil {
		t.Error("missing login subcommand for wrike")
	}
}

func TestReviewCommandFlags(t *testing.T) {
	if ReviewCmd.Use != "review" {
		t.Errorf("Use = %s, want review", ReviewCmd.Use)
	}
	for _, flag := range []string{"approve", "reject", "message", "fix"} {
		if f := ReviewCmd.Flags().Lookup(flag); f == nil {
			t.Errorf("--%s flag should exist on review", flag)
		}
	}
}

func TestCheckpointsGotoSubcommand(t *testing.T) {
	// CheckpointsCmd should have a "goto" subcommand
	var found bool
	for _, sub := range CheckpointsCmd.Commands() {
		if sub.Use == "goto <sha>" {
			found = true

			break
		}
	}
	if !found {
		t.Error("missing 'goto' subcommand on checkpoints")
	}
}

// findProviderLogin finds the login subcommand of a provider command.
func findProviderLogin(cmd *cobra.Command) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Use == "login" {
			return sub
		}
	}

	return nil
}

// === Pure function tests ===

func TestNestedGet(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]any
		path    string
		want    any
		wantErr bool
	}{
		{
			name: "top-level string",
			m:    map[string]any{"foo": "bar"},
			path: "foo",
			want: "bar",
		},
		{
			name: "nested value",
			m:    map[string]any{"workers": map[string]any{"max": 5}},
			path: "workers.max",
			want: 5,
		},
		{
			name:    "missing top-level key",
			m:       map[string]any{"foo": "bar"},
			path:    "baz",
			wantErr: true,
		},
		{
			name:    "nested path on non-map value",
			m:       map[string]any{"foo": "bar"},
			path:    "foo.baz",
			wantErr: true,
		},
		{
			name: "deeply nested",
			m:    map[string]any{"a": map[string]any{"b": map[string]any{"c": true}}},
			path: "a.b",
			want: map[string]any{"c": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nestedGet(tt.m, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("nestedGet() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !tt.wantErr {
				// Simple equality check for basic types
				switch w := tt.want.(type) {
				case string:
					if got != w {
						t.Errorf("nestedGet() = %v, want %v", got, w)
					}
				case int:
					if got != w {
						t.Errorf("nestedGet() = %v, want %v", got, w)
					}
				case map[string]any:
					if got == nil {
						t.Errorf("nestedGet() = nil, want map")
					}
				}
			}
		})
	}
}
