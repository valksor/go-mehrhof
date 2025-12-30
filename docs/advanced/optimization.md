# Performance & Cost Optimization

Optimize Mehrhof for speed, cost efficiency, and resource usage.

## Provider Caching

Mehrhof includes an **in-memory TTL cache** for provider API responses. This reduces redundant API calls and helps avoid rate limits.

### How It Works

The cache stores provider responses with configurable time-to-live (TTL) values:

| Resource Type | Default TTL | Description |
|--------------|-------------|-------------|
| Issues | 5 minutes | Task/issue data from providers |
| Comments | 1 minute | Issue comments and discussions |
| Metadata | 30 minutes | Repositories, projects, users |
| Databases | 1 hour | Database queries and listings |
| Plugins | 10 minutes | Plugin metadata and manifests |

### Configuration

Enable or disable caching globally:

```yaml
# .mehrhof/config.yaml
cache:
  enabled: true
```

Disable caching for a specific provider:

```yaml
github:
  cache:
    disabled: false
```

### Cache Behavior

- **Thread-safe**: Concurrent access is protected by read-write mutexes
- **Automatic expiration**: Expired entries are removed on access
- **Periodic cleanup**: Optional background cleanup of stale entries
- **In-memory only**: Cache is lost on process restart

### When to Disable Caching

Disable caching only when:
- Testing provider integration changes
- Debugging stale data issues
- Working with frequently-changing resources

## Agent Selection Strategy

Choosing the right agent for each workflow step balances cost, speed, and quality.

### Per-Step Agent Configuration

Configure different agents for different workflow phases:

```yaml
# .mehrhof/config.yaml
agent:
  default: claude
  steps:
    planning:
      name: claude          # Use Opus for planning (quality > speed)
      env:
        MAX_TOKENS: "16384"
      args: ["--max-turns", "15"]
    implementing:
      name: claude-sonnet   # Use Sonnet for implementation (speed > quality)
    reviewing:
      name: claude          # Use Opus for review (catches more issues)
```

### Cost vs Capability Trade-offs

| Agent | Use Case | Cost | Speed | Quality |
|-------|----------|------|-------|---------|
| claude-opus | Planning, code review | High | Slow | Best |
| claude-sonnet | Implementation | Medium | Fast | Good |
| claude-haiku | Quick questions | Low | Fastest | Adequate |

### Recommended Patterns

**Quality-critical tasks:**
```yaml
agent:
  steps:
    planning: claude-opus
    implementing: claude-sonnet
    reviewing: claude-opus
```

**Cost-optimized tasks:**
```yaml
agent:
  steps:
    planning: claude-sonnet
    implementing: claude-haiku
    reviewing: claude-sonnet
```

## Timeout and Retry Configuration

Adjust timeouts and retries to match your network conditions and task complexity.

```yaml
# .mehrhof/config.yaml
agent:
  timeout: 300      # Timeout in seconds (default: 5 minutes)
  max_retries: 3    # Retry failed requests (default: 3)
```

### Guidelines

| Scenario | Timeout | Retries |
|----------|---------|---------|
| Stable network, simple tasks | 180 | 2 |
| Default (most cases) | 300 | 3 |
| Unstable network | 600 | 5 |
| Large codebases, complex tasks | 900 | 3 |

### Timeout Errors

If you see timeout errors:

```
Error: Agent timeout after 300 seconds
```

Increase the timeout in your config:

```yaml
agent:
  timeout: 600  # 10 minutes
```

## Parallel Tasks with Worktrees

Git worktrees enable multiple tasks to run simultaneously without conflicts.

### Setting Up Worktrees

```bash
# Create a worktree for a parallel task
mehr start --worktree file:feature-a.md

# The worktree is created at ../project-worktrees/<task-id>/
cd ../project-worktrees/<task-id>

# Work on this task independently
mehr plan && mehr implement
```

### Benefits

- **True parallelism**: Multiple tasks run simultaneously
- **Isolation**: Each task has its own working directory
- **No context switching**: Stay focused on one task per worktree
- **Safe git operations**: No branch conflicts

### Worktree Best Practices

1. **One task per worktree**: Avoid mixing multiple tasks in one worktree
2. **Clean up when done**: Remove completed worktrees to save disk space
3. **Name descriptively**: Use clear task references for easy identification

### Removing Worktrees

```bash
# After finishing a task
cd ../project-worktrees/<task-id>
mehr finish

# Remove the worktree
git worktree remove ../project-worktrees/<task-id>
```

### Limitations

- Each worktree consumes disk space (full working copy)
- Git operations run separately (no shared git state)
- Some commands may not auto-detect the task from worktree

## Performance Tuning

### Session Retention

Old sessions consume disk space. Adjust retention to match your workflow:

```yaml
# .mehrhof/config.yaml
workflow:
  session_retention_days: 30  # Default: 30 days
```

Reduce for disk savings, increase for audit trails.

### Auto-Commit Behavior

Disable auto-commit for manual git control:

```yaml
# .mehrhof/config.yaml
git:
  auto_commit: false
```

Use this when you want to:
- Review changes before committing
- Use custom commit messages
- Integrate with custom git workflows

## Cost Tracking

Monitor token usage and costs with the `cost` command:

```bash
mehr cost                 # Show current task costs
mehr cost --by-step       # Breakdown by workflow step
mehr cost --all           # Show all historical tasks
mehr cost --summary       # Aggregate statistics
```

### Cost Reduction Tips

1. **Use Sonnet for implementation**: Code generation is straightforward for faster models
2. **Limit max turns**: Prevent runaway conversations
   ```yaml
   agents:
     sonnet-fast:
       extends: claude-sonnet
       args: ["--max-turns", "5"]
   ```
3. **Reuse specs**: Run `mehr implement` multiple times on the same spec instead of replanning
4. **Chat efficiently**: Provide clear context to reduce back-and-forth

## Resource Cleanup

### Clean Up Old Sessions

```bash
# Remove sessions older than retention period
# This happens automatically, but you can trigger manually:
rm -rf .mehrhof/work/*/sessions/*
```

### Clean Up Old Tasks

```bash
# List all tasks
mehr list

# Abandon and remove completed tasks
mehr abandon <task-id>
rm -rf .mehrhof/work/<task-id>
```

### Cache Clearing

The cache is in-memory and clears automatically on process exit. No manual cleanup needed.

## See Also

- [Agents - Per-Step Configuration](../concepts/agents.md#per-step-agent-configuration)
- [Configuration Files](../configuration/files.md)
- [Tasks - Worktrees](../concepts/tasks.md#parallel-tasks-with-worktrees)
