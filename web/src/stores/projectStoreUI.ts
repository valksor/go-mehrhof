import type { StateCreator } from 'zustand'
import { SocketClient } from '../lib/socket'
import { debounce } from '../lib/debounce'
import { reconnectDelay } from '../lib/reconnect'
import { useScreenshotStore, type Screenshot } from './screenshotStore'
import { sendNotification, requestNotificationPermission } from '../lib/notify'
import { worktreeEvents } from '../lib/events'
import type { FailureClass } from '../lib/events'
import type { PhaseMetrics } from '../types/conductor'
import type {
  ProjectState,
  UISlice,
  TaskState,
} from './projectStore.types'

const ACTIVE_PHASES = new Set(['planning', 'implementing', 'simplifying', 'optimizing'])

export const createUISlice: StateCreator<ProjectState, [], [], UISlice> = (set, get) => ({
  connected: false,
  connecting: false,
  reconnectAttempt: 0,
  reconnectTimeoutId: null,
  connectionVersion: 0,
  worktreeId: null,
  client: null,
  unsubscribeSocket: null,
  output: [],
  lastSeq: 0,
  loading: false,
  error: null,
  dryRunMode: false,
  prBodyOverride: null,

  toggleDryRun: () => {
    set(s => ({ dryRunMode: !s.dryRunMode }))
  },

  setPrBodyOverride: (body: string | null) => {
    set({ prBodyOverride: body })
  },

  connect: async (worktreeId: string) => {
    if (import.meta.env.DEV) {
      console.log('[kvelmo] projectStore.connect called with:', worktreeId)
    }
    if (get().connected || get().connecting) {
      if (get().worktreeId === worktreeId) {
        if (import.meta.env.DEV) {
          console.log('[kvelmo] projectStore already connected/connecting to same worktree, skipping')
        }
        return
      }
      // Switching to a different worktree — disconnect first
      get().disconnect()
    }

    // Clean up previous subscription if any
    get().unsubscribeSocket?.()

    const thisVersion = get().connectionVersion + 1
    set({ connecting: true, worktreeId, error: null, connectionVersion: thisVersion, unsubscribeSocket: null })

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/ws/worktree/${encodeURIComponent(worktreeId)}`
    if (import.meta.env.DEV) {
      console.log('[kvelmo] Connecting to worktree:', url)
    }

    let progressInterval: ReturnType<typeof setInterval> | null = null

    try {
      const client = new SocketClient(url)

      // Auto-reconnect on disconnect (exponential backoff, matching globalStore pattern)
      client.setOnDisconnect(() => {
        if (get().connectionVersion !== thisVersion) return

        const attempt = get().reconnectAttempt + 1
        const delay = reconnectDelay(attempt)
        const delaySec = Math.round(delay / 1000)

        const timeoutId = setTimeout(() => {
          set({ reconnectTimeoutId: null })
          const wId = get().worktreeId
          if (wId) {
            void get().connect(wId)
          }
        }, delay)

        set({
          connected: false,
          connecting: false,
          reconnectAttempt: attempt,
          reconnectTimeoutId: timeoutId,
          client: null,
          error: `Connection lost. Reconnecting in ${delaySec}s... (attempt ${attempt})`
        })
      })

      // Create debounced refresh to prevent RPC flood on rapid event streams
      const debouncedRefresh = debounce(() => get().refreshStatus(), 300)
      const debouncedLoadQueue = debounce(() => get().loadQueue(), 500)

      // Progress estimation polling (3s interval, only during active phases)
      progressInterval = setInterval(async () => {
        if (!ACTIVE_PHASES.has(get().state)) return
        try {
          const progress = await client.call<{ active: boolean; percent?: number; eta_seconds?: number; calibrated?: boolean }>('progress.get', {})
          if (progress.active) {
            set({ phaseProgress: { percent: progress.percent ?? 0, eta: progress.eta_seconds ?? -1, calibrated: progress.calibrated ?? false } })
          }
        } catch {
          // Ignore polling errors
        }
      }, 3000)

      // Handle streaming events
      const unsubscribe = client.subscribe((data: unknown) => {
        // Dispatch to typed event emitter so typed handlers receive events
        worktreeEvents.dispatch(data)

        const msg = data as {
          seq?: number
          type?: string
          state?: TaskState
          message?: string
          content?: string
          job_id?: string
          error?: string
        }

        // Deduplicate: skip events already processed (can occur when replay and
        // live channel overlap briefly on reconnect). Must check before updating lastSeq.
        if (msg.seq !== undefined && msg.seq <= get().lastSeq) {
          return
        }

        // Track the highest seq seen for reconnect replay
        if (msg.seq !== undefined) {
          set(s => ({ lastSeq: Math.max(s.lastSeq, msg.seq!) }))
        }

        if (msg.type === 'heartbeat') {
          return
        } else if (msg.type === 'state_changed') {
          const newState = msg.state || 'none'
          const updates: Partial<ProjectState> = { state: newState }
          if (newState !== 'failed') {
            updates.phaseError = null
          }
          if (!ACTIVE_PHASES.has(newState)) {
            updates.phaseProgress = null
          }
          set(updates)
          get().appendOutput(`State: ${msg.state}`)
          debouncedRefresh()
          if (msg.state === 'planned') {
            void sendNotification('Planning Complete', 'Specification is ready for review')
          } else if (msg.state === 'implemented') {
            void sendNotification('Implementation Complete', 'Code is ready for review')
          }
        } else if (msg.type === 'task_abandoned' || msg.type === 'task_deleted' || msg.type === 'task_reset') {
          set({ state: msg.state || 'none' })
          get().appendOutput(msg.message || `Task ${msg.type.replace('task_', '')}`)
          debouncedRefresh()
        } else if (msg.type === 'job_output' || msg.type === 'stream') {
          if (msg.content || msg.message) {
            get().appendOutput(msg.content || msg.message || '')
          }
        } else if (msg.type === 'checkpoint_created') {
          get().appendOutput(`Checkpoint created: ${msg.message}`)
          debouncedRefresh()
        } else if (msg.type === 'job_completed') {
          get().appendOutput('Job completed')
          debouncedRefresh()
          void sendNotification('Task Completed', get().task?.title || 'Job finished successfully')
        } else if (msg.type === 'phase_failure_classified') {
          const classified = msg as { failure_class?: FailureClass; failure_message?: string; message?: string; phase?: string }
          if (classified.failure_class) {
            set({
              phaseError: {
                message: classified.failure_message || classified.message || 'Unknown error',
                class: classified.failure_class,
                phase: classified.phase || 'unknown',
              },
            })
          }
        } else if (msg.type === 'job_failed') {
          get().appendOutput(`Job failed: ${msg.error || msg.content}`)
          set({ error: msg.error || 'Job failed' })
          void sendNotification('Task Failed', msg.error || 'A job has failed')
        } else if (msg.type === 'screenshot_captured') {
          const screenshot = (msg as { data?: Screenshot }).data
          if (screenshot) {
            useScreenshotStore.getState().handleScreenshotCaptured(screenshot)
          }
        } else if (msg.type === 'screenshot_deleted') {
          const data = (msg as { data?: { id?: string } }).data
          if (data?.id) {
            useScreenshotStore.getState().handleScreenshotDeleted(data.id)
          }
        } else if (msg.type === 'user_prompt') {
          const data = (msg as { data?: { prompt_id?: string; question?: string } }).data
          if (data?.prompt_id) {
            set({ qualityPrompt: { id: data.prompt_id, question: data.question ?? 'Quality gate question' } })
          }
        } else if (msg.type === 'worktree_provisioned') {
          const provData = (msg as { data?: { files_copied?: string[]; symlinks_created?: string[]; commands_run?: string[] } }).data
          const parts: string[] = []
          if (provData?.files_copied?.length) parts.push(`${provData.files_copied.length} files copied`)
          if (provData?.symlinks_created?.length) parts.push(`${provData.symlinks_created.length} symlinks`)
          if (provData?.commands_run?.length) parts.push(`${provData.commands_run.length} commands`)
          get().appendOutput(`Worktree provisioned: ${parts.join(', ')}`)
        } else if (msg.type === 'warning') {
          get().appendOutput(`\u26a0 ${msg.message || 'Warning'}`)
        } else if (msg.type === 'error') {
          get().appendOutput(`Error: ${msg.error || msg.message || 'Unknown error'}`)
          set({ error: msg.error || msg.message || 'An error occurred' })
        } else if (msg.type === 'task_queued' || msg.type === 'task_dequeued' || msg.type === 'queue_advancing') {
          debouncedLoadQueue()
          if (msg.type === 'queue_advancing') {
            get().appendOutput(msg.message || 'Loading next queued task...')
          }
        } else if (msg.type === 'task_finished') {
          get().appendOutput(msg.message || 'Task finished')
          debouncedRefresh()
          debouncedLoadQueue()
        } else if (msg.type === 'spec_changed') {
          get().appendOutput(msg.message || 'Specification changed')
          debouncedRefresh()
        } else if (msg.type === 'ci_fix_watching') {
          const ciMsg = msg as { data?: { attempt?: number; max_attempts?: number } }
          set({ ciFixStatus: { active: true, attempt: ciMsg.data?.attempt, maxAttempts: ciMsg.data?.max_attempts } })
          get().appendOutput(msg.message || 'CI fix: watching pipeline...')
        } else if (msg.type === 'ci_fix_success') {
          set({ ciFixStatus: { active: false, result: 'success' } })
          get().appendOutput(msg.message || 'CI fix: pipeline passed')
          void sendNotification('CI Fix Success', 'Pipeline passed after fix')
        } else if (msg.type === 'ci_fix_exhausted') {
          set({ ciFixStatus: { active: false, result: 'failed' } })
          get().appendOutput(msg.message || 'CI fix: all attempts exhausted')
          void sendNotification('CI Fix Failed', 'Pipeline still failing after all fix attempts')
        } else if (msg.type === 'ci_fix_attempt_failed') {
          get().appendOutput(msg.message || 'CI fix: attempt failed, retrying...')
        } else if (msg.type === 'ci_fix_started') {
          get().appendOutput(msg.message || 'CI fix: starting fix job...')
        } else if (msg.type === 'autofix_attempt') {
          const afMsg = msg as { data?: { attempt?: number; max_attempts?: number } }
          set({ autoFixStatus: { active: true, attempt: afMsg.data?.attempt, maxAttempts: afMsg.data?.max_attempts } })
          get().appendOutput(msg.message || 'Auto-fix: attempting quality gate fix...')
        } else if (msg.type === 'autofix_success') {
          set({ autoFixStatus: { active: false, result: 'success' } })
          get().appendOutput(msg.message || 'Auto-fix: quality gate passed')
          void sendNotification('Auto-Fix Success', 'Quality gate passed after fix')
        } else if (msg.type === 'autofix_exhausted') {
          set({ autoFixStatus: { active: false, result: 'failed' } })
          get().appendOutput(msg.message || 'Auto-fix: all attempts exhausted')
          void sendNotification('Auto-Fix Failed', 'Quality gate still failing after all fix attempts')
        } else if (msg.type === 'autofix_job_failed') {
          get().appendOutput(msg.message || 'Auto-fix: fix job failed, retrying...')
        } else if (msg.type === 'autofix_started') {
          get().appendOutput(msg.message || 'Auto-fix: starting fix job...')
        } else if (msg.type === 'consensus_review_complete') {
          get().appendOutput(msg.message || 'Consensus review complete')
          debouncedRefresh()
        } else if (msg.type === 'spec_alignment_started') {
          get().appendOutput(msg.message || 'Spec alignment check started')
        } else if (msg.type === 'spec_alignment_complete') {
          get().appendOutput(msg.message || 'Spec alignment check complete')
          debouncedRefresh()
        } else if (msg.type === 'router_decision') {
          const rd = msg as { data?: { action?: string; reason?: string; attempt?: number; max_retries?: number } }
          const detail = rd.data ? `Router: ${rd.data.action ?? 'advance'}${rd.data.reason ? ` — ${rd.data.reason}` : ''}` : (msg.message || 'Router decision made')
          get().appendOutput(detail)
        } else if (msg.type === 'node_approval_required') {
          const nodeMsg = msg as { node_id?: string; message?: string }
          if (nodeMsg.node_id) {
            set(s => ({
              pendingNodeApprovals: [
                ...s.pendingNodeApprovals.filter(n => n.nodeId !== nodeMsg.node_id),
                { nodeId: nodeMsg.node_id!, message: nodeMsg.message || '' },
              ],
            }))
            get().appendOutput(`Node awaiting approval: ${nodeMsg.node_id}`)
          }
        } else if (msg.type === 'node_completed' || msg.type === 'node_failed') {
          const nodeMsg = msg as { node_id?: string }
          if (nodeMsg.node_id) {
            set(s => ({
              pendingNodeApprovals: s.pendingNodeApprovals.filter(n => n.nodeId !== nodeMsg.node_id),
            }))
          }
        } else if (msg.type === 'risk_evaluated') {
          try {
            const rawData = (msg as { data?: unknown }).data
            let riskData: { score: number; factors: Record<string, number>; level: string } | null = null
            if (typeof rawData === 'string') {
              riskData = JSON.parse(rawData) as { score: number; factors: Record<string, number>; level: string }
            } else if (rawData && typeof rawData === 'object') {
              riskData = rawData as { score: number; factors: Record<string, number>; level: string }
            }
            if (riskData && typeof riskData.score === 'number') {
              set({ riskScore: riskData })
              get().appendOutput(`Risk evaluated: ${riskData.score.toFixed(2)} (${riskData.level})`)
            }
          } catch {
            // Ignore parse errors for risk data
          }
        }
      })

      if (import.meta.env.DEV) {
        console.log('[kvelmo] Worktree WebSocket connecting...')
      }
      await client.connect()
      if (import.meta.env.DEV) {
        console.log('[kvelmo] Worktree WebSocket connected!')
      }

      // Store cleanup function for subscription and debounced handlers
      set({
        client,
        connected: true,
        connecting: false,
        reconnectAttempt: 0,
        error: null,
        unsubscribeSocket: () => {
          debouncedRefresh.cancel()
          debouncedLoadQueue.cancel()
          if (progressInterval) clearInterval(progressInterval)
          unsubscribe()
          worktreeEvents.clear()
        }
      })

      void requestNotificationPermission()

      // Activate server-side event streaming. Pass last known seq so missed
      // events are replayed if this is a reconnect.
      await client.call('stream.subscribe', { last_seq: get().lastSeq })

      // Initial data fetch (parallel — these are independent)
      await Promise.all([
        get().refreshStatus(),
        get().loadQueue(),
        useScreenshotStore.getState().load(client),
      ])
    } catch (err) {
      console.error('[kvelmo] Worktree connection error:', err)
      if (progressInterval) clearInterval(progressInterval)
      set({
        connected: false,
        connecting: false,
        error: err instanceof Error ? err.message : 'Connection failed'
      })
    }
  },

  disconnect: () => {
    // Clean up subscription and debounced handlers
    get().unsubscribeSocket?.()

    const client = get().client
    if (client) {
      client.close()
    }
    const timeoutId = get().reconnectTimeoutId
    if (timeoutId) {
      clearTimeout(timeoutId)
    }
    set({
      connected: false,
      connecting: false,
      reconnectAttempt: 0,
      reconnectTimeoutId: null,
      worktreeId: null,
      client: null,
      unsubscribeSocket: null,
      task: null,
      state: 'none',
      output: [],
      lastSeq: 0,
      checkpoints: [],
      redoStack: [],
      gitStatus: null,
      reviews: [],
      reviewDetails: {},
      taskQueue: [],
      qualityPrompt: null,
      phaseMetrics: null,
      needsRecovery: null,
      skipPhases: [],
      tags: [],
      pendingNodeApprovals: [],
      ciFixStatus: null,
      autoFixStatus: null,
      forks: [],
      recap: null,
    })
  },

  appendOutput: (line: string) => {
    set(state => {
      const newOutput = [...state.output, `[${new Date().toLocaleTimeString()}] ${line}`]
      // Cap at 5000 lines to prevent unbounded memory growth
      if (newOutput.length > 5000) {
        return { output: newOutput.slice(-5000) }
      }
      return { output: newOutput }
    })
  },

  clearOutput: () => {
    set({ output: [] })
  },

  refreshStatus: async () => {
    const client = get().client
    if (!client) return

    try {
      // First fetch status (required to update task state)
      const result = await client.call<{
        state: TaskState
        path: string
        task?: {
          id: string
          title: string
          source: string
          branch?: string
          worktree_path?: string
          context_items?: Array<{ type: string; ref: string; label: string }>
        }
        phase_metrics?: Record<string, PhaseMetrics>
        needs_recovery?: string
        skip_phases?: string[]
      }>('status', {})

      set({
        state: result.state,
        phaseMetrics: result.phase_metrics ?? null,
        needsRecovery: result.needs_recovery ?? null,
        skipPhases: result.skip_phases ?? [],
      })

      if (result.task) {
        set({
          task: {
            id: result.task.id,
            title: result.task.title,
            state: result.state,
            source: result.task.source,
            branch: result.task.branch,
            worktreePath: result.task.worktree_path,
            contextItems: result.task.context_items?.map((ci: { type: string; ref: string; label: string }) => ({
              type: ci.type,
              ref: ci.ref,
              label: ci.label,
            }))
          }
        })
      }

      // Fetch remaining data in parallel (independent of each other)
      await Promise.all([
        // Checkpoints
        client.call<{
          checkpoints: Array<{ sha: string; message: string; author: string; timestamp: string }>
          redo_stack: Array<{ sha: string; message: string; author: string; timestamp: string }>
        }>('checkpoints', {}).then(checkpointsResult => {
          set({
            checkpoints: (checkpointsResult.checkpoints || []).map(c => ({
              sha: c.sha,
              message: c.message,
              timestamp: c.timestamp,
            })),
            redoStack: (checkpointsResult.redo_stack || []).map(c => ({
              sha: c.sha,
              message: c.message,
              timestamp: c.timestamp,
            })),
          })
        }).catch((err) => {
          // Checkpoints may not be available (e.g., no git repo, no task loaded)
          if (import.meta.env.DEV) {
            console.debug('[kvelmo] Checkpoints not available:', err)
          }
        }),

        // Git status
        get().refreshGitStatus(),

        // Review history
        get().loadReviews(),

        // Tags
        get().loadTags(),

        // Progress estimation (only when a phase is active)
        (['planning', 'implementing', 'simplifying', 'optimizing'].includes(result.state)
          ? client.call<{ active: boolean; percent?: number; eta_seconds?: number; calibrated?: boolean }>('progress.get', {}).then(progress => {
              if (progress.active) {
                set({ phaseProgress: { percent: progress.percent ?? 0, eta: progress.eta_seconds ?? -1, calibrated: progress.calibrated ?? false } })
              } else {
                set({ phaseProgress: null })
              }
            }).catch(() => { set({ phaseProgress: null }) })
          : Promise.resolve(set({ phaseProgress: null }))
        ),
      ])
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Status refresh failed' })
    }
  },
})
