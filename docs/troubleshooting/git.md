# Git Issues

Problems with Git repositories and branches.

## "Not a git repository"

**Cause:** Running in non-git directory.

**Solution:**

```bash
# Initialize git
git init

# Or navigate to git repo
cd /path/to/repo
```

## "Working directory dirty"

**Cause:** Uncommitted changes exist.

**Solution:**

```bash
# Commit changes
git add .
git commit -m "WIP"

# Or stash
git stash

# Then run mehr command
mehr finish
```

## "Merge conflict"

**Cause:** Conflicts during `mehr finish`.

**Solution:**

```bash
# Check conflicting files
git status

# Resolve conflicts manually
vim conflicting-file.go

# Mark resolved
git add conflicting-file.go

# Complete merge
git commit
```

## "Cannot delete branch"

**Cause:** Branch is protected or checked out elsewhere.

**Solution:**

```bash
# Switch to another branch first
git checkout main

# Force delete if needed
git branch -D task/abc12345
```
