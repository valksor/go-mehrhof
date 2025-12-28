# Specification File Format

Reference for implementation specification files.

## Overview

Specification files are markdown documents with optional YAML frontmatter. They contain detailed implementation plans generated during `mehr plan`.

## Location

```
.mehrhof/work/<task-id>/specifications/
├── specification-1.md
├── specification-2.md
└── specification-3.md
```

## File Structure

### Complete Example

````markdown
---
title: Health Endpoint Implementation
status: draft
priority: 1
created_at: 2025-01-15T10:30:00Z
updated_at: 2025-01-15T10:30:00Z
completed_at: null
dependencies: []
tags:
  - api
  - monitoring
---

# Health Endpoint Implementation

## Overview

Implement a health check endpoint for service monitoring.

## Goals

- Provide service health status
- Enable monitoring integration
- Support load balancer health checks

## Implementation Details

### 1. Create Handler

Create `internal/api/health.go`:

```go
package api

type HealthHandler struct {
    version string
}

func NewHealthHandler(version string) *HealthHandler {
    return &HealthHandler{version: version}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    response := map[string]interface{}{
        "status":    "ok",
        "timestamp": time.Now().UTC().Format(time.RFC3339),
        "version":   h.version,
    }
    json.NewEncoder(w).Encode(response)
}
```
````

````

### 2. Register Route

Update `cmd/server/main.go`:

```go
healthHandler := api.NewHealthHandler(version)
router.Handle("/health", healthHandler)
````

### 3. Add Tests

Create `internal/api/health_test.go` with:

- Test successful response
- Test response format
- Test status code

## Files to Modify

| File                          | Action | Description        |
| ----------------------------- | ------ | ------------------ |
| `internal/api/health.go`      | Create | Health handler     |
| `internal/api/health_test.go` | Create | Handler tests      |
| `cmd/server/main.go`          | Modify | Route registration |

## Acceptance Criteria

- [ ] GET /health returns 200
- [ ] Response is valid JSON
- [ ] Contains status, timestamp, version
- [ ] Response time < 10ms

## Notes

- Use existing router pattern
- No authentication required
- Follow project code style

````

## Frontmatter Fields

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `title` | string | Spec title |
| `status` | string | Current status |

### Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `priority` | int | 1 | Implementation priority |
| `created_at` | datetime | - | Creation timestamp |
| `updated_at` | datetime | - | Last modification |
| `completed_at` | datetime | null | Completion timestamp |
| `dependencies` | array | [] | IDs of dependent specs |
| `tags` | array | [] | Categorization tags |

### Status Values

| Status | Meaning |
|--------|---------|
| `draft` | Initial state, may change |
| `ready` | Approved for implementation |
| `implementing` | Currently being implemented |
| `done` | Implementation complete |
| `blocked` | Waiting on dependencies |

## Body Sections

### Standard Sections

| Section | Purpose |
|---------|---------|
| Overview | Brief description |
| Goals | What this achieves |
| Implementation Details | Step-by-step instructions |
| Files to Modify | Affected files list |
| Acceptance Criteria | Definition of done |
| Notes | Additional context |

### Implementation Details

Code blocks should specify language:

```markdown
```go
package main

func main() {
    // Implementation
}
````

````

### Files to Modify

Table format recommended:

```markdown
| File | Action | Description |
|------|--------|-------------|
| `path/to/file.go` | Create | New file |
| `path/to/other.go` | Modify | Update function |
| `path/to/old.go` | Delete | No longer needed |
````

### Acceptance Criteria

Use checklist format:

```markdown
## Acceptance Criteria

- [ ] Feature works as described
- [ ] Tests pass
- [ ] Documentation updated
```

## Multiple Specifications

Complex tasks may have multiple specification files:

```
specifications/
├── specification-1.md    # Core functionality
├── specification-2.md    # API endpoints
├── specification-3.md    # Database schema
└── specification-4.md    # Tests
```

### Dependencies

Specifications can depend on others:

```yaml
---
title: API Endpoints
dependencies:
  - specification-1 # Requires core functionality
---
```

### Priority

Higher priority specs are implemented first:

```yaml
---
title: Core Functionality
priority: 1
---
```

```yaml
---
title: Nice-to-have Feature
priority: 3
---
```

## Agent Output Format

During `mehr implement`, agents reference specifications:

```
Reading specification-1.md...
Implementing: Health Endpoint
Creating: internal/api/health.go
...
```

## Manual Editing

You can edit specification files manually:

```bash
vim .mehrhof/work/*/specifications/specification-1.md
```

**Cautions:**

- Running `mehr plan` again may overwrite changes
- Keep edits compatible with expected format
- Update `updated_at` timestamp

## Validation

Specification files are validated on read:

- Valid YAML frontmatter (if present)
- Required fields present
- Valid status value

Invalid specs produce warnings but don't block operations.

## Examples

### Minimal Spec

```markdown
---
title: Add Button
status: ready
---

# Add Button

Add a submit button to the form.

## Implementation

Update `templates/form.html`:

- Add button element
- Style with existing classes
```

### Detailed Spec

See the complete example at the top of this document.

### API Spec

````markdown
---
title: User API Endpoints
status: ready
tags:
  - api
  - rest
---

# User API Endpoints

## Endpoints

### GET /users

List all users.

**Response:**

```json
{
  "users": [
    { "id": 1, "name": "Alice" },
    { "id": 2, "name": "Bob" }
  ]
}
```
````

### POST /users

Create a new user.

**Request:**

```json
{
  "name": "Charlie",
  "email": "charlie@example.com"
}
```

## Implementation

1. Create `internal/api/users.go`
2. Define handler methods
3. Register routes

```

## See Also

- [Storage Structure](reference/storage.md) - Where specs are stored
- [plan command](../cli/plan.md) - Creating specs
- [implement command](../cli/implement.md) - Using specs
```
