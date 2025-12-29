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

### Use Chat Mode

```bash
mehr chat "Use stateless JWT tokens, not sessions. Store nothing server-side."
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
mehr chat "Token expiry should be 24 hours. Include refresh token with 7 day expiry."
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

### Interactive Discussion

```bash
mehr chat
```

```
You: The password hashing looks weak. What are you using?

Agent: I'm using bcrypt with cost factor 10. The implementation is in
internal/auth/password.go. Would you like me to increase the cost factor
or use a different algorithm?

You: Use Argon2id instead with recommended parameters.

Agent: Understood. I'll update the password hashing to use Argon2id with:
- Memory: 64MB
- Iterations: 3
- Parallelism: 4
This provides better resistance against GPU attacks.

You: exit
```

### Implement Changes

```bash
mehr undo
mehr implement
```

## Effective Chat Patterns

### Be Specific

```bash
# Vague (less effective)
mehr chat "Make it more secure"

# Specific (more effective)
mehr chat "Add rate limiting: max 5 login attempts per minute per IP"
```

### Explain Why

```bash
# Just what
mehr chat "Use Redis for token storage"

# What and why (better)
mehr chat "Use Redis for token storage because we need to invalidate tokens across multiple server instances"
```

### Reference Existing Code

```bash
mehr chat "Use the same error response format as internal/api/errors.go"
```

### Ask Questions

```bash
mehr chat "What's the best way to handle password reset tokens?"
```

## Workflow Summary

```
mehr start task.md
mehr plan
├── Review specs
├── mehr chat "refinement..."
└── mehr plan (if major changes)

mehr implement
├── Review code
├── mehr undo
├── mehr chat "adjustment..."
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

Add details through chat:

```bash
mehr chat "Also need input validation"
mehr chat "Support both JSON and form-encoded requests"
mehr chat "Log all auth attempts for audit"
```

### 3. Review Before Implementing

Always check specs before `mehr implement`:

```bash
mehr plan
cat .mehrhof/work/*/specifications/*.md
# Looks wrong?
mehr chat "Actually, change X to Y"
mehr plan  # Regenerate
```

### 4. Use Undo Freely

Undo is cheap. Don't hesitate:

```bash
mehr implement
# Not right
mehr undo
mehr chat "..."
mehr implement
# Still not right
mehr undo
# ...
```

### 5. Keep Notes Focused

Notes accumulate. Keep them relevant:

```bash
# Good: Specific, actionable
mehr chat "Validate email format using regexp"

# Avoid: Vague, contradictory over time
mehr chat "Maybe use sessions"
mehr chat "Actually use JWT"
mehr chat "Or maybe sessions are fine"
```

## When to Start Fresh

Sometimes it's better to start over:

```bash
mehr delete --yes
# Rewrite task.md with better requirements
mehr start task.md
```

Consider this when:

- Requirements changed fundamentally
- Notes have become contradictory
- Too many iterations without progress

## Next Steps

- [Recovering from Mistakes](tutorials/undo-mistakes.md) - Deep dive on undo/redo
- [Your First Task](tutorials/first-task.md) - Basic workflow
- [talk command](../cli/chat.md) - Talk reference
