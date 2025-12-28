# mehr init

Initialize the task workspace.

## Synopsis

```bash
mehr init
```

## Description

The `init` command sets up the Mehrhof workspace in your project. It:

1. Creates the `.mehrhof/` directory for task storage
2. Creates a default `config.yaml` with sensible defaults
3. Updates `.gitignore` to exclude task-specific files

This is typically a one-time setup per project. Running `init` again is safe - it won't overwrite existing configuration.

## Flags

This command has no specific flags. Global flags (`--verbose`, `--no-color`) are available.

## Examples

### Initialize a New Project

```bash
cd my-project
mehr init
```

Output:

```
Created config file: .mehrhof/config.yaml
Workspace initialized in /path/to/my-project
```

### Re-running Init

If the workspace is already initialized:

```bash
mehr init
```

Output:

```
Config file already exists: .mehrhof/config.yaml
Workspace initialized in /path/to/my-project
```

## What Gets Created

### Directory Structure

```
.mehrhof/
├── config.yaml    # Workspace configuration
├── work/          # Task work directories (created as needed)
├── locks/         # File locks for concurrent access
└── planned/       # Standalone planning sessions
```

### Default Configuration

The generated `config.yaml` includes sensible defaults:

```yaml
git:
  auto_commit: true
  commit_prefix: "[{key}]"
  branch_pattern: "{type}/{key}--{slug}"
  sign_commits: false

agent:
  default: claude
  timeout: 300
  max_retries: 3

providers:
  default: "" # Set to "file" to allow bare references

workflow:
  auto_init: true
  session_retention_days: 30
```

### Gitignore Updates

The command ensures `.gitignore` excludes:

- `.mehrhof/work/` - Task work directories
- `.mehrhof/locks/` - Lock files
- `.mehrhof/.active_task` - Active task reference

But keeps:

- `.mehrhof/config.yaml` - Should be committed for team sharing

## Auto-Initialization

Most commands (like `mehr start`) will automatically initialize the workspace if needed, so explicit `init` is optional. However, running `init` first lets you customize the configuration before starting tasks.

## After Initializing

1. **Review configuration:**

   ```bash
   cat .mehrhof/config.yaml
   ```

2. **Customize settings** (optional):
   - Set default provider
   - Configure agent aliases
   - Adjust git settings

3. **Start your first task:**
   ```bash
   mehr start file:task.md
   ```

## See Also

- [start](cli/start.md) - Register a new task
- [Configuration Overview](../configuration/overview.md) - Configuration options
- [Configuration Files](../configuration/files.md) - File locations and formats
