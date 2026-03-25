# kvelmo changelog

Generate a changelog from commits between two git refs, grouped by category.

## Usage

```bash
kvelmo changelog <source> <target> [note] [flags]
```

Both `source` and `target` are required and can be any git ref: tags, branches, or commit SHAs.

An optional `note` is rendered as a blockquote at the top of the output.

## Flags

| Flag | Description |
|------|-------------|
| `--full` | Include commit body/description text under each entry |
| `--json` | Output raw JSON response (entries array + markdown) |

## Examples

```bash
# Changelog between two tags
kvelmo changelog v1.0.0 v1.1.0

# Between branches
kvelmo changelog main release/2.0

# Between commits with full descriptions
kvelmo changelog abc1234 def5678 --full

# With a note
kvelmo changelog dev main "only frontend changes"

# JSON output for scripting
kvelmo changelog v1.0.0 v1.1.0 --json
```

## Output Format

Commits are auto-categorized by their subject line and grouped:

```
### Added
- Add worktree provisioning (abc12345)

### Changed
- Update dependency versions (def67890)

### Fixed
- Fix socket reconnect race (9ab01234)

### Removed
- Remove deprecated auth middleware (567cdef0)
```

Categories are determined by commit message prefixes:
- **Added**: `add`, or default for commits not matching other patterns
- **Changed**: `change`, `update`, `refactor`
- **Fixed**: `fix`, or contains `bug`
- **Removed**: `remove`, `delete`

With `--full`, commit body text appears indented under each entry.

## Web UI

Available via the toolbar dropdown menu in ProjectView (More tools > Changelog). The panel provides source/target inputs, a "Full descriptions" checkbox, and a copy-to-clipboard button.

## RPC Method

```
changelog.generate
```

**Params:**
- `source` (string, required): Source git ref
- `target` (string, required): Target git ref
- `full` (boolean): Include commit body text
- `note` (string): Optional note rendered as blockquote at top of output

**Returns:**
- `entries` (array): Structured changelog entries with sha, message, author, date, category, body
- `markdown` (string): Pre-rendered markdown output
- `note` (string): The note if one was provided

## Related

- [export](/cli/export.md) - Export task history and metrics
- [report](/cli/report.md) - Generate compliance reports
