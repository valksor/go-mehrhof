# Review Phase

The review phase is your final check before submission.

## Starting Review

When your task is in the `implemented` state:

1. Click **Review** in the actions panel
2. Review the changes in the Changes panel
3. Decide to approve or reject

## What to Review

### Code Changes

1. Open the **Changes** panel
2. Review each modified file
3. Check:
   - Does it match the specification?
   - Is the code correct?
   - Are there any bugs?
   - Is the style consistent?

### Test Results

If tests were run:

- Check the Output panel for results
- Ensure all tests pass

### Security

kvelmo can run security scans:

- Look for warnings in the Output panel
- Address any security issues before submitting

## Approving Changes

If you're satisfied:

1. Click **Submit** to create a PR
2. The task transitions to `submitted`

## Rejecting Changes

If something needs to change:

1. Click **Undo** to revert to `implemented` state
2. Make adjustments:
   - Edit the specification
   - Add context
   - Re-implement
3. Review again

## Making Manual Adjustments

You can edit files manually before submitting:

1. Open your editor
2. Make changes
3. The Changes panel updates automatically
4. Continue with review/submit

## The Changes Panel

The Changes panel shows:

| Column | Description            |
| ------ | ---------------------- |
| File   | Modified file path     |
| Status | Added/Modified/Deleted |
| Lines  | Lines changed          |

Click a file to see the diff.

## Diff View

The diff view shows:

- Red lines = removed
- Green lines = added
- Context lines = unchanged

## State Transition

| Before        | After (approve) | After (reject)           |
| ------------- | --------------- | ------------------------ |
| `implemented` | `submitted`     | `implemented` (via undo) |

Prefer the command line? See [kvelmo review](/cli/review.md).
