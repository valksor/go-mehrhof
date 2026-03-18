package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/valksor/kvelmo/cmd/kvelmo/commands"
	"github.com/valksor/kvelmo/pkg/cli"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/watchdog"
)

var rootCmd = &cobra.Command{
	Use:           "kvelmo",
	Short:         "Task lifecycle orchestrator",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s %s (%s) built %s\n", meta.Name, meta.Version, meta.Commit, meta.BuildTime)
	},
}

var licenseCmd = &cobra.Command{
	Use:   "license",
	Short: "Print license information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(meta.License)
	},
}

var genManPagesCmd = &cobra.Command{
	Use:    "gen-man-pages [directory]",
	Short:  "Generate man pages",
	Hidden: true,
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "man"
		if len(args) > 0 {
			dir = args[0]
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create man dir: %w", err)
		}
		header := &doc.GenManHeader{
			Title:   "KVELMO",
			Section: "1",
		}
		if err := doc.GenManTree(cmd.Root(), header, dir); err != nil {
			return fmt.Errorf("generate man pages: %w", err)
		}
		fmt.Printf("Man pages generated in %s/\n", dir)

		return nil
	},
}

func init() {
	rootCmd.Long = meta.Name + ": Socket-first task lifecycle orchestration for AI-assisted development.\n\nTask States:\n  None         No active task\n  Loaded       Task fetched, branch created\n  Planning     Agent generating specification\n  Planned      Specification complete\n  Implementing Agent writing code\n  Implemented  Code complete, ready for review\n  Optimizing   Agent improving code quality (optional)\n  Reviewing    Human review in progress\n  Submitted    PR created"

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(licenseCmd)
	rootCmd.AddCommand(commands.ServeCmd)
	rootCmd.AddCommand(commands.StartCmd)
	rootCmd.AddCommand(commands.StatusCmd)
	rootCmd.AddCommand(commands.WatchCmd)
	rootCmd.AddCommand(commands.TuiCmd)
	rootCmd.AddCommand(commands.StopCmd)
	rootCmd.AddCommand(commands.RetryCmd)
	rootCmd.AddCommand(commands.ShutdownCmd)
	rootCmd.AddCommand(commands.ProjectsCmd)
	rootCmd.AddCommand(commands.WorkersCmd)
	rootCmd.AddCommand(commands.PlanCmd)
	rootCmd.AddCommand(commands.ImplementCmd)
	rootCmd.AddCommand(commands.SimplifyCmd)
	rootCmd.AddCommand(commands.OptimizeCmd)
	rootCmd.AddCommand(commands.ReviewCmd)
	rootCmd.AddCommand(commands.SubmitCmd)
	rootCmd.AddCommand(commands.FinishCmd)
	rootCmd.AddCommand(commands.RefreshCmd)
	rootCmd.AddCommand(commands.UndoCmd)
	rootCmd.AddCommand(commands.RedoCmd)
	rootCmd.AddCommand(commands.ConfigCmd)
	rootCmd.AddCommand(commands.CompletionCmd)
	rootCmd.AddCommand(commands.BrowserCmd)
	rootCmd.AddCommand(commands.AbortCmd)
	rootCmd.AddCommand(commands.ResetCmd)
	rootCmd.AddCommand(commands.ChatCmd)
	rootCmd.AddCommand(commands.CheckpointsCmd)
	rootCmd.AddCommand(commands.DiffCmd)
	rootCmd.AddCommand(commands.GitCmd)
	rootCmd.AddCommand(commands.ScreenshotsCmd)
	rootCmd.AddCommand(commands.MemoryCmd)
	rootCmd.AddCommand(commands.ShowCmd)

	// Core feature commands
	rootCmd.AddCommand(commands.AbandonCmd)
	rootCmd.AddCommand(commands.DeleteCmd)
	rootCmd.AddCommand(commands.UpdateCmd)
	rootCmd.AddCommand(commands.ListCmd)
	rootCmd.AddCommand(commands.FilesCmd)
	rootCmd.AddCommand(commands.BrowseCmd)
	rootCmd.AddCommand(commands.JobsCmd)
	rootCmd.AddCommand(commands.PipeCmd)
	rootCmd.AddCommand(commands.RecordingsCmd)
	rootCmd.AddCommand(commands.DiagnoseCmd)
	rootCmd.AddCommand(commands.CleanupCmd)
	rootCmd.AddCommand(commands.LogsCmd)
	rootCmd.AddCommand(commands.ExplainCmd)
	rootCmd.AddCommand(commands.StatsCmd)

	// Provider commands (login, etc.)
	rootCmd.AddCommand(commands.GitHubCmd)
	rootCmd.AddCommand(commands.GitLabCmd)
	rootCmd.AddCommand(commands.LinearCmd)
	rootCmd.AddCommand(commands.WrikeCmd)

	// Remote operations (approve/merge PR)
	rootCmd.AddCommand(commands.RemoteCmd)

	// Quality gate controls
	rootCmd.AddCommand(commands.QualityCmd)

	// Approval & review gates
	rootCmd.AddCommand(commands.ApproveCmd)
	rootCmd.AddCommand(commands.ChecklistCmd)

	// Security scanning
	rootCmd.AddCommand(commands.SecurityCmd)

	// Task queue management
	rootCmd.AddCommand(commands.QueueCmd)

	// Prompt (PS1 integration)
	rootCmd.AddCommand(commands.PromptCmd)

	// Backup and restore
	rootCmd.AddCommand(commands.BackupCmd)
	rootCmd.AddCommand(commands.RestoreCmd)

	// Observability
	rootCmd.AddCommand(commands.ActivityCmd)

	// Notifications
	rootCmd.AddCommand(commands.NotifyCmd)

	// Workflow policy
	rootCmd.AddCommand(commands.PolicyCmd)

	// Task tagging
	rootCmd.AddCommand(commands.TagCmd)

	// Data export
	rootCmd.AddCommand(commands.ExportCmd)

	// Compliance reports
	rootCmd.AddCommand(commands.ReportCmd)

	// Template catalog
	rootCmd.AddCommand(commands.CatalogCmd)

	// Audit log
	rootCmd.AddCommand(commands.AuditCmd)

	// Access token management
	rootCmd.AddCommand(commands.AccessCmd)

	// CI pipeline status
	rootCmd.AddCommand(commands.CICmd)

	// Workflow transition hooks
	rootCmd.AddCommand(commands.HooksCmd)

	// Agent management
	rootCmd.AddCommand(commands.AgentCmd)

	// Provider connection testing (subcommand of config)
	commands.ConfigCmd.AddCommand(commands.ProviderTestCmd)

	// Interactive tutorial
	rootCmd.AddCommand(commands.TutorialCmd)

	// Batch operations across projects
	rootCmd.AddCommand(commands.BatchCmd)

	// Quick-fix workflow
	rootCmd.AddCommand(commands.QuickCmd)

	// Raw JSON-RPC access
	rootCmd.AddCommand(commands.RPCCmd)

	// Hidden utilities
	rootCmd.AddCommand(genManPagesCmd)

	cli.RegisterPersistentFlags(rootCmd)

	// Enable Cobra's built-in prefix matching for unambiguous command prefixes
	cobra.EnablePrefixMatching = true

	// Start memory leak watchdog before every command.
	// Short-lived commands exit before the window fills; long-running ones
	// (serve, plan, implement, …) are monitored throughout their lifetime.
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		watchdog.Start(context.Background(), watchdog.DefaultConfig())
		cli.InitColor()

		// Configure structured logging
		level := slog.LevelWarn
		if cli.Debug {
			level = slog.LevelDebug
		} else if cli.Verbose {
			level = slog.LevelInfo
		}
		opts := &slog.HandlerOptions{Level: level}
		var handler slog.Handler
		if cli.LogFormat == "json" {
			handler = slog.NewJSONHandler(os.Stderr, opts)
		} else {
			handler = slog.NewTextHandler(os.Stderr, opts)
		}
		slog.SetDefault(slog.New(handler))
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Try disambiguation on "unknown command" errors
		if isUnknownCommandError(err) {
			args := os.Args[1:]
			if len(args) > 0 {
				if match, suggestions := cli.DisambiguateCommand(rootCmd, args[0]); match != nil {
					rootCmd.SetArgs(append([]string{match.Name()}, args[1:]...))
					if err2 := rootCmd.Execute(); err2 != nil {
						fmt.Fprintln(os.Stderr, err2)
						os.Exit(cli.ExitCodeFromError(err2))
					}

					return
				} else if len(suggestions) > 0 {
					fmt.Fprint(os.Stderr, cli.FormatAmbiguousError(args[0], suggestions))
					os.Exit(cli.ExitUsage)
				}
			}
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitCodeFromError(err))
	}
}

// isUnknownCommandError checks whether the error is a cobra "unknown command" error.
func isUnknownCommandError(err error) bool {
	return strings.Contains(err.Error(), "unknown command")
}
