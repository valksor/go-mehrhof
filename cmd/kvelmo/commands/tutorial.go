package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/meta"
)

var TutorialCmd = &cobra.Command{
	Use:   "tutorial",
	Short: "Interactive kvelmo walkthrough",
	Long:  "Step-by-step guide to using kvelmo for AI-assisted development.",
	RunE:  runTutorial,
}

func runTutorial(_ *cobra.Command, _ []string) error {
	fmt.Println("Welcome to kvelmo!")
	fmt.Println()
	fmt.Println("kvelmo orchestrates AI agents to implement tasks from planning through PR submission.")
	fmt.Println()
	fmt.Println("Quick Start:")
	fmt.Println()
	fmt.Printf("  1. Start the server:         %s serve\n", meta.Name)
	fmt.Printf("  2. Load a task:              %s start github:owner/repo#123\n", meta.Name)
	fmt.Printf("  3. Plan the implementation:  %s plan\n", meta.Name)
	fmt.Printf("  4. Implement the plan:       %s implement\n", meta.Name)
	fmt.Printf("  5. Review the changes:       %s review\n", meta.Name)
	fmt.Printf("  6. Submit a PR:              %s submit\n", meta.Name)
	fmt.Printf("  7. Finish after merge:       %s finish\n", meta.Name)
	fmt.Println()
	fmt.Println("Optional steps between implement and review:")
	fmt.Printf("  - Simplify code:             %s simplify\n", meta.Name)
	fmt.Printf("  - Optimize quality:          %s optimize\n", meta.Name)
	fmt.Println()
	fmt.Println("Useful commands:")
	fmt.Printf("  - Check status:              %s status\n", meta.Name)
	fmt.Printf("  - Undo last step:            %s undo\n", meta.Name)
	fmt.Printf("  - View checkpoints:          %s checkpoints\n", meta.Name)
	fmt.Printf("  - Chat with agent:           %s chat\n", meta.Name)
	fmt.Printf("  - View task stats:           %s stats\n", meta.Name)
	fmt.Println()
	fmt.Println("Web UI: Open http://localhost:6337 after running 'serve'")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  - Initialize config:         %s config init\n", meta.Name)
	fmt.Printf("  - Show current config:       %s config show\n", meta.Name)
	fmt.Printf("  - Diagnose setup:            %s diagnose\n", meta.Name)
	fmt.Println()
	fmt.Println("For more: https://github.com/valksor/kvelmo")

	return nil
}
