package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/display"
	"github.com/valksor/go-mehrhof/internal/template"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage task templates",
	Long: `Manage and apply task templates for common development patterns.

Templates pre-configure task frontmatter, agent selection, and workflow settings.

Available templates:
  bug-fix  - Bug fix tasks with stricter validation
  feature  - New feature development
  refactor - Code refactoring (quality-focused)
  docs     - Documentation changes (skips quality checks)
  test     - Adding or improving tests
  chore    - Maintenance tasks and chores`,
	RunE: runTemplatesList,
}

var templateShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show template details",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateShow,
}

var templateApplyCmd = &cobra.Command{
	Use:   "apply <name> <file>",
	Short: "Apply template to a task file",
	Long: `Apply a template to a task file, adding frontmatter configuration.

The template frontmatter is merged with any existing frontmatter in the file.
Template values take precedence over existing values.

Examples:
  mehr templates apply bug-fix task.md
  mehr templates apply feature new-feature.md`,
	Args: cobra.ExactArgs(2),
	RunE: runTemplateApply,
}

func init() {
	rootCmd.AddCommand(templatesCmd)
	templatesCmd.AddCommand(templateShowCmd)
	templatesCmd.AddCommand(templateApplyCmd)
}

func runTemplatesList(cmd *cobra.Command, args []string) error {
	names := template.BuiltInTemplates()

	fmt.Println("Available templates:")
	fmt.Println()

	for _, name := range names {
		tpl, err := template.LoadBuiltIn(name)
		if err != nil {
			continue
		}
		fmt.Printf("  %-12s %s\n", display.Bold(name), tpl.GetDescription())
	}

	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  mehr templates show <name>     Show template details")
	fmt.Println("  mehr templates apply <name> <file>   Apply template to file")
	fmt.Println("  mehr start --template <name> file:task.md")

	return nil
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	name := strings.ToLower(args[0])

	tpl, err := template.LoadBuiltIn(name)
	if err != nil {
		return fmt.Errorf("load template: %w", err)
	}

	fmt.Printf("Template: %s\n", display.Bold(tpl.Name))
	fmt.Printf("Description: %s\n\n", tpl.Description)

	// Show frontmatter
	if len(tpl.Frontmatter) > 0 {
		fmt.Println(display.Bold("Frontmatter:"))
		for k, v := range tpl.Frontmatter {
			fmt.Printf("  %s: %v\n", k, v)
		}
		fmt.Println()
	}

	// Show agent config
	if tpl.Agent != "" {
		fmt.Println(display.Bold("Agent:"))
		fmt.Printf("  Default: %s\n", tpl.Agent)
		if len(tpl.AgentSteps) > 0 {
			fmt.Println("  Per-step:")
			for step, cfg := range tpl.AgentSteps {
				fmt.Printf("    %s: %v\n", step, cfg)
			}
		}
		fmt.Println()
	}

	// Show git config
	if len(tpl.Git) > 0 {
		fmt.Println(display.Bold("Git:"))
		for k, v := range tpl.Git {
			fmt.Printf("  %s: %s\n", k, v)
		}
		fmt.Println()
	}

	// Show workflow config
	if len(tpl.Workflow) > 0 {
		fmt.Println(display.Bold("Workflow:"))
		for k, v := range tpl.Workflow {
			fmt.Printf("  %s: %v\n", k, v)
		}
		fmt.Println()
	}

	// Show example
	fmt.Println(display.Bold("Example usage:"))
	fmt.Printf("  mehr templates apply %s my-task.md\n", tpl.Name)
	fmt.Printf("  mehr start --template %s file:my-task.md\n", tpl.Name)

	return nil
}

func runTemplateApply(cmd *cobra.Command, args []string) error {
	name := strings.ToLower(args[0])
	filePath := args[1]

	tpl, err := template.LoadBuiltIn(name)
	if err != nil {
		return fmt.Errorf("load template: %w", err)
	}

	// Read existing content
	var content string
	data, err := os.ReadFile(filePath)
	if err != nil {
		// If file doesn't exist, start with empty content
		if !os.IsNotExist(err) {
			return fmt.Errorf("read file: %w", err)
		}
		// Add a placeholder title if file is new
		content = "# Task Title\n\nDescribe your task here.\n"
	} else {
		content = string(data)
	}

	// Apply template
	newContent := tpl.ApplyToContent(content)

	// Write back
	if err := os.WriteFile(filePath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("Applied template '%s' to %s\n", display.Bold(tpl.Name), filePath)

	// Show what was added
	if len(tpl.Frontmatter) > 0 {
		fmt.Println("\nFrontmatter added:")
		for k, v := range tpl.Frontmatter {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	return nil
}
