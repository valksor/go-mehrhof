import type { State } from '../types/conductor'

// ---------------------------------------------------------------------------
// Worktree event payloads (conductor events forwarded via stream.subscribe)
// ---------------------------------------------------------------------------

/** Base fields present on every conductor event (injected by the server). */
interface BaseEvent {
  seq?: number
  timestamp?: string
}

/** State machine transition. */
export interface StateChangedEvent extends BaseEvent {
  type: 'state_changed'
  state: State
  message?: string
}

/** Heartbeat (can be ignored). */
export interface HeartbeatEvent extends BaseEvent {
  type: 'heartbeat'
}

/** Job streaming output (plan, implement, optimize, simplify). */
export interface JobOutputEvent extends BaseEvent {
  type: 'job_output'
  job_id: string
  node_id?: string
  content?: string
  message?: string
}

/** Job completed successfully. */
export interface JobCompletedEvent extends BaseEvent {
  type: 'job_completed'
  job_id?: string
  message?: string
}

/** Job failed. */
export interface JobFailedEvent extends BaseEvent {
  type: 'job_failed'
  job_id?: string
  error?: string
  content?: string
  message?: string
  failure_class?: FailureClass
  failure_message?: string
}

/** Phase failure classified (emitted before retry/skip decision). */
export interface PhaseFailureClassifiedEvent extends BaseEvent {
  type: 'phase_failure_classified'
  error?: string
  message?: string
  failure_class?: FailureClass
  failure_message?: string
}

/** Git checkpoint created. */
export interface CheckpointCreatedEvent extends BaseEvent {
  type: 'checkpoint_created'
  message: string
}

/** Task lifecycle events. */
export interface TaskLifecycleEvent extends BaseEvent {
  type:
    | 'task_started'
    | 'task_abandoned'
    | 'task_deleted'
    | 'task_reset'
    | 'task_stopped'
    | 'task_aborted'
    | 'task_submitted'
    | 'task_updated'
    | 'task_finished'
  state?: State
  message?: string
}

/** Phase started events. */
export interface PhaseStartedEvent extends BaseEvent {
  type:
    | 'planning_started'
    | 'implementing_started'
    | 'optimizing_started'
    | 'simplifying_started'
    | 'review_started'
    | 'review_fix_started'
  state?: State
  job_id?: string
  message?: string
}

/** Queue events. */
export interface QueueEvent extends BaseEvent {
  type: 'task_queued' | 'task_dequeued' | 'queue_advancing' | 'queue_advance_failed'
  message?: string
}

/** Quality gate user prompt. */
export interface UserPromptEvent extends BaseEvent {
  type: 'user_prompt'
  data?: { prompt_id?: string; question?: string }
}

/** Screenshot events. */
export interface ScreenshotCapturedEvent extends BaseEvent {
  type: 'screenshot_captured'
  data?: unknown
}

export interface ScreenshotDeletedEvent extends BaseEvent {
  type: 'screenshot_deleted'
  data?: { id?: string }
}

/** Warning event. */
export interface WarningEvent extends BaseEvent {
  type: 'warning'
  message?: string
}

/** Error event. */
export interface ErrorEvent extends BaseEvent {
  type: 'error'
  error?: string
  message?: string
}

/** Graph node events. */
export interface GraphNodeEvent extends BaseEvent {
  type:
    | 'node_queued'
    | 'node_started'
    | 'node_completed'
    | 'node_failed'
    | 'node_fail_routed'
    | 'node_skipped'
    | 'node_iteration'
    | 'node_retry'
    | 'node_approval_required'
  node_id?: string
  job_id?: string
  message?: string
  error?: string
}

/** Phase progress/control events. */
export interface PhaseControlEvent extends BaseEvent {
  type: 'phase_progress' | 'phase_retry' | 'phase_skipped' | 'iteration_retry' | 'auto_advance' | 'auto_advance_failed'
  state?: State
  message?: string
}

/** PR events. */
export interface PREvent extends BaseEvent {
  type: 'pr_approved' | 'pr_merged'
  state?: State
  message?: string
  data?: unknown
}

/** Approval events. */
export interface ApprovalEvent extends BaseEvent {
  type: 'approval_required' | 'transition_approved'
  state?: State
  message?: string
}

/** Hook error. */
export interface HookErrorEvent extends BaseEvent {
  type: 'hook_error'
  error?: string
  message?: string
}

/** Canary violation. */
export interface CanaryViolationEvent extends BaseEvent {
  type: 'canary_violation'
  message?: string
}

/** Worktree provisioned. */
export interface WorktreeProvisionedEvent extends BaseEvent {
  type: 'worktree_provisioned'
  message?: string
  data?: {
    files_copied?: string[]
    symlinks_created?: string[]
    commands_run?: string[]
  }
}

/** Spec changed. */
export interface SpecChangedEvent extends BaseEvent {
  type: 'spec_changed'
  message?: string
}

/** CI fix loop events. */
export interface CIFixEvent extends BaseEvent {
  type: 'ci_fix_watching' | 'ci_fix_success' | 'ci_fix_exhausted' | 'ci_fix_attempt_failed' | 'ci_fix_started'
  message?: string
  data?: { attempt?: number; max_attempts?: number }
}

/** Consensus review complete. */
export interface ConsensusReviewCompleteEvent extends BaseEvent {
  type: 'consensus_review_complete'
  message?: string
  data?: unknown
}

/** Spec alignment check events. */
export interface SpecAlignmentEvent extends BaseEvent {
  type: 'spec_alignment_started' | 'spec_alignment_complete'
  message?: string
  job_id?: string
}

/** Router decision. */
export interface RouterDecisionEvent extends BaseEvent {
  type: 'router_decision'
  message?: string
  data?: { action?: string; reason?: string; target_phase?: string; attempt?: number; max_retries?: number }
}

/** Union of all worktree (conductor) events. */
export type WorktreeEvent =
  | StateChangedEvent
  | HeartbeatEvent
  | JobOutputEvent
  | JobCompletedEvent
  | JobFailedEvent
  | CheckpointCreatedEvent
  | TaskLifecycleEvent
  | PhaseStartedEvent
  | QueueEvent
  | UserPromptEvent
  | ScreenshotCapturedEvent
  | ScreenshotDeletedEvent
  | WarningEvent
  | ErrorEvent
  | GraphNodeEvent
  | PhaseControlEvent
  | PhaseFailureClassifiedEvent
  | PREvent
  | ApprovalEvent
  | HookErrorEvent
  | CanaryViolationEvent
  | WorktreeProvisionedEvent
  | SpecChangedEvent
  | CIFixEvent
  | ConsensusReviewCompleteEvent
  | RouterDecisionEvent
  | SpecAlignmentEvent

/** All worktree event type strings. */
export type WorktreeEventType = WorktreeEvent['type']

// ---------------------------------------------------------------------------
// Chat event payloads (streamed from global socket via chat.send)
// ---------------------------------------------------------------------------

export type FailureClass = 'hard_stop' | 'recoverable' | 'degraded' | 'skippable'

export type SubagentStatus = 'started' | 'completed' | 'failed'
export type DangerLevel = 'safe' | 'caution' | 'dangerous'

export interface ChatStreamEvent {
  type: 'stream' | 'assistant'
  job_id: string
  content?: string
  timestamp?: string
}

export interface ChatJobStartedEvent {
  type: 'job_started'
  job_id: string
  content?: string
  timestamp?: string
}

export interface ChatJobCompletedEvent {
  type: 'job_completed'
  job_id: string
  result?: string
  content?: string
  timestamp?: string
}

export interface ChatJobFailedEvent {
  type: 'job_failed'
  job_id: string
  error?: string
  content?: string
  timestamp?: string
}

export interface ChatSubagentEvent {
  type: 'subagent'
  job_id: string
  subagent?: {
    id: string
    type: string
    description: string
    status: SubagentStatus
    duration?: number
    exit_reason?: string
  }
  timestamp?: string
}

export interface ChatPermissionEvent {
  type: 'permission'
  job_id: string
  permission_request?: {
    id: string
    tool: string
    danger_level?: DangerLevel
    danger_reason?: string
  }
  timestamp?: string
}

/** Union of all chat events. */
export type ChatEvent =
  | ChatStreamEvent
  | ChatJobStartedEvent
  | ChatJobCompletedEvent
  | ChatJobFailedEvent
  | ChatSubagentEvent
  | ChatPermissionEvent

export type ChatEventType = ChatEvent['type']

// ---------------------------------------------------------------------------
// Global socket notifications (JSON-RPC notifications, not events)
// ---------------------------------------------------------------------------

export interface StateChangedNotification {
  method: 'task_state_changed'
  params?: { project_path?: string; state?: string }
}

// ---------------------------------------------------------------------------
// TypedEventEmitter
// ---------------------------------------------------------------------------

/** Map from event type string to its payload. Built from the union type. */
type EventPayloadMap = {
  [E in WorktreeEvent as E['type']]: E
} & {
  [E in ChatEvent as E['type']]: E
}

/** Type-safe event handler. */
export type EventHandler<K extends keyof EventPayloadMap> = (data: EventPayloadMap[K]) => void

/**
 * TypedEventEmitter wraps raw socket subscriptions with type safety.
 *
 * Usage:
 * ```ts
 * const emitter = new TypedEventEmitter()
 * const unsub = emitter.on('state_changed', (e) => {
 *   // e is StateChangedEvent — fully typed
 *   console.log(e.state)
 * })
 * ```
 */
export class TypedEventEmitter {
  private handlers = new Map<string, Set<(data: unknown) => void>>()

  /** Subscribe to a typed event. Returns an unsubscribe function. */
  on<K extends keyof EventPayloadMap>(event: K, handler: EventHandler<K>): () => void {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set())
    }
    this.handlers.get(event)!.add(handler as (data: unknown) => void)

    return () => {
      this.handlers.get(event)?.delete(handler as (data: unknown) => void)
    }
  }

  /** Emit an event to all registered handlers. */
  emit<K extends keyof EventPayloadMap>(event: K, data: EventPayloadMap[K]): void {
    this.handlers.get(event)?.forEach((handler) => handler(data))
  }

  /** Remove a specific handler, or all handlers for an event. */
  off<K extends keyof EventPayloadMap>(event: K, handler?: EventHandler<K>): void {
    if (handler) {
      this.handlers.get(event)?.delete(handler as (data: unknown) => void)
    } else {
      this.handlers.delete(event)
    }
  }

  /** Remove all handlers. */
  clear(): void {
    this.handlers.clear()
  }

  /**
   * Dispatch a raw message object. Reads `msg.type` and emits to the
   * matching typed handler. Useful as a bridge from the raw SocketClient
   * subscribe callback.
   */
  dispatch(msg: unknown): void {
    const typed = msg as { type?: string }
    if (typed.type && this.handlers.has(typed.type)) {
      this.handlers.get(typed.type)!.forEach((handler) => handler(msg))
    }
  }
}

/** Singleton emitter for worktree events. */
export const worktreeEvents = new TypedEventEmitter()

/** Singleton emitter for chat events (global socket). */
export const chatEvents = new TypedEventEmitter()
