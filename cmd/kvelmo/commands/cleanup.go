package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var CleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove stale socket files",
	Long: `Remove stale socket files that may be left over from crashed processes.

This command detects sockets that exist but aren't responding and removes them.
Use --dry-run to see what would be removed without actually deleting.
Use --force to skip confirmation prompts.`,
	RunE: runCleanup,
}

func init() {
	CleanupCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompts")
	CleanupCmd.Flags().Bool("dry-run", false, "Show what would be removed without deleting")
	CleanupCmd.Flags().Bool("git", false, "Also detect orphaned git worktrees")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	cleanupForce, _ := cmd.Flags().GetBool("force")
	cleanupDry, _ := cmd.Flags().GetBool("dry-run")
	var staleFiles []string

	// Check global socket
	globalPath := socket.GlobalSocketPath()
	globalSocketStale := isStaleSocket(globalPath)
	if globalSocketStale {
		staleFiles = append(staleFiles, globalPath)
	}

	// Check global lock (stale if socket doesn't exist or is unresponsive)
	lockPath := socket.GlobalLockPath()
	if (!socket.SocketExists(globalPath) || globalSocketStale) && fileExists(lockPath) {
		staleFiles = append(staleFiles, lockPath)
	}

	// Check worktree sockets
	worktreesDir := filepath.Join(socket.BaseDir(), "worktrees")
	if entries, err := os.ReadDir(worktreesDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sock" {
				sockPath := filepath.Join(worktreesDir, entry.Name())
				if isStaleSocket(sockPath) {
					staleFiles = append(staleFiles, sockPath)
				}
			}
		}
	}

	// Check for orphaned git worktrees
	cleanupGit, _ := cmd.Flags().GetBool("git")
	var orphanedWorktrees []string
	var gitRepo *git.Repository
	if cleanupGit {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not determine working directory: %v\n", cwdErr)
		} else if repo, openErr := git.Open(cwd); openErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not open git repository: %v\n", openErr)
		} else {
			gitRepo = repo
			worktrees, listErr := repo.ListWorktrees(context.Background())
			if listErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not list worktrees: %v\n", listErr)
			} else {
				for _, wt := range worktrees {
					if wt.Bare || wt.Path == cwd {
						continue
					}
					// Check if the worktree directory still exists
					if _, statErr := os.Stat(wt.Path); os.IsNotExist(statErr) {
						orphanedWorktrees = append(orphanedWorktrees, wt.Path)
					}
				}
			}
		}
	}

	if len(staleFiles) == 0 && len(orphanedWorktrees) == 0 {
		fmt.Println("No stale sockets or orphaned worktrees found.")

		return nil
	}

	if len(staleFiles) > 0 {
		fmt.Printf("Found %d stale socket(s):\n", len(staleFiles))
		for _, f := range staleFiles {
			fmt.Printf("  • %s\n", f)
		}
	}

	if len(orphanedWorktrees) > 0 {
		fmt.Printf("Found %d orphaned git worktree(s):\n", len(orphanedWorktrees))
		for _, wt := range orphanedWorktrees {
			fmt.Printf("  • %s\n", wt)
		}
	}

	if cleanupDry {
		fmt.Println("\n(dry-run: nothing removed)")

		return nil
	}

	if !cleanupForce {
		fmt.Print("\nClean up? [y/N]: ")
		var response string
		_, _ = fmt.Scanln(&response) // EOF or error treated as decline
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")

			return nil
		}
	}

	removed := 0
	for _, f := range staleFiles {
		if err := os.Remove(f); err != nil {
			fmt.Printf("  ✗ Failed to remove %s: %v\n", f, err)
		} else {
			fmt.Printf("  ✓ Removed %s\n", f)
			removed++
		}
	}

	// Prune orphaned worktree refs
	if len(orphanedWorktrees) > 0 && gitRepo != nil {
		if pruneErr := gitRepo.PruneWorktrees(context.Background()); pruneErr != nil {
			fmt.Printf("  ✗ Failed to prune worktrees: %v\n", pruneErr)
		} else {
			fmt.Printf("  ✓ Pruned %d orphaned worktree ref(s)\n", len(orphanedWorktrees))
			removed += len(orphanedWorktrees)
		}
	}

	fmt.Printf("\nCleaned up %d item(s).\n", removed)
	if len(staleFiles) > 0 {
		fmt.Printf("Run '%s serve' to start fresh.\n", meta.Name)
	}

	return nil
}

// isStaleSocket checks if a socket file exists but isn't responding.
func isStaleSocket(path string) bool {
	if !socket.SocketExists(path) {
		return false
	}

	// Try to connect with short timeout
	client, err := socket.NewClient(path, socket.WithTimeout(500*time.Millisecond))
	if err != nil {
		// Socket exists but can't connect - it's stale
		return true
	}
	_ = client.Close()

	// Socket is responsive - not stale
	return false
}
