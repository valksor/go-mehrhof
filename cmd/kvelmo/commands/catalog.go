package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var catalogListJSON bool

var CatalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Task template catalog",
	Long:  "Browse, import, and use task templates for standardized workflows.",
}

var catalogListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates",
	RunE:  runCatalogList,
}

var catalogUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Start a task from a template",
	Args:  cobra.ExactArgs(1),
	RunE:  runCatalogUse,
}

var catalogAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Import a template file into the catalog",
	Args:  cobra.ExactArgs(1),
	RunE:  runCatalogAdd,
}

func init() {
	catalogListCmd.Flags().BoolVar(&catalogListJSON, "json", false, "Output as JSON")

	CatalogCmd.AddCommand(catalogListCmd)
	CatalogCmd.AddCommand(catalogUseCmd)
	CatalogCmd.AddCommand(catalogAddCmd)
}

func runCatalogList(_ *cobra.Command, _ []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "catalog.list", nil)
	if err != nil {
		return fmt.Errorf("catalog.list: %w", err)
	}

	if catalogListJSON {
		fmt.Println(string(resp.Result))

		return nil
	}

	var result struct {
		Templates []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Source      string `json:"source"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	if len(result.Templates) == 0 {
		fmt.Println("No templates in catalog.")
		fmt.Println("Add templates with: kvelmo catalog add <path>")

		return nil
	}

	for _, t := range result.Templates {
		fmt.Printf("  %-20s %s\n", t.Name, t.Description)
	}

	return nil
}

func runCatalogUse(_ *cobra.Command, args []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "catalog.get", map[string]any{"name": args[0]})
	if err != nil {
		return fmt.Errorf("catalog.get: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("catalog.get: %s", resp.Error.Message)
	}

	// Extract template source
	var tmpl struct {
		Source      string `json:"source"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(resp.Result, &tmpl); err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	if tmpl.Source == "" {
		// No source in template, just show the template info
		fmt.Printf("Template: %s\n", tmpl.Name)
		if tmpl.Description != "" {
			fmt.Printf("Description: %s\n", tmpl.Description)
		}
		fmt.Println("This template has no source to start from.")

		return nil
	}

	// Start task from template source via worktree socket
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		return fmt.Errorf("no worktree socket running\nRun '%s start' first, then use catalog use", meta.Name)
	}

	wtClient, err := socket.NewClient(wtPath, socket.WithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree: %w", err)
	}
	defer func() { _ = wtClient.Close() }()

	wtCtx, wtCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wtCancel()

	_, err = wtClient.Call(wtCtx, "start", map[string]any{"source": tmpl.Source})
	if err != nil {
		return fmt.Errorf("start from template: %w", err)
	}

	fmt.Printf("Task started from template '%s'\n", tmpl.Name)
	fmt.Printf("Source: %s\n", tmpl.Source)

	return nil
}

func runCatalogAdd(_ *cobra.Command, args []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "catalog.import", map[string]any{"path": args[0]})
	if err != nil {
		return fmt.Errorf("catalog.import: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("catalog.import: %s", resp.Error.Message)
	}

	fmt.Println("Template imported successfully.")

	return nil
}
