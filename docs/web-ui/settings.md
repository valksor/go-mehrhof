# Settings

Configure kvelmo through the Web UI settings panel.

## Opening Settings

Click **Settings** in the sidebar to open the configuration panel.

## Configuration Sections

### General

| Setting       | Description                             |
| ------------- | --------------------------------------- |
| Default Agent | AI agent to use (claude, codex, custom) |
| Web Port      | Port for the Web UI (default: 6337)     |

### Agents

Configure AI agent settings:

| Setting     | Description                  |
| ----------- | ---------------------------- |
| Agent       | Agent to use for each phase  |
| Retry Count | Number of retries on failure |
| Retry Delay | Delay between retries        |

### Workers

Worker pool configuration:

| Setting     | Description                |
| ----------- | -------------------------- |
| Max Workers | Maximum concurrent workers |
| Queue Size  | Maximum jobs in queue      |

### Git

Git integration settings:

| Setting        | Description                  |
| -------------- | ---------------------------- |
| Auto Commit    | Automatically commit changes |
| Branch Pattern | Pattern for branch names     |
| Target Branch  | Default merge target         |

### Providers

Task source configuration:

| Setting      | Description           |
| ------------ | --------------------- |
| GitHub Token | Personal access token |
| GitLab Token | Personal access token |
| Wrike Token  | API token             |

## Saving Settings

1. Make your changes
2. Click **Save**
3. Settings are persisted to `~/.valksor/kvelmo/kvelmo.yaml`

## Resetting to Defaults

Click **Reset** to restore default settings.

## Project-Specific Settings

Some settings can be overridden per-project in `.valksor/kvelmo.yaml`:

```json
{
  "agent": "claude",
  "git": {
    "branch_pattern": "feature/{slug}"
  }
}
```

Project settings override global settings.

## Environment Variables

Some settings can be set via environment variables:

| Variable            | Setting          |
| ------------------- | ---------------- |
| `KVELMO_SOCKET_DIR` | Socket directory |
| `KVELMO_WEB_ADDR`   | Web UI address   |
| `GITHUB_TOKEN`      | GitHub API token |
| `GITLAB_TOKEN`      | GitLab API token |

Environment variables override config file settings.

## Settings Priority

Settings are applied in this order (highest priority first):

1. Environment variables
2. Project config (`.valksor/kvelmo.yaml`)
3. Global config (`~/.valksor/kvelmo/kvelmo.yaml`)
4. Defaults

Prefer the command line? See [kvelmo config](/cli/config.md).
