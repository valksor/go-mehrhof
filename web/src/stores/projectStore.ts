import { create } from 'zustand'
import { SocketClient } from '../lib/socket'
import { debounce } from '../lib/debounce'
import { reconnectDelay } from '../lib/reconnect'
import { useScreenshotStore, Screenshot } from './screenshotStore'
import { sendNotification, requestNotificationPermission } from '../lib/notify'
import { worktreeEvents } from '../lib/events'
import type { FailureClass } from '../lib/events'
import type { PhaseMetrics } from '../types/conductor'

type TaskState =
  | 'none'
  | 'loaded'
  | 'planning'
  | 'planned'
  | 'implementing'
  | 'implemented'
  | 'simplifying'
  | 'optimizing'
  | 'reviewing'
  | 'submitted'
  | 'failed'
  | 'waiting'
  | 'paused'

interface ContextItem {
  type: string
  ref: string
  label: string
}

interface Task {
  id: string
  title: string
  state: TaskState
  source: string
  description?: string
  branch?: string
  worktreePath?: string
  contextItems?: ContextItem[]
}

interface Checkpoint {
  sha: string
  message: string
  timestamp: string
}

interface FileChange {
  path: string
  status: 'added' | 'modified' | 'deleted' | 'renamed'
}

interface GitStatus {
  branch: string
  hasChanges: boolean
}

interface GitLogEntry {
  sha: string
  message: string
  author: string
  date: string
}

export interface BrowseEntry {
  name: string
  path: string
  is_dir: boolean
  size?: number
  modified?: string
}

export interface FilesEntry {
  path: string
  size?: number
  modified?: string
}

export interface CacheStats {
  enabled: boolean
  entries: number
  hits: number
  misses: number
  hit_rate: number
  tokens_saved: number
}

export interface Review {
  number: number
  timestamp: string
  approved: boolean
  message: string
}

export interface ReviewDetail {
  number: number
  timestamp: string
  approved: boolean
  message: string
  content: string
  findings: string[]
}

interface ReviewOptions {
  approve?: boolean
  reject?: boolean
  message?: string
  fix?: boolean
}

interface SubmitOptions {
  title?: string
  body?: string
  draft?: boolean
  reviewers?: string[]
  labels?: string[]
  delete_branch?: boolean
  dry_run?: boolean
  sections?: Record<string, string>
}

interface SubmitPreview {
  title: string
  body: string
  branch: string
  base_branch: string
  diff_stat?: string
  checkpoints: number
  specifications: number
}

export interface QueuedTask {
  id: string
  source: string
  title: string
  added_at: string
  position: number
}

export interface SpecEntry {
  path: string
  content: string
}

interface FinishOptions {
  delete_remote?: boolean
  force?: boolean
}

interface FinishResult {
  previous_branch: string
  current_branch: string
  branch_deleted: boolean
  remote_branch_deleted: boolean
}

interface RefreshResult {
  task_id: string
  branch: string
  pr_status: string
  pr_merged: boolean
  pr_url: string
  commits_behind_base: number
  action: string
  message: string
}

interface UpdateResult {
  changed: boolean
  specification_generated: boolean
}

export interface RecapData {
  state: TaskState
  path: string
  task: { id: string; title: string; source: string; branch?: string } | null
  last_checkpoint: { sha: string; message: string; timestamp: string } | null
  checkpoint_count: number
  files_changed: FileChange[] | undefined
  phase_metrics: Record<string, PhaseMetrics> | null
  tags: string[]
  last_activity: string
  next_action: string
  last_error: string
}

interface ProjectState {
  // Connection
  connected: boolean
  connecting: boolean
  reconnectAttempt: number
  reconnectTimeoutId: ReturnType<typeof setTimeout> | null
  connectionVersion: number
  worktreeId: string | null
  client: SocketClient | null
  unsubscribeSocket: (() => void) | null // Cleanup function for socket subscription + debounced handlers

  // Task state
  task: Task | null
  state: TaskState

  // Output stream
  output: string[]
  lastSeq: number

  // Git state
  checkpoints: Checkpoint[]
  redoStack: Checkpoint[]
  gitStatus: GitStatus | null
  fileChanges: FileChange[]

  // Review history
  reviews: Review[]
  reviewDetails: Record<number, ReviewDetail>

  // UI state
  loading: boolean
  error: string | null

  // Task queue
  taskQueue: QueuedTask[]

  // Quality gate prompt (set when conductor needs a yes/no answer)
  qualityPrompt: { id: string; question: string } | null

  // Phase failure classification
  phaseError: { message: string; class: FailureClass; phase: string } | null

  // Phase metrics from status RPC
  phaseMetrics: Record<string, PhaseMetrics> | null

  // Recovery state: interrupted phase name if recovery is needed
  needsRecovery: string | null

  // Skip phases: phases that will be skipped during auto-advance
  skipPhases: string[]

  // Tags
  tags: string[]

  // Pending graph node approvals (set by node_approval_required events)
  pendingNodeApprovals: { nodeId: string; message: string }[]

  // CI fix loop status
  ciFixStatus: { active: boolean; attempt?: number; maxAttempts?: number; result?: 'success' | 'failed' } | null

  // Dry-run mode
  dryRunMode: boolean
  toggleDryRun: () => void

  // PR body override (set from PRPreviewPanel editing)
  prBodyOverride: string | null
  setPrBodyOverride: (body: string | null) => void

  // Connection
  connect: (worktreeId: string) => Promise<void>
  disconnect: () => void

  // Task actions
  start: (source: string, autoAdvance?: boolean, contextItems?: ContextItem[]) => Promise<void>
  quickStart: (source: string) => Promise<void>
  plan: () => Promise<void>
  implement: () => Promise<void>
  simplify: () => Promise<void>
  optimize: () => Promise<void>
  review: (options?: ReviewOptions) => Promise<void>
  submit: (options?: SubmitOptions) => Promise<void>
  stop: () => Promise<void>
  abort: () => Promise<void>
  reset: () => Promise<void>
  retry: () => Promise<void>
  abandon: (keepBranch?: boolean) => Promise<void>
  deleteTask: () => Promise<void>
  update: () => Promise<UpdateResult>
  finish: (options?: FinishOptions) => Promise<FinishResult | null>
  refresh: () => Promise<RefreshResult | null>
  approveTransition: (event: string) => Promise<void>
  approveNode: (nodeId: string) => Promise<void>
  rejectNode: (nodeId: string) => Promise<void>
  approveRemote: (comment?: string) => Promise<void>
  mergeRemote: (method?: string) => Promise<void>

  // Queue actions
  queueTask: (source: string, title?: string) => Promise<QueuedTask | null>
  dequeueTask: (id: string) => Promise<void>
  loadQueue: () => Promise<void>
  reorderQueue: (id: string, position: number) => Promise<void>

  // Tags
  loadTags: () => Promise<void>
  addTag: (tag: string) => Promise<void>
  removeTag: (tag: string) => Promise<void>

  // Quality gate
  respondToPrompt: (promptId: string, answer: boolean) => Promise<void>

  // Checkpoint navigation
  undo: (steps?: number) => Promise<void>
  redo: (steps?: number) => Promise<void>
  goToCheckpoint: (sha: string) => Promise<void>
  previewCheckpoint: (sha: string) => Promise<{ diff: string; stat: string } | null>

  // Git operations
  refreshGitStatus: () => Promise<void>
  getGitDiff: (cached?: boolean) => Promise<string>
  getGitLog: (count?: number) => Promise<GitLogEntry[]>

  // Review history
  loadReviews: () => Promise<void>
  loadReview: (number: number) => Promise<ReviewDetail | null>

  // Specifications & plans
  loadSpec: () => Promise<SpecEntry[]>
  loadPlan: () => Promise<SpecEntry[]>

  // Git diff against branch
  getGitDiffAgainst: (ref: string, stat?: boolean) => Promise<string>

  // File browser
  browseFiles: (path?: string, filesOnly?: boolean) => Promise<BrowseEntry[]>
  listFiles: (path?: string, extensions?: string[], maxDepth?: number) => Promise<FilesEntry[]>

  // Output
  appendOutput: (line: string) => void
  clearOutput: () => void

  // Status refresh
  refreshStatus: () => Promise<void>

  // Recap (resume context)
  recap: RecapData | null
  loadRecap: () => Promise<void>

  // Response cache stats
  cacheStats: CacheStats | null
  loadCacheStats: () => Promise<void>
  clearCache: () => Promise<void>
}

export const useProjectStore = create<ProjectState>((set, get) => ({
  connected: false,
  connecting: false,
  reconnectAttempt: 0,
  reconnectTimeoutId: null,
  connectionVersion: 0,
  worktreeId: null,
  client: null,
  unsubscribeSocket: null,
  recap: null,

  task: null,
  state: 'none',
  output: [],
  lastSeq: 0,
  checkpoints: [],
  redoStack: [],
  gitStatus: null,
  fileChanges: [],
  reviews: [],
  reviewDetails: {},
  loading: false,
  error: null,
  taskQueue: [],
  qualityPrompt: null,
  phaseError: null,
  phaseMetrics: null,
  needsRecovery: null,
  skipPhases: [],
  tags: [],
  pendingNodeApprovals: [],
  ciFixStatus: null,
  cacheStats: null,
  dryRunMode: false,
  toggleDryRun: () => {
    set(s => ({ dryRunMode: !s.dryRunMode }))
  },

  prBodyOverride: null,
  setPrBodyOverride: (body: string | null) => {
    set({ prBodyOverride: body })
  },

  connect: async (worktreeId: string) => {
    if (import.meta.env.DEV) {
      console.log('[kvelmo] projectStore.connect called with:', worktreeId)
    }
    if (get().connected || get().connecting) {
      if (import.meta.env.DEV) {
        console.log('[kvelmo] projectStore already connected/connecting, skipping')
      }
      return
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
            get().connect(wId)
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
          set(updates)
          get().appendOutput(`State: ${msg.state}`)
          debouncedRefresh()
          if (msg.state === 'planned') {
            sendNotification('Planning Complete', 'Specification is ready for review')
          } else if (msg.state === 'implemented') {
            sendNotification('Implementation Complete', 'Code is ready for review')
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
          sendNotification('Task Completed', get().task?.title || 'Job finished successfully')
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
          sendNotification('Task Failed', msg.error || 'A job has failed')
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
          sendNotification('CI Fix Success', 'Pipeline passed after fix')
        } else if (msg.type === 'ci_fix_exhausted') {
          set({ ciFixStatus: { active: false, result: 'failed' } })
          get().appendOutput(msg.message || 'CI fix: all attempts exhausted')
          sendNotification('CI Fix Failed', 'Pipeline still failing after all fix attempts')
        } else if (msg.type === 'ci_fix_attempt_failed') {
          get().appendOutput(msg.message || 'CI fix: attempt failed, retrying...')
        } else if (msg.type === 'ci_fix_started') {
          get().appendOutput(msg.message || 'CI fix: starting fix job...')
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
          unsubscribe()
          worktreeEvents.clear()
        }
      })

      requestNotificationPermission()

      // Activate server-side event streaming. Pass last known seq so missed
      // events are replayed if this is a reconnect.
      await client.call('stream.subscribe', { last_seq: get().lastSeq })

      // Initial status fetch
      await get().refreshStatus()

      // Load task queue
      await get().loadQueue()

      // Load screenshots (pass client to avoid circular import)
      await useScreenshotStore.getState().load(client)
    } catch (err) {
      console.error('[kvelmo] Worktree connection error:', err)
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
      recap: null,
    })
  },

  start: async (source: string, autoAdvance: boolean = false, contextItems?: ContextItem[]) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput(`Loading task from ${source}...${autoAdvance ? ' (auto-advance enabled)' : ''}`)

    try {
      const params: Record<string, unknown> = { source, auto_advance: autoAdvance }
      if (contextItems && contextItems.length > 0) {
        params.context_items = contextItems
      }
      const result = await client.call<{ status: string; state: TaskState }>('start', params)
      set({ state: result.state, loading: false })
      await get().refreshStatus()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Start failed' })
    }
  },

  quickStart: async (source: string) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput(`Quick fix: loading task from ${source} (skip plan, auto-advance)...`)

    try {
      const result = await client.call<{ status: string; state: TaskState }>('start', {
        source,
        auto_advance: true,
        skip_phases: ['plan'],
      })
      set({ state: result.state, loading: false })
      get().appendOutput('Quick fix started — will auto-advance through implement and submit.')
      await get().refreshStatus()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Quick start failed' })
    }
  },

  plan: async () => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Starting planning...')

    try {
      const result = await client.call<{ status: string; state: TaskState; job_id?: string }>('plan', { dry_run: get().dryRunMode })
      set({ state: result.state, loading: false })
      get().appendOutput(`Planning job started: ${result.job_id || ''}`)
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Plan failed' })
    }
  },

  implement: async () => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Starting implementation...')

    try {
      const result = await client.call<{ status: string; state: TaskState; job_id?: string }>('implement', { dry_run: get().dryRunMode })
      set({ state: result.state, loading: false })
      get().appendOutput(`Implementation job started: ${result.job_id || ''}`)
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Implement failed' })
    }
  },

  simplify: async () => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Starting simplification...')

    try {
      const result = await client.call<{ status: string; state: TaskState; job_id?: string }>('simplify', { dry_run: get().dryRunMode })
      set({ state: result.state, loading: false })
      get().appendOutput(`Simplification job started: ${result.job_id || ''}`)
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Simplify failed' })
    }
  },

  optimize: async () => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Starting optimization...')

    try {
      const result = await client.call<{ status: string; state: TaskState; job_id?: string }>('optimize', { dry_run: get().dryRunMode })
      set({ state: result.state, loading: false })
      get().appendOutput(`Optimization job started: ${result.job_id || ''}`)
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Optimize failed' })
    }
  },

  review: async (options?: ReviewOptions) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Starting review...')

    try {
      const result = await client.call<{ status: string; state: TaskState }>('review', {
        approve: options?.approve ?? false,
        reject: options?.reject ?? false,
        message: options?.message,
        fix: options?.fix ?? false
      })
      set({ state: result.state, loading: false })
      await get().loadReviews()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Review failed' })
    }
  },

  submit: async (options?: SubmitOptions) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput(options?.dry_run ? 'Generating PR preview...' : 'Submitting...')

    try {
      const result = await client.call<{ status: string; state: TaskState; preview?: SubmitPreview }>('submit', {
        title: options?.title,
        body: options?.body,
        draft: options?.draft ?? false,
        reviewers: options?.reviewers ?? [],
        labels: options?.labels ?? [],
        delete_branch: options?.delete_branch ?? false,
        dry_run: options?.dry_run ?? false,
        sections: options?.sections,
      })
      if (options?.dry_run && result.preview) {
        set({ loading: false })
        get().appendOutput(`PR preview ready: ${result.preview.title}`)
        // Lazy import to avoid circular dependency (layoutStore imports projectStore)
        const { useLayoutStore } = await import('./layoutStore')
        const { openTab, setActiveTab, closeTab } = useLayoutStore.getState()
        // Close existing preview tab to ensure fresh data (openTab skips data update for existing IDs)
        closeTab('submit-preview')
        openTab({
          id: 'submit-preview',
          type: 'submit-preview',
          title: 'PR Preview',
          data: { preview: result.preview },
          closeable: true,
        })
        setActiveTab('submit-preview')
      } else {
        set({ state: result.state, loading: false })
        get().appendOutput('Task submitted!')
      }
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Submit failed' })
    }
  },

  stop: async () => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })

    try {
      const result = await client.call<{ status: string; state: TaskState }>('stop', {})
      set({ state: result.state, loading: false })
      get().appendOutput('Operation stopped')
      await get().refreshStatus()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Stop failed' })
    }
  },

  abort: async () => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })

    try {
      const result = await client.call<{ status: string; state: TaskState }>('abort', {})
      set({ state: result.state, loading: false })
      get().appendOutput('Task aborted')
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Abort failed' })
    }
  },

  reset: async () => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })

    try {
      const result = await client.call<{ status: string; state: TaskState }>('reset', {})
      set({ state: result.state, loading: false })
      get().appendOutput('Task reset')
      await get().refreshStatus()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Reset failed' })
    }
  },

  retry: async () => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Retrying failed phase...')

    try {
      // Step 1: Get status to confirm failed state and determine phase
      const status = await client.call<{
        state: TaskState
        last_error?: string
      }>('status', {})

      if (status.state !== 'failed') {
        set({ loading: false, error: `Retry requires failed state, currently '${status.state}'` })
        return
      }

      // Phase inference uses substring matching on the error message.
      // Good enough for retry UX — the backend doesn't report the failed phase directly.
      const lastError = (status.last_error || '').toLowerCase()
      let phase = 'plan'
      if (lastError.includes('implement')) phase = 'implement'
      else if (lastError.includes('simplify')) phase = 'simplify'
      else if (lastError.includes('optimize')) phase = 'optimize'
      else if (lastError.includes('plan')) phase = 'plan'

      get().appendOutput(`Failed phase detected: ${phase}`)

      // Step 2: Reset the task
      const resetResult = await client.call<{ status: string; state: TaskState }>('reset', {})
      set({ state: resetResult.state })
      get().appendOutput(`Task reset (state: ${resetResult.state})`)

      // Step 3: Re-run the failed phase
      get().appendOutput(`Submitting ${phase} phase...`)
      const phaseResult = await client.call<{ status: string; state: TaskState; job_id?: string }>(phase, {})
      set({ state: phaseResult.state, loading: false })
      get().appendOutput(`${phase.charAt(0).toUpperCase() + phase.slice(1)} job submitted: ${phaseResult.job_id || ''}`)
      await get().refreshStatus()
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Retry failed'
      set({ loading: false, error: errorMsg })
      get().appendOutput(`Retry failed: ${errorMsg}. Task may be in reset state - check status.`)
    }
  },

  abandon: async (keepBranch: boolean = false) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Abandoning task...')

    try {
      const result = await client.call<{ status: string; state: TaskState }>('abandon', { keep_branch: keepBranch })
      set({ state: result.state, loading: false })
      get().appendOutput('Task abandoned')
      await get().refreshStatus()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Abandon failed' })
    }
  },

  update: async (): Promise<UpdateResult> => {
    const client = get().client
    if (!client) return { changed: false, specification_generated: false }

    set({ loading: true, error: null })
    get().appendOutput('Updating task from source...')

    try {
      const result = await client.call<UpdateResult>('update', {})
      set({ loading: false })
      get().appendOutput(
        result.changed
          ? `Task updated from source${result.specification_generated ? ' — new specification generated' : ''}`
          : 'Task is already up to date'
      )
      await get().refreshStatus()
      return result
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Update failed' })
      return { changed: false, specification_generated: false }
    }
  },

  finish: async (options?: FinishOptions): Promise<FinishResult | null> => {
    const client = get().client
    if (!client) return null

    set({ loading: true, error: null })
    get().appendOutput('Finishing task...')

    try {
      const result = await client.call<FinishResult>('task.finish', {
        delete_remote: options?.delete_remote ?? false,
        force: options?.force ?? false
      })
      set({ state: 'none', task: null, loading: false })
      get().appendOutput(`Finished! Switched to ${result.current_branch}`)
      if (result.branch_deleted) {
        get().appendOutput(`Deleted local branch: ${result.previous_branch}`)
      }
      if (result.remote_branch_deleted) {
        get().appendOutput(`Deleted remote branch: ${result.previous_branch}`)
      }
      await get().refreshStatus()
      return result
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Finish failed' })
      return null
    }
  },

  refresh: async (): Promise<RefreshResult | null> => {
    const client = get().client
    if (!client) return null

    set({ loading: true, error: null })
    get().appendOutput('Checking PR status...')

    try {
      const result = await client.call<RefreshResult>('task.refresh', {})
      set({ loading: false })
      get().appendOutput(result.message)
      if (result.pr_url) {
        get().appendOutput(`PR: ${result.pr_url}`)
      }
      return result
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Refresh failed' })
      return null
    }
  },

  deleteTask: async () => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Deleting task...')

    try {
      await client.call('delete', {})
      set({ state: 'none', task: null, loading: false })
      get().appendOutput('Task deleted')
      await get().refreshStatus()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Delete failed' })
    }
  },

  approveTransition: async (event: string) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput(`Approving transition: ${event}...`)

    try {
      await client.call('approve', { event })
      set({ loading: false })
      get().appendOutput(`Transition approved: ${event}`)
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Approval failed' })
    }
  },

  approveNode: async (nodeId: string) => {
    const client = get().client
    if (!client) return

    get().appendOutput(`Approving node: ${nodeId}...`)
    try {
      await client.call('approve.node', { node_id: nodeId })
      set(s => ({
        pendingNodeApprovals: s.pendingNodeApprovals.filter(n => n.nodeId !== nodeId),
      }))
      get().appendOutput(`Node approved: ${nodeId}`)
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Node approval failed' })
    }
  },

  rejectNode: async (nodeId: string) => {
    const client = get().client
    if (!client) return

    get().appendOutput(`Rejecting node: ${nodeId}...`)
    try {
      await client.call('approve.node', { node_id: nodeId, reject: true })
      set(s => ({
        pendingNodeApprovals: s.pendingNodeApprovals.filter(n => n.nodeId !== nodeId),
      }))
      get().appendOutput(`Node rejected: ${nodeId}`)
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Node rejection failed' })
    }
  },

  approveRemote: async (comment?: string) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Approving PR...')

    try {
      await client.call('remote.approve', { comment: comment ?? '' })
      set({ loading: false })
      get().appendOutput('PR approved')
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Approve failed' })
    }
  },

  mergeRemote: async (method?: string) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput('Merging PR...')

    try {
      await client.call('remote.merge', { method: method ?? 'rebase' })
      set({ loading: false })
      get().appendOutput('PR merged')
      await get().refreshStatus()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Merge failed' })
    }
  },

  respondToPrompt: async (promptId: string, answer: boolean) => {
    const client = get().client
    if (!client) return

    try {
      await client.call('quality.respond', { prompt_id: promptId, answer })
      set({ qualityPrompt: null })
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Quality response failed' })
    }
  },

  queueTask: async (source: string, title?: string): Promise<QueuedTask | null> => {
    const client = get().client
    if (!client) return null

    try {
      const result = await client.call<QueuedTask>('queue.add', { source, title: title ?? '' })
      await get().loadQueue()
      return result
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Queue add failed' })
      return null
    }
  },

  dequeueTask: async (id: string) => {
    const client = get().client
    if (!client) return

    try {
      await client.call('queue.remove', { id })
      await get().loadQueue()
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Queue remove failed' })
    }
  },

  loadQueue: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{ queue: QueuedTask[]; count: number }>('queue.list', {})
      set({ taskQueue: result.queue || [] })
    } catch {
      // Queue may not be available
    }
  },

  reorderQueue: async (id: string, position: number) => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{ queue: QueuedTask[]; count: number }>('queue.reorder', { id, position })
      set({ taskQueue: result.queue || [] })
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Queue reorder failed' })
    }
  },

  loadTags: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{ tags: string[] }>('task.tag', { action: 'list' })
      set({ tags: result.tags || [] })
    } catch {
      // Tags may not be available (no active task)
    }
  },

  addTag: async (tag: string) => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{ tags: string[] }>('task.tag', { action: 'add', tags: [tag] })
      set({ tags: result.tags || [] })
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Failed to add tag' })
    }
  },

  removeTag: async (tag: string) => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{ tags: string[] }>('task.tag', { action: 'remove', tags: [tag] })
      set({ tags: result.tags || [] })
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Failed to remove tag' })
    }
  },

  undo: async (steps: number = 1) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })

    try {
      const result = await client.call<{ status: string; state: TaskState; steps: number }>('undo', { steps })
      set({ state: result.state, loading: false })
      get().appendOutput(`Undo complete (${result.steps} step${result.steps > 1 ? 's' : ''})`)
      await get().refreshStatus()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Undo failed' })
    }
  },

  redo: async (steps: number = 1) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })

    try {
      const result = await client.call<{ status: string; state: TaskState; steps: number }>('redo', { steps })
      set({ state: result.state, loading: false })
      get().appendOutput(`Redo complete (${result.steps} step${result.steps > 1 ? 's' : ''})`)
      await get().refreshStatus()
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Redo failed' })
    }
  },

  goToCheckpoint: async (sha: string) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput(`Navigating to checkpoint ${sha.slice(0, 8)}...`)

    try {
      await client.call<{ status: string; sha: string }>('checkpoint.goto', { sha })
      get().appendOutput(`Restored to checkpoint ${sha.slice(0, 8)}`)
      await get().refreshStatus()
      set({ loading: false })
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Checkpoint navigation failed' })
    }
  },

  previewCheckpoint: async (sha: string) => {
    const client = get().client
    if (!client) return null

    try {
      return await client.call<{ diff: string; stat: string }>('checkpoint.preview', { sha })
    } catch {
      return null
    }
  },

  refreshGitStatus: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{
        branch: string
        has_changes: boolean
        files: Array<{ path: string; status: 'added' | 'modified' | 'deleted' | 'renamed' }>
      }>('git.status', {})
      set({
        gitStatus: {
          branch: result.branch,
          hasChanges: result.has_changes,
        },
        fileChanges: (result.files || []).map(f => ({
          path: f.path,
          status: f.status,
        }))
      })
    } catch (err) {
      // Git status may not be available
      console.warn('Could not fetch git status:', err)
    }
  },

  getGitDiff: async (cached: boolean = false): Promise<string> => {
    const client = get().client
    if (!client) return ''

    try {
      const result = await client.call<{ diff: string }>('git.diff', { cached })
      return result.diff || ''
    } catch (err) {
      console.warn('Could not fetch git diff:', err)
      return ''
    }
  },

  getGitLog: async (count: number = 10): Promise<GitLogEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ entries: GitLogEntry[] }>('git.log', { count })
      return result.entries || []
    } catch (err) {
      console.warn('Could not fetch git log:', err)
      return []
    }
  },

  loadReviews: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{ reviews: Review[] }>('review.list', {})
      set({ reviews: result.reviews || [] })
    } catch (err) {
      console.warn('Could not fetch review history:', err)
    }
  },

  loadReview: async (number: number): Promise<ReviewDetail | null> => {
    const client = get().client
    if (!client) return null

    // Return cached if available
    const cached = get().reviewDetails[number]
    if (cached) return cached

    try {
      const result = await client.call<ReviewDetail>('review.view', { number })
      set(state => ({
        reviewDetails: { ...state.reviewDetails, [number]: result }
      }))
      return result
    } catch (err) {
      console.warn('Could not fetch review detail:', err)
      return null
    }
  },

  loadSpec: async (): Promise<SpecEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ specifications: SpecEntry[] }>('show.spec', {})
      return result.specifications || []
    } catch (err) {
      console.warn('Could not load specifications:', err)
      return []
    }
  },

  loadPlan: async (): Promise<SpecEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ specifications: SpecEntry[] }>('show.plan', {})
      return result.specifications || []
    } catch (err) {
      console.warn('Could not load plan:', err)
      return []
    }
  },

  getGitDiffAgainst: async (ref: string, stat: boolean = false): Promise<string> => {
    const client = get().client
    if (!client) return ''

    try {
      const result = await client.call<{ diff: string }>('git.diff_against', { ref, stat })
      return result.diff || ''
    } catch (err) {
      console.warn('Could not fetch diff against ref:', err)
      return ''
    }
  },

  browseFiles: async (path?: string, filesOnly: boolean = false): Promise<BrowseEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ entries: BrowseEntry[] }>('browse', { path, files: filesOnly })
      return result.entries || []
    } catch (err) {
      console.warn('Could not browse files:', err)
      return []
    }
  },

  listFiles: async (path?: string, extensions?: string[], maxDepth?: number): Promise<FilesEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ files: FilesEntry[] }>('files.list', {
        path,
        extensions,
        max_depth: maxDepth
      })
      return result.files || []
    } catch (err) {
      console.warn('Could not list files:', err)
      return []
    }
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
      ])
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Status refresh failed' })
    }
  },

  loadRecap: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<RecapData>('recap', {})
      set({ recap: result })
    } catch {
      // Recap may not be available
    }
  },

  loadCacheStats: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<CacheStats>('cache.stats', {})
      set({ cacheStats: result })
    } catch {
      // Cache stats may not be available
    }
  },

  clearCache: async () => {
    const client = get().client
    if (!client) return

    try {
      await client.call('cache.clear', {})
      set({ cacheStats: { enabled: true, entries: 0, hits: 0, misses: 0, hit_rate: 0, tokens_saved: 0 } })
    } catch {
      // Ignore clear failures
    }
  }
}))
