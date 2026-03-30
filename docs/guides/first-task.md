# Your First Task

This walkthrough shows the first full task flow with the Web UI as the primary path, while keeping the CLI visible as a companion tool.

## Before You Start

Make sure you have:

- kvelmo installed
- a working project directory
- Git available
- at least one configured agent path

Quick checks:

```bash
kvelmo version
kvelmo diagnose
```

## Step 1: Start the Local Server

From the project you want to work in:

```bash
cd /path/to/your/project
kvelmo serve --open
```

If the browser does not open automatically, go to `http://localhost:6337`.

## Step 2: Create a Task

In the Web UI:

1. Open the target project
2. Start a new task
3. Add a concise title
4. Add a concrete description with enough implementation detail to review

Example task:

```text
Add a GET /hello endpoint that returns "Hello, World!" and include a basic test.
```

## Step 3: Plan

Run the planning step from the browser.

What to check:

- does the task understanding match what you asked for
- does the plan touch the right files or areas
- are there gaps in testing, migration, or rollout expectations

If the plan is off, adjust the task description or add clarifying notes before continuing.

## Step 4: Implement

Run the implementation step.

Use the Web UI to watch:

- live output
- file changes
- task state
- related context such as logs or chat

If you want a terminal view while the same task is running:

```bash
kvelmo status
kvelmo watch
```

## Step 5: Simplify or Optimize if Needed

Some tasks benefit from an extra refinement pass after the first implementation succeeds.

Use:

- `simplify` when the code works but feels heavier than necessary
- `optimize` when you want a quality-focused follow-up pass

```bash
kvelmo simplify
kvelmo optimize
```

These phases are optional, not mandatory for every task.

## Step 6: Review

Review the result before submission.

Focus on:

- diff quality
- test coverage expectations
- findings, policy, or CI signals
- whether the task intent was actually met

If the result is not acceptable, recover instead of forcing it forward:

```bash
kvelmo undo
kvelmo redo
kvelmo reset
```

## Step 7: Submit

Submit the task when it is ready to become a pull request.

At this point kvelmo moves from local execution into the pull request and review process.

## Step 8: Finish

After merge, run cleanup:

```bash
kvelmo finish
```

## CLI Equivalent

If you prefer to run the same flow in the terminal:

```bash
kvelmo start "Add a GET /hello endpoint that returns Hello, World! and include a basic test."
kvelmo plan
kvelmo implement
# kvelmo simplify   # optional cleanup pass
# kvelmo optimize   # optional quality pass
kvelmo review
kvelmo submit
kvelmo finish
```

## What This Tutorial Shows

This is the common path, not the whole surface area.

kvelmo also supports:

- external task providers
- task queues and groups
- recordings, exports, and activity logs
- policy, CI, and security surfaces
- TUI and desktop interfaces over the same local state

## Next Reading

- [Quickstart](/quickstart.md)
- [Web UI Getting Started](/web-ui/getting-started.md)
- [CLI Overview](/cli/index.md)
- [Workflow Concepts](/concepts/workflow.md)
