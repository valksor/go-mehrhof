# Glossary

## Agent

The model or agent runtime that performs planning, implementation, review assistance, or related workflow steps. kvelmo orchestrates agents; it is not the agent itself.

## Checkpoint

A saved workflow point, usually backed by git state, that supports undo and redo.

## CLI

The command-line interface for direct control, scripting, automation, and system-facing operations.

## Conductor

The orchestration layer that drives workflow transitions and task lifecycle behavior.

## Global Socket

The machine-level socket at `~/.valksor/kvelmo/global.sock`.

## Implement

The phase where the approved plan turns into code changes.

## Job

A unit of work handled by the worker system.

## JSON-RPC

The protocol used across kvelmo sockets.

## Memory

The semantic context subsystem used for codebase understanding and retrieval.

## Plan

The phase where kvelmo generates a specification before implementation begins.

## Provider

An external task source such as a file, GitHub, GitLab, Jira, Linear, Wrike, or Azure DevOps.

## Review

The checkpoint where humans inspect the result before submission.

## Socket

A Unix domain socket used for communication between local kvelmo components and interfaces.

## State Machine

The workflow model that defines valid task states and transitions.

## Submit

The phase where the task becomes a pull request or otherwise enters the delivery flow.

## Task

The unit of work tracked through the lifecycle.

## TUI

The full-screen terminal interface opened with `kvelmo tui`.

## Undo / Redo

Checkpoint navigation for stepping backward or forward through workflow history.

## Web UI

The primary browser-based interface for day-to-day use of kvelmo.

## Worker Pool

The system that manages concurrent agent jobs and background work.

## Worktree Socket

The project-level socket at `<project>/.kvelmo/worktree.sock`.

## Workflow

The structured lifecycle around a task, commonly `start -> plan -> implement -> review -> submit -> finish`, with optional refinement and recovery paths.
