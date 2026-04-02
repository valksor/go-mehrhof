import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import {
  useGlobalStore,
  type AgentStatus,
  type Job,
  type MemoryResult,
  type TaskGroup,
  type TimedSnapshot,
} from './globalStore'
import type { WorktreeInfo, WorkerInfo } from '../types/socket'

// Mock SocketClient to avoid real WebSocket connections
vi.mock('../lib/socket', () => ({
  SocketClient: vi.fn().mockImplementation(() => ({
    connect: vi.fn().mockResolvedValue(undefined),
    close: vi.fn(),
    call: vi.fn().mockResolvedValue({}),
    subscribe: vi.fn(),
    setOnDisconnect: vi.fn(),
  }))
}))

const createMockProject = (overrides: Partial<WorktreeInfo> = {}): WorktreeInfo => ({
  id: 'proj-1',
  path: '/workspace/project',
  socket_path: '/tmp/proj.sock',
  state: 'none',
  ...overrides,
})

const createMockWorker = (overrides: Partial<WorkerInfo> = {}): WorkerInfo => ({
  id: 'worker-1',
  agent_name: 'claude',
  status: 'idle',
  is_default: false,
  ...overrides,
})

describe('globalStore', () => {
  beforeEach(() => {
    // Reset to initial state (preserve actions, using merge=false requires full state)
    useGlobalStore.setState({
      connected: false,
      connecting: false,
      reconnectAttempt: 0,
      reconnectTimeoutId: null,
      connectionVersion: 0,
      client: null,
      projects: [],
      workers: [],
      workerStats: null,
      memoryStats: null,
      jobs: [],
      taskGroups: [],
      metrics: null,
      metricsHistory: [],
      agentStatus: null,
      selectedProjectId: null,
      selectedProject: null,
      loading: false,
      error: null,
      activeTasks: [],
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  describe('initial state', () => {
    it('starts disconnected', () => {
      expect(useGlobalStore.getState().connected).toBe(false)
    })

    it('starts not connecting', () => {
      expect(useGlobalStore.getState().connecting).toBe(false)
    })

    it('has null client', () => {
      expect(useGlobalStore.getState().client).toBeNull()
    })

    it('has empty projects list', () => {
      expect(useGlobalStore.getState().projects).toEqual([])
    })

    it('has empty workers list', () => {
      expect(useGlobalStore.getState().workers).toEqual([])
    })

    it('has null workerStats', () => {
      expect(useGlobalStore.getState().workerStats).toBeNull()
    })

    it('has null memoryStats', () => {
      expect(useGlobalStore.getState().memoryStats).toBeNull()
    })

    it('has no selected project', () => {
      expect(useGlobalStore.getState().selectedProject).toBeNull()
      expect(useGlobalStore.getState().selectedProjectId).toBeNull()
    })

    it('starts not loading', () => {
      expect(useGlobalStore.getState().loading).toBe(false)
    })

    it('starts with no error', () => {
      expect(useGlobalStore.getState().error).toBeNull()
    })

    it('has null agentStatus', () => {
      expect(useGlobalStore.getState().agentStatus).toBeNull()
    })

    it('has empty activeTasks', () => {
      expect(useGlobalStore.getState().activeTasks).toEqual([])
    })

    it('has empty jobs list', () => {
      expect(useGlobalStore.getState().jobs).toEqual([])
    })

    it('has reconnectAttempt=0', () => {
      expect(useGlobalStore.getState().reconnectAttempt).toBe(0)
    })

    it('has connectionVersion=0', () => {
      expect(useGlobalStore.getState().connectionVersion).toBe(0)
    })
  })

  describe('selectProject', () => {
    it('sets selectedProject and selectedProjectId', () => {
      const project = createMockProject({ id: 'proj-42' })
      useGlobalStore.getState().selectProject(project)
      expect(useGlobalStore.getState().selectedProject).toEqual(project)
      expect(useGlobalStore.getState().selectedProjectId).toBe('proj-42')
    })

    it('clears selection when called with null', () => {
      useGlobalStore.setState({
        selectedProject: createMockProject(),
        selectedProjectId: 'proj-1',
      })
      useGlobalStore.getState().selectProject(null)
      expect(useGlobalStore.getState().selectedProject).toBeNull()
      expect(useGlobalStore.getState().selectedProjectId).toBeNull()
    })

    it('stores project id in sessionStorage', () => {
      const project = createMockProject({ id: 'proj-session' })
      useGlobalStore.getState().selectProject(project)
      // eslint-disable-next-line @typescript-eslint/unbound-method -- sessionStorage is mocked in vitest
      expect(sessionStorage.setItem).toHaveBeenCalledWith('kvelmo-selectedProjectId', 'proj-session')
    })

    it('removes from sessionStorage when cleared', () => {
      useGlobalStore.getState().selectProject(null)
      // eslint-disable-next-line @typescript-eslint/unbound-method -- sessionStorage is mocked in vitest
      expect(sessionStorage.removeItem).toHaveBeenCalledWith('kvelmo-selectedProjectId')
    })

    it('can switch between projects', () => {
      const p1 = createMockProject({ id: 'proj-1' })
      const p2 = createMockProject({ id: 'proj-2', path: '/other' })
      useGlobalStore.getState().selectProject(p1)
      expect(useGlobalStore.getState().selectedProjectId).toBe('proj-1')
      useGlobalStore.getState().selectProject(p2)
      expect(useGlobalStore.getState().selectedProjectId).toBe('proj-2')
      expect(useGlobalStore.getState().selectedProject).toEqual(p2)
    })
  })

  describe('state management via setState', () => {
    it('can set projects list', () => {
      const projects = [
        createMockProject({ id: 'p1' }),
        createMockProject({ id: 'p2', path: '/workspace/other' }),
      ]
      useGlobalStore.setState({ projects })
      expect(useGlobalStore.getState().projects).toEqual(projects)
    })

    it('can set workers list', () => {
      const workers = [
        createMockWorker({ id: 'w1' }),
        createMockWorker({ id: 'w2', agent_name: 'codex' }),
      ]
      useGlobalStore.setState({ workers })
      expect(useGlobalStore.getState().workers).toEqual(workers)
    })

    it('can set connected', () => {
      useGlobalStore.setState({ connected: true })
      expect(useGlobalStore.getState().connected).toBe(true)
    })

    it('can set error', () => {
      useGlobalStore.setState({ error: 'Connection failed' })
      expect(useGlobalStore.getState().error).toBe('Connection failed')
    })

    it('can set loading', () => {
      useGlobalStore.setState({ loading: true })
      expect(useGlobalStore.getState().loading).toBe(true)
    })

    it('can set agentStatus', () => {
      const status = {
        checks: [{ name: 'api-key', status: 'passed' as const }],
        agent_available: true,
        simulation_mode: false,
      }
      useGlobalStore.setState({ agentStatus: status })
      expect(useGlobalStore.getState().agentStatus).toEqual(status)
    })

    it('can set activeTasks', () => {
      const tasks = [
        { id: 't1', title: 'Fix bug', state: 'implementing', worktree_id: 'w1', source: 'gh:repo#1' },
      ]
      useGlobalStore.setState({ activeTasks: tasks as never })
      expect(useGlobalStore.getState().activeTasks).toHaveLength(1)
    })
  })

  describe('disconnect', () => {
    it('sets connected to false', () => {
      useGlobalStore.setState({ connected: true })
      useGlobalStore.getState().disconnect()
      expect(useGlobalStore.getState().connected).toBe(false)
    })

    it('sets connecting to false', () => {
      useGlobalStore.setState({ connecting: true })
      useGlobalStore.getState().disconnect()
      expect(useGlobalStore.getState().connecting).toBe(false)
    })

    it('resets reconnectAttempt to 0', () => {
      useGlobalStore.setState({ reconnectAttempt: 5 })
      useGlobalStore.getState().disconnect()
      expect(useGlobalStore.getState().reconnectAttempt).toBe(0)
    })

    it('sets client to null', () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useGlobalStore.setState({ client: { close: vi.fn() } as any })
      useGlobalStore.getState().disconnect()
      expect(useGlobalStore.getState().client).toBeNull()
    })

    it('clears pending reconnect timeout', () => {
      const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout')
      const id = setTimeout(() => {}, 999999)
      useGlobalStore.setState({ reconnectTimeoutId: id })
      useGlobalStore.getState().disconnect()
      expect(clearTimeoutSpy).toHaveBeenCalledWith(id)
      clearTimeoutSpy.mockRestore()
    })

    it('preserves projects list after disconnect', () => {
      const projects = [createMockProject()]
      useGlobalStore.setState({ projects })
      useGlobalStore.getState().disconnect()
      // disconnect only resets connection state, not data
      expect(useGlobalStore.getState().projects).toEqual(projects)
    })
  })

  describe('persistence', () => {
    it('uses kvelmo-global as storage key', () => {
      // Trigger a state change to exercise persist
      useGlobalStore.getState().selectProject(null)
      // eslint-disable-next-line @typescript-eslint/unbound-method -- localStorage is mocked in vitest
      expect(localStorage.setItem).toHaveBeenCalledWith(
        'kvelmo-global',
        expect.any(String)
      )
    })
  })

  describe('selectNextProject', () => {
    it('selects first project when none selected', () => {
      const projects = [
        createMockProject({ id: 'p1' }),
        createMockProject({ id: 'p2', path: '/b' }),
      ]
      useGlobalStore.setState({ projects, selectedProject: null })
      useGlobalStore.getState().selectNextProject()
      expect(useGlobalStore.getState().selectedProjectId).toBe('p1')
    })

    it('advances to next project', () => {
      const projects = [
        createMockProject({ id: 'p1' }),
        createMockProject({ id: 'p2', path: '/b' }),
        createMockProject({ id: 'p3', path: '/c' }),
      ]
      useGlobalStore.setState({ projects, selectedProject: projects[0], selectedProjectId: 'p1' })
      useGlobalStore.getState().selectNextProject()
      expect(useGlobalStore.getState().selectedProjectId).toBe('p2')
    })

    it('stays at last project when already at end', () => {
      const projects = [
        createMockProject({ id: 'p1' }),
        createMockProject({ id: 'p2', path: '/b' }),
      ]
      useGlobalStore.setState({ projects, selectedProject: projects[1], selectedProjectId: 'p2' })
      useGlobalStore.getState().selectNextProject()
      expect(useGlobalStore.getState().selectedProjectId).toBe('p2')
    })

    it('does nothing when projects list is empty', () => {
      useGlobalStore.getState().selectNextProject()
      expect(useGlobalStore.getState().selectedProject).toBeNull()
    })
  })

  describe('selectPrevProject', () => {
    it('selects last project when none selected', () => {
      const projects = [
        createMockProject({ id: 'p1' }),
        createMockProject({ id: 'p2', path: '/b' }),
      ]
      useGlobalStore.setState({ projects, selectedProject: null })
      useGlobalStore.getState().selectPrevProject()
      // When no project is selected, currentIdx defaults to projects.length, so prevIdx = length - 1
      expect(useGlobalStore.getState().selectedProjectId).toBe('p2')
    })

    it('moves to previous project', () => {
      const projects = [
        createMockProject({ id: 'p1' }),
        createMockProject({ id: 'p2', path: '/b' }),
        createMockProject({ id: 'p3', path: '/c' }),
      ]
      useGlobalStore.setState({ projects, selectedProject: projects[2], selectedProjectId: 'p3' })
      useGlobalStore.getState().selectPrevProject()
      expect(useGlobalStore.getState().selectedProjectId).toBe('p2')
    })

    it('stays at first project when already at beginning', () => {
      const projects = [
        createMockProject({ id: 'p1' }),
        createMockProject({ id: 'p2', path: '/b' }),
      ]
      useGlobalStore.setState({ projects, selectedProject: projects[0], selectedProjectId: 'p1' })
      useGlobalStore.getState().selectPrevProject()
      expect(useGlobalStore.getState().selectedProjectId).toBe('p1')
    })

    it('does nothing when projects list is empty', () => {
      useGlobalStore.getState().selectPrevProject()
      expect(useGlobalStore.getState().selectedProject).toBeNull()
    })
  })

  // ---------------------------------------------------------------------------
  // loadMetrics / loadMetricsHistory / loadHealth / batchAction
  // ---------------------------------------------------------------------------

  describe('loadMetrics', () => {
    it('sets metrics on success', async () => {
      const metrics = { jobs_submitted: 10, jobs_completed: 8, rpc_requests: 100 }
      const client = makeMockClient(() => metrics)
      injectClient(client)
      await useGlobalStore.getState().loadMetrics()
      expect(useGlobalStore.getState().metrics).toEqual(metrics)
    })

    it('does nothing when no client', async () => {
      await useGlobalStore.getState().loadMetrics()
      // No error, just silent
    })

    it('silently ignores errors', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('metrics unavailable'))
      injectClient(client)
      await expect(useGlobalStore.getState().loadMetrics()).resolves.not.toThrow()
    })
  })

  describe('loadHealth', () => {
    it('sets worktreeHealth on success', async () => {
      const worktrees = [{ id: 'wt1', path: '/a', state: 'none', healthy: true }]
      const client = makeMockClient(() => ({ worktrees }))
      injectClient(client)
      await useGlobalStore.getState().loadHealth()
      expect(useGlobalStore.getState().worktreeHealth).toEqual(worktrees)
    })

    it('defaults to empty array when key missing', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().loadHealth()
      expect(useGlobalStore.getState().worktreeHealth).toEqual([])
    })

    it('does nothing when no client', async () => {
      const before = useGlobalStore.getState().worktreeHealth
      await useGlobalStore.getState().loadHealth()
      expect(useGlobalStore.getState().worktreeHealth).toEqual(before)
    })
  })

  describe('batchAction', () => {
    it('calls tasks.batch with action', async () => {
      const client = makeMockClient((method) => {
        if (method === 'tasks.batch') return { total: 2, results: [{ path: '/a', state: 'none', success: true }] }
        return { tasks: [] }
      })
      injectClient(client)
      const result = await useGlobalStore.getState().batchAction('abort')
      expect(client.call).toHaveBeenCalledWith('tasks.batch', { action: 'abort' })
      expect(result.total).toBe(2)
      expect(result.succeeded).toBe(1)
    })

    it('includes non-empty filters', async () => {
      const client = makeMockClient((method) => {
        if (method === 'tasks.batch') return { total: 0, results: [] }
        return { tasks: [] }
      })
      injectClient(client)
      await useGlobalStore.getState().batchAction('stop', { state: 'implementing', tag: 'urgent' })
      expect(client.call).toHaveBeenCalledWith('tasks.batch', {
        action: 'stop',
        filter: { state: 'implementing', tag: 'urgent' },
      })
    })

    it('returns empty results when no client', async () => {
      const result = await useGlobalStore.getState().batchAction('stop')
      expect(result).toEqual({ total: 0, succeeded: 0, results: [] })
    })
  })

  // ---------------------------------------------------------------------------
  // Helpers for async method tests
  // ---------------------------------------------------------------------------

  const makeMockClient = (callImpl?: (method: string) => unknown) => ({
    call: vi.fn().mockImplementation((method: string) => {
      if (callImpl) return Promise.resolve(callImpl(method))
      return Promise.resolve({})
    }),
    subscribe: vi.fn(),
    connect: vi.fn().mockResolvedValue(undefined),
    close: vi.fn(),
    setOnDisconnect: vi.fn(),
  })

  const injectClient = (client: ReturnType<typeof makeMockClient>) => {
    useGlobalStore.setState({ client: client as never, connected: true })
  }

  // Creates a mutable handler ref that works with TypeScript's control flow analysis
  type EventHandler = (msg: unknown) => void
  const createSubscriberRef = () => {
    const ref: { current: EventHandler | null } = { current: null }
    return ref
  }

  // ---------------------------------------------------------------------------
  // loadProjects
  // ---------------------------------------------------------------------------

  describe('loadProjects', () => {
    it('sets projects on success', async () => {
      const projects = [createMockProject({ id: 'p1' }), createMockProject({ id: 'p2', path: '/b' })]
      const client = makeMockClient(() => ({ projects }))
      injectClient(client)
      await useGlobalStore.getState().loadProjects()
      expect(useGlobalStore.getState().projects).toEqual(projects)
      expect(useGlobalStore.getState().loading).toBe(false)
    })

    it('defaults to empty array when response has no projects key', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().loadProjects()
      expect(useGlobalStore.getState().projects).toEqual([])
    })

    it('sets error when client.call rejects', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('network error'))
      injectClient(client)
      await useGlobalStore.getState().loadProjects()
      expect(useGlobalStore.getState().error).toBe('network error')
      expect(useGlobalStore.getState().loading).toBe(false)
    })

    it('sets error when no client', async () => {
      await useGlobalStore.getState().loadProjects()
      expect(useGlobalStore.getState().error).toBe('Not connected')
    })

    it('calls projects.list RPC method', async () => {
      const client = makeMockClient(() => ({ projects: [] }))
      injectClient(client)
      await useGlobalStore.getState().loadProjects()
      expect(client.call).toHaveBeenCalledWith('projects.list')
    })
  })

  // ---------------------------------------------------------------------------
  // addProject
  // ---------------------------------------------------------------------------

  describe('addProject', () => {
    it('calls projects.register with path', async () => {
      const client = makeMockClient(() => ({ projects: [] }))
      injectClient(client)
      await useGlobalStore.getState().addProject('/new/path')
      expect(client.call).toHaveBeenCalledWith('projects.register', { path: '/new/path' })
    })

    it('reloads projects after adding', async () => {
      const newProject = createMockProject({ id: 'new-proj' })
      const client = makeMockClient(() => ({ projects: [newProject] }))
      injectClient(client)
      await useGlobalStore.getState().addProject('/new/path')
      expect(useGlobalStore.getState().projects).toEqual([newProject])
    })

    it('sets error when no client', async () => {
      await useGlobalStore.getState().addProject('/some/path')
      expect(useGlobalStore.getState().error).toBe('Not connected')
    })

    it('sets error when call rejects', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('permission denied'))
      injectClient(client)
      await useGlobalStore.getState().addProject('/bad/path')
      expect(useGlobalStore.getState().error).toBe('permission denied')
    })
  })

  // ---------------------------------------------------------------------------
  // removeProject
  // ---------------------------------------------------------------------------

  describe('removeProject', () => {
    it('calls projects.unregister with id', async () => {
      const client = makeMockClient(() => ({ projects: [] }))
      injectClient(client)
      await useGlobalStore.getState().removeProject('proj-1')
      expect(client.call).toHaveBeenCalledWith('projects.unregister', { id: 'proj-1' })
    })

    it('clears selection when removing selected project', async () => {
      const client = makeMockClient(() => ({ projects: [] }))
      injectClient(client)
      useGlobalStore.setState({ selectedProjectId: 'proj-1', selectedProject: createMockProject() })
      await useGlobalStore.getState().removeProject('proj-1')
      expect(useGlobalStore.getState().selectedProjectId).toBeNull()
      expect(useGlobalStore.getState().selectedProject).toBeNull()
    })

    it('preserves selection when removing a different project', async () => {
      const client = makeMockClient(() => ({ projects: [] }))
      injectClient(client)
      const selected = createMockProject({ id: 'proj-kept' })
      useGlobalStore.setState({ selectedProjectId: 'proj-kept', selectedProject: selected })
      await useGlobalStore.getState().removeProject('proj-other')
      expect(useGlobalStore.getState().selectedProjectId).toBe('proj-kept')
    })

    it('sets error when no client', async () => {
      await useGlobalStore.getState().removeProject('proj-1')
      expect(useGlobalStore.getState().error).toBe('Not connected')
    })

    it('sets error when call rejects', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('not found'))
      injectClient(client)
      await useGlobalStore.getState().removeProject('proj-1')
      expect(useGlobalStore.getState().error).toBe('not found')
    })
  })

  // ---------------------------------------------------------------------------
  // loadWorkers
  // ---------------------------------------------------------------------------

  describe('loadWorkers', () => {
    it('sets workers and workerStats on success', async () => {
      const workers = [createMockWorker({ id: 'w1' })]
      const stats = { total: 1, idle: 1, busy: 0, failed: 0 }
      const client = makeMockClient(() => ({ workers, stats }))
      injectClient(client)
      await useGlobalStore.getState().loadWorkers()
      expect(useGlobalStore.getState().workers).toEqual(workers)
      expect(useGlobalStore.getState().workerStats).toEqual(stats)
    })

    it('defaults workers to empty array when missing', async () => {
      const client = makeMockClient(() => ({ stats: null }))
      injectClient(client)
      await useGlobalStore.getState().loadWorkers()
      expect(useGlobalStore.getState().workers).toEqual([])
    })

    it('does nothing when no client', async () => {
      await useGlobalStore.getState().loadWorkers()
      // No error set, just a no-op
      expect(useGlobalStore.getState().workers).toEqual([])
    })

    it('does not throw when call rejects', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('server error'))
      injectClient(client)
      await expect(useGlobalStore.getState().loadWorkers()).resolves.not.toThrow()
      warnSpy.mockRestore()
    })

    it('calls workers.list RPC method', async () => {
      const client = makeMockClient(() => ({ workers: [], stats: null }))
      injectClient(client)
      await useGlobalStore.getState().loadWorkers()
      expect(client.call).toHaveBeenCalledWith('workers.list')
    })
  })

  // ---------------------------------------------------------------------------
  // loadWorkerStats
  // ---------------------------------------------------------------------------

  describe('loadWorkerStats', () => {
    it('sets workerStats on success', async () => {
      const stats = { total: 3, idle: 2, busy: 1, failed: 0 }
      const client = makeMockClient(() => stats)
      injectClient(client)
      await useGlobalStore.getState().loadWorkerStats()
      expect(useGlobalStore.getState().workerStats).toEqual(stats)
    })

    it('does nothing when no client', async () => {
      await useGlobalStore.getState().loadWorkerStats()
      expect(useGlobalStore.getState().workerStats).toBeNull()
    })

    it('calls workers.stats RPC method', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().loadWorkerStats()
      expect(client.call).toHaveBeenCalledWith('workers.stats', {})
    })
  })

  // ---------------------------------------------------------------------------
  // addWorker
  // ---------------------------------------------------------------------------

  describe('addWorker', () => {
    it('calls workers.add with agent type', async () => {
      const client = makeMockClient(() => ({ workers: [] }))
      injectClient(client)
      await useGlobalStore.getState().addWorker('claude')
      expect(client.call).toHaveBeenCalledWith('workers.add', { agent: 'claude' })
    })

    it('reloads workers after adding', async () => {
      const workers = [createMockWorker({ id: 'w-new', agent_name: 'codex' })]
      const client = makeMockClient(() => ({ workers, stats: null }))
      injectClient(client)
      await useGlobalStore.getState().addWorker('codex')
      expect(useGlobalStore.getState().workers).toEqual(workers)
    })

    it('sets loading false after success', async () => {
      const client = makeMockClient(() => ({ workers: [] }))
      injectClient(client)
      await useGlobalStore.getState().addWorker('claude')
      expect(useGlobalStore.getState().loading).toBe(false)
    })

    it('sets error when no client', async () => {
      await useGlobalStore.getState().addWorker('claude')
      expect(useGlobalStore.getState().error).toBe('Not connected')
    })

    it('sets error when call rejects', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('quota exceeded'))
      injectClient(client)
      await useGlobalStore.getState().addWorker('claude')
      expect(useGlobalStore.getState().error).toBe('quota exceeded')
      expect(useGlobalStore.getState().loading).toBe(false)
    })
  })

  // ---------------------------------------------------------------------------
  // removeWorker
  // ---------------------------------------------------------------------------

  describe('removeWorker', () => {
    it('calls workers.remove with id', async () => {
      const client = makeMockClient(() => ({ workers: [] }))
      injectClient(client)
      await useGlobalStore.getState().removeWorker('worker-1')
      expect(client.call).toHaveBeenCalledWith('workers.remove', { id: 'worker-1' })
    })

    it('reloads workers after removing', async () => {
      const client = makeMockClient(() => ({ workers: [], stats: null }))
      injectClient(client)
      useGlobalStore.setState({ workers: [createMockWorker()] })
      await useGlobalStore.getState().removeWorker('worker-1')
      expect(useGlobalStore.getState().workers).toEqual([])
    })

    it('sets error when no client', async () => {
      await useGlobalStore.getState().removeWorker('w1')
      expect(useGlobalStore.getState().error).toBe('Not connected')
    })

    it('sets error when call rejects', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('worker not found'))
      injectClient(client)
      await useGlobalStore.getState().removeWorker('w1')
      expect(useGlobalStore.getState().error).toBe('worker not found')
    })
  })

  // ---------------------------------------------------------------------------
  // stopChat
  // ---------------------------------------------------------------------------

  describe('stopChat', () => {
    it('calls chat.stop with worktree_id', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().stopChat('wt-1')
      expect(client.call).toHaveBeenCalledWith('chat.stop', { worktree_id: 'wt-1' })
    })

    it('includes job_id when provided', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().stopChat('wt-1', 'job-42')
      expect(client.call).toHaveBeenCalledWith('chat.stop', { worktree_id: 'wt-1', job_id: 'job-42' })
    })

    it('omits job_id when not provided', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().stopChat('wt-2')
      const callArg = client.call.mock.calls[0][1] as Record<string, string>
      expect(callArg).not.toHaveProperty('job_id')
    })

    it('sets error when no client', async () => {
      await useGlobalStore.getState().stopChat('wt-1')
      expect(useGlobalStore.getState().error).toBe('Not connected')
    })

    it('sets error when call rejects', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('not running'))
      injectClient(client)
      await useGlobalStore.getState().stopChat('wt-1')
      expect(useGlobalStore.getState().error).toBe('not running')
    })
  })

  // ---------------------------------------------------------------------------
  // loadAgentStatus
  // ---------------------------------------------------------------------------

  describe('loadAgentStatus', () => {
    it('sets agentStatus on success', async () => {
      const status: AgentStatus = {
        checks: [{ name: 'api-key', status: 'passed' }],
        agent_available: true,
        simulation_mode: false,
      }
      const client = makeMockClient(() => status)
      injectClient(client)
      await useGlobalStore.getState().loadAgentStatus()
      expect(useGlobalStore.getState().agentStatus).toEqual(status)
    })

    it('does nothing when no client', async () => {
      await useGlobalStore.getState().loadAgentStatus()
      expect(useGlobalStore.getState().agentStatus).toBeNull()
    })

    it('does not throw when call rejects', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('agent unavailable'))
      injectClient(client)
      await expect(useGlobalStore.getState().loadAgentStatus()).resolves.not.toThrow()
      warnSpy.mockRestore()
    })

    it('calls agent.status RPC method', async () => {
      const client = makeMockClient(() => ({ checks: [], agent_available: false, simulation_mode: true }))
      injectClient(client)
      await useGlobalStore.getState().loadAgentStatus()
      expect(client.call).toHaveBeenCalledWith('agent.status')
    })

    it('stores simulation_mode flag', async () => {
      const client = makeMockClient(() => ({
        checks: [],
        agent_available: false,
        simulation_mode: true,
      }))
      injectClient(client)
      await useGlobalStore.getState().loadAgentStatus()
      expect(useGlobalStore.getState().agentStatus?.simulation_mode).toBe(true)
    })
  })

  // ---------------------------------------------------------------------------
  // loadActiveTasks
  // ---------------------------------------------------------------------------

  describe('loadActiveTasks', () => {
    it('sets activeTasks on success', async () => {
      const tasks = [
        { id: 't1', title: 'Fix bug', state: 'implementing', worktree_id: 'w1', source: 'gh:repo#1' },
      ]
      const client = makeMockClient(() => ({ tasks }))
      injectClient(client)
      await useGlobalStore.getState().loadActiveTasks()
      expect(useGlobalStore.getState().activeTasks).toHaveLength(1)
      expect(useGlobalStore.getState().activeTasks[0].id).toBe('t1')
    })

    it('defaults to empty array when tasks key missing', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().loadActiveTasks()
      expect(useGlobalStore.getState().activeTasks).toEqual([])
    })

    it('does nothing when no client', async () => {
      await useGlobalStore.getState().loadActiveTasks()
      expect(useGlobalStore.getState().activeTasks).toEqual([])
    })

    it('does not throw when call rejects', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('tasks unavailable'))
      injectClient(client)
      await expect(useGlobalStore.getState().loadActiveTasks()).resolves.not.toThrow()
      warnSpy.mockRestore()
    })

    it('calls tasks.list RPC method', async () => {
      const client = makeMockClient(() => ({ tasks: [] }))
      injectClient(client)
      await useGlobalStore.getState().loadActiveTasks()
      expect(client.call).toHaveBeenCalledWith('tasks.list')
    })
  })

  // ---------------------------------------------------------------------------
  // searchMemory
  // ---------------------------------------------------------------------------

  describe('searchMemory', () => {
    it('returns results on success', async () => {
      const results: MemoryResult[] = [
        { id: 'm1', type: 'note', content: 'test', score: 0.9, task_id: 't1', created_at: '2026-01-01' },
      ]
      const client = makeMockClient(() => ({ results }))
      injectClient(client)
      const out = await useGlobalStore.getState().searchMemory('test query')
      expect(out).toEqual(results)
    })

    it('returns empty array when results missing', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      const out = await useGlobalStore.getState().searchMemory('q')
      expect(out).toEqual([])
    })

    it('returns empty array when no client', async () => {
      const out = await useGlobalStore.getState().searchMemory('q')
      expect(out).toEqual([])
    })

    it('returns empty array when call rejects', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('search failed'))
      injectClient(client)
      const out = await useGlobalStore.getState().searchMemory('q')
      expect(out).toEqual([])
      warnSpy.mockRestore()
    })

    it('calls memory.search with query and default limit', async () => {
      const client = makeMockClient(() => ({ results: [] }))
      injectClient(client)
      await useGlobalStore.getState().searchMemory('needle')
      expect(client.call).toHaveBeenCalledWith('memory.search', { query: 'needle', limit: 10 })
    })

    it('respects custom limit', async () => {
      const client = makeMockClient(() => ({ results: [] }))
      injectClient(client)
      await useGlobalStore.getState().searchMemory('q', 25)
      expect(client.call).toHaveBeenCalledWith('memory.search', { query: 'q', limit: 25 })
    })
  })

  // ---------------------------------------------------------------------------
  // loadMemoryStats
  // ---------------------------------------------------------------------------

  describe('loadMemoryStats', () => {
    it('sets memoryStats on success', async () => {
      const stats = { total: 42, size_bytes: 1024 }
      const client = makeMockClient(() => stats)
      injectClient(client)
      await useGlobalStore.getState().loadMemoryStats()
      expect(useGlobalStore.getState().memoryStats).toEqual(stats)
    })

    it('does nothing when no client', async () => {
      await useGlobalStore.getState().loadMemoryStats()
      expect(useGlobalStore.getState().memoryStats).toBeNull()
    })

    it('calls memory.stats RPC method', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().loadMemoryStats()
      expect(client.call).toHaveBeenCalledWith('memory.stats', {})
    })
  })

  // ---------------------------------------------------------------------------
  // clearMemory
  // ---------------------------------------------------------------------------

  describe('clearMemory', () => {
    it('calls memory.clear RPC method', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().clearMemory()
      expect(client.call).toHaveBeenCalledWith('memory.clear', {})
    })

    it('sets memoryStats to null after clearing', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      useGlobalStore.setState({ memoryStats: { total: 10, size_bytes: 512 } as never })
      await useGlobalStore.getState().clearMemory()
      expect(useGlobalStore.getState().memoryStats).toBeNull()
    })

    it('does nothing when no client', async () => {
      await useGlobalStore.getState().clearMemory()
      // Should be silent
      expect(useGlobalStore.getState().memoryStats).toBeNull()
    })

    it('does not throw when call rejects', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('memory locked'))
      injectClient(client)
      await expect(useGlobalStore.getState().clearMemory()).resolves.not.toThrow()
      warnSpy.mockRestore()
    })
  })

  // ---------------------------------------------------------------------------
  // loadJobs
  // ---------------------------------------------------------------------------

  describe('loadJobs', () => {
    it('sets jobs on success', async () => {
      const jobs: Job[] = [
        { id: 'j1', type: 'chat', status: 'running', worktree_id: 'wt1', created_at: '2026-01-01' },
      ]
      const client = makeMockClient(() => ({ jobs }))
      injectClient(client)
      await useGlobalStore.getState().loadJobs()
      expect(useGlobalStore.getState().jobs).toEqual(jobs)
    })

    it('defaults jobs to empty array when key missing', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().loadJobs()
      expect(useGlobalStore.getState().jobs).toEqual([])
    })

    it('does nothing when no client', async () => {
      await useGlobalStore.getState().loadJobs()
      expect(useGlobalStore.getState().jobs).toEqual([])
    })

    it('calls jobs.list RPC method', async () => {
      const client = makeMockClient(() => ({ jobs: [] }))
      injectClient(client)
      await useGlobalStore.getState().loadJobs()
      expect(client.call).toHaveBeenCalledWith('jobs.list', {})
    })

    it('does not throw when call rejects', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('jobs unavailable'))
      injectClient(client)
      await expect(useGlobalStore.getState().loadJobs()).resolves.not.toThrow()
      warnSpy.mockRestore()
    })
  })

  // ---------------------------------------------------------------------------
  // loadJob
  // ---------------------------------------------------------------------------

  describe('loadJob', () => {
    it('returns the job on success', async () => {
      const job: Job = { id: 'j1', type: 'chat', status: 'done', worktree_id: 'wt1', created_at: '2026-01-01' }
      const client = makeMockClient(() => job)
      injectClient(client)
      const result = await useGlobalStore.getState().loadJob('j1')
      expect(result).toEqual(job)
    })

    it('calls jobs.get with id', async () => {
      const job: Job = { id: 'j1', type: 'chat', status: 'done', worktree_id: 'wt1', created_at: '2026-01-01' }
      const client = makeMockClient(() => job)
      injectClient(client)
      await useGlobalStore.getState().loadJob('j1')
      expect(client.call).toHaveBeenCalledWith('jobs.get', { id: 'j1' })
    })

    it('returns null when no client', async () => {
      const result = await useGlobalStore.getState().loadJob('j1')
      expect(result).toBeNull()
    })

    it('returns null when call rejects', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('not found'))
      injectClient(client)
      const result = await useGlobalStore.getState().loadJob('missing')
      expect(result).toBeNull()
      warnSpy.mockRestore()
    })
  })

  // ---------------------------------------------------------------------------
  // loadMetricsHistory
  // ---------------------------------------------------------------------------

  describe('loadMetricsHistory', () => {
    it('sets metricsHistory when enabled is true', async () => {
      const entries: TimedSnapshot[] = [
        { timestamp: '2026-01-01T00:00:00Z', jobs_submitted: 5, jobs_completed: 3, jobs_failed: 1, jobs_in_progress: 1, rpc_requests: 20, rpc_errors: 0, agent_connects: 2, events_dropped: 0 },
        { timestamp: '2026-01-01T01:00:00Z', jobs_submitted: 8, jobs_completed: 6, jobs_failed: 1, jobs_in_progress: 1, rpc_requests: 35, rpc_errors: 1, agent_connects: 3, events_dropped: 0 },
      ]
      const client = makeMockClient(() => ({ entries, enabled: true }))
      injectClient(client)
      await useGlobalStore.getState().loadMetricsHistory()
      expect(client.call).toHaveBeenCalledWith('metrics.history', {})
      expect(useGlobalStore.getState().metricsHistory).toEqual(entries)
    })

    it('does not set metricsHistory when enabled is false', async () => {
      const entries: TimedSnapshot[] = [
        { timestamp: '2026-01-01T00:00:00Z', jobs_submitted: 5, jobs_completed: 3, jobs_failed: 1, jobs_in_progress: 1, rpc_requests: 20, rpc_errors: 0, agent_connects: 2, events_dropped: 0 },
      ]
      const client = makeMockClient(() => ({ entries, enabled: false }))
      injectClient(client)
      await useGlobalStore.getState().loadMetricsHistory()
      expect(client.call).toHaveBeenCalledWith('metrics.history', {})
      expect(useGlobalStore.getState().metricsHistory).toEqual([])
    })

    it('does nothing without a client', async () => {
      await useGlobalStore.getState().loadMetricsHistory()
      expect(useGlobalStore.getState().metricsHistory).toEqual([])
    })

    it('silently handles errors', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('metrics unavailable'))
      injectClient(client)
      await useGlobalStore.getState().loadMetricsHistory()
      expect(useGlobalStore.getState().metricsHistory).toEqual([])
    })
  })

  // ---------------------------------------------------------------------------
  // loadTaskGroups
  // ---------------------------------------------------------------------------

  describe('loadTaskGroups', () => {
    const sampleGroup: TaskGroup = {
      id: 'tg-1',
      label: 'Release v2',
      tasks: [{ project_dir: '/proj', task_id: 'task-1', state: 'loaded' }],
      status: 'pending',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    }

    it('sets taskGroups on success', async () => {
      const client = makeMockClient(() => ({ groups: [sampleGroup] }))
      injectClient(client)
      await useGlobalStore.getState().loadTaskGroups()
      expect(client.call).toHaveBeenCalledWith('taskgroup.list', {})
      expect(useGlobalStore.getState().taskGroups).toEqual([sampleGroup])
    })

    it('defaults to empty array when groups is missing', async () => {
      const client = makeMockClient(() => ({}))
      injectClient(client)
      await useGlobalStore.getState().loadTaskGroups()
      expect(useGlobalStore.getState().taskGroups).toEqual([])
    })

    it('does nothing without a client', async () => {
      await useGlobalStore.getState().loadTaskGroups()
      expect(useGlobalStore.getState().taskGroups).toEqual([])
    })

    it('handles errors gracefully', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('rpc fail'))
      injectClient(client)
      await useGlobalStore.getState().loadTaskGroups()
      expect(useGlobalStore.getState().taskGroups).toEqual([])
      warnSpy.mockRestore()
    })
  })

  // ---------------------------------------------------------------------------
  // createTaskGroup
  // ---------------------------------------------------------------------------

  describe('createTaskGroup', () => {
    const createdGroup: TaskGroup = {
      id: 'tg-new',
      label: 'New Group',
      tasks: [],
      status: 'pending',
      created_at: '2026-04-01T00:00:00Z',
      updated_at: '2026-04-01T00:00:00Z',
    }

    it('calls taskgroup.create and reloads groups', async () => {
      const client = makeMockClient((method: string) => {
        if (method === 'taskgroup.create') return createdGroup
        if (method === 'taskgroup.list') return { groups: [createdGroup] }
        return {}
      })
      injectClient(client)
      const result = await useGlobalStore.getState().createTaskGroup('New Group')
      expect(client.call).toHaveBeenCalledWith('taskgroup.create', { label: 'New Group' })
      expect(result).toEqual(createdGroup)
      expect(useGlobalStore.getState().taskGroups).toEqual([createdGroup])
    })

    it('returns null without a client', async () => {
      const result = await useGlobalStore.getState().createTaskGroup('No Client')
      expect(result).toBeNull()
    })

    it('returns null on error', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('create failed'))
      injectClient(client)
      const result = await useGlobalStore.getState().createTaskGroup('Fail')
      expect(result).toBeNull()
      warnSpy.mockRestore()
    })
  })

  // ---------------------------------------------------------------------------
  // addTaskToGroup
  // ---------------------------------------------------------------------------

  describe('addTaskToGroup', () => {
    it('calls taskgroup.add with correct params and reloads', async () => {
      const client = makeMockClient((method: string) => {
        if (method === 'taskgroup.list') return { groups: [] }
        return {}
      })
      injectClient(client)
      await useGlobalStore.getState().addTaskToGroup('tg-1', 'task-42', '/projects/foo')
      expect(client.call).toHaveBeenCalledWith('taskgroup.add', {
        id: 'tg-1',
        task_id: 'task-42',
        project_dir: '/projects/foo',
        state: 'loaded',
      })
      // Should also call loadTaskGroups
      expect(client.call).toHaveBeenCalledWith('taskgroup.list', {})
    })

    it('does nothing without a client', async () => {
      await useGlobalStore.getState().addTaskToGroup('tg-1', 'task-1', '/proj')
      expect(useGlobalStore.getState().taskGroups).toEqual([])
    })

    it('handles errors gracefully', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('add failed'))
      injectClient(client)
      await useGlobalStore.getState().addTaskToGroup('tg-1', 'task-1', '/proj')
      warnSpy.mockRestore()
    })
  })

  // ---------------------------------------------------------------------------
  // submitTaskGroup
  // ---------------------------------------------------------------------------

  describe('submitTaskGroup', () => {
    it('calls taskgroup.submit and reloads groups', async () => {
      const client = makeMockClient((method: string) => {
        if (method === 'taskgroup.list') return { groups: [] }
        return {}
      })
      injectClient(client)
      await useGlobalStore.getState().submitTaskGroup('tg-1')
      expect(client.call).toHaveBeenCalledWith('taskgroup.submit', { id: 'tg-1' })
      expect(client.call).toHaveBeenCalledWith('taskgroup.list', {})
    })

    it('does nothing without a client', async () => {
      await useGlobalStore.getState().submitTaskGroup('tg-1')
      expect(useGlobalStore.getState().taskGroups).toEqual([])
    })

    it('handles errors gracefully', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('submit failed'))
      injectClient(client)
      await useGlobalStore.getState().submitTaskGroup('tg-1')
      warnSpy.mockRestore()
    })
  })

  // ---------------------------------------------------------------------------
  // getTaskGroupStatus
  // ---------------------------------------------------------------------------

  describe('getTaskGroupStatus', () => {
    const statusGroup: TaskGroup = {
      id: 'tg-1',
      label: 'Status Group',
      tasks: [{ project_dir: '/proj', task_id: 'task-1', state: 'implementing' }],
      status: 'in_progress',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-04-01T00:00:00Z',
    }

    it('returns task group status', async () => {
      const client = makeMockClient(() => statusGroup)
      injectClient(client)
      const result = await useGlobalStore.getState().getTaskGroupStatus('tg-1')
      expect(client.call).toHaveBeenCalledWith('taskgroup.status', { id: 'tg-1' })
      expect(result).toEqual(statusGroup)
    })

    it('returns null without a client', async () => {
      const result = await useGlobalStore.getState().getTaskGroupStatus('tg-1')
      expect(result).toBeNull()
    })

    it('returns null on error', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('status failed'))
      injectClient(client)
      const result = await useGlobalStore.getState().getTaskGroupStatus('tg-1')
      expect(result).toBeNull()
      warnSpy.mockRestore()
    })
  })

  // ---------------------------------------------------------------------------
  // removeTaskGroup
  // ---------------------------------------------------------------------------

  describe('removeTaskGroup', () => {
    it('calls taskgroup.remove and reloads groups', async () => {
      const client = makeMockClient((method: string) => {
        if (method === 'taskgroup.list') return { groups: [] }
        return {}
      })
      injectClient(client)
      await useGlobalStore.getState().removeTaskGroup('tg-1')
      expect(client.call).toHaveBeenCalledWith('taskgroup.remove', { id: 'tg-1' })
      expect(client.call).toHaveBeenCalledWith('taskgroup.list', {})
    })

    it('does nothing without a client', async () => {
      await useGlobalStore.getState().removeTaskGroup('tg-1')
      expect(useGlobalStore.getState().taskGroups).toEqual([])
    })

    it('handles errors gracefully', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValueOnce(new Error('remove failed'))
      injectClient(client)
      await useGlobalStore.getState().removeTaskGroup('tg-1')
      warnSpy.mockRestore()
    })
  })

  // ---------------------------------------------------------------------------
  // notification dispatch: task_state_changed triggers loadActiveTasks
  // ---------------------------------------------------------------------------

  describe('notification dispatch', () => {
    it('task_state_changed notification triggers loadActiveTasks', async () => {
      // Capture the subscriber registered on the mock client by wiring the mock
      // client directly so we can drive the subscribe callback without going
      // through the real connect() flow (which would try to construct SocketClient).
      const subscriberRef = createSubscriberRef()

      const client = makeMockClient()
      client.subscribe.mockImplementationOnce((handler: (msg: unknown) => void) => {
        subscriberRef.current = handler
      })
      // Simulate the subscribe call that connect() would register
      client.subscribe((msg: unknown) => {
        const notification = msg as { method?: string }
        if (notification.method === 'task_state_changed') {
          void useGlobalStore.getState().loadActiveTasks()
        }
      })

      // Inject a client with loadActiveTasks that we can observe
      const tasks = [{ id: 'tsk-1', title: 'task', state: 'implementing', worktree_id: 'w1', source: 'gh:r#1' }]
      client.call.mockResolvedValue({ tasks })
      injectClient(client)

      expect(subscriberRef.current).not.toBeNull()
      if (subscriberRef.current) {
        subscriberRef.current({ method: 'task_state_changed', params: {} })
        // Allow the microtask queue to flush
        await Promise.resolve()
        expect(useGlobalStore.getState().activeTasks).toHaveLength(1)
      }
    })

    it('unrelated notification method does not trigger loadActiveTasks', async () => {
      const subscriberRef = createSubscriberRef()

      const client = makeMockClient()
      client.subscribe.mockImplementationOnce((handler: (msg: unknown) => void) => {
        subscriberRef.current = handler
      })
      client.subscribe((msg: unknown) => {
        const notification = msg as { method?: string }
        if (notification.method === 'task_state_changed') {
          void useGlobalStore.getState().loadActiveTasks()
        }
      })
      injectClient(client)

      if (subscriberRef.current) {
        subscriberRef.current({ method: 'other_event', params: {} })
        await Promise.resolve()
        // call should not have been invoked for tasks.list
        const taskCalls = (client.call as ReturnType<typeof vi.fn>).mock.calls.filter(
          c => c[0] === 'tasks.list'
        )
        expect(taskCalls).toHaveLength(0)
      }
    })
  })
})
