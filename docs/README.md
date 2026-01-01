# Valksor Mehrhof

AI-powered task automation for software development. Mehrhof orchestrates AI agents to help you plan, implement, and review code changes.

---

**Note:** Provider and agent integrations with third-party services (GitHub, Jira, Claude, Gemini, etc.) are experimental. Report issues at [github.com/valksor/go-mehrhof/issues](https://github.com/valksor/go-mehrhof/issues).

---

## Get Started in 5 Minutes

```bash
# Install
curl -L https://github.com/valksor/go-mehrhof/releases/latest/download/mehr-darwin-arm64 -o mehr
chmod +x mehr && sudo mv mehr /usr/local/bin/

# Initialize in your project
cd your-project && mehr init

# Create and run a task
echo "Create a /health endpoint returning JSON status" > task.md
mehr start task.md && mehr plan && mehr implement && mehr finish
```

[Full Quickstart Guide](quickstart.md)

## Key Features

- **AI Workflow** - Automatic planning, implementation, and code review
- **Git Integration** - Auto branches, checkpoints, undo/redo, squash merge
- **15+ Task Sources** - GitHub, GitLab, Jira, Linear, Notion, Asana, and more
- **Parallel Tasks** - Run multiple tasks in isolated git worktrees
- **Auto Mode** - Full automation: `mehr auto task.md`

## How It Works

```
mehr init → mehr start → mehr plan → mehr implement → mehr finish
```

1. **init** - Set up workspace (one-time)
2. **start** - Begin task, create git branch
3. **plan** - AI generates implementation specs
4. **implement** - AI executes the specs
5. **finish** - Merge changes, clean up

## Documentation

| Section | Description |
|---------|-------------|
| [Quickstart](quickstart.md) | Install and run your first task |
| [Guides](guides/first-task.md) | Step-by-step tutorials |
| [Concepts](concepts/workflow.md) | Understand the workflow |
| [AI Agents](agents/index.md) | Agent configuration and aliases |
| [Providers](providers/index.md) | Task sources (GitHub, Jira, etc.) |
| [Configuration](configuration/index.md) | Customize behavior |
| [CLI Reference](cli/index.md) | All commands and flags |
| [Troubleshooting](troubleshooting/index.md) | Common issues |
