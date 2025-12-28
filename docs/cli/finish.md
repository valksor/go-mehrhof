# mehr finish

Complete the task and merge changes to the target branch, or create a pull request.

## Synopsis

```bash
mehr finish [flags]
```

## Description

The `finish` command completes the current task by:

1. Running quality checks (if available)
2. Squash-merging changes to the target branch, OR creating a pull request
3. Deleting the task branch (unless creating PR)
4. Cleaning up the work directory

By default, it does **not** push to remote. Use `--pr` to create a pull request instead of merging locally.

## Flags

| Flag               | Short | Type   | Default | Description                            |
| ------------------ | ----- | ------ | ------- | -------------------------------------- |
| `--no-push`        |       | bool   | false   | Don't push after merge                 |
| `--no-delete`      |       | bool   | false   | Keep task branch after merge           |
| `--no-squash`      |       | bool   | false   | Regular merge (no squash)              |
| `--target`         | `-t`  | string | main    | Target branch to merge into            |
| `--skip-quality`   |       | bool   | false   | Skip quality checks                    |
| `--quality-target` |       | string | quality | Make target for quality checks         |
| `--pr`             |       | bool   | false   | Create pull request instead of merging |
| `--draft`          |       | bool   | false   | Create PR as draft (requires `--pr`)   |
| `--pr-title`       |       | string | auto    | Custom PR title (requires `--pr`)      |
| `--pr-body`        |       | string | auto    | Custom PR body (requires `--pr`)       |

## Examples

### Basic Finish

```bash
mehr finish
```

Output:

```
Finishing task a1b2c3d4...
Running quality checks...
  make quality: PASSED
Merging to main...
  Squash merge: SUCCESS
Cleaning up...
  Branch deleted: task/a1b2c3d4
  Work directory removed
Task completed!
```

### Keep Branch

```bash
mehr finish --no-delete
```

Merge but keep the task branch for reference.

### Regular Merge

```bash
mehr finish --no-squash
```

Preserve individual commits instead of squashing.

### Different Target Branch

```bash
mehr finish --target develop
```

Merge to `develop` instead of `main`.

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

### Create Pull Request

```bash
mehr finish --pr
```

Creates a PR instead of merging locally. Requires a provider that supports PR creation (e.g., GitHub):

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

### Create Draft PR

```bash
mehr finish --pr --draft
```

Creates a draft PR for early feedback.

### Custom PR Title and Body

```bash
mehr finish --pr --pr-title "Fix: Authentication bug" --pr-body "Resolves issue with login flow"
```

Override the auto-generated PR title and body.

## Quality Checks

If your project has a Makefile with a `quality` target, it runs automatically:

```makefile
quality:
	go fmt ./...
	golangci-lint run
```

If quality checks modify files (e.g., auto-formatting), you'll be prompted:

```
Quality checks modified files:
  - src/api/handler.go (formatted)

Continue with modified files? [y/N]
```

## What Happens

1. **Quality Checks** (unless skipped)
   - Runs `make quality`
   - Handles auto-modifications

2. **Merge**
   - Switches to target branch
   - Performs squash merge
   - Commit message includes task summary

3. **Cleanup**
   - Deletes task branch (unless `--no-delete`)
   - Removes `.mehrhof/work/<id>/`
   - Clears active task

## Merge Commit

The squash merge creates a single commit:

```
[task] Complete: Add authentication feature

Implemented JWT-based authentication with:
- Login endpoint
- Logout endpoint
- Token refresh

Task: a1b2c3d4
```

## After Finish

The task is complete. To push:

```bash
git push origin main
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

- [delete](cli/delete.md) - Abandon without merging
- [review](cli/review.md) - Review before finishing
- [Workflow](../concepts/workflow.md) - Task lifecycle
