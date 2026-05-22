# Sub-tasks

A **sub-task** lets a single phase fan out into an isolated, self-contained mini
lifecycle that runs in its own git worktree. It is how kvelmo parallelizes or
decomposes work within a phase — a node in a phase's execution graph can, instead
of running one agent prompt, spawn a sub-task that runs its own sequence of
phases (plan, implement, …) against an isolated copy of the repository.

## When to use it

Reach for a sub-task when a phase needs to do a chunk of work that deserves its
own branch and its own plan→implement cycle, separate from the parent task — for
example, building one component of a larger change in isolation so it can be
reviewed or discarded independently.

If you just want the normal single-agent phase, you don't need sub-tasks — they
are an opt-in extension, not part of the default `start → plan → implement →
review → submit` flow.

## How to trigger one

Sub-tasks are defined declaratively in a **phase graph definition**. Drop a YAML
file at `<project>/.kvelmo/graphs/<phase>.yaml` (currently the `plan` phase is
graph-driven) containing a node with a `sub_task` block:

```yaml
nodes:
  - id: build-cache-layer
    label: Cache layer
    sub_task:
      title: Cache layer
      description: Add an LRU cache in front of the store
      phases: [plan, implement]
      branch: feature/cache-layer # optional; auto-generated when omitted
      metadata: # optional; passed to the sub-task
        owner: platform
```

A node may carry **either** a `prompt` **or** a `sub_task`, never both.

## What happens

When the scheduler reaches a sub-task node, kvelmo:

1. Creates an isolated git worktree from the current commit.
2. Runs the configured `phases` in order inside that worktree (each is an agent
   job — `plan`, `implement`, `simplify`, `optimize`, `review`).
3. Commits the resulting changes to the sub-task's branch so they are preserved.
4. Removes the worktree directory (the **branch is kept** — that's where the
   work lives).
5. Returns the combined phase output as the node's result, which downstream
   nodes can consume.

The orchestrator emits `sub_task_started`, `sub_task_output`, and
`sub_task_completed` (or `sub_task_failed`) events throughout.

## Branches and failures

- **Branch name** — taken from `branch` if set, otherwise auto-generated as
  `kvelmo-subtask/<task-id>/<title>-<id>`. Auto-generated names include a unique
  suffix, so re-running a task never collides with a previous run's branch.
- **On failure** — the sub-task returns an error and the branch is still retained
  for inspection, with its commit message prefixed `Sub-task (failed):` so a
  failed run is obvious.
- **No nesting** — a sub-task cannot itself contain sub-tasks; that guard
  prevents unbounded recursion.

## Limitations

- Only phases from the standard lifecycle are valid (`plan`, `implement`,
  `simplify`, `optimize`, `review`); an unknown phase is rejected up-front.
- The graph-definition format is an advanced, file-driven surface; there is no
  dedicated CLI command to author sub-tasks.
