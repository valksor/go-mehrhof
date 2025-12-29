# Tutorial: Your First Task

Learn Mehrhof by building a complete feature from start to finish.

## What We'll Build

A simple HTTP health check endpoint that returns:

- Service status
- Timestamp
- Version number

## Prerequisites

- Mehrhof installed (`mehr version` works)
- Claude CLI installed and configured (`claude --version` works)
- A Go project (or any project)

## Step 1: Create the Task File

Create a markdown file describing what you want:

````bash
cat > health-endpoint.md << 'EOF'
# Add Health Check Endpoint

Create a `/health` endpoint for monitoring.

## Requirements

- Return HTTP 200 when healthy
- JSON response format
- Include these fields:
  - status: "ok" or "error"
  - timestamp: current time in ISO 8601
  - version: application version

## Example Response

```json
{
  "status": "ok",
  "timestamp": "2025-01-15T10:30:00Z",
  "version": "1.0.0"
}
```

## Constraints

- Use the existing HTTP router
- No authentication required
- Response time under 10ms
  EOF

````

**Tip:** Be specific about requirements. The more detail you provide, the better the AI understands your needs.

## Step 2: Initialize (If Needed)

If this is your first time using Mehrhof in this project:

```bash
mehr init
```

You should see:

```
Initialized task workspace
Created: .mehrhof/
Updated: .gitignore
```

## Step 3: Start the Task

Register the task:

```bash
mehr start health-endpoint.md
```

Output:

```
Task registered: a1b2c3d4
Branch created: task/a1b2c3d4
Switched to branch task/a1b2c3d4
```

Verify you're on the new branch:

```bash
git branch --show-current
# task/a1b2c3d4
```

## Step 4: Generate Specifications

Run the planning phase:

```bash
mehr plan
```

The AI analyzes your requirements and creates detailed specs:

```
Planning task a1b2c3d4...
Analyzing requirements...
Created: specification-1.md
Planning complete. 1 specification created.
```

## Step 5: Review the Plan

Check what was generated:

```bash
mehr status
```

```
Task: a1b2c3d4
State: idle
Source: health-endpoint.md
Specifications:
  - specification-1.md (ready)
```

Read the specification:

```bash
cat .mehrhof/work/*/specifications/specification-1.md
```

You'll see a detailed implementation plan:

```markdown
# Health Endpoint Implementation

## Overview

Add a /health endpoint to the existing HTTP server...

## Files to Modify

- cmd/server/main.go - Add route registration
- internal/api/health.go - New handler (create)
- internal/api/health_test.go - Tests (create)

## Implementation Steps

1. Create health handler struct...
2. Implement ServeHTTP method...
3. Register route in main.go...
```

## Step 6: Add Clarification (Optional)

Want to refine the plan? Use `dialogue`:

```bash
mehr chat "Use the chi router pattern we already have"
```

```
Note added. This will be included in implementation.
```

## Step 7: Implement

Generate the code:

```bash
mehr implement
```

```
Implementing task a1b2c3d4...
Reading 1 specification...
Created:  internal/api/health.go
Created:  internal/api/health_test.go
Modified: cmd/server/main.go
Implementation complete. 3 files changed.
```

## Step 8: Review Changes

See what was created:

```bash
git diff --stat
```

```
 cmd/server/main.go          |  5 +++++
 internal/api/health.go      | 42 ++++++++++++++++++++++++++++++
 internal/api/health_test.go | 38 +++++++++++++++++++++++++++
 3 files changed, 85 insertions(+)
```

Review the actual code:

```bash
git diff
```

## Step 9: Test

Run your tests:

```bash
go test ./...
```

Try the endpoint (if you can run the server):

```bash
curl http://localhost:8080/health
```

## Step 10: Iterate If Needed

Not happy with the result?

```bash
# Undo the implementation
mehr undo

# Add more context
mehr chat "The handler should use our standard response helper"

# Try again
mehr implement
```

## Step 11: Finish

When satisfied, complete the task:

```bash
mehr finish
```

```
Finishing task a1b2c3d4...
Running quality checks...
  make quality: PASSED
Merging to main...
  Squash merge: SUCCESS
Cleaning up...
  Branch deleted: task/a1b2c3d4
Task completed!
```

## What You've Learned

1. **Task files** describe what you want in markdown
2. **`mehr start`** creates a branch and workspace
3. **`mehr plan`** generates detailed specifications
4. **`mehr chat`** adds context and refinements
5. **`mehr implement`** generates code from specs
6. **`mehr undo`** reverts if needed
7. **`mehr finish`** merges and cleans up

## Next Steps

- [Iterative Development](tutorials/iterative-development.md) - Refine with chat mode
- [Recovering from Mistakes](tutorials/undo-mistakes.md) - Master undo/redo
- [CLI Reference](../cli/overview.md) - All commands
