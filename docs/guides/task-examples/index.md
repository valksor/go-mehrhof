# Task Examples

Templates showing what a well-formed kvelmo task looks like. Copy one into your project, edit it, and load it with `kvelmo start --from file:<path>`.

## Available templates

- [Feature template](feature-example.md) — example feature request ("Add User Authentication")
- [Bugfix template](bugfix-example.md) — example bug fix ("Pagination Returns Wrong Total Count")

## Task file format

Task files are Markdown documents with:

1. **Title** — H1 heading describing the task
2. **Description** — Overview of what needs to be done
3. **Requirements** — Bulleted list of requirements
4. **Acceptance Criteria** — Checkboxes for completion criteria
5. **Technical Notes** — Implementation details (optional)

## Loading a task

```bash
# Start kvelmo with a local task file
kvelmo start --from file:./my-task.md

# Or load from GitHub
kvelmo start --from github:owner/repo#123

# Or from a URL
kvelmo start --from https://github.com/owner/repo/issues/123
```

The templates linked above (`feature-example.md`, `bugfix-example.md`) can be saved into your repo and loaded the same way.

## Configuration

See `kvelmo config --help` for configuration options:

```bash
# Show current config
kvelmo config show

# Initialize default config
kvelmo config init

# Set a value
kvelmo config set max_workers 8
kvelmo config set default_model claude-3-opus
```

## Workflow recap

1. **Start** — Load a task and create a feature branch
2. **Plan** — AI generates implementation specification
3. **Implement** — AI writes code based on specification
4. **Review** — Human reviews the implementation
5. **Submit** — Create a pull request

```bash
kvelmo start --from file:task.md
kvelmo plan
kvelmo implement
kvelmo review
kvelmo submit
```
