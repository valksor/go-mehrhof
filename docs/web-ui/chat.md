# Chat

Communicate with AI agents and control the task lifecycle from the chat panel.

## Sending Messages

Type a message and press **Enter** to send. Press **Shift+Enter** for a newline. The AI agent responds in the same thread with streaming output.

Quick actions at the bottom of the chat:

- **Clear chat** — Remove all messages from the current session
- **Export chat** — Download conversation history as JSON

## Slash Commands

Type `/` to open the command autocomplete. Commands drive the task lifecycle directly from chat.

| Command              | Description                                  |
| -------------------- | -------------------------------------------- |
| `/quick <source>`    | Load, implement, and submit in one step      |
| `/plan`              | Run planning phase                           |
| `/plan!`             | Force re-run planning                        |
| `/implement`         | Run implementation phase                     |
| `/implement!`        | Force re-run implementation                  |
| `/simplify`          | Run code simplification pass                 |
| `/optimize`          | Run optimization pass                        |
| `/review`            | Review and approve implementation            |
| `/review fix`        | Review with automatic fixes                  |
| `/submit`            | Submit pull request (opens confirmation)     |
| `/finish`            | Clean up after merge (opens confirmation)    |
| `/undo`              | Undo to previous checkpoint                  |
| `/redo`              | Redo to next checkpoint                      |
| `/stop`              | Stop current operation (preserves state)     |
| `/abort`             | Abort current operation                      |
| `/update`            | Refresh task from source                     |
| `/status`            | Show current task state                      |
| `/explain`           | Ask agent to explain its last action         |
| `/tag add <name>`    | Add a tag to the task                        |
| `/tag remove <name>` | Remove a tag                                 |
| `/tags`              | List current tags                            |
| `/abandon`           | Abandon current task (opens confirmation)    |
| `/delete`            | Delete task permanently (opens confirmation) |

Commands are context-aware and only appear when available for the current task state.

## @file Mentions

Type `@` followed by a filename to reference project files. An autocomplete dropdown searches your project and lets you pick a file. Use **Up/Down** to navigate, **Tab** to select, **Esc** to dismiss.

## Screenshot Attachments

Attach screenshots from the Screenshots panel. Attached screenshots appear as badges above the input. Click the **X** on a badge to remove it, or **Clear all** to remove all attachments. Screenshots are appended to your message when sent.

## Task Source Detection

Pasting a GitHub/GitLab issue URL or a source shorthand (e.g. `github:owner/repo#123`) prompts you to load it as a task.

## Action Buttons

AI and system messages may include action buttons for workflow decisions like approving or rejecting a review. Subagent activity (planning, implementing) appears as inline status indicators showing the operation type, status, and duration.
