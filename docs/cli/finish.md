# mehr finish

Complete the task by creating a pull request or merging locally.

## Synopsis

```bash
mehr finish [flags]
```

## Description

The `finish` command completes the current task by:

1. Running quality checks (if available)
2. Creating a pull request (if provider supports it), OR merging locally
3. Keeping the task branch by default (use `--delete` to remove)
4. Cleaning up the work directory

**Default behavior**:
- For `github:` tasks → Creates PR automatically
- For `file:`/`directory:` tasks → Prompts to choose (merge / mark done / cancel)
- Use `--merge` flag to force local merge

## Flags

| Flag               | Short | Type   | Default | Description                                |
| ------------------ | ----- | ------ | ------- | ------------------------------------------ |
| `--merge`          |       | bool   | false   | Force local merge instead of creating PR    |
| `--delete`         |       | bool   | false   | Delete task branch after merge              |
| `--push`           |       | bool   | false   | Push to remote after local merge            |
| `--no-squash`      |       | bool   | false   | Regular merge (no squash)                   |
| `--target`         | `-t`  | string | auto    | Target branch to merge into                 |
| `--skip-quality`   |       | bool   | false   | Skip quality checks                         |
| `--quality-target` |       | string | quality | Make target for quality checks            |
| `--draft`          |       | bool   | false   | Create PR as draft                          |
| `--pr-title`       |       | string | auto    | Custom PR title                             |
| `--pr-body`        |       | string | auto    | Custom PR body                              |

## Examples

### Basic Finish (GitHub task)

```bash
mehr finish
```

For `github:` tasks, automatically creates a PR:

```
Finishing task a1b2c3d4...
Running quality checks...
  make quality: PASSED
Pushing branch to origin...
Creating pull request...
  PR #42: [#5] Add authentication feature
  https://github.com/owner/repo/pull/42
Task completed!
```

### Basic Finish (file task)

```bash
mehr finish
```

For `file:`/`directory:` tasks, prompts for action:

```
The provider for this task does not support pull requests.
What would you like to do?
  1. Merge changes to target branch locally
  2. Mark task as done (no merge)
  3. Cancel

Enter choice (1-3):
```

### Force Local Merge

```bash
mehr finish --merge
```

Perform local merge instead of creating PR (works for any provider).

### Merge and Delete Branch

```bash
mehr finish --merge --delete
```

Merge locally and clean up the task branch.

### Merge and Push

```bash
mehr finish --merge --push --delete
```

Full local workflow: merge, push to remote, delete branch.

### Create Draft PR

```bash
mehr finish --draft
```

Creates a draft PR for early feedback (GitHub tasks only).

### Custom PR Title and Body

```bash
mehr finish --pr-title "Fix: Authentication bug" --pr-body "Resolves login flow issue"
```

Override the auto-generated PR title and body.

### Regular Merge (No Squash)

```bash
mehr finish --merge --no-squash
```

Preserve individual commits instead of squashing.

### Different Target Branch

```bash
mehr finish --merge --target develop
```

Merge to `develop` instead of auto-detected base branch.

### Skip Quality Checks

```bash
mehr finish --skip-quality
```

Skip running `make quality`.

### Custom Quality Target

```bash
mehr finish --quality-target lint
```

Run `make lint` instead of `make quality`.

## Quality Checks

If your project has a Makefile with a `quality` target, it runs automatically:

```makefile
quality:
	go fmt ./...
	golangci-lint run
	go test ./...
```

If quality checks modify files (e.g., auto-formatting), you'll be prompted:

```
Quality checks modified files:
  - src/api/handler.go (formatted)

Continue with modified files? [y/N]
```

## What Happens

### With PR Creation (default for github:, gitlab:, etc.)

1. **Quality Checks** (unless skipped)
2. **Push branch** to remote
3. **Create PR** via provider API
4. **Task marked done** (branch preserved)

### With Local Merge (`--merge` flag)

1. **Quality Checks** (unless skipped)
2. **Switch** to target branch
3. **Merge** (squash by default)
4. **Push** (if `--push` flag used)
5. **Cleanup** (if `--delete` flag used)
6. **Task marked done**

## Pull Request Contents

The PR is automatically populated with:

- **Title**: `[#123] Task title` or `Task title`
- **Body**:
  - Task description
  - Specifications summary
  - Changed files (diff stat)
  - Test plan checklist

Example:

```markdown
## Summary

Implementation for: Add authentication feature

Closes #123

## Implementation Details

### Specification 1
[First 500 chars of spec...]

## Changes

```
 src/auth/jwt.go       | 45 +++++++++++++++++++
 src/auth/middleware.go | 89 +++++++++++++++++++++++++++++++++++
 src/api/routes.go      | 12 +++++
```

## Test Plan

- [ ] Manual testing
- [ ] Unit tests pass
- [ ] Code review

---
*Generated by [Mehrhof](https://github.com/valksor/go-mehrhof)*
```

## Merge Commit

When using local merge with squash, creates a single commit:

```
[#123] Complete: Add authentication feature

Implemented JWT-based authentication with:
- Login endpoint
- Logout endpoint
- Token refresh

Task: a1b2c3d4
```

## Error Handling

### Merge Conflicts

```
Error: Merge conflict detected
Files with conflicts:
  - src/api/routes.go

Resolve conflicts and run:
  git add .
  mehr finish
```

### Quality Check Failures

```
Error: Quality checks failed
  golangci-lint: 2 issues found

Fix issues and retry:
  mehr finish
```

### Dirty Working Directory

```
Error: Working directory has uncommitted changes
Commit or stash changes before finishing.
```

## Aborting Finish

If finish is interrupted, recover with:

```bash
# Check current state
git status

# If on target branch with conflicts
git merge --abort

# Return to task branch
git checkout task/a1b2c3d4
```

## See Also

- [delete](./delete.md) - Abandon without merging
- [review](./review.md) - Review before finishing
- [Workflow](../concepts/workflow.md) - Task lifecycle
