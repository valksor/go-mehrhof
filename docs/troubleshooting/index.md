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
ls $(go env GOPATH)/bin/mehr

export PATH="$PATH:$(go env GOPATH)/bin"

echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc
```

### "go: command not found"

**Cause:** Go not installed.

```bash
brew install go

sudo apt install golang-go

go version
```

### Build Fails

**Cause:** Missing dependencies or old Go version.

```bash
go version

go mod tidy

make build
```

---

## Claude CLI Issues

### Claude Not Found

**Cause:** Claude CLI not installed or not in PATH.

```bash
which claude
claude --version

```

### Claude Authentication Issues

**Cause:** Claude CLI not properly configured.

Mehrhof calls Claude CLI, so authentication is handled by Claude:

```bash
claude "Hello"

```

### "Rate limited"

**Cause:** Too many API requests.

```bash
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
mehr start file:task.md

git branch -a | grep task/
git checkout task/abc12345
```

### "Task already exists"

**Cause:** Task with same source already registered.

```bash
mehr abandon --yes

mehr start file:task.md
```

### "Cannot find task"

**Cause:** Work directory missing.

```bash
ls .mehrhof/work/

mehr start file:task.md
```

### "No specifications generated"

**Cause:** Agent couldn't understand requirements.

```bash
vim task.md

mehr plan
```

### "Specifications are incomplete"

**Cause:** Requirements too vague.

```bash
mehr note "Include error handling for all edge cases"

mehr plan
```

### "No specification files found"

**Cause:** Planning phase not run.

```bash
mehr plan

mehr implement
```

### "Implementation timeout"

**Cause:** Task too complex or slow API.

```yaml
# Increase timeout in config.yaml
agent:
  timeout: 900  # 15 minutes
```

Or break into smaller specifications:

```bash
mehr note "Focus on just the API handler first"
mehr plan
```

### "Generated code has errors"

**Cause:** Agent misunderstood requirements.

```bash
mehr undo

mehr note "Use the existing error handling pattern in internal/errors/"

mehr implement
```

### "Cannot undo: no checkpoints"

**Cause:** At initial state.

```bash
mehr status

mehr abandon
mehr start file:task.md
```

### "Cannot redo: nothing to redo"

**Cause:** Redo stack empty.

```bash
git reflog

git checkout abc1234 -- path/to/file
```

---

## Git Issues

### "Not a git repository"

**Cause:** Running in non-git directory.

```bash
git init

cd /path/to/repo
```

### "Working directory dirty"

**Cause:** Uncommitted changes exist.

```bash
git add .
git commit -m "WIP"

git stash

mehr finish
```

### "Merge conflict"

**Cause:** Conflicts during `mehr finish`.

```bash
git status

vim conflicting-file.go

git add conflicting-file.go

git commit
```

### "Cannot delete branch"

**Cause:** Branch is protected or checked out elsewhere.

```bash
git checkout main

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
mehr --verbose plan

cat .mehrhof/config.yaml
```

**"Invalid configuration"**

```bash
mehr config validate

cat .mehrhof/config.yaml | python -c "import yaml,sys; yaml.safe_load(sys.stdin)"
```

---

## Report Issues

1. Note the error message
2. Check Mehrhof version: `mehr version`
3. Gather relevant logs
4. Report at [github.com/valksor/go-mehrhof/issues](https://github.com/valksor/go-mehrhof/issues)
