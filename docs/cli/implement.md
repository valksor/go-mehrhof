# mehr implement

Implement the specifications for the active task.

## Synopsis

```bash
mehr implement [flags]
```

**Aliases:** `impl`, `i`

## Description

The `implement` command runs the implementation phase where the AI agent:

1. Reads all SPEC files in the work directory
2. Reviews notes and context
3. Generates or modifies code
4. Creates a checkpoint for undo support

**Prerequisite:** At least one SPEC file must exist. Run `mehr plan` first.

## Flags

| Flag                   | Short | Type   | Default | Description                       |
| ---------------------- | ----- | ------ | ------- | --------------------------------- |
| `--dry-run`            | `-n`  | bool   | false   | Preview changes without applying  |
| `--verbose`            | `-v`  | bool   | false   | Show agent output in real-time    |
| `--agent-implementing` |       | string |         | Override agent for implementation |

## Examples

### Basic Implementation

```bash
mehr implement
```

Output:

```
Implementing task a1b2c3d4...
Reading 2 specifications...
Modified: src/api/handler.go
Created:  src/api/auth.go
Modified: src/api/routes.go
Implementation complete. 3 files changed.
```

### Dry Run

```bash
mehr implement --dry-run
```

Preview what would change without modifying files:

```
DRY RUN - No changes will be made
Would modify: src/api/handler.go
Would create: src/api/auth.go
Would modify: src/api/routes.go
```

### Verbose Output

```bash
mehr implement --verbose
```

See the agent's reasoning and code generation in real-time.

### Override Implementation Agent

```bash
mehr implement --agent-implementing claude
```

Use a specific agent for code generation. See [AI Agents](../agents/index.md#per-step-agent-configuration).

## What Happens

1. **Validation**
   - Checks for existing SPEC files
   - Fails if no specs found

2. **Context Preparation**
   - Reads all SPEC files
   - Includes notes from `notes.md`
   - Gathers relevant existing code

3. **Agent Execution**
   - Prompts agent with specs and context
   - Agent generates code
   - Output parsed for file changes

4. **File Operations**
   - Creates new files
   - Modifies existing files
   - Reports all changes

5. **Checkpoint**
   - Git commit created
   - Added to undo stack

## File Changes

The agent outputs structured file changes:

```
<<FILE:src/api/handler.go>>
package api

func Handler(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
<<END FILE>>
```

Mehrhof parses this and applies changes safely.

## Iterating

Implementation can be run multiple times:

```bash
mehr implement           # First attempt
mehr undo               # Didn't like it
mehr note "Use interfaces instead of concrete types"
mehr implement          # Try again with guidance
```

## Error Handling

If implementation fails:

```
Error: Implementation failed
Reason: Agent timeout after 300 seconds
Tip: Increase agent.timeout in .mehrhof/config.yaml
```

No changes are applied on error. Your code remains unchanged.

## After Implementation

Review the changes:

```bash
git diff
```

Run tests:

```bash
make test
```

If satisfied:

```bash
mehr finish
```

If not:

```bash
mehr undo
```

## See Also

- [plan](cli/plan.md) - Create specifications first
- [undo](cli/undo.md) - Revert changes
- [finish](cli/finish.md) - Complete the task
