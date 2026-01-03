# Workflow Plugin Development

> **Status: Implemented**
>
> Workflow plugins are now integrated into the state machine via the `MachineBuilder` pattern. Custom phases, guards, and effects are registered when plugins load, extending the base workflow dynamically.

Workflow plugins extend Mehrhof's state machine with custom phases, guards, and effects. Use them for approval workflows, notifications, CI/CD integration, or custom validation steps.

## Key Features

- **Dynamic Phase Insertion**: Add custom phases using `after` or `before` to specify insertion points
- **Plugin Guards**: Control transitions with custom guard conditions evaluated via JSON-RPC
- **Critical Effects**: Mark effects as `critical: true` to block workflow on failure; non-critical effects log errors but continue

## Manifest Structure

```yaml
version: "1"
name: "approval"
type: "workflow"
description: "Manager approval workflow"
protocol: "1"

executable:
  command: ["python3", "./approval-workflow.py"]

workflow:
  phases:
    - name: "approval"
      description: "Wait for manager approval"
      after: "reviewing" # Insert after this phase
  guards:
    - name: "requiresApproval"
      description: "Check if task needs approval"
  effects:
    - name: "notifySlack"
      description: "Send Slack notification"
      critical: false # Non-critical: log errors but continue
    - name: "createJiraComment"
      description: "Add comment to Jira"
      critical: true # Critical: workflow fails if this effect fails

env:
  SLACK_WEBHOOK:
    description: "Slack webhook URL"
    required: false
  JIRA_TOKEN:
    description: "Jira API token"
    required: false
```

## Required Methods

### workflow.init

Initialize and declare available extensions.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "workflow.init",
  "params": { "config": {} }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "phases": [
      {
        "name": "approval",
        "description": "Manager approval required",
        "after": "reviewing"
      }
    ],
    "guards": [
      {
        "name": "requiresApproval",
        "description": "Check if approval is needed based on task labels"
      }
    ],
    "effects": [
      {
        "name": "notifySlack",
        "description": "Post message to Slack channel"
      }
    ]
  }
}
```

### workflow.evaluateGuard

Evaluate a guard condition.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "workflow.evaluateGuard",
  "params": {
    "guard": "requiresApproval",
    "context": {
      "task_id": "abc123",
      "labels": ["critical", "security"],
      "phase": "reviewing"
    }
  }
}
```

**Response (guard passes):**

```json
{ "jsonrpc": "2.0", "id": 2, "result": { "allowed": true } }
```

**Response (guard blocks):**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "allowed": false,
    "reason": "Security tasks require manager approval"
  }
}
```

### workflow.executeEffect

Execute a side effect.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "workflow.executeEffect",
  "params": {
    "effect": "notifySlack",
    "context": {
      "task_id": "abc123",
      "title": "Fix login bug",
      "phase": "implementing",
      "event": "phase_entered"
    }
  }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "success": true,
    "message": "Slack notification sent"
  }
}
```

**Error Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "success": false,
    "error": "Slack webhook returned 403"
  }
}
```

## Phase Configuration

Phases can be inserted at specific points in the workflow:

```yaml
phases:
  - name: "approval"
    after: "reviewing" # After reviewing, before done

  - name: "security-scan"
    before: "implementing" # Before implementing, after planning

  - name: "deploy-staging"
    after: "implementing" # After implementing, before reviewing
```

**Default Phase Order:**

```
idle → planning → implementing → reviewing → done
```

**With Custom Phases:**

```
idle → planning → security-scan → implementing → deploy-staging → reviewing → approval → done
```

## Guard Types

Guards control transitions between phases:

| Guard Type | Use Case                               |
| ---------- | -------------------------------------- |
| Blocking   | Prevent transition until condition met |
| Advisory   | Warn but allow override                |
| Validation | Check prerequisites                    |

```python
def evaluate_guard(guard_name, context):
    if guard_name == "hasTests":
        # Blocking guard - must have tests
        test_count = context.get("test_count", 0)
        if test_count == 0:
            return {"allowed": False, "reason": "No tests found"}
        return {"allowed": True}

    if guard_name == "hasDocumentation":
        # Advisory guard - warn but allow
        has_docs = context.get("has_docs", False)
        if not has_docs:
            return {
                "allowed": True,
                "warning": "No documentation found - consider adding some"
            }
        return {"allowed": True}
```

## Effect Types

Effects are side effects triggered by workflow events:

| Event                | When Triggered       |
| -------------------- | -------------------- |
| `phase_entered`      | Entering a new phase |
| `phase_exited`       | Leaving a phase      |
| `task_started`       | Task registered      |
| `task_completed`     | Task finished        |
| `checkpoint_created` | Git checkpoint made  |

## Complete Example: Approval Workflow

```python
#!/usr/bin/env python3
"""Approval workflow plugin for Mehrhof."""

import json
import os
import sys
import urllib.request

SLACK_WEBHOOK = os.environ.get("SLACK_WEBHOOK", "")
APPROVAL_LABELS = {"critical", "security", "production"}

def send_slack(message):
    """Send message to Slack."""
    if not SLACK_WEBHOOK:
        return False

    data = json.dumps({"text": message}).encode()
    req = urllib.request.Request(
        SLACK_WEBHOOK,
        data=data,
        headers={"Content-Type": "application/json"}
    )

    try:
        with urllib.request.urlopen(req) as resp:
            return resp.status == 200
    except Exception:
        return False

def handle_init(params):
    return {
        "phases": [
            {
                "name": "approval",
                "description": "Wait for manager approval",
                "after": "reviewing"
            }
        ],
        "guards": [
            {
                "name": "requiresApproval",
                "description": "Check if task needs approval based on labels"
            },
            {
                "name": "isApproved",
                "description": "Check if approval has been granted"
            }
        ],
        "effects": [
            {
                "name": "notifySlack",
                "description": "Send Slack notification"
            },
            {
                "name": "requestApproval",
                "description": "Request manager approval via Slack"
            }
        ]
    }

def handle_evaluate_guard(params):
    guard = params.get("guard", "")
    context = params.get("context", {})

    if guard == "requiresApproval":
        labels = set(context.get("labels", []))
        needs_approval = bool(labels & APPROVAL_LABELS)

        if needs_approval:
            matching = labels & APPROVAL_LABELS
            return {
                "allowed": False,
                "reason": f"Task has {', '.join(matching)} label(s) - requires approval"
            }
        return {"allowed": True}

    if guard == "isApproved":
        approved = context.get("metadata", {}).get("approved", False)
        if not approved:
            return {
                "allowed": False,
                "reason": "Awaiting manager approval"
            }
        return {"allowed": True}

    return {"allowed": True}

def handle_execute_effect(params):
    effect = params.get("effect", "")
    context = params.get("context", {})

    if effect == "notifySlack":
        task_id = context.get("task_id", "unknown")
        title = context.get("title", "Unknown task")
        phase = context.get("phase", "unknown")
        event = context.get("event", "")

        message = f":robot_face: *{title}* ({task_id})\n"
        if event == "phase_entered":
            message += f"Entered *{phase}* phase"
        elif event == "task_completed":
            message += ":white_check_mark: Task completed!"

        success = send_slack(message)
        return {
            "success": success,
            "message": "Notification sent" if success else "Failed to send"
        }

    if effect == "requestApproval":
        task_id = context.get("task_id", "unknown")
        title = context.get("title", "Unknown task")

        message = (
            f":warning: *Approval Required*\n"
            f"Task: *{title}* ({task_id})\n"
            f"Please review and approve."
        )

        success = send_slack(message)
        return {
            "success": success,
            "message": "Approval request sent" if success else "Failed to send"
        }

    return {"success": True}

def handle_request(request):
    method = request.get("method", "")
    params = request.get("params", {})

    handlers = {
        "workflow.init": handle_init,
        "workflow.evaluateGuard": handle_evaluate_guard,
        "workflow.executeEffect": handle_execute_effect,
    }

    if method in handlers:
        return handlers[method](params)

    return {"error": {"code": -32601, "message": f"Method not found: {method}"}}

def main():
    for line in sys.stdin:
        if not line.strip():
            continue

        try:
            request = json.loads(line)
            result = handle_request(request)

            response = {"jsonrpc": "2.0", "id": request.get("id")}
            if isinstance(result, dict) and "error" in result:
                response["error"] = result["error"]
            else:
                response["result"] = result

            print(json.dumps(response), flush=True)

        except json.JSONDecodeError:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": None,
                "error": {"code": -32700, "message": "Parse error"}
            }), flush=True)

if __name__ == "__main__":
    main()
```

## Testing Your Workflow

```bash
python3 ./my-workflow.py
{"jsonrpc":"2.0","id":1,"method":"workflow.init","params":{}}

{"jsonrpc":"2.0","id":2,"method":"workflow.evaluateGuard","params":{"guard":"requiresApproval","context":{"labels":["security"]}}}

{"jsonrpc":"2.0","id":3,"method":"workflow.executeEffect","params":{"effect":"notifySlack","context":{"task_id":"123","title":"Test","phase":"implementing"}}}

mehr plugins validate my-workflow
```

## Tips

1. **Fail gracefully** - Effects should not block the workflow if they fail
2. **Log failures** - Write to stderr for debugging
3. **Idempotent effects** - Effects may be called multiple times
4. **Guard reasons** - Provide clear, actionable reasons when blocking
5. **Async effects** - For slow operations, consider async execution
