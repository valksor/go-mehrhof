# Quick Start Guide

Get started with Mehrhof in 5 minutes.

## Prerequisites

- Go 1.25 or later (for building from source)
- Git
- Claude CLI installed and configured (Mehrhof calls Claude for AI operations)

## Installation

```bash
# Clone and build
git clone <repository-url>
cd go-mehrhof
make install
```

This installs `mehr` to your `$GOPATH/bin`.

## Setup

### 1. Ensure Claude CLI Works

Mehrhof delegates AI operations to Claude CLI. Make sure it's configured:

```bash
# Verify Claude works
claude --version
```

If you haven't set up Claude yet, follow Claude's setup instructions first.

### 2. Initialize a Project

Navigate to your project directory and initialize:

```bash
cd your-project
mehr init
```

This creates a `.mehrhof/` directory for storing task data.

## Your First Task

### Step 1: Create a Task File

Create a markdown file describing what you want to build:

```bash
cat > feature.md << 'EOF'
# Add Health Check Endpoint

Create a `/health` endpoint that returns:
- HTTP 200 when the service is healthy
- JSON response with status and timestamp

Requirements:
- Should not require authentication
- Include version number in response
EOF
```

### Step 2: Start the Task

Register the task and create a branch:

```bash
mehr start feature.md
```

Output:

```
Task registered: a1b2c3d4
Branch created: task/a1b2c3d4
Switched to branch task/a1b2c3d4
```

### Step 3: Generate Specifications

Run the planning phase:

```bash
mehr plan
```

The AI will analyze your requirements and create specification files in `.mehrhof/work/<id>/specifications/`.

### Step 4: Review the Plan

Check what was generated:

```bash
mehr status
```

You can read the specification files directly:

```bash
cat .mehrhof/work/*/specifications/specification-1.md
```

### Step 5: Implement

Run the implementation phase:

```bash
mehr implement
```

The AI generates code based on your specifications.

### Step 6: Review Changes

Check what was changed:

```bash
git diff
```

### Step 7: Complete the Task

When satisfied, finish and merge:

```bash
mehr finish
```

This squash-merges your changes to the main branch and cleans up.

## Common Workflows

### Add Notes During Development

Use `note` to add context:

```bash
mehr note "Use the existing logger instead of fmt.Println"
```

### Undo a Mistake

If the AI made changes you don't want:

```bash
mehr undo
```

You can redo if needed:

```bash
mehr redo
```

### Delete a Task

Abandon a task without merging:

```bash
mehr abandon
```

## Next Steps

- [Workflow Concepts](concepts/workflow.md) - Understand the task lifecycle
- [CLI Reference](cli/overview.md) - All available commands
- [Configuration](configuration/overview.md) - Customize behavior
