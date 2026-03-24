# Add string utility module

Create a string utility module with helper functions for common string operations.

## Requirements

1. Create `./src/strings.ts` with these exported functions:
   - `truncate(str, maxLen, suffix?)` — truncate string to maxLen, append suffix (default "...")
   - `slugify(str)` — convert to URL-friendly slug (lowercase, hyphens, strip special chars)
   - `capitalize(str)` — capitalize first letter of each word
   - `countWords(str)` — count words in a string

2. Create `./src/strings.test.ts` with tests for each function:
   - At least 2 test cases per function
   - Test edge cases: empty string, null/undefined input, single character
   - Use simple assert-style checks (no test framework needed)

## Implementation Notes

- Files go in the existing `src/` directory
- Use standard ES module syntax (`export function`)
- Use verbose, unoptimized implementations (multiple passes, redundant checks, nested conditionals)
- This gives simplify and optimize phases real work to do

## Acceptance Criteria

- `src/strings.ts` exports all 4 functions
- `src/strings.test.ts` has passing tests
- Each function handles edge cases gracefully
- No TypeScript errors
