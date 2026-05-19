import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { STORE_DEBOUNCE_MS } from '../lib/constants'
import { SocketClient } from '../lib/socket'
import { debounce } from '../lib/debounce'
import { reconnectDelay } from '../lib/reconnect'
import { storeName } from '../meta'
import type { WorktreeInfo, WorkerInfo, WorkersStats, MemoryStatsResponse, TaskListSummary } from '../types/socket'

// Agent status from agent.status RPC
export interface AgentCheckResult {
  name: string
  status: 'passed' | 'failed' | 'warning'
  detail?: string
  fix?: string
}

export interface AgentStatus {
  checks: AgentCheckResult[]
  agent_available: boolean
  simulation_mode: boolean
}

// Worktree health from system.health RPC
export interface WorktreeHealth {
  id: string
  path: string
  state: string
  healthy: boolean | null
  last_ping?: string
}

// Types not yet in Go (UI-only or need backend work)
export interface Job {
  id: string
  type: string
  status: string
  worktree_id: string
  created_at: string
  updated_at?: string
  error?: string
  result?: Record<string, unknown>
}

export interface DocumentOutcome {
  success: boolean
  pr_merged: boolean
  ci_passed_first_try: boolean
  human_changes_needed: boolean
}

export interface MemoryResult {
  id: string
  type: string
  content: string
  score: number
  task_id: string
  created_at: string
  outcome?: DocumentOutcome
}

export interface TaskGroupRef {
  project_dir: string
  task_id: string
  state: string
}

export interface TaskGroup {
  id: string
  label: string
  tasks: TaskGroupRef[]
  status: string
  created_at: string
  updated_at: string
}

export interface TimedSnapshot {
  timestamp: string
  jobs_submitted: number
  jobs_completed: number
  jobs_failed: number
  jobs_in_progress: number
  rpc_requests: number
  rpc_errors: number
  agent_connects: number
  events_dropped: number
}

export interface Metrics {
  jobs_submitted: number
  jobs_completed: number
  jobs_failed: number
  jobs_in_progress: number
  rpc_requests: number
  rpc_errors: number
  avg_latency_ms: number
  p99_latency_ms: number
  agent_connects: number
  agent_disconnects: number
  events_dropped: number
  permissions_approved: number
  permissions_denied: number
  tokens_consumed: number
  agents?: Record<string, { tokens: number; requests: number; errors: number; avg_latency_ms: number }>
}

interface GlobalState {
  // Connection
  connected: boolean
  connecting: boolean
  reconnectAttempt: number
  reconnectTimeoutId: ReturnType<typeof setTimeout> | null
  connectionVersion: number // Incremented on each connect attempt to prevent stale disconnect handlers
  client: SocketClient | null
  unsubscribeSocket: (() => void) | null // Cleanup function for socket subscription

  // Data
  projects: WorktreeInfo[]
  workers: WorkerInfo[]
  workerStats: WorkersStats | null
  metrics: Metrics | null
  metricsHistory: TimedSnapshot[]
  memoryStats: MemoryStatsResponse | null
  selectedProjectId: string | null
  selectedProject: WorktreeInfo | null
  loading: boolean
  error: string | null

  // Agent status
  agentStatus: AgentStatus | null

  // System health (per-worktree health from system.health RPC)
  worktreeHealth: WorktreeHealth[]

  // Active tasks across all projects
  activeTasks: TaskListSummary[]

  // Jobs
  jobs: Job[]

  // Connection
  connect: () => Promise<void>
  disconnect: () => void

  // Projects
  loadProjects: () => Promise<void>
  addProject: (path: string) => Promise<void>
  removeProject: (id: string) => Promise<void>
  selectProject: (project: WorktreeInfo | null) => void

  selectNextProject: () => void
  selectPrevProject: () => void

  // Workers
  loadWorkers: () => Promise<void>
  loadWorkerStats: () => Promise<void>
  loadMetrics: () => Promise<void>
  loadMetricsHistory: () => Promise<void>
  addWorker: (agent: string) => Promise<void>
  removeWorker: (id: string) => Promise<void>

  // Chat
  stopChat: (worktreeId: string, jobId?: string) => Promise<void>

  // Agent
  loadAgentStatus: () => Promise<void>

  // Health
  loadHealth: () => Promise<void>

  // Tasks
  loadActiveTasks: () => Promise<void>
  batchAction: (
    action: string,
    filters?: { state?: string; tag?: string; match?: string },
  ) => Promise<{
    total: number
    succeeded: number
    results: Array<{ path: string; state: string; success: boolean; error?: string }>
  }>

  // Memory
  searchMemory: (query: string, limit?: number) => Promise<MemoryResult[]>
  loadMemoryStats: () => Promise<void>
  clearMemory: () => Promise<void>

  // Jobs
  loadJobs: () => Promise<void>
  loadJob: (id: string) => Promise<Job | null>

  // Task Groups
  taskGroups: TaskGroup[]
  loadTaskGroups: () => Promise<void>
  createTaskGroup: (label: string) => Promise<TaskGroup | null>
  addTaskToGroup: (groupId: string, taskId: string, projectDir: string) => Promise<void>
  submitTaskGroup: (groupId: string) => Promise<void>
  getTaskGroupStatus: (groupId: string) => Promise<TaskGroup | null>
  removeTaskGroup: (id: string) => Promise<void>
}

export const useGlobalStore = create<GlobalState>()(
  persist(
    (set, get) => ({
      connected: false,
      connecting: false,
      reconnectAttempt: 0,
      reconnectTimeoutId: null,
      connectionVersion: 0,
      client: null,
      unsubscribeSocket: null,

      projects: [],
      workers: [],
      workerStats: null,
      metrics: null,
      metricsHistory: [],
      memoryStats: null,
      jobs: [],
      taskGroups: [],
      agentStatus: null,
      worktreeHealth: [],
      selectedProjectId: null,
      selectedProject: null,
      loading: false,
      error: null,
      activeTasks: [],
      connect: async () => {
        if (get().connected || get().connecting) return

        // Increment connection version to invalidate stale disconnect handlers
        const thisVersion = get().connectionVersion + 1
        set({ connecting: true, error: null, connectionVersion: thisVersion })

        // Support dynamic port injection for Tauri desktop app
        // In dev mode (Vite), connect directly to Go server on 6337
        // In production (Tauri), use ?port= param or window.location.host
        const getBackendHost = () => {
          const params = new URLSearchParams(window.location.search)
          const port = params.get('port')
          // Validate port is numeric to prevent URL injection
          if (port && /^\d+$/.test(port)) return `localhost:${port}`
          // Dev mode: Vite runs on different port, connect directly to Go server
          if (import.meta.env.DEV) return 'localhost:6337'
          return window.location.host
        }

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
        const url = `${protocol}//${getBackendHost()}/ws/global`
        if (import.meta.env.DEV) {
          console.log('[kvelmo] Connecting to:', url)
        }
        const client = new SocketClient(url)

        // Handle disconnection with auto-reconnect (exponential backoff)
        // Capture thisVersion to detect stale handlers from previous connect attempts
        client.setOnDisconnect(() => {
          // Ignore if this handler is from a stale connection attempt
          if (get().connectionVersion !== thisVersion) return

          const attempt = get().reconnectAttempt + 1
          const delay = reconnectDelay(attempt)
          const delaySec = Math.round(delay / 1000)

          const timeoutId = setTimeout(() => {
            set({ reconnectTimeoutId: null })
            void get().connect()
          }, delay)

          set({
            connected: false,
            connecting: false,
            reconnectAttempt: attempt,
            reconnectTimeoutId: timeoutId,
            client: null,
            error: `Connection lost. Reconnecting in ${delaySec}s... (attempt ${attempt})`,
          })
        })

        try {
          await client.connect()

          // Clean up previous subscription if any (defensive - shouldn't happen with new client)
          get().unsubscribeSocket?.()

          // Create debounced loaders to prevent RPC flood on rapid state changes
          const debouncedLoadTasks = debounce(() => get().loadActiveTasks(), STORE_DEBOUNCE_MS)
          const debouncedLoadWorkers = debounce(() => get().loadWorkers(), STORE_DEBOUNCE_MS)

          // Subscribe to server-pushed notifications (e.g., task_state_changed)
          const unsubscribe = client.subscribe((msg: unknown) => {
            const notification = msg as { method?: string; params?: Record<string, string> }
            if (notification.method === 'task_state_changed') {
              // Debounced refresh to prevent cascade on rapid updates
              debouncedLoadTasks()
            } else if (notification.method === 'worker_changed') {
              debouncedLoadWorkers()
            }
          })

          set({
            client,
            connected: true,
            connecting: false,
            reconnectAttempt: 0,
            error: null,
            unsubscribeSocket: () => {
              debouncedLoadTasks.cancel()
              debouncedLoadWorkers.cancel()
              unsubscribe()
            },
          })

          // Load initial data
          await get().loadProjects()
          await get().loadWorkers()
          await get().loadActiveTasks()
          await get().loadAgentStatus()
          void get().loadHealth()
          void get().loadMetrics()
        } catch (err) {
          set({
            connected: false,
            connecting: false,
            error: err instanceof Error ? err.message : 'Connection failed',
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
        // Clear any pending reconnect timeout
        const timeoutId = get().reconnectTimeoutId
        if (timeoutId) {
          clearTimeout(timeoutId)
        }
        set({
          connected: false,
          connecting: false,
          reconnectAttempt: 0,
          reconnectTimeoutId: null,
          client: null,
          unsubscribeSocket: null,
        })
      },

      loadProjects: async () => {
        const client = get().client
        if (!client) {
          set({ error: 'Not connected' })
          return
        }

        set({ loading: true, error: null })
        try {
          const result = await client.call<{ projects: WorktreeInfo[] }>('projects.list')
          const projects = result.projects || []

          set({ projects, loading: false })
        } catch (err) {
          set({
            error: err instanceof Error ? err.message : 'Failed to load projects',
            loading: false,
          })
        }
      },

      addProject: async (path: string) => {
        const client = get().client
        if (!client) {
          set({ error: 'Not connected' })
          return
        }

        set({ loading: true, error: null })
        try {
          await client.call<{ id: string }>('projects.register', { path })
          // Reload projects
          await get().loadProjects()
        } catch (err) {
          set({
            error: err instanceof Error ? err.message : 'Failed to add project',
            loading: false,
          })
        }
      },

      removeProject: async (id: string) => {
        const client = get().client
        if (!client) {
          set({ error: 'Not connected' })
          return
        }

        set({ loading: true, error: null })
        try {
          await client.call('projects.unregister', { id })
          // Clear selection if removed
          const selectedId = get().selectedProjectId
          if (selectedId === id) {
            set({ selectedProject: null, selectedProjectId: null })
          }
          // Reload projects
          await get().loadProjects()
        } catch (err) {
          set({
            error: err instanceof Error ? err.message : 'Failed to remove project',
            loading: false,
          })
        }
      },

      selectProject: (project) => {
        set({
          selectedProject: project,
          selectedProjectId: project?.id || null,
        })
        // Persist to sessionStorage for page refresh survival (cleared on tab close)
        if (project) {
          sessionStorage.setItem('kvelmo-selectedProjectId', project.id)
        } else {
          sessionStorage.removeItem('kvelmo-selectedProjectId')
        }
      },

      selectNextProject: () => {
        const { projects, selectedProject } = get()
        if (projects.length === 0) return
        const currentIdx = selectedProject ? projects.findIndex((p) => p.id === selectedProject.id) : -1
        const nextIdx = Math.min(currentIdx + 1, projects.length - 1)
        get().selectProject(projects[nextIdx])
      },

      selectPrevProject: () => {
        const { projects, selectedProject } = get()
        if (projects.length === 0) return
        const currentIdx = selectedProject ? projects.findIndex((p) => p.id === selectedProject.id) : projects.length
        const prevIdx = Math.max(currentIdx - 1, 0)
        get().selectProject(projects[prevIdx])
      },

      loadWorkers: async () => {
        const client = get().client
        if (!client) return

        try {
          const result = await client.call<{ workers: WorkerInfo[]; stats: WorkersStats }>('workers.list')
          set({
            workers: result.workers || [],
            workerStats: result.stats || null,
          })
        } catch (err) {
          console.warn('Failed to load workers:', err)
        }
      },

      loadWorkerStats: async () => {
        const client = get().client
        if (!client) return

        try {
          const result = await client.call<WorkersStats>('workers.stats', {})
          set({ workerStats: result })
        } catch (err) {
          console.warn('Failed to load worker stats:', err)
        }
      },

      loadMetrics: async () => {
        const client = get().client
        if (!client) return
        try {
          const result = await client.call<Metrics>('metrics', {})
          set({ metrics: result })
        } catch {
          // Metrics are optional, don't show errors
        }
      },

      loadMetricsHistory: async () => {
        const client = get().client
        if (!client) return
        try {
          const result = await client.call<{ entries: TimedSnapshot[]; enabled: boolean }>('metrics.history', {})
          if (result.enabled) {
            set({ metricsHistory: result.entries })
          }
        } catch {
          // History is optional
        }
      },

      addWorker: async (agent: string) => {
        const client = get().client
        if (!client) {
          set({ error: 'Not connected' })
          return
        }

        set({ loading: true, error: null })
        try {
          await client.call('workers.add', { agent })
          await get().loadWorkers()
          set({ loading: false })
        } catch (err) {
          set({
            error: err instanceof Error ? err.message : 'Failed to add worker',
            loading: false,
          })
        }
      },

      removeWorker: async (id: string) => {
        const client = get().client
        if (!client) {
          set({ error: 'Not connected' })
          return
        }

        set({ loading: true, error: null })
        try {
          await client.call('workers.remove', { id })
          await get().loadWorkers()
          set({ loading: false })
        } catch (err) {
          set({
            error: err instanceof Error ? err.message : 'Failed to remove worker',
            loading: false,
          })
        }
      },

      stopChat: async (worktreeId: string, jobId?: string) => {
        const client = get().client
        if (!client) {
          set({ error: 'Not connected' })
          return
        }

        try {
          const params: Record<string, string> = { worktree_id: worktreeId }
          if (jobId) {
            params.job_id = jobId
          }
          await client.call('chat.stop', params)
        } catch (err) {
          set({ error: err instanceof Error ? err.message : 'Failed to stop chat' })
        }
      },

      loadAgentStatus: async () => {
        const client = get().client
        if (!client) return

        try {
          const result = await client.call<AgentStatus>('agent.status')
          set({ agentStatus: result })
        } catch (err) {
          console.warn('Failed to load agent status:', err)
        }
      },

      loadHealth: async () => {
        const client = get().client
        if (!client) return

        try {
          const result = await client.call<{ worktrees: WorktreeHealth[] }>('system.health')
          set({ worktreeHealth: result.worktrees || [] })
        } catch {
          // Non-critical — health data is informational
        }
      },

      loadActiveTasks: async () => {
        const client = get().client
        if (!client) return

        try {
          const result = await client.call<{ tasks: TaskListSummary[] }>('tasks.list')
          set({ activeTasks: result.tasks || [] })
        } catch (err) {
          console.warn('Failed to load active tasks:', err)
        }
      },

      batchAction: async (action: string, filters?: { state?: string; tag?: string; match?: string }) => {
        const client = get().client
        if (!client)
          return {
            total: 0,
            succeeded: 0,
            results: [] as Array<{ path: string; state: string; success: boolean; error?: string }>,
          }

        const params: Record<string, unknown> = { action }
        if (filters) {
          const filter = Object.fromEntries(Object.entries(filters).filter(([, v]) => v))
          if (Object.keys(filter).length > 0) {
            params.filter = filter
          }
        }

        const result = await client.call<{
          total: number
          results: Array<{ path: string; state: string; success: boolean; error?: string }>
        }>('tasks.batch', params)
        const succeeded = result.results?.filter((r) => r.success).length ?? 0

        // Refresh task list after batch action
        await get().loadActiveTasks()

        return { total: result.total, succeeded, results: result.results ?? [] }
      },

      searchMemory: async (query: string, limit: number = 10): Promise<MemoryResult[]> => {
        const client = get().client
        if (!client) return []

        try {
          const result = await client.call<{ results: MemoryResult[] }>('memory.search', { query, limit })
          return result.results || []
        } catch (err) {
          console.warn('Failed to search memory:', err)
          return []
        }
      },

      loadMemoryStats: async () => {
        const client = get().client
        if (!client) return

        try {
          const result = await client.call<MemoryStatsResponse>('memory.stats', {})
          set({ memoryStats: result })
        } catch (err) {
          console.warn('Failed to load memory stats:', err)
        }
      },

      clearMemory: async () => {
        const client = get().client
        if (!client) return

        try {
          await client.call('memory.clear', {})
          set({ memoryStats: null })
        } catch (err) {
          console.warn('Failed to clear memory:', err)
        }
      },

      loadJobs: async () => {
        const client = get().client
        if (!client) return

        try {
          const result = await client.call<{ jobs: Job[] }>('jobs.list', {})
          set({ jobs: result.jobs || [] })
        } catch (err) {
          console.warn('Failed to load jobs:', err)
        }
      },

      loadJob: async (id: string): Promise<Job | null> => {
        const client = get().client
        if (!client) return null

        try {
          const result = await client.call<Job>('jobs.get', { id })
          return result
        } catch (err) {
          console.warn('Failed to load job:', err)
          return null
        }
      },

      loadTaskGroups: async () => {
        const client = get().client
        if (!client) return

        try {
          const result = await client.call<{ groups: TaskGroup[] }>('taskgroup.list', {})
          set({ taskGroups: result.groups || [] })
        } catch (err) {
          console.warn('Failed to load task groups:', err)
        }
      },

      createTaskGroup: async (label: string): Promise<TaskGroup | null> => {
        const client = get().client
        if (!client) return null

        try {
          const result = await client.call<TaskGroup>('taskgroup.create', { label })
          await get().loadTaskGroups()
          return result
        } catch (err) {
          console.warn('Failed to create task group:', err)
          return null
        }
      },

      addTaskToGroup: async (groupId: string, taskId: string, projectDir: string) => {
        const client = get().client
        if (!client) return

        try {
          await client.call('taskgroup.add', { id: groupId, task_id: taskId, project_dir: projectDir, state: 'loaded' })
          await get().loadTaskGroups()
        } catch (err) {
          console.warn('Failed to add task to group:', err)
        }
      },

      submitTaskGroup: async (groupId: string) => {
        const client = get().client
        if (!client) return

        try {
          await client.call('taskgroup.submit', { id: groupId })
          await get().loadTaskGroups()
        } catch (err) {
          console.warn('Failed to submit task group:', err)
        }
      },

      getTaskGroupStatus: async (groupId: string): Promise<TaskGroup | null> => {
        const client = get().client
        if (!client) return null

        try {
          const result = await client.call<TaskGroup>('taskgroup.status', { id: groupId })
          return result
        } catch (err) {
          console.warn('Failed to get task group status:', err)
          return null
        }
      },

      removeTaskGroup: async (id: string) => {
        const client = get().client
        if (!client) return

        try {
          await client.call('taskgroup.remove', { id })
          await get().loadTaskGroups()
        } catch (err) {
          console.warn('Failed to remove task group:', err)
        }
      },
    }),
    {
      name: storeName('global'),
      // Project selection persisted via sessionStorage in selectProject() - survives refresh but not tab close
      partialize: () => ({}),
    },
  ),
)
