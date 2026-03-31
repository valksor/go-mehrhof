import type { StateCreator } from 'zustand'
import type { ForkInfo } from '../types/conductor'
import type {
  ProjectState,
  TaskSlice,
  TaskState,
  ContextItem,
  ReviewOptions,
  SubmitOptions,
  SubmitPreview,
  FinishOptions,
  FinishResult,
  RefreshResult,
  UpdateResult,
  QueuedTask,
  RecapData,
  CacheStats,
} from './projectStore.types'

export const createTaskSlice: StateCreator<ProjectState, [], [], TaskSlice> = (set, get) => ({
  task: null,
  state: 'none',
  taskQueue: [],
  qualityPrompt: null,
  phaseError: null,
  phaseMetrics: null,
  needsRecovery: null,
  skipPhases: [],
  tags: [],
  pendingNodeApprovals: [],
  riskScore: null,
  ciFixStatus: null,
  autoFixStatus: null,
  phaseProgress: null,
  forks: [],
  recap: null,
  cacheStats: null,

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

    set({ loading: true, error: null })
    get().appendOutput(`Approving node: ${nodeId}...`)
    try {
      await client.call('approve.node', { node_id: nodeId })
      set(s => ({
        loading: false,
        pendingNodeApprovals: s.pendingNodeApprovals.filter(n => n.nodeId !== nodeId),
      }))
      get().appendOutput(`Node approved: ${nodeId}`)
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Node approval failed' })
    }
  },

  rejectNode: async (nodeId: string) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput(`Rejecting node: ${nodeId}...`)
    try {
      await client.call('approve.node', { node_id: nodeId, reject: true })
      set(s => ({
        loading: false,
        pendingNodeApprovals: s.pendingNodeApprovals.filter(n => n.nodeId !== nodeId),
      }))
      get().appendOutput(`Node rejected: ${nodeId}`)
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Node rejection failed' })
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

  evaluateRisk: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{ score: number; factors: Record<string, number>; level: string }>('risk.evaluate', {})
      set({ riskScore: result })
    } catch {
      // Risk evaluation may not be available
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
      await get().loadCacheStats()
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Cache clear failed' })
    }
  },

  createFork: async (label: string): Promise<ForkInfo | null> => {
    const client = get().client
    if (!client) return null

    try {
      const result = await client.call<ForkInfo>('fork.create', { label })
      await get().listForks()
      return result
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Failed to create fork' })
      return null
    }
  },

  listForks: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{ forks: ForkInfo[] }>('fork.list', {})
      set({ forks: result.forks || [] })
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Failed to list forks' })
    }
  },

  compareForks: async (): Promise<Record<string, unknown> | null> => {
    const client = get().client
    if (!client) return null

    try {
      const result = await client.call<Record<string, unknown>>('fork.compare', {})
      return result
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Failed to compare forks' })
      return null
    }
  },

  selectFork: async (forkId: string) => {
    const client = get().client
    if (!client) return

    try {
      await client.call('fork.select', { fork_id: forkId })
      await get().listForks()
      await get().refreshStatus()
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Failed to select fork' })
    }
  },
})
