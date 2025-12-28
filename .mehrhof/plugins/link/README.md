# Link Provider Plugin

Load task content from any URL - web pages, GitHub issues, Pastebin, and more.

## Usage

```bash
# Start a task from any URL
mehr start link:https://github.com/user/repo/issues/123
mehr start link:https://example.com/task-description.html
mehr start link:https://pastebin.com/abc123

# Also accepts url: scheme
mehr start url:https://docs.example.com/feature-spec
```

## Features

| Source Type   | Support | Notes                                                 |
| ------------- | ------- | ----------------------------------------------------- |
| GitHub Issues | Full    | Fetches via API, extracts title, body, labels, status |
| GitHub PRs    | Full    | Same as issues                                        |
| Web Pages     | Good    | Extracts title from `<title>`, content from `<body>`  |
| Pastebin      | Good    | Auto-converts to raw URL                              |
| Markdown      | Good    | Extracts first `#` heading as title                   |
| Plain Text    | Basic   | Uses content as-is                                    |
| JSON          | Basic   | Pretty-prints JSON content                            |

## Branch Naming

The plugin returns metadata used for intelligent branch naming:

| URL Type                 | externalKey | taskType | Example Branch            |
| ------------------------ | ----------- | -------- | ------------------------- |
| GitHub issue #42         | `42`        | `issue`  | `issue/42--fix-login-bug` |
| GitHub PR #123           | `123`       | `pr`     | `pr/123--add-dark-mode`   |
| Regular URL `/docs/spec` | `spec`      | `task`   | `task/spec--title-slug`   |

## Configuration

Enable in `.mehrhof/config.yaml`:

```yaml
plugins:
  enabled:
    - link
```

## Requirements

- Python 3.6+
- No external dependencies (uses only standard library)

## Capabilities

| Capability | Supported |
| ---------- | --------- |
| `read`     | Yes       |
| `snapshot` | Yes       |
| `list`     | No        |
| `comment`  | No        |

## How It Works

1. **Match**: Recognizes `link:` or `url:` prefixes
2. **Parse**: Validates URL format
3. **Fetch**:
   - GitHub URLs → API call for structured data
   - Pastebin → Convert to raw URL
   - Other → HTTP GET with content extraction
4. **Return**: WorkUnit with title, description, status, and naming metadata

## Example Output

For `link:https://github.com/golang/go/issues/42`:

```
Task registered: d67ec437
  Title: Numeric literals with spaces as delimiters
  Key: 42
  Source: link:https://github.com/golang/go/issues/42
  State: idle
  Branch: issue/42--numeric-literals-with-spaces-as-delimiters
```

## Troubleshooting

### "Invalid URL" error

Ensure the URL includes the scheme (`https://`):

```bash
# Wrong
mehr start link:github.com/user/repo

# Correct
mehr start link:https://github.com/user/repo/issues/1
```

### GitHub rate limiting

For unauthenticated requests, GitHub allows 60 requests/hour. The plugin falls back to HTML scraping if API fails.

### Plugin not found

Verify the plugin is enabled:

```bash
mehr plugins list
```
