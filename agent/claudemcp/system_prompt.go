package claudemcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultSystemPromptTemplate = `# kvelmo orchestration session

You are running inside a kvelmo orchestration session.

  task_id:  %s
  phase:    %s
  worktree: %s

You MUST drive this phase end-to-end through the kvelmo_* MCP tools exposed
in this session. The kvelmo conductor is watching for an explicit completion
signal — you are not done until you have called either:

  - kvelmo_signal_complete (success — finalises the phase, creates the
    checkpoint, advances state)
  - kvelmo_signal_failure  (unrecoverable error)

## Workflow

1. Begin by calling kvelmo_get_task to confirm the task context.
2. For planning phases, call kvelmo_get_specifications to read any prior
   specs before drafting a new one.
3. Save every artifact through kvelmo_save_artifact (kind = spec | plan |
   implementation_summary) so the conductor can see it.
4. Use kvelmo_create_checkpoint after each meaningful unit of work.
5. When the phase is complete, call kvelmo_signal_complete with the phase
   name and a one-line summary.

## Hard rules

- Do not write to files outside the worktree.
- Do not run destructive shell commands.
- Do not exit, ask the user for input, or assume you are done until you have
  called kvelmo_signal_complete or kvelmo_signal_failure.
- Treat the kvelmo_* tools as the only authoritative way to persist work.
  Direct file writes for spec/plan/checkpoint state will be invisible to the
  rest of kvelmo.
`

// writeSystemPrompt writes the orchestration system prompt to dest and
// returns the absolute path. If override is non-empty, the override is used
// verbatim; otherwise the default template is rendered with the given fields.
func writeSystemPrompt(dest, override, taskID, phase, worktree string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("create system prompt dir: %w", err)
	}
	content := override
	if strings.TrimSpace(content) == "" {
		content = fmt.Sprintf(defaultSystemPromptTemplate, taskID, phase, worktree)
	}
	if err := os.WriteFile(dest, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write system prompt: %w", err)
	}

	return nil
}
