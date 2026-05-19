# kvelmo diff

Show what the agent changed.

## Usage

```bash
kvelmo diff
```

## Description

Shows the diff against the last checkpoint, highlighting what the AI agent changed. Falls back to regular git diff if no checkpoints exist.

## Options

| Flag     | Description            |
| -------- | ---------------------- |
| `--stat` | Show only file summary |

## Examples

```bash
# Show full diff
kvelmo diff

# Show file summary only
kvelmo diff --stat
```

## Output

```diff
diff --git a/internal/auth/middleware.go b/internal/auth/middleware.go
index abc123..def456 100644
--- a/internal/auth/middleware.go
+++ b/internal/auth/middleware.go
@@ -15,6 +15,12 @@ func AuthMiddleware(next http.Handler) http.Handler {
+    // Validate JWT token
+    token := r.Header.Get("Authorization")
```

## Related

- [checkpoints](/cli/checkpoints.md) — List git checkpoints
- [undo](/cli/undo.md) — Revert to previous checkpoint
