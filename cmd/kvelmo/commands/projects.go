package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var (
	projectsTimeout time.Duration
	projectsVerbose bool
	projectsJSON    bool
)

var ProjectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all registered worktrees",
	Long:  "Query the global socket for all registered worktrees and their status.",
	RunE:  runProjects,
}

var projectsAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Register a project with the global socket",
	Long:  "Register a project directory so it appears in the global project list.",
	Args:  cobra.ExactArgs(1),
	RunE:  runProjectsAdd,
}

var projectsRemoveCmd = &cobra.Command{
	Use:     "remove <id>",
	Aliases: []string{"rm", "unregister"},
	Short:   "Unregister a project from the global socket",
	Long:    "Remove a project from the global project list by its ID or path.",
	Args:    cobra.ExactArgs(1),
	RunE:    runProjectsRemove,
}

func init() {
	ProjectsCmd.Flags().DurationVarP(&projectsTimeout, "timeout", "t", 5*time.Second, "Connection timeout")
	ProjectsCmd.Flags().BoolVarP(&projectsVerbose, "verbose", "v", false, "Show socket paths")
	ProjectsCmd.Flags().BoolVar(&projectsJSON, "json", false, "Output raw JSON response")
	ProjectsCmd.AddCommand(projectsAddCmd)
	ProjectsCmd.AddCommand(projectsRemoveCmd)
}

func runProjects(cmd *cobra.Command, args []string) error {
	globalPath := socket.GlobalSocketPath()

	if projectsVerbose {
		fmt.Printf("Global socket: %s\n\n", globalPath)
	}

	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " start' in a project first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(projectsTimeout))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), projectsTimeout)
	defer cancel()

	resp, err := client.Call(ctx, "projects.list", nil)
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

	if projectsJSON {
		return outputJSON(resp.Result)
	}

	var result socket.ProjectListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse projects: %w", err)
	}

	if len(result.Projects) == 0 {
		fmt.Println("No projects registered")

		return nil
	}

	fmt.Println("Registered Projects:")
	fmt.Println("--------------------")
	for _, p := range result.Projects {
		stateStr := "Task: " + capitalize(p.State)
		fmt.Printf("  %s\n", p.Path)
		fmt.Printf("    State: %s\n", stateStr)
		fmt.Printf("    Last seen: %s\n", p.LastSeen.Format(time.RFC3339))
		if projectsVerbose {
			fmt.Printf("    Socket: %s\n", p.SocketPath)
		}
		fmt.Println()
	}

	return nil
}

func runProjectsRemove(cmd *cobra.Command, args []string) error {
	identifier := args[0]
	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(projectsTimeout))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), projectsTimeout)
	defer cancel()

	_, err = client.Call(ctx, "projects.unregister", socket.UnregisterParams{
		ID: identifier,
	})
	if err != nil {
		return fmt.Errorf("unregister project: %w", err)
	}

	fmt.Printf("Project unregistered: %s\n", identifier)

	return nil
}

func runProjectsAdd(cmd *cobra.Command, args []string) error {
	path := args[0]
	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(projectsTimeout))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), projectsTimeout)
	defer cancel()

	resp, err := client.Call(ctx, "projects.register", socket.RegisterParams{
		Path:       path,
		SocketPath: socket.WorktreeSocketPath(path),
	})
	if err != nil {
		return fmt.Errorf("register project: %w", err)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Project registered: %s (id: %s)\n", path, result.ID)

	return nil
}
