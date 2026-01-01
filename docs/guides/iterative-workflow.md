# Tutorial: Iterative Development

Learn how to refine AI-generated code through conversation and iteration.

## The Iterative Approach

Instead of expecting perfect code on the first try:

1. Start with basic requirements
2. Review the output
3. Add clarifications
4. Regenerate
5. Repeat until satisfied

## Scenario

You're building a user authentication system. Let's iterate to get it right.

## Round 1: Basic Requirements

### Create Initial Task

```bash
cat > auth.md << 'EOF'
# User Authentication

Implement user authentication for the API.

## Requirements
- Login with email/password
- JWT tokens
- Logout functionality
EOF

mehr start auth.md
mehr plan
```

### Review Specs

```bash
cat .mehrhof/work/*/specifications/specification-1.md
```

The AI might have made assumptions you don't agree with.

### First Implementation

```bash
mehr implement
git diff
```

**Issue:** The AI used session-based auth, but you wanted stateless JWT.

## Round 2: Add Clarification

### Add Notes

```bash
mehr note "Use stateless JWT tokens, not sessions. Store nothing server-side."
```

### Undo and Retry

```bash
mehr undo
mehr plan    # Regenerate specs with new context
```

Check the updated spec:

```bash
cat .mehrhof/work/*/specifications/specification-*.md
```

Now it should mention stateless JWT.

### Implement Again

```bash
mehr implement
git diff
```

**Better!** But the token expiry is too short.

## Round 3: Fine-tune Details

### Add More Context

```bash
mehr note "Token expiry should be 24 hours. Include refresh token with 7 day expiry."
```

### Iterate

```bash
mehr undo
mehr implement
```

Check the token configuration:

```bash
grep -r "expir" --include="*.go"
```

## Round 4: Request Specific Changes

### Add Detailed Notes

```bash
mehr note "Use Argon2id for password hashing with recommended parameters: Memory 64MB, Iterations 3, Parallelism 4. This provides better resistance against GPU attacks than bcrypt."
```

### Implement Changes

```bash
mehr undo
mehr implement
```

## Effective Note Patterns

### Be Specific

```bash
# Vague (less effective)
mehr note "Make it more secure"

# Specific (more effective)
mehr note "Add rate limiting: max 5 login attempts per minute per IP"
```

### Explain Why

```bash
# Just what
mehr note "Use Redis for token storage"

# What and why (better)
mehr note "Use Redis for token storage because we need to invalidate tokens across multiple server instances"
```

### Reference Existing Code

```bash
mehr note "Use the same error response format as internal/api/errors.go"
```

### Ask Questions

```bash
mehr note "What's the best way to handle password reset tokens?"
```

## Workflow Summary

```
mehr start task.md
mehr plan
├── Review specs
├── mehr note "refinement..."
└── mehr plan (if major changes)

mehr implement
├── Review code
├── mehr undo
├── mehr note "adjustment..."
└── mehr implement

[Repeat until satisfied]

mehr finish
```

## Tips for Effective Iteration

### 1. Start Simple

Don't overload the initial task file. Start with core requirements:

```markdown
# Feature Name

Basic description.

## Must Have

- Core requirement 1
- Core requirement 2
```

### 2. Build Up Context

Add details through notes:

```bash
mehr note "Also need input validation"
mehr note "Support both JSON and form-encoded requests"
mehr note "Log all auth attempts for audit"
```

### 3. Review Before Implementing

Always check specs before `mehr implement`:

```bash
mehr plan
cat .mehrhof/work/*/specifications/*.md
# Looks wrong?
mehr note "Actually, change X to Y"
mehr plan  # Regenerate
```

### 4. Use Undo Freely

Undo is cheap. Don't hesitate:

```bash
mehr implement
# Not right
mehr undo
mehr note "..."
mehr implement
# Still not right
mehr undo
# ...
```

### 5. Keep Notes Focused

Notes accumulate. Keep them relevant:

```bash
# Good: Specific, actionable
mehr note "Validate email format using regexp"

# Avoid: Vague, contradictory over time
mehr note "Maybe use sessions"
mehr note "Actually use JWT"
mehr note "Or maybe sessions are fine"
```

## When to Start Fresh

Sometimes it's better to start over:

```bash
mehr abandon --yes
# Rewrite task.md with better requirements
mehr start task.md
```

Consider this when:

- Requirements changed fundamentally
- Notes have become contradictory
- Too many iterations without progress

## Next Steps

- [Recovery & Undo](recovery.md) - Deep dive on undo/redo
- [Your First Task](first-task.md) - Basic workflow
- [note command](../cli/note.md) - Add notes to the task
