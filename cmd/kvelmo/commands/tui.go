package commands

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/tui"
)

// TuiCmd opens the interactive terminal UI for the current project.
var TuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the interactive terminal UI",
	Long:  "Open the full-screen terminal UI for the current project. Streams live output, supports chat, and allows workflow control without a browser.",
	RunE:  runTui,
}

func init() {
	TuiCmd.Flags().StringP("layout", "l", "", "Layout: stacked (default) or dashboard")
}

func runTui(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	layout := tui.LayoutStacked
	cfg, _, _, err := settings.LoadEffective(cwd)
	if err == nil && cfg != nil && cfg.TUI.Layout == "dashboard" {
		layout = tui.LayoutDashboard
	}
	if layoutFlag, _ := cmd.Flags().GetString("layout"); layoutFlag != "" {
		switch layoutFlag {
		case "dashboard":
			layout = tui.LayoutDashboard
		case "stacked":
			layout = tui.LayoutStacked
		default:
			return fmt.Errorf("unknown layout %q: must be stacked or dashboard", layoutFlag)
		}
	}

	m := tui.NewModel(cwd, layout)
	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()

	return err
}
