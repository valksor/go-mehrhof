# Claude CLI Issues

Problems with the Claude AI agent.

## Claude Not Found

**Cause:** Claude CLI not installed or not in PATH.

**Solution:**

```bash
# Check if Claude is available
which claude
claude --version

# If not found, install Claude CLI first
# Follow Claude's installation instructions
```

## Claude Authentication Issues

**Cause:** Claude CLI not properly configured.

**Solution:**
Mehrhof calls Claude CLI, so authentication is handled by Claude's configuration. Fix issues in Claude first:

```bash
# Test Claude directly
claude "Hello"

# If that fails, configure Claude CLI
# (Follow Claude's setup documentation)
```

## "Rate limited"

**Cause:** Too many API requests.

**Solution:**

```bash
# Wait and retry (automatic based on agent.timeout in config)
# Or increase timeout in .mehrhof/config.yaml:
# agent:
#   timeout: 600
```

## "Slow response times"

**Cause:** Large codebase or complex task.

**Solution:**

Increase timeout in config and use verbose mode to see progress:

```bash
mehr --verbose plan
```

## "High memory usage"

**Cause:** Large session logs or many checkpoints.

**Solution:**

```bash
# Clean old sessions
find .mehrhof/work/*/sessions/ -mtime +7 -delete
```

Or configure retention in `.mehrhof/config.yaml`:

```yaml
workflow:
  session_retention_days: 7
```
