package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Show the workflow state machine diagram",
	Long: `Display a visual representation of the Mehrhof workflow state machine.

This shows all valid states and transitions in the workflow, helping you understand
how tasks flow from start to completion.

See also:
  mehr status                 - Show current task state
  mehr guide                 - Get context-aware next actions

States:
  idle          Ready for next action (central hub state)
  planning      AI agent creating specifications
  implementing  AI agent implementing code
  reviewing     Code review in progress
  waiting       Waiting for your answer to agent question
  checkpointing Creating git checkpoint
  reverting     Undo operation (restore previous checkpoint)
  restoring     Redo operation (restore forward checkpoint)
  done          Task completed successfully
  failed        Task failed with error`,
	RunE: runWorkflow,
}

func init() {
	rootCmd.AddCommand(workflowCmd)
}

func runWorkflow(cmd *cobra.Command, args []string) error {
	// Display the workflow diagram
	fmt.Print(`
╔══════════════════════════════════════════════════════════════════════════════╗
║                           MEHRHOF WORKFLOW STATE MACHINE                         ║
╚══════════════════════════════════════════════════════════════════════════════╝

                                    ┌─────────────────┐
                                    │    start        │
                                    │  (task created) │
                                    └────────┬────────┘
                                             │
                                             ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              ┌──────────┐                                       │
│                    ┌───────▶│   idle   │◀──────┐                              │
│                    │        └────┬─────┘       │                              │
│                    │             │             │                              │
│  ┌─────────────────┴──────┐     │     ┌───────┴──────────────┐              │
│  │                          │     │     │                       │              │
│  ▼                          ▼     │     ▼                       ▼              │
│┌──────────────┐        ┌─────────────┐   ┌──────────────┐   ┌──────────────┐  │
│ │  restoring  │        │ reverting  │   │  checkpoint  │   │    done      │  │
│ │   (redo)    │        │   (undo)   │   │    (after)   │   │  (finish)    │  │
│ └──────┬───────┘        └─────┬───────┘   └──────┬───────┘   └──────┬───────┘  │
│        │                     │                  │                   │           │
│        └─────────────────────┴──────────────────┴───────────────────┘           │
│                                                                              │
│  ╔════════════════════════════════════════════════════════════════════════╗   │
│  ║                        ACTIVE WORKFLOW PHASES                            ║   │
│  ╠════════════════════════════════════════════════════════════════════════╣   │
│  ║                                                                          ║   │
│  ║   ┌──────────┐     plan      ┌──────────┐                              ║   │
│  ║   │   idle   │──────────────▶│ planning │                              ║   │
│  ║   └──────────┘                └────┬─────┘                              ║   │
│  ║                                      │                                   ║   │
│  ║            ┌─────────────────────────┼─────────────────┐               ║   │
│  ║            │                         │                 │               ║   │
│  ║            ▼                         │                 ▼               ║   │
│  ║     ┌───────────┐            ┌────────┴───────┐   ┌───────────┐     ║   │
│  ║     │ waiting   │◀───────────│    idle       │   │  failed   │     ║   │
│  ║     │(question)│   answer   │  (ready to    │   └───────────┘     ║   │
│  ║     └─────┬─────┘            │   continue)   │                      ║   │
│  ║           │                 └────────────────┘                      ║   │
│  ║           ▼                                                        ║   │
│  ║     ┌───────────┐                                                   ║   │
│  ║     │ planning  │                                                   ║   │
│  ║     │ (resume)  │                                                   ║   │
│  ║     └───────────┘                                                   ║   │
│  ║                                                                          ║   │
│  ║   ┌──────────┐  implement    ┌─────────────┐                          ║   │
│  ║   │   idle   │──────────────▶│implementing │                          ║   │
│  ║   └──────────┘                └──────┬──────┘                          ║   │
│  ║                                      │                                  ║   │
│  ║                                      ▼                                  ║   │
│  ║                              ┌─────────────┐                          ║   │
│  ║                              │    idle     │◀───────┐                   ║   │
│  ║                              └──────┬──────┘        │                   ║   │
│  ║                                     │               │                   ║   │
│  ║                 ┌────────────────────┴─┐   ┌───────┴──────┐           ║   │
│  ║                 │  review (optional)   │   │   finish    │           ║   │
│  ║                 ▼                     │   │              │           ║   │
│  ║          ┌─────────────┐              │   │              ▼           ║   │
│  ║          │  reviewing  │              │   │         ┌───────────┐  ║   │
│  ║          └──────┬──────┘              │   │         │   done    │  ║   │
│  ║                 │                     │   │         └───────────┘  ║   │
│  ║                 ▼                     │   │                            ║   │
│  ║          ┌─────────────┐              │   │                            ║   │
│  ║          │    idle     │              └───┴────────────────────────    ║   │
│  ║          └─────────────┘                                              ║   │
│  ║                                                                          ║   │
│  ╚════════════════════════════════════════════════════════════════════════╝   │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────────┘

COMMANDS BY STATE:

  From idle (central hub):
    mehr start <ref>     Begin new task
    mehr plan             Create specifications
    mehr implement        Generate code from specifications
    mehr review           Run code review (optional)
    mehr finish           Complete task (creates PR or merges)
    mehr undo             Revert to previous checkpoint
    mehr redo             Restore forward checkpoint
    mehr status           Show current state
    mehr guide            Get context-aware suggestions

  From waiting (agent needs input):
    mehr answer "<response>"  Answer agent's question
    mehr note "<message>"     Add a note to the conversation

  From done:
    mehr start <ref>     Start a new task
    mehr status --all     View all tasks

KEY TRANSITIONS:
  • idle → planning         "mehr plan"
  • planning → idle         Planning completes
  • planning → waiting      Agent asks question
  • waiting → idle          Answer provided, ready to continue
  • idle → implementing     "mehr implement" (requires specifications)
  • implementing → idle     Implementation completes
  • idle → reviewing        "mehr review" (optional)
  • reviewing → idle        Review completes
  • idle → done             "mehr finish"
  • idle → reverting        "mehr undo"
  • idle → restoring        "mehr redo"
  • any → failed            Error (reset with "mehr start" on same task)
`)

	return nil
}
