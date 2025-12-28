# Troubleshooting

Solutions for common issues with Mehrhof.

## Installation Issues

### "command not found: mehr"

**Cause:** Binary not in PATH.

**Solution:**

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

**Solution:**

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

**Solution:**

```bash
# Check Go version (need 1.21+)
go version

# Update dependencies
go mod tidy

# Rebuild
make build
```

## Claude CLI Issues

### Claude Not Found

**Cause:** Claude CLI not installed or not in PATH.

**Solution:**

```bash
# Check if Claude is available
which claude
claude --version

# If not found, install Claude CLI first
# Follow Claude's installation instructions
```

### Claude Authentication Issues

**Cause:** Claude CLI not properly configured.

**Solution:**
Mehrhof calls Claude CLI, so authentication is handled by Claude's configuration. Fix issues in Claude first:

```bash
# Test Claude directly
claude "Hello"

# If that fails, configure Claude CLI
# (Follow Claude's setup documentation)
```

### "Rate limited"

**Cause:** Too many API requests.

**Solution:**

```bash
# Wait and retry (automatic)
# Or increase timeout
export MEHR_AGENT_TIMEOUT=600

# Reduce concurrent usage
```

## Task Issues

### "No active task"

**Cause:** No task started or not on task branch.

**Solution:**

```bash
# Start a new task
mehr start task.md

# Or switch to existing task branch
git branch -a | grep task/
git checkout task/abc12345
```

### "Task already exists"

**Cause:** Task with same source already registered.

**Solution:**

```bash
# Delete existing task
mehr delete --yes

# Start fresh
mehr start task.md
```

### "Cannot find task"

**Cause:** Work directory missing or corrupted.

**Solution:**

```bash
# Check if work directory exists
ls .mehrhof/work/

# If missing, start fresh
mehr start task.md
```

## Git Issues

### "Not a git repository"

**Cause:** Running in non-git directory.

**Solution:**

```bash
# Initialize git
git init

# Or navigate to git repo
cd /path/to/repo
```

### "Working directory dirty"

**Cause:** Uncommitted changes exist.

**Solution:**

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

**Solution:**

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

**Solution:**

```bash
# Switch to another branch first
git checkout main

# Force delete if needed
git branch -D task/abc12345
```

## Planning Issues

### "No specifications generated"

**Cause:** Agent couldn't understand requirements.

**Solution:**

```bash
# Add more detail to task file
vim task.md

# Be specific about requirements
# Include examples

# Try again
mehr plan
```

### "Specs are incomplete"

**Cause:** Requirements too vague.

**Solution:**

```bash
# Add clarification
mehr talk "Include error handling for all edge cases"

# Regenerate
mehr plan
```

## Implementation Issues

### "No SPEC files found"

**Cause:** Planning phase not run.

**Solution:**

```bash
# Run planning first
mehr plan

# Then implement
mehr implement
```

### "Implementation timeout"

**Cause:** Task too complex or slow API.

**Solution:**

```bash
# Increase timeout
export MEHR_AGENT_TIMEOUT=900  # 15 minutes

# Or break into smaller specs
mehr talk "Focus on just the API handler first"
mehr plan
```

### "Generated code has errors"

**Cause:** Agent misunderstood requirements.

**Solution:**

```bash
# Undo
mehr undo

# Provide more context
mehr talk "Use the existing error handling pattern in internal/errors/"

# Try again
mehr implement
```

## Undo/Redo Issues

### "Cannot undo: no checkpoints"

**Cause:** At initial state or no checkpoints created.

**Solution:**

```bash
# Check checkpoint status
mehr status

# If truly at initial state, nothing to undo
# Start fresh if needed
mehr delete
mehr start task.md
```

### "Cannot redo: nothing to redo"

**Cause:** Redo stack empty (new changes cleared it).

**Solution:**

```bash
# Check git reflog
git reflog

# Find the commit you want
git show abc1234

# Restore specific files
git checkout abc1234 -- path/to/file
```

## Performance Issues

### "Slow response times"

**Cause:** Large codebase or complex task.

**Solution:**

```bash
# Increase timeout
export MEHR_AGENT_TIMEOUT=600

# Use verbose to see progress
mehr plan --verbose
```

### "High memory usage"

**Cause:** Large session logs or many checkpoints.

**Solution:**

```bash
# Clean old sessions
find .mehrhof/work/*/sessions/ -mtime +7 -delete

# Reduce session retention
export MEHR_STORAGE_SESSIONRETENTIONDAYS=7
```

## Configuration Issues

### "Settings not applied"

**Cause:** Environment variable not set or lower priority config overriding.

**Solution:**

```bash
# Check current value
echo $MEHR_AGENT_DEFAULT

# Environment variables have highest priority
export MEHR_AGENT_DEFAULT=claude

# Check .env.local for overrides
cat .env.local
```

### "Invalid configuration"

**Cause:** Malformed YAML or invalid values.

**Solution:**

```bash
# Validate YAML syntax
cat .mehrhof/config.yaml | python -c "import yaml,sys; yaml.safe_load(sys.stdin)"

# Check valid values
# agent.default: claude
# ui.format: text or json
```

## Getting Help

### Check Version

```bash
mehr version
```

### Enable Verbose Mode

```bash
mehr --verbose <command>
```

### View Logs

```bash
cat .mehrhof/work/*/sessions/*.yaml
```

### Report Issues

1. Note the error message
2. Check Mehrhof version
3. Gather relevant logs
4. Report at project issue tracker

## Quick Fixes

| Problem            | Quick Fix                                          |
| ------------------ | -------------------------------------------------- |
| Command not found  | `export PATH="$PATH:$(go env GOPATH)/bin"`         |
| Claude not working | Ensure Claude CLI is installed: `claude --version` |
| No active task     | `mehr start task.md`                               |
| Bad implementation | `mehr undo`                                        |
| Merge conflict     | Resolve manually, `git add .`, `git commit`        |
| Timeout            | `export MEHR_AGENT_TIMEOUT=600`                    |
| Start fresh        | `mehr delete --yes && mehr start task.md`        |
