# mehr review

Run automated code review on current changes.

## Synopsis

```bash
mehr review [flags]
```

## Description

The `review` command runs an automated code review on the task's changes. By default, it uses CodeRabbit to analyze:

- Code quality issues
- Potential bugs
- Style violations
- Security concerns

Review results are saved to the work directory.

## Flags

| Flag       | Short | Type   | Default      | Description        |
| ---------- | ----- | ------ | ------------ | ------------------ |
| `--tool`   |       | string | coderabbit   | Review tool to use |
| `--output` | `-o`  | string | REVIEW-N.txt | Output file name   |

## Examples

### Basic Review

```bash
mehr review
```

Output:

```
Running code review...
Tool: CodeRabbit
Analyzing 5 changed files...

Review Status: ISSUES
Found 3 issues:
  - src/api/handler.go:45 - Missing error check
  - src/api/auth.go:23 - Potential nil pointer
  - src/api/routes.go:12 - Unused import

Review saved to: .mehrhof/work/a1b2c3d4/reviews/REVIEW-1.txt
```

### Custom Output File

```bash
mehr review --output security-review.txt
```

### Specify Tool

```bash
mehr review --tool coderabbit
```

## Review Status

| Status   | Meaning                          |
| -------- | -------------------------------- |
| COMPLETE | No issues found                  |
| ISSUES   | Issues found that need attention |
| ERROR    | Review tool failed to run        |

## Review Output

Reviews are saved as text files:

```
Code Review Report
==================
Tool: CodeRabbit
Date: 2025-01-15 10:30:00

Files Analyzed:
- src/api/handler.go
- src/api/auth.go
- src/api/routes.go

Issues Found:
-------------

[HIGH] src/api/handler.go:45
Missing error check on database query.
Suggestion: Handle the error case.

[MEDIUM] src/api/auth.go:23
Potential nil pointer dereference.
Suggestion: Add nil check before access.

[LOW] src/api/routes.go:12
Unused import "fmt".
Suggestion: Remove unused import.
```

## When to Review

### After Implementation

```bash
mehr implement
mehr review
```

### Before Finishing

```bash
mehr review
# Fix any issues...
mehr finish
```

## Acting on Review

### Fix Issues

After review, address issues and re-implement:

```bash
mehr review
# Issues found...
mehr chat "Fix the error handling in handler.go"
mehr implement
mehr review  # Verify fixes
```

### Ignore Issues

If issues are false positives or acceptable:

```bash
mehr review
# Issues found but acceptable...
mehr finish  # Proceed anyway
```

## Review History

Multiple reviews are saved with incremental names:

```
.mehrhof/work/<id>/reviews/
├── REVIEW-1.txt    # First review
├── REVIEW-2.txt    # After fixes
└── REVIEW-3.txt    # Final review
```

## Requirements

### CodeRabbit

Ensure CodeRabbit CLI is installed and configured:

```bash
# Install CodeRabbit CLI
# (Follow CodeRabbit documentation)

# Verify installation
coderabbit --version
```

## Troubleshooting

### "Review tool not found"

Install the required review tool:

```bash
# For CodeRabbit
npm install -g @coderabbit/cli
```

### "No changes to review"

Ensure you have uncommitted changes or are on a task branch with modifications.

### "Review timeout"

Large changesets may take longer. The tool will retry automatically.

## See Also

- [implement](cli/implement.md) - Generate code
- [finish](cli/finish.md) - Complete task
- [Workflow](../concepts/workflow.md) - Review phase
