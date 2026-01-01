# Troubleshooting

Solutions for common issues with Mehrhof.

## Quick Fixes

| Problem | Quick Fix |
|---------|-----------|
| Command not found | `export PATH="$PATH:$(go env GOPATH)/bin"` |
| Claude not working | Ensure Claude CLI is installed: `claude --version` |
| No active task | `mehr start file:task.md` |
| Bad implementation | `mehr undo` |
| Merge conflict | Resolve manually, `git add .`, `git commit` |
| Timeout | Increase `agent.timeout` in `.mehrhof/config.yaml` |
| Start fresh | `mehr abandon --yes && mehr start file:task.md` |

---

## Installation Issues

### "command not found: mehr"

**Cause:** Binary not in PATH.

```bash
# Check if installed
ls $(go env GOPATH)/bin/mehr

# Add to PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Make permanent (add to ~/.bashrc or ~/.zshrc)
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc
```

### "go: command not found"

**Cause:** Go not installed.

```bash
# macOS
brew install go

# Linux
sudo apt install golang-go

# Verify
go version
```

### Build Fails

**Cause:** Missing dependencies or old Go version.

```bash
# Check Go version (need 1.25+)
go version

# Update dependencies
go mod tidy

# Rebuild
make build
```

---

## Claude CLI Issues

### Claude Not Found

**Cause:** Claude CLI not installed or not in PATH.

```bash
# Check if Claude is available
which claude
claude --version

# If not found, install Claude CLI first
```

### Claude Authentication Issues

**Cause:** Claude CLI not properly configured.

Mehrhof calls Claude CLI, so authentication is handled by Claude:

```bash
# Test Claude directly
claude "Hello"

# If that fails, configure Claude CLI first
```

### "Rate limited"

**Cause:** Too many API requests.

```bash
# Wait and retry (automatic based on config)
# Or increase timeout in .mehrhof/config.yaml:
```

```yaml
agent:
  timeout: 600
```

### Slow Response Times

**Cause:** Large codebase or complex task.

```bash
mehr --verbose plan
```

### High Memory Usage

**Cause:** Large session logs.

```bash
# Clean old sessions
find .mehrhof/work/*/sessions/ -mtime +7 -delete
```

Or configure retention:

```yaml
workflow:
  session_retention_days: 7
```

---

## Task Issues

### "No active task"

**Cause:** No task started or not on task branch.

```bash
# Start a new task
mehr start file:task.md

# Or switch to existing task branch
git branch -a | grep task/
git checkout task/abc12345
```

### "Task already exists"

**Cause:** Task with same source already registered.

```bash
# Delete existing task
mehr abandon --yes

# Start fresh
mehr start file:task.md
```

### "Cannot find task"

**Cause:** Work directory missing.

```bash
# Check if work directory exists
ls .mehrhof/work/

# If missing, start fresh
mehr start file:task.md
```

### "No specifications generated"

**Cause:** Agent couldn't understand requirements.

```bash
# Add more detail to task file
vim task.md

# Be specific about requirements, include examples
mehr plan
```

### "Specs are incomplete"

**Cause:** Requirements too vague.

```bash
# Add clarification
mehr note "Include error handling for all edge cases"

# Regenerate
mehr plan
```

### "No SPEC files found"

**Cause:** Planning phase not run.

```bash
# Run planning first
mehr plan

# Then implement
mehr implement
```

### "Implementation timeout"

**Cause:** Task too complex or slow API.

```yaml
# Increase timeout in config.yaml
agent:
  timeout: 900  # 15 minutes
```

Or break into smaller specs:

```bash
mehr note "Focus on just the API handler first"
mehr plan
```

### "Generated code has errors"

**Cause:** Agent misunderstood requirements.

```bash
# Undo
mehr undo

# Provide more context
mehr note "Use the existing error handling pattern in internal/errors/"

# Try again
mehr implement
```

### "Cannot undo: no checkpoints"

**Cause:** At initial state.

```bash
# Check checkpoint status
mehr status

# Start fresh if needed
mehr abandon
mehr start file:task.md
```

### "Cannot redo: nothing to redo"

**Cause:** Redo stack empty.

```bash
# Check git reflog
git reflog

# Restore specific files
git checkout abc1234 -- path/to/file
```

---

## Git Issues

### "Not a git repository"

**Cause:** Running in non-git directory.

```bash
# Initialize git
git init

# Or navigate to git repo
cd /path/to/repo
```

### "Working directory dirty"

**Cause:** Uncommitted changes exist.

```bash
# Commit changes
git add .
git commit -m "WIP"

# Or stash
git stash

# Then run mehr command
mehr finish
```

### "Merge conflict"

**Cause:** Conflicts during `mehr finish`.

```bash
# Check conflicting files
git status

# Resolve conflicts manually
vim conflicting-file.go

# Mark resolved
git add conflicting-file.go

# Complete merge
git commit
```

### "Cannot delete branch"

**Cause:** Branch is protected or checked out elsewhere.

```bash
# Switch to another branch first
git checkout main

# Force delete if needed
git branch -D task/abc12345
```

---

## Diagnostics

### Check Version

```bash
mehr version
```

### Enable Verbose Mode

```bash
mehr --verbose <command>
```

### View Session Logs

```bash
cat .mehrhof/work/*/sessions/*.yaml
```

### Validate Configuration

```bash
mehr config validate
```

### Configuration Issues

**"Settings not applied"**

```bash
# CLI flags override config settings
mehr --verbose plan  # verbose always enabled

# Check config file
cat .mehrhof/config.yaml
```

**"Invalid configuration"**

```bash
# Validate YAML syntax
mehr config validate

# Or manually check
cat .mehrhof/config.yaml | python -c "import yaml,sys; yaml.safe_load(sys.stdin)"
```

---

## Report Issues

1. Note the error message
2. Check Mehrhof version: `mehr version`
3. Gather relevant logs
4. Report at [github.com/valksor/go-mehrhof/issues](https://github.com/valksor/go-mehrhof/issues)
