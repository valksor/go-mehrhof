package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var JobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Manage and inspect jobs",
	Long:  `List and inspect jobs in the worker pool.`,
}

var jobsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all jobs",
	RunE:  runJobsList,
}

var jobsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get full details of a job",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobsGet,
}

var (
	jobsListJSON bool
	jobsGetJSON  bool
)

func init() {
	JobsCmd.AddCommand(jobsListCmd)
	JobsCmd.AddCommand(jobsGetCmd)

	jobsListCmd.Flags().BoolVar(&jobsListJSON, "json", false, "Output as JSON")
	jobsGetCmd.Flags().BoolVar(&jobsGetJSON, "json", false, "Output as JSON")

	jobsGetCmd.ValidArgsFunction = completeJobIDs
}

func completeJobIDs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client, err := socket.NewClient(socket.GlobalSocketPath(), socket.WithTimeout(2*time.Second))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "jobs.list", nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var result struct {
		Jobs []struct {
			ID string `json:"id"`
		} `json:"jobs"`
	}
	if json.Unmarshal(resp.Result, &result) != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ids := make([]string, 0, len(result.Jobs))
	for _, j := range result.Jobs {
		ids = append(ids, j.ID)
	}

	return ids, cobra.ShellCompDirectiveNoFileComp
}

type JobInfo struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	WorktreeID  string `json:"worktree_id,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	Result      any    `json:"result,omitempty"`
	Error       string `json:"error,omitempty"`
}

func runJobsList(cmd *cobra.Command, args []string) error {
	gPath := socket.GlobalSocketPath()
	if !socket.SocketExists(gPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(gPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "jobs.list", nil)
	if err != nil {
		return fmt.Errorf("jobs.list call: %w", err)
	}

	if jobsListJSON {
		var pretty any
		if jsonErr := json.Unmarshal(resp.Result, &pretty); jsonErr != nil {
			fmt.Println(string(resp.Result))

			return nil
		}
		out, jsonErr := json.MarshalIndent(pretty, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))

			return nil
		}
		fmt.Println(string(out))

		return nil
	}

	var result struct {
		Jobs []JobInfo `json:"jobs"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Jobs) == 0 {
		fmt.Println("No jobs")

		return nil
	}

	fmt.Printf("%-36s  %-16s  %-12s  %-36s  %s\n", "ID", "Type", "Status", "Worktree", "Created")
	fmt.Printf("%-36s  %-16s  %-12s  %-36s  %s\n",
		"-----------------------------------",
		"----------------",
		"------------",
		"-----------------------------------",
		"-------",
	)
	for _, j := range result.Jobs {
		worktree := j.WorktreeID
		if worktree == "" {
			worktree = "-"
		}
		created := j.CreatedAt
		if created == "" {
			created = "-"
		}
		fmt.Printf("%-36s  %-16s  %-12s  %-36s  %s\n", j.ID, j.Type, j.Status, worktree, created)
	}

	return nil
}

func runJobsGet(cmd *cobra.Command, args []string) error {
	id := args[0]

	gPath := socket.GlobalSocketPath()
	if !socket.SocketExists(gPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(gPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "jobs.get", map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("jobs.get call: %w", err)
	}

	if jobsGetJSON {
		var pretty any
		if jsonErr := json.Unmarshal(resp.Result, &pretty); jsonErr != nil {
			fmt.Println(string(resp.Result))

			return nil
		}
		out, jsonErr := json.MarshalIndent(pretty, "", "  ")
		if jsonErr != nil {
			fmt.Println(string(resp.Result))

			return nil
		}
		fmt.Println(string(out))

		return nil
	}

	var job JobInfo
	if err := json.Unmarshal(resp.Result, &job); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("ID:          %s\n", job.ID)
	fmt.Printf("Type:        %s\n", job.Type)
	fmt.Printf("Status:      %s\n", job.Status)
	if job.WorktreeID != "" {
		fmt.Printf("Worktree:    %s\n", job.WorktreeID)
	}
	if job.CreatedAt != "" {
		fmt.Printf("Created:     %s\n", job.CreatedAt)
	}
	if job.StartedAt != "" {
		fmt.Printf("Started:     %s\n", job.StartedAt)
	}
	if job.CompletedAt != "" {
		fmt.Printf("Completed:   %s\n", job.CompletedAt)
	}
	if job.Error != "" {
		fmt.Printf("Error:       %s\n", job.Error)
	}
	if job.Result != nil {
		resultJSON, marshalErr := json.MarshalIndent(job.Result, "             ", "  ")
		if marshalErr == nil {
			fmt.Printf("Result:      %s\n", string(resultJSON))
		}
	}

	return nil
}
