# Creating Tasks

Learn how to create and start tasks in the Web UI.

## Creating a New Task

1. Click **New Task** in the actions panel
2. Fill in the task details:
   - **Title** — A short description (e.g., "Add user authentication")
   - **Description** — Detailed requirements
3. Click **Start**

## Task Title

The title should be:

- Concise (3-10 words)
- Action-oriented (starts with a verb)
- Specific enough to understand the goal

**Good titles:**

- "Add login page with email authentication"
- "Fix null pointer in checkout flow"
- "Update API documentation for v2 endpoints"

**Avoid:**

- "Fix bug" (too vague)
- "Implement the new feature we discussed" (unclear)
- "Updates" (not specific)

## Task Description

The description provides context for the AI agent. Include:

- **What** should be done
- **Why** it's needed (context)
- **Where** in the codebase (if known)
- **Constraints** or requirements
- **Examples** of expected behavior

### Example Description

```
Add a GET /api/users endpoint that returns a list of users.

Requirements:
- Return JSON array of user objects
- Each user has: id, name, email, createdAt
- Support pagination with ?page=N&limit=M
- Default limit is 20, max is 100
- Require authentication via Bearer token

Example response:
{
  "users": [...],
  "total": 150,
  "page": 1,
  "limit": 20
}
```

## Loading from Providers

Instead of creating tasks manually, you can load from external sources:

### From a File

If you have a task file:

1. Click **Load from File**
2. Select the markdown file
3. Task details are populated automatically

### From GitHub

1. Click **Load from Provider**
2. Select **GitHub**
3. Enter the issue reference (e.g., `owner/repo#123`)
4. Click **Load**

### From Other Providers

kvelmo supports multiple task sources:

- **GitHub** — Issues and PRs
- **GitLab** — Issues and MRs
- **Wrike** — Tasks

## What Happens When You Start

When you click **Start**:

1. kvelmo validates the task
2. A new git branch is created
3. Task metadata is stored in `.kvelmo/`
4. State transitions to `loaded`
5. The Plan button becomes available

## Branch Naming

kvelmo creates branches automatically. The default pattern is:

```
feature/<task-id>--<slug>
```

Example: `feature/123--add-user-auth`

## Next Steps

After creating a task:

1. Review the task in the dashboard
2. Click **Plan** to generate a specification
3. Continue through the [workflow](/concepts/workflow.md)

Prefer the command line? See [kvelmo start](/cli/start.md).
