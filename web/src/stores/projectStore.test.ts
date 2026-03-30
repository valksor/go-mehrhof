import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useProjectStore } from './projectStore'

// Track the last subscribe callback registered by any SocketClient instance
let _capturedSubscribeCallback: ((data: unknown) => void) | null = null

// Mock SocketClient to avoid real WebSocket.
// Must use `function` (not arrow) so vitest allows `new MockSocketClient(...)`.
vi.mock('../lib/socket', () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const MockSocketClient = vi.fn(function MockSocketClient(this: any) {
    this.connect = vi.fn().mockResolvedValue(undefined)
    this.close = vi.fn()
    // Default call implementation handles all RPC methods used during connect()
    this.call = vi.fn().mockImplementation((method: string) => {
      if (method === 'stream.subscribe') return Promise.resolve({})
      if (method === 'status') return Promise.resolve({ state: 'none', path: '/' })
      if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
      if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
      if (method === 'review.list') return Promise.resolve({ reviews: [] })
      if (method === 'queue.list') return Promise.resolve({ queue: [], count: 0 })
      return Promise.resolve({})
    })
    this.subscribe = vi.fn().mockImplementation((cb: (data: unknown) => void) => {
      _capturedSubscribeCallback = cb
      // Return unsubscribe function
      return () => { _capturedSubscribeCallback = null }
    })
    this.setOnDisconnect = vi.fn()
  })
  return { SocketClient: MockSocketClient }
})

// Mock notify to avoid Notification API
vi.mock('../lib/notify', () => ({
  sendNotification: vi.fn(),
  requestNotificationPermission: vi.fn(),
}))

// Mock screenshotStore to avoid side effects
vi.mock('./screenshotStore', () => ({
  useScreenshotStore: {
    getState: vi.fn().mockReturnValue({
      handleScreenshotCaptured: vi.fn(),
      handleScreenshotDeleted: vi.fn(),
      load: vi.fn().mockResolvedValue(undefined),
    })
  }
}))

const initialState = {
  connected: false,
  connecting: false,
  reconnectAttempt: 0,
  reconnectTimeoutId: null,
  connectionVersion: 0,
  worktreeId: null,
  client: null,
  task: null,
  state: 'none' as const,
  output: [] as string[],
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
}

// Helper to create a fully typed mock SocketClient
function makeMockClient() {
  return {
    call: vi.fn(),
    subscribe: vi.fn().mockReturnValue(() => {}), // Return unsubscribe function
    connect: vi.fn().mockResolvedValue(undefined),
    close: vi.fn(),
    setOnDisconnect: vi.fn(),
  }
}

// refreshStatus calls client.call('status'), client.call('checkpoints'),
// and then refreshGitStatus (git.status) and loadReviews (review.list).
// This helper returns a mock that handles all of those so tests that trigger
// refreshStatus don't need to wire every response manually.
function makeClientWithAutoRefresh(overrides: Record<string, Record<string, unknown>> = {}) {
  const client = makeMockClient()
  client.call.mockImplementation((method: string) => {
    if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj', ...overrides.status })
    if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [], ...overrides.checkpoints })
    if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [], ...overrides['git.status'] })
    if (method === 'review.list') return Promise.resolve({ reviews: [], ...overrides['review.list'] })
    return Promise.resolve({})
  })
  return client
}

describe('projectStore', () => {
  beforeEach(() => {
    // Reset to initial state (preserve actions)
    useProjectStore.setState(initialState)
    _capturedSubscribeCallback = null
  })

  afterEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers() // Ensure fake timers are cleaned up
  })

  // ─── initial state ──────────────────────────────────────────────────────────

  describe('initial state', () => {
    it('has connected=false', () => {
      expect(useProjectStore.getState().connected).toBe(false)
    })

    it('has connecting=false', () => {
      expect(useProjectStore.getState().connecting).toBe(false)
    })

    it('has state=none', () => {
      expect(useProjectStore.getState().state).toBe('none')
    })

    it('has no task', () => {
      expect(useProjectStore.getState().task).toBeNull()
    })

    it('has empty output', () => {
      expect(useProjectStore.getState().output).toEqual([])
    })

    it('has lastSeq=0', () => {
      expect(useProjectStore.getState().lastSeq).toBe(0)
    })

    it('has empty checkpoints', () => {
      expect(useProjectStore.getState().checkpoints).toEqual([])
    })

    it('has empty redoStack', () => {
      expect(useProjectStore.getState().redoStack).toEqual([])
    })

    it('has null gitStatus', () => {
      expect(useProjectStore.getState().gitStatus).toBeNull()
    })

    it('has empty fileChanges', () => {
      expect(useProjectStore.getState().fileChanges).toEqual([])
    })

    it('has empty reviews', () => {
      expect(useProjectStore.getState().reviews).toEqual([])
    })

    it('has empty reviewDetails', () => {
      expect(useProjectStore.getState().reviewDetails).toEqual({})
    })

    it('has loading=false', () => {
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('has no error', () => {
      expect(useProjectStore.getState().error).toBeNull()
    })

    it('has empty taskQueue', () => {
      expect(useProjectStore.getState().taskQueue).toEqual([])
    })

    it('has null qualityPrompt', () => {
      expect(useProjectStore.getState().qualityPrompt).toBeNull()
    })

    it('has null worktreeId', () => {
      expect(useProjectStore.getState().worktreeId).toBeNull()
    })

    it('has null client', () => {
      expect(useProjectStore.getState().client).toBeNull()
    })

    it('has reconnectAttempt=0', () => {
      expect(useProjectStore.getState().reconnectAttempt).toBe(0)
    })
  })

  // ─── appendOutput ───────────────────────────────────────────────────────────

  describe('appendOutput', () => {
    it('appends a line to output', () => {
      useProjectStore.getState().appendOutput('hello world')
      const { output } = useProjectStore.getState()
      expect(output).toHaveLength(1)
      expect(output[0]).toContain('hello world')
    })

    it('includes a timestamp in the output line', () => {
      useProjectStore.getState().appendOutput('test line')
      const { output } = useProjectStore.getState()
      // Format: [HH:MM:SS AM/PM] test line
      expect(output[0]).toMatch(/\[.*\] test line/)
    })

    it('appends multiple lines in order', () => {
      useProjectStore.getState().appendOutput('first')
      useProjectStore.getState().appendOutput('second')
      useProjectStore.getState().appendOutput('third')
      const { output } = useProjectStore.getState()
      expect(output).toHaveLength(3)
      expect(output[0]).toContain('first')
      expect(output[1]).toContain('second')
      expect(output[2]).toContain('third')
    })
  })

  // ─── clearOutput ────────────────────────────────────────────────────────────

  describe('clearOutput', () => {
    it('clears all output lines', () => {
      useProjectStore.setState({ output: ['[12:00:00] line1', '[12:00:01] line2'] })
      useProjectStore.getState().clearOutput()
      expect(useProjectStore.getState().output).toEqual([])
    })

    it('works when output is already empty', () => {
      useProjectStore.getState().clearOutput()
      expect(useProjectStore.getState().output).toEqual([])
    })
  })

  // ─── disconnect ─────────────────────────────────────────────────────────────

  describe('disconnect', () => {
    it('resets connected to false', () => {
      useProjectStore.setState({ connected: true })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().connected).toBe(false)
    })

    it('resets connecting to false', () => {
      useProjectStore.setState({ connecting: true })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().connecting).toBe(false)
    })

    it('resets state to none', () => {
      useProjectStore.setState({ state: 'implementing' })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().state).toBe('none')
    })

    it('resets task to null', () => {
      useProjectStore.setState({
        task: { id: 't1', title: 'Test', state: 'planned', source: 'github:repo#1' }
      })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().task).toBeNull()
    })

    it('resets output to empty array', () => {
      useProjectStore.setState({ output: ['[12:00:00] some line'] })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().output).toEqual([])
    })

    it('resets lastSeq to 0', () => {
      useProjectStore.setState({ lastSeq: 42 })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().lastSeq).toBe(0)
    })

    it('resets checkpoints to empty array', () => {
      useProjectStore.setState({
        checkpoints: [{ sha: 'abc', message: 'init', timestamp: '2026-01-01' }]
      })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().checkpoints).toEqual([])
    })

    it('resets redoStack to empty array', () => {
      useProjectStore.setState({
        redoStack: [{ sha: 'def', message: 'redo', timestamp: '2026-01-01' }]
      })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().redoStack).toEqual([])
    })

    it('resets gitStatus to null', () => {
      useProjectStore.setState({ gitStatus: { branch: 'main', hasChanges: true } })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().gitStatus).toBeNull()
    })

    it('resets reviews to empty array', () => {
      useProjectStore.setState({
        reviews: [{ number: 1, timestamp: '2026-01-01', approved: true, message: 'LGTM' }]
      })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().reviews).toEqual([])
    })

    it('resets reviewDetails to empty object', () => {
      useProjectStore.setState({
        reviewDetails: {
          1: {
            number: 1,
            timestamp: '2026-01-01',
            approved: true,
            message: 'LGTM',
            content: 'content',
            findings: []
          }
        }
      })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().reviewDetails).toEqual({})
    })

    it('resets taskQueue to empty array', () => {
      useProjectStore.setState({
        taskQueue: [{ id: 'q1', source: 'gh:x#1', title: 'task', added_at: '2026-01-01', position: 0 }]
      })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().taskQueue).toEqual([])
    })

    it('resets qualityPrompt to null', () => {
      useProjectStore.setState({ qualityPrompt: { id: 'qp1', question: 'Continue?' } })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().qualityPrompt).toBeNull()
    })

    it('resets reconnectAttempt to 0', () => {
      useProjectStore.setState({ reconnectAttempt: 3 })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().reconnectAttempt).toBe(0)
    })

    it('resets worktreeId to null', () => {
      useProjectStore.setState({ worktreeId: 'my-worktree' })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().worktreeId).toBeNull()
    })

    it('resets client to null', () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: { close: vi.fn() } as any })
      useProjectStore.getState().disconnect()
      expect(useProjectStore.getState().client).toBeNull()
    })

    it('clears reconnect timeout when present', () => {
      const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout')
      const timeoutId = setTimeout(() => {}, 999999)
      useProjectStore.setState({ reconnectTimeoutId: timeoutId })
      useProjectStore.getState().disconnect()
      expect(clearTimeoutSpy).toHaveBeenCalledWith(timeoutId)
      clearTimeoutSpy.mockRestore()
    })
  })

  // ─── state management via setState ──────────────────────────────────────────

  describe('state management via setState', () => {
    it('can set connected', () => {
      useProjectStore.setState({ connected: true })
      expect(useProjectStore.getState().connected).toBe(true)
    })

    it('can set error', () => {
      useProjectStore.setState({ error: 'Something went wrong' })
      expect(useProjectStore.getState().error).toBe('Something went wrong')
    })

    it('can set loading', () => {
      useProjectStore.setState({ loading: true })
      expect(useProjectStore.getState().loading).toBe(true)
    })

    it('can set task state', () => {
      useProjectStore.setState({ state: 'planning' })
      expect(useProjectStore.getState().state).toBe('planning')
    })

    it('can set checkpoints', () => {
      const checkpoints = [
        { sha: 'abc123', message: 'checkpoint 1', timestamp: '2026-01-01T00:00:00Z' },
        { sha: 'def456', message: 'checkpoint 2', timestamp: '2026-01-02T00:00:00Z' },
      ]
      useProjectStore.setState({ checkpoints })
      expect(useProjectStore.getState().checkpoints).toEqual(checkpoints)
    })

    it('can set taskQueue', () => {
      const taskQueue = [
        { id: 'q1', source: 'github:repo#1', title: 'Fix bug', added_at: '2026-01-01', position: 0 },
        { id: 'q2', source: 'github:repo#2', title: 'Add feature', added_at: '2026-01-02', position: 1 },
      ]
      useProjectStore.setState({ taskQueue })
      expect(useProjectStore.getState().taskQueue).toEqual(taskQueue)
    })

    it('can set qualityPrompt', () => {
      useProjectStore.setState({ qualityPrompt: { id: 'qp1', question: 'Proceed with plan?' } })
      expect(useProjectStore.getState().qualityPrompt).toEqual({ id: 'qp1', question: 'Proceed with plan?' })
    })

    it('can set reviews', () => {
      const reviews = [
        { number: 1, timestamp: '2026-01-01', approved: false, message: 'Needs changes' },
      ]
      useProjectStore.setState({ reviews })
      expect(useProjectStore.getState().reviews).toEqual(reviews)
    })

    it('can set gitStatus', () => {
      useProjectStore.setState({ gitStatus: { branch: 'feature/new', hasChanges: true } })
      expect(useProjectStore.getState().gitStatus).toEqual({ branch: 'feature/new', hasChanges: true })
    })

    it('can set fileChanges', () => {
      const fileChanges = [
        { path: 'src/main.go', status: 'modified' as const },
        { path: 'README.md', status: 'added' as const },
      ]
      useProjectStore.setState({ fileChanges })
      expect(useProjectStore.getState().fileChanges).toEqual(fileChanges)
    })
  })

  // ─── task state transitions ──────────────────────────────────────────────────

  describe('task state transitions', () => {
    it('can transition from none to loaded', () => {
      useProjectStore.setState({ state: 'loaded' })
      expect(useProjectStore.getState().state).toBe('loaded')
    })

    it('can transition to planning', () => {
      useProjectStore.setState({ state: 'planning' })
      expect(useProjectStore.getState().state).toBe('planning')
    })

    it('can transition to planned', () => {
      useProjectStore.setState({ state: 'planned' })
      expect(useProjectStore.getState().state).toBe('planned')
    })

    it('can transition to implementing', () => {
      useProjectStore.setState({ state: 'implementing' })
      expect(useProjectStore.getState().state).toBe('implementing')
    })

    it('can transition to implemented', () => {
      useProjectStore.setState({ state: 'implemented' })
      expect(useProjectStore.getState().state).toBe('implemented')
    })

    it('can transition to reviewing', () => {
      useProjectStore.setState({ state: 'reviewing' })
      expect(useProjectStore.getState().state).toBe('reviewing')
    })

    it('can transition to submitted', () => {
      useProjectStore.setState({ state: 'submitted' })
      expect(useProjectStore.getState().state).toBe('submitted')
    })

    it('can transition to failed', () => {
      useProjectStore.setState({ state: 'failed' })
      expect(useProjectStore.getState().state).toBe('failed')
    })

    it('can transition to paused', () => {
      useProjectStore.setState({ state: 'paused' })
      expect(useProjectStore.getState().state).toBe('paused')
    })

    it('can transition to waiting', () => {
      useProjectStore.setState({ state: 'waiting' })
      expect(useProjectStore.getState().state).toBe('waiting')
    })
  })

  // ─── start ──────────────────────────────────────────────────────────────────

  describe('start', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().start('github:repo#1')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call with correct args on success', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'start') return Promise.resolve({ status: 'ok', state: 'loaded' })
        if (method === 'status') return Promise.resolve({ state: 'loaded', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().start('github:repo#1')

      expect(client.call).toHaveBeenCalledWith('start', { source: 'github:repo#1', auto_advance: false })
      expect(useProjectStore.getState().state).toBe('loaded')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Start failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().start('github:repo#1')

      expect(useProjectStore.getState().error).toBe('Start failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── plan ───────────────────────────────────────────────────────────────────

  describe('plan', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().plan()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call with dry_run=false by default', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ status: 'ok', state: 'planning', job_id: 'j1' })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().plan()

      expect(client.call).toHaveBeenCalledWith('plan', { dry_run: false })
      expect(useProjectStore.getState().state).toBe('planning')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Plan failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().plan()

      expect(useProjectStore.getState().error).toBe('Plan failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── implement ──────────────────────────────────────────────────────────────

  describe('implement', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().implement()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call with dry_run=false by default', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ status: 'ok', state: 'implementing', job_id: 'j3' })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().implement()

      expect(client.call).toHaveBeenCalledWith('implement', { dry_run: false })
      expect(useProjectStore.getState().state).toBe('implementing')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Implement failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().implement()

      expect(useProjectStore.getState().error).toBe('Implement failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── simplify ───────────────────────────────────────────────────────────────

  describe('simplify', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().simplify()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call simplify on success', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ status: 'ok', state: 'simplifying', job_id: 'j5' })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().simplify()

      expect(client.call).toHaveBeenCalledWith('simplify', { dry_run: false })
      expect(useProjectStore.getState().state).toBe('simplifying')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Simplify failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().simplify()

      expect(useProjectStore.getState().error).toBe('Simplify failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── optimize ───────────────────────────────────────────────────────────────

  describe('optimize', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().optimize()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call optimize on success', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ status: 'ok', state: 'optimizing', job_id: 'j6' })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().optimize()

      expect(client.call).toHaveBeenCalledWith('optimize', { dry_run: false })
      expect(useProjectStore.getState().state).toBe('optimizing')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Optimize failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().optimize()

      expect(useProjectStore.getState().error).toBe('Optimize failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── review ─────────────────────────────────────────────────────────────────

  describe('review', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().review()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call review with default options', async () => {
      const client = makeMockClient()
      client.call.mockImplementation((method: string) => {
        if (method === 'review') return Promise.resolve({ status: 'ok', state: 'reviewing' })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().review()

      expect(client.call).toHaveBeenCalledWith('review', {
        approve: false,
        reject: false,
        message: undefined,
        fix: false,
      })
      expect(useProjectStore.getState().state).toBe('reviewing')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call review with approve=true', async () => {
      const client = makeMockClient()
      client.call.mockImplementation((method: string) => {
        if (method === 'review') return Promise.resolve({ status: 'ok', state: 'reviewing' })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().review({ approve: true, message: 'LGTM' })

      expect(client.call).toHaveBeenCalledWith('review', {
        approve: true,
        reject: false,
        message: 'LGTM',
        fix: false,
      })
    })

    it('calls client.call review with reject=true', async () => {
      const client = makeMockClient()
      client.call.mockImplementation((method: string) => {
        if (method === 'review') return Promise.resolve({ status: 'ok', state: 'reviewing' })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().review({ reject: true, message: 'Needs work' })

      expect(client.call).toHaveBeenCalledWith('review', {
        approve: false,
        reject: true,
        message: 'Needs work',
        fix: false,
      })
    })

    it('calls client.call review with fix=true', async () => {
      const client = makeMockClient()
      client.call.mockImplementation((method: string) => {
        if (method === 'review') return Promise.resolve({ status: 'ok', state: 'reviewing' })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().review({ fix: true })

      expect(client.call).toHaveBeenCalledWith('review', {
        approve: false,
        reject: false,
        message: undefined,
        fix: true,
      })
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Review failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().review()

      expect(useProjectStore.getState().error).toBe('Review failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── submit ─────────────────────────────────────────────────────────────────

  describe('submit', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().submit()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call submit with default options', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ status: 'ok', state: 'submitted' })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().submit()

      expect(client.call).toHaveBeenCalledWith('submit', {
        title: undefined,
        body: undefined,
        draft: false,
        reviewers: [],
        labels: [],
        delete_branch: false,
        dry_run: false,
        sections: undefined,
      })
      expect(useProjectStore.getState().state).toBe('submitted')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call submit with all options', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ status: 'ok', state: 'submitted' })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().submit({
        title: 'My PR',
        body: 'Fixes #1',
        draft: true,
        reviewers: ['alice'],
        labels: ['bug'],
        delete_branch: true,
      })

      expect(client.call).toHaveBeenCalledWith('submit', {
        title: 'My PR',
        body: 'Fixes #1',
        draft: true,
        reviewers: ['alice'],
        labels: ['bug'],
        delete_branch: true,
        dry_run: false,
        sections: undefined,
      })
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Submit failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().submit()

      expect(useProjectStore.getState().error).toBe('Submit failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── abort ──────────────────────────────────────────────────────────────────

  describe('abort', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().abort()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call abort on success', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ status: 'ok', state: 'loaded' })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().abort()

      expect(client.call).toHaveBeenCalledWith('abort', {})
      expect(useProjectStore.getState().state).toBe('loaded')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Abort failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().abort()

      expect(useProjectStore.getState().error).toBe('Abort failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── reset ──────────────────────────────────────────────────────────────────

  describe('reset', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().reset()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call reset on success', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'reset') return Promise.resolve({ status: 'ok', state: 'none' })
        if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().reset()

      expect(client.call).toHaveBeenCalledWith('reset', {})
      expect(useProjectStore.getState().state).toBe('none')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Reset failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().reset()

      expect(useProjectStore.getState().error).toBe('Reset failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── abandon ────────────────────────────────────────────────────────────────

  describe('abandon', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().abandon()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call abandon with keep_branch=false by default', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'abandon') return Promise.resolve({ status: 'ok', state: 'none' })
        if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().abandon()

      expect(client.call).toHaveBeenCalledWith('abandon', { keep_branch: false })
      expect(useProjectStore.getState().state).toBe('none')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call abandon with keep_branch=true', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'abandon') return Promise.resolve({ status: 'ok', state: 'none' })
        if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().abandon(true)

      expect(client.call).toHaveBeenCalledWith('abandon', { keep_branch: true })
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Abandon failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().abandon()

      expect(useProjectStore.getState().error).toBe('Abandon failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── update ─────────────────────────────────────────────────────────────────

  describe('update', () => {
    it('returns default when no client', async () => {
      const result = await useProjectStore.getState().update()
      expect(result).toEqual({ changed: false, specification_generated: false })
    })

    it('returns result with changed=true', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'update') return Promise.resolve({ changed: true, specification_generated: true })
        if (method === 'status') return Promise.resolve({ state: 'loaded', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().update()

      expect(client.call).toHaveBeenCalledWith('update', {})
      expect(result).toEqual({ changed: true, specification_generated: true })
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('returns result with changed=false', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'update') return Promise.resolve({ changed: false, specification_generated: false })
        if (method === 'status') return Promise.resolve({ state: 'loaded', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().update()

      expect(result).toEqual({ changed: false, specification_generated: false })
    })

    it('returns default and sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Update failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().update()

      expect(result).toEqual({ changed: false, specification_generated: false })
      expect(useProjectStore.getState().error).toBe('Update failed')
    })
  })

  // ─── finish ─────────────────────────────────────────────────────────────────

  describe('finish', () => {
    it('returns null when no client', async () => {
      const result = await useProjectStore.getState().finish()
      expect(result).toBeNull()
    })

    it('calls client.call task.finish on success and returns branch info', async () => {
      const finishResult = {
        previous_branch: 'feature/my-task',
        current_branch: 'main',
        branch_deleted: true,
        remote_branch_deleted: false,
      }
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'task.finish') return Promise.resolve(finishResult)
        if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().finish()

      expect(client.call).toHaveBeenCalledWith('task.finish', {
        delete_remote: false,
        force: false,
      })
      expect(result).toEqual(finishResult)
      expect(useProjectStore.getState().state).toBe('none')
      expect(useProjectStore.getState().task).toBeNull()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls task.finish with custom options', async () => {
      const finishResult = {
        previous_branch: 'feature/done',
        current_branch: 'main',
        branch_deleted: true,
        remote_branch_deleted: true,
      }
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'task.finish') return Promise.resolve(finishResult)
        if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().finish({ delete_remote: true, force: true })

      expect(client.call).toHaveBeenCalledWith('task.finish', {
        delete_remote: true,
        force: true,
      })
    })

    it('returns null and sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Finish failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().finish()

      expect(result).toBeNull()
      expect(useProjectStore.getState().error).toBe('Finish failed')
    })
  })

  // ─── refresh ────────────────────────────────────────────────────────────────

  describe('refresh', () => {
    it('returns null when no client', async () => {
      const result = await useProjectStore.getState().refresh()
      expect(result).toBeNull()
    })

    it('calls task.refresh and returns result with pr_url', async () => {
      const refreshResult = {
        task_id: 't1',
        branch: 'feature/x',
        pr_status: 'open',
        pr_merged: false,
        pr_url: 'https://github.com/org/repo/pull/42',
        commits_behind_base: 0,
        action: 'none',
        message: 'PR is open',
      }
      const client = makeMockClient()
      client.call.mockResolvedValue(refreshResult)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().refresh()

      expect(client.call).toHaveBeenCalledWith('task.refresh', {})
      expect(result).toEqual(refreshResult)
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls task.refresh and returns result without pr_url', async () => {
      const refreshResult = {
        task_id: 't1',
        branch: 'feature/x',
        pr_status: '',
        pr_merged: false,
        pr_url: '',
        commits_behind_base: 0,
        action: 'none',
        message: 'No PR yet',
      }
      const client = makeMockClient()
      client.call.mockResolvedValue(refreshResult)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().refresh()

      expect(result).toEqual(refreshResult)
    })

    it('returns null and sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Refresh failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().refresh()

      expect(result).toBeNull()
      expect(useProjectStore.getState().error).toBe('Refresh failed')
    })
  })

  // ─── deleteTask ─────────────────────────────────────────────────────────────

  describe('deleteTask', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().deleteTask()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call delete on success', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'delete') return Promise.resolve({})
        if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().deleteTask()

      expect(client.call).toHaveBeenCalledWith('delete', {})
      expect(useProjectStore.getState().state).toBe('none')
      expect(useProjectStore.getState().task).toBeNull()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Delete failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().deleteTask()

      expect(useProjectStore.getState().error).toBe('Delete failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── approveRemote ──────────────────────────────────────────────────────────

  describe('approveRemote', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().approveRemote()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call remote.approve with default empty comment', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({})
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().approveRemote()

      expect(client.call).toHaveBeenCalledWith('remote.approve', { comment: '' })
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call remote.approve with provided comment', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({})
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().approveRemote('Looks good!')

      expect(client.call).toHaveBeenCalledWith('remote.approve', { comment: 'Looks good!' })
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Approve failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().approveRemote()

      expect(useProjectStore.getState().error).toBe('Approve failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── mergeRemote ────────────────────────────────────────────────────────────

  describe('mergeRemote', () => {
    it('returns early without calling client when no client', async () => {
      await useProjectStore.getState().mergeRemote()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call remote.merge with default method rebase', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'remote.merge') return Promise.resolve({})
        if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().mergeRemote()

      expect(client.call).toHaveBeenCalledWith('remote.merge', { method: 'rebase' })
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call remote.merge with custom method', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'remote.merge') return Promise.resolve({})
        if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().mergeRemote('squash')

      expect(client.call).toHaveBeenCalledWith('remote.merge', { method: 'squash' })
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Merge failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().mergeRemote()

      expect(useProjectStore.getState().error).toBe('Merge failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── respondToPrompt ────────────────────────────────────────────────────────

  describe('respondToPrompt', () => {
    it('returns early without calling client when no client', async () => {
      useProjectStore.setState({ qualityPrompt: { id: 'qp1', question: 'Continue?' } })
      await useProjectStore.getState().respondToPrompt('qp1', true)
      // qualityPrompt not cleared since no client
      expect(useProjectStore.getState().qualityPrompt).not.toBeNull()
    })

    it('calls client.call quality.respond and clears qualityPrompt on success', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({})
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true, qualityPrompt: { id: 'qp1', question: 'Continue?' } })

      await useProjectStore.getState().respondToPrompt('qp1', true)

      expect(client.call).toHaveBeenCalledWith('quality.respond', { prompt_id: 'qp1', answer: true })
      expect(useProjectStore.getState().qualityPrompt).toBeNull()
    })

    it('sets error on failure and preserves qualityPrompt', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Quality response failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true, qualityPrompt: { id: 'qp1', question: 'Continue?' } })

      await useProjectStore.getState().respondToPrompt('qp1', false)

      expect(useProjectStore.getState().error).toBe('Quality response failed')
      expect(useProjectStore.getState().qualityPrompt).not.toBeNull()
    })
  })

  // ─── queueTask ──────────────────────────────────────────────────────────────

  describe('queueTask', () => {
    it('returns null when no client', async () => {
      const result = await useProjectStore.getState().queueTask('github:repo#1')
      expect(result).toBeNull()
    })

    it('calls client.call queue.add and returns result', async () => {
      const queued = { id: 'q1', source: 'github:repo#1', title: 'Fix bug', added_at: '2026-01-01', position: 0 }
      const client = makeMockClient()
      client.call.mockImplementation((method: string) => {
        if (method === 'queue.add') return Promise.resolve(queued)
        if (method === 'queue.list') return Promise.resolve({ queue: [queued], count: 1 })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().queueTask('github:repo#1', 'Fix bug')

      expect(client.call).toHaveBeenCalledWith('queue.add', { source: 'github:repo#1', title: 'Fix bug' })
      expect(result).toEqual(queued)
      expect(useProjectStore.getState().taskQueue).toEqual([queued])
    })

    it('calls queue.add with empty title when title omitted', async () => {
      const queued = { id: 'q2', source: 'github:repo#2', title: '', added_at: '2026-01-01', position: 0 }
      const client = makeMockClient()
      client.call.mockImplementation((method: string) => {
        if (method === 'queue.add') return Promise.resolve(queued)
        if (method === 'queue.list') return Promise.resolve({ queue: [queued], count: 1 })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().queueTask('github:repo#2')

      expect(client.call).toHaveBeenCalledWith('queue.add', { source: 'github:repo#2', title: '' })
    })

    it('returns null and sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Queue add failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().queueTask('github:repo#1')

      expect(result).toBeNull()
      expect(useProjectStore.getState().error).toBe('Queue add failed')
    })
  })

  // ─── dequeueTask ────────────────────────────────────────────────────────────

  describe('dequeueTask', () => {
    it('returns early when no client', async () => {
      // Should not throw
      await useProjectStore.getState().dequeueTask('q1')
    })

    it('calls client.call queue.remove and reloads queue on success', async () => {
      const client = makeMockClient()
      client.call.mockImplementation((method: string) => {
        if (method === 'queue.remove') return Promise.resolve({})
        if (method === 'queue.list') return Promise.resolve({ queue: [], count: 0 })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true, taskQueue: [{ id: 'q1', source: 'x', title: 't', added_at: '2026', position: 0 }] })

      await useProjectStore.getState().dequeueTask('q1')

      expect(client.call).toHaveBeenCalledWith('queue.remove', { id: 'q1' })
      expect(useProjectStore.getState().taskQueue).toEqual([])
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Queue remove failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().dequeueTask('q1')

      expect(useProjectStore.getState().error).toBe('Queue remove failed')
    })
  })

  // ─── loadQueue ──────────────────────────────────────────────────────────────

  describe('loadQueue', () => {
    it('returns early when no client', async () => {
      await useProjectStore.getState().loadQueue()
      expect(useProjectStore.getState().taskQueue).toEqual([])
    })

    it('sets taskQueue from result', async () => {
      const queue = [
        { id: 'q1', source: 'github:repo#1', title: 'Task 1', added_at: '2026-01-01', position: 0 },
        { id: 'q2', source: 'github:repo#2', title: 'Task 2', added_at: '2026-01-02', position: 1 },
      ]
      const client = makeMockClient()
      client.call.mockResolvedValue({ queue, count: 2 })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().loadQueue()

      expect(client.call).toHaveBeenCalledWith('queue.list', {})
      expect(useProjectStore.getState().taskQueue).toEqual(queue)
    })
  })

  // ─── reorderQueue ───────────────────────────────────────────────────────────

  describe('reorderQueue', () => {
    it('returns early when no client', async () => {
      await useProjectStore.getState().reorderQueue('q1', 2)
    })

    it('calls queue.reorder and sets taskQueue on success', async () => {
      const reorderedQueue = [
        { id: 'q2', source: 'github:repo#2', title: 'Task 2', added_at: '2026-01-02', position: 0 },
        { id: 'q1', source: 'github:repo#1', title: 'Task 1', added_at: '2026-01-01', position: 1 },
      ]
      const client = makeMockClient()
      client.call.mockResolvedValue({ queue: reorderedQueue, count: 2 })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().reorderQueue('q1', 1)

      expect(client.call).toHaveBeenCalledWith('queue.reorder', { id: 'q1', position: 1 })
      expect(useProjectStore.getState().taskQueue).toEqual(reorderedQueue)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Queue reorder failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().reorderQueue('q1', 2)

      expect(useProjectStore.getState().error).toBe('Queue reorder failed')
    })
  })

  // ─── undo ───────────────────────────────────────────────────────────────────

  describe('undo', () => {
    it('returns early when no client', async () => {
      await useProjectStore.getState().undo()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call undo with default steps=1', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'undo') return Promise.resolve({ status: 'ok', state: 'planned', steps: 1 })
        if (method === 'status') return Promise.resolve({ state: 'planned', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().undo()

      expect(client.call).toHaveBeenCalledWith('undo', { steps: 1 })
      expect(useProjectStore.getState().state).toBe('planned')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call undo with explicit steps count', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'undo') return Promise.resolve({ status: 'ok', state: 'none', steps: 3 })
        if (method === 'status') return Promise.resolve({ state: 'none', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().undo(3)

      expect(client.call).toHaveBeenCalledWith('undo', { steps: 3 })
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Undo failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().undo()

      expect(useProjectStore.getState().error).toBe('Undo failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── redo ───────────────────────────────────────────────────────────────────

  describe('redo', () => {
    it('returns early when no client', async () => {
      await useProjectStore.getState().redo()
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call redo with default steps=1', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'redo') return Promise.resolve({ status: 'ok', state: 'implementing', steps: 1 })
        if (method === 'status') return Promise.resolve({ state: 'implementing', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().redo()

      expect(client.call).toHaveBeenCalledWith('redo', { steps: 1 })
      expect(useProjectStore.getState().state).toBe('implementing')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call redo with explicit steps count', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'redo') return Promise.resolve({ status: 'ok', state: 'implementing', steps: 2 })
        if (method === 'status') return Promise.resolve({ state: 'implementing', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().redo(2)

      expect(client.call).toHaveBeenCalledWith('redo', { steps: 2 })
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Redo failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().redo()

      expect(useProjectStore.getState().error).toBe('Redo failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── goToCheckpoint ─────────────────────────────────────────────────────────

  describe('goToCheckpoint', () => {
    it('returns early when no client', async () => {
      await useProjectStore.getState().goToCheckpoint('abc123')
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('calls client.call checkpoint.goto on success', async () => {
      const client = makeClientWithAutoRefresh()
      client.call.mockImplementation((method: string) => {
        if (method === 'checkpoint.goto') return Promise.resolve({ status: 'ok', sha: 'abc123' })
        if (method === 'status') return Promise.resolve({ state: 'planned', path: '/proj' })
        if (method === 'checkpoints') return Promise.resolve({ checkpoints: [], redo_stack: [] })
        if (method === 'git.status') return Promise.resolve({ branch: 'main', has_changes: false, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().goToCheckpoint('abc123')

      expect(client.call).toHaveBeenCalledWith('checkpoint.goto', { sha: 'abc123' })
      expect(useProjectStore.getState().loading).toBe(false)
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Checkpoint navigation failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().goToCheckpoint('abc123')

      expect(useProjectStore.getState().error).toBe('Checkpoint navigation failed')
      expect(useProjectStore.getState().loading).toBe(false)
    })
  })

  // ─── refreshGitStatus ───────────────────────────────────────────────────────

  describe('refreshGitStatus', () => {
    it('returns early when no client', async () => {
      await useProjectStore.getState().refreshGitStatus()
      expect(useProjectStore.getState().gitStatus).toBeNull()
    })

    it('sets gitStatus and fileChanges on success', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({
        branch: 'feature/test',
        has_changes: true,
        files: [
          { path: 'src/main.go', status: 'modified' },
          { path: 'pkg/new.go', status: 'added' },
        ],
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().refreshGitStatus()

      expect(client.call).toHaveBeenCalledWith('git.status', {})
      expect(useProjectStore.getState().gitStatus).toEqual({ branch: 'feature/test', hasChanges: true })
      expect(useProjectStore.getState().fileChanges).toEqual([
        { path: 'src/main.go', status: 'modified' },
        { path: 'pkg/new.go', status: 'added' },
      ])
    })

    it('logs warning on error and does not set state', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('git unavailable'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().refreshGitStatus()

      expect(warnSpy).toHaveBeenCalledWith('Could not fetch git status:', expect.any(Error))
      expect(useProjectStore.getState().gitStatus).toBeNull()
      warnSpy.mockRestore()
    })
  })

  // ─── getGitDiff ─────────────────────────────────────────────────────────────

  describe('getGitDiff', () => {
    it('returns empty string when no client', async () => {
      const diff = await useProjectStore.getState().getGitDiff()
      expect(diff).toBe('')
    })

    it('returns diff string on success', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ diff: 'diff --git a/file.go ...' })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const diff = await useProjectStore.getState().getGitDiff()

      expect(client.call).toHaveBeenCalledWith('git.diff', { cached: false })
      expect(diff).toBe('diff --git a/file.go ...')
    })

    it('passes cached=true when requested', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ diff: 'staged diff' })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const diff = await useProjectStore.getState().getGitDiff(true)

      expect(client.call).toHaveBeenCalledWith('git.diff', { cached: true })
      expect(diff).toBe('staged diff')
    })

    it('returns empty string on error', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('git diff failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const diff = await useProjectStore.getState().getGitDiff()

      expect(diff).toBe('')
      warnSpy.mockRestore()
    })
  })

  // ─── getGitLog ──────────────────────────────────────────────────────────────

  describe('getGitLog', () => {
    it('returns empty array when no client', async () => {
      const log = await useProjectStore.getState().getGitLog()
      expect(log).toEqual([])
    })

    it('returns log entries on success', async () => {
      const entries = [
        { sha: 'abc123', message: 'Fix bug', author: 'Alice', date: '2026-01-01' },
        { sha: 'def456', message: 'Add feature', author: 'Bob', date: '2026-01-02' },
      ]
      const client = makeMockClient()
      client.call.mockResolvedValue({ entries })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const log = await useProjectStore.getState().getGitLog()

      expect(client.call).toHaveBeenCalledWith('git.log', { count: 10 })
      expect(log).toEqual(entries)
    })

    it('uses custom count', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ entries: [] })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().getGitLog(5)

      expect(client.call).toHaveBeenCalledWith('git.log', { count: 5 })
    })

    it('returns empty array on error', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('git log failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const log = await useProjectStore.getState().getGitLog()

      expect(log).toEqual([])
      warnSpy.mockRestore()
    })
  })

  // ─── loadReviews ────────────────────────────────────────────────────────────

  describe('loadReviews', () => {
    it('returns early when no client', async () => {
      await useProjectStore.getState().loadReviews()
      expect(useProjectStore.getState().reviews).toEqual([])
    })

    it('sets reviews on success', async () => {
      const reviews = [
        { number: 1, timestamp: '2026-01-01', approved: true, message: 'LGTM' },
        { number: 2, timestamp: '2026-01-02', approved: false, message: 'Needs changes' },
      ]
      const client = makeMockClient()
      client.call.mockResolvedValue({ reviews })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().loadReviews()

      expect(client.call).toHaveBeenCalledWith('review.list', {})
      expect(useProjectStore.getState().reviews).toEqual(reviews)
    })
  })

  // ─── loadReview ─────────────────────────────────────────────────────────────

  describe('loadReview', () => {
    it('returns null when no client', async () => {
      const result = await useProjectStore.getState().loadReview(1)
      expect(result).toBeNull()
    })

    it('fetches and caches review detail on success', async () => {
      const detail = {
        number: 1,
        timestamp: '2026-01-01',
        approved: true,
        message: 'LGTM',
        content: 'full review content',
        findings: ['finding 1'],
      }
      const client = makeMockClient()
      client.call.mockResolvedValue(detail)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().loadReview(1)

      expect(client.call).toHaveBeenCalledWith('review.view', { number: 1 })
      expect(result).toEqual(detail)
      expect(useProjectStore.getState().reviewDetails[1]).toEqual(detail)
    })

    it('returns cached review without re-fetching', async () => {
      const detail = {
        number: 1,
        timestamp: '2026-01-01',
        approved: true,
        message: 'LGTM',
        content: 'cached content',
        findings: [],
      }
      const client = makeMockClient()
      client.call.mockResolvedValue(detail)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true, reviewDetails: { 1: detail } })

      const result = await useProjectStore.getState().loadReview(1)

      expect(client.call).not.toHaveBeenCalled()
      expect(result).toEqual(detail)
    })

    it('returns null on error', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('review not found'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().loadReview(99)

      expect(result).toBeNull()
      warnSpy.mockRestore()
    })
  })

  // ─── browseFiles ────────────────────────────────────────────────────────────

  describe('browseFiles', () => {
    it('returns empty array when no client', async () => {
      const result = await useProjectStore.getState().browseFiles()
      expect(result).toEqual([])
    })

    it('returns entries on success', async () => {
      const entries = [
        { name: 'src', path: '/proj/src', is_dir: true },
        { name: 'main.go', path: '/proj/main.go', is_dir: false, size: 1024 },
      ]
      const client = makeMockClient()
      client.call.mockResolvedValue({ entries })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().browseFiles('/proj')

      expect(client.call).toHaveBeenCalledWith('browse', { path: '/proj', files: false })
      expect(result).toEqual(entries)
    })

    it('passes filesOnly=true when requested', async () => {
      const client = makeMockClient()
      client.call.mockResolvedValue({ entries: [] })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().browseFiles('/proj', true)

      expect(client.call).toHaveBeenCalledWith('browse', { path: '/proj', files: true })
    })

    it('returns empty array on error', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('browse failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().browseFiles()

      expect(result).toEqual([])
      warnSpy.mockRestore()
    })
  })

  // ─── listFiles ──────────────────────────────────────────────────────────────

  describe('listFiles', () => {
    it('returns empty array when no client', async () => {
      const result = await useProjectStore.getState().listFiles()
      expect(result).toEqual([])
    })

    it('returns files on success', async () => {
      const files = [
        { path: 'src/main.go', size: 512 },
        { path: 'pkg/util.go', size: 256 },
      ]
      const client = makeMockClient()
      client.call.mockResolvedValue({ files })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().listFiles('/proj', ['.go'], 3)

      expect(client.call).toHaveBeenCalledWith('files.list', {
        path: '/proj',
        extensions: ['.go'],
        max_depth: 3,
      })
      expect(result).toEqual(files)
    })

    it('returns empty array on error', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('list failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      const result = await useProjectStore.getState().listFiles()

      expect(result).toEqual([])
      warnSpy.mockRestore()
    })
  })

  // ─── refreshStatus ──────────────────────────────────────────────────────────

  describe('refreshStatus', () => {
    it('returns early when no client', async () => {
      await useProjectStore.getState().refreshStatus()
      expect(useProjectStore.getState().state).toBe('none')
    })

    it('sets state and task from status response', async () => {
      const client = makeMockClient()
      client.call.mockImplementation((method: string) => {
        if (method === 'status') return Promise.resolve({
          state: 'planned',
          path: '/proj',
          task: { id: 't1', title: 'My Task', source: 'github:repo#1', branch: 'feature/t1', worktree_path: '/proj' },
        })
        if (method === 'checkpoints') return Promise.resolve({
          checkpoints: [{ sha: 'abc', message: 'cp1', author: 'dev', timestamp: '2026-01-01' }],
          redo_stack: [],
        })
        if (method === 'git.status') return Promise.resolve({ branch: 'feature/t1', has_changes: true, files: [] })
        if (method === 'review.list') return Promise.resolve({ reviews: [] })
        return Promise.resolve({})
      })
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().refreshStatus()

      expect(useProjectStore.getState().state).toBe('planned')
      expect(useProjectStore.getState().task).toEqual({
        id: 't1',
        title: 'My Task',
        state: 'planned',
        source: 'github:repo#1',
        branch: 'feature/t1',
        worktreePath: '/proj',
      })
      expect(useProjectStore.getState().checkpoints).toEqual([
        { sha: 'abc', message: 'cp1', timestamp: '2026-01-01' },
      ])
    })

    it('sets error on failure', async () => {
      const client = makeMockClient()
      client.call.mockRejectedValue(new Error('Status refresh failed'))
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      useProjectStore.setState({ client: client as any, connected: true })

      await useProjectStore.getState().refreshStatus()

      expect(useProjectStore.getState().error).toBe('Status refresh failed')
    })
  })

  // ─── event handler (subscribe callback) ────────────────────────────────────

  describe('subscribe event handler', () => {
    // Helper that wires up a connected store by calling connect().
    // The module-level mock auto-captures the subscribe callback into
    // _capturedSubscribeCallback and the instance into _capturedSocketClientInstance.
    async function setupWithSubscribe() {
      const { sendNotification } = await import('../lib/notify')
      const { useScreenshotStore } = await import('./screenshotStore')

      useProjectStore.setState(initialState)
      await useProjectStore.getState().connect('my-worktree')

      const client = useProjectStore.getState().client!

      return {
        client,
        fire: (data: unknown) => {
          if (_capturedSubscribeCallback) _capturedSubscribeCallback(data)
        },
        sendNotification,
        useScreenshotStore,
      }
    }

    it('state_changed with state=planned triggers notification', async () => {
      const { fire, sendNotification } = await setupWithSubscribe()
      fire({ type: 'state_changed', state: 'planned', seq: 1 })
      expect(useProjectStore.getState().state).toBe('planned')
      expect(sendNotification).toHaveBeenCalledWith('Planning Complete', 'Specification is ready for review')
    })

    it('state_changed with state=implemented triggers notification', async () => {
      const { fire, sendNotification } = await setupWithSubscribe()
      fire({ type: 'state_changed', state: 'implemented', seq: 2 })
      expect(useProjectStore.getState().state).toBe('implemented')
      expect(sendNotification).toHaveBeenCalledWith('Implementation Complete', 'Code is ready for review')
    })

    it('state_changed with other state does not trigger notification', async () => {
      const { fire, sendNotification } = await setupWithSubscribe()
      fire({ type: 'state_changed', state: 'reviewing', seq: 3 })
      expect(useProjectStore.getState().state).toBe('reviewing')
      // sendNotification may have been called by connect() setup, reset and check
      vi.clearAllMocks()
      fire({ type: 'state_changed', state: 'submitted', seq: 4 })
      expect(sendNotification).not.toHaveBeenCalled()
    })

    it('task_abandoned sets state from message', async () => {
      const { fire } = await setupWithSubscribe()
      fire({ type: 'task_abandoned', state: 'none', message: 'Abandoned!', seq: 5 })
      expect(useProjectStore.getState().state).toBe('none')
    })

    it('task_deleted sets state and appends output', async () => {
      const { fire } = await setupWithSubscribe()
      fire({ type: 'task_deleted', state: 'none', seq: 6 })
      expect(useProjectStore.getState().state).toBe('none')
    })

    it('task_reset sets state and appends output', async () => {
      const { fire } = await setupWithSubscribe()
      fire({ type: 'task_reset', state: 'loaded', message: 'Reset!', seq: 7 })
      expect(useProjectStore.getState().state).toBe('loaded')
    })

    it('job_output with content appends to output', async () => {
      const { fire } = await setupWithSubscribe()
      const prevOutputLen = useProjectStore.getState().output.length
      fire({ type: 'job_output', content: 'some output line', seq: 8 })
      expect(useProjectStore.getState().output.length).toBeGreaterThan(prevOutputLen)
      const last = useProjectStore.getState().output.at(-1)!
      expect(last).toContain('some output line')
    })

    it('stream with message appends to output', async () => {
      const { fire } = await setupWithSubscribe()
      const prevOutputLen = useProjectStore.getState().output.length
      fire({ type: 'stream', message: 'streaming message', seq: 9 })
      expect(useProjectStore.getState().output.length).toBeGreaterThan(prevOutputLen)
      const last = useProjectStore.getState().output.at(-1)!
      expect(last).toContain('streaming message')
    })

    it('job_output without content/message does not append', async () => {
      const { fire } = await setupWithSubscribe()
      const prevOutputLen = useProjectStore.getState().output.length
      fire({ type: 'job_output', seq: 10 })
      // No extra line appended
      expect(useProjectStore.getState().output.length).toBe(prevOutputLen)
    })

    it('checkpoint_created appends to output', async () => {
      const { fire } = await setupWithSubscribe()
      const prevOutputLen = useProjectStore.getState().output.length
      fire({ type: 'checkpoint_created', message: 'saved state', seq: 11 })
      expect(useProjectStore.getState().output.length).toBeGreaterThan(prevOutputLen)
      const last = useProjectStore.getState().output.at(-1)!
      expect(last).toContain('Checkpoint created: saved state')
    })

    it('job_completed appends output and triggers notification', async () => {
      const { fire, sendNotification } = await setupWithSubscribe()
      vi.clearAllMocks()
      const prevOutputLen = useProjectStore.getState().output.length
      fire({ type: 'job_completed', seq: 12 })
      expect(useProjectStore.getState().output.length).toBeGreaterThan(prevOutputLen)
      expect(sendNotification).toHaveBeenCalledWith('Task Completed', expect.any(String))
    })

    it('job_failed appends error to output and sets error state', async () => {
      const { fire } = await setupWithSubscribe()
      fire({ type: 'job_failed', error: 'something went wrong', seq: 13 })
      const last = useProjectStore.getState().output.at(-1)!
      expect(last).toContain('Job failed: something went wrong')
      expect(useProjectStore.getState().error).toBe('something went wrong')
    })

    it('screenshot_captured calls handleScreenshotCaptured', async () => {
      const { fire, useScreenshotStore } = await setupWithSubscribe()
      const screenshot = { id: 's1', path: '/tmp/s1.png', timestamp: '2026-01-01' }
      fire({ type: 'screenshot_captured', data: screenshot, seq: 14 })
      expect(useScreenshotStore.getState().handleScreenshotCaptured).toHaveBeenCalledWith(screenshot)
    })

    it('screenshot_deleted calls handleScreenshotDeleted', async () => {
      const { fire, useScreenshotStore } = await setupWithSubscribe()
      fire({ type: 'screenshot_deleted', data: { id: 's1' }, seq: 15 })
      expect(useScreenshotStore.getState().handleScreenshotDeleted).toHaveBeenCalledWith('s1')
    })

    it('user_prompt sets qualityPrompt', async () => {
      const { fire } = await setupWithSubscribe()
      fire({ type: 'user_prompt', data: { prompt_id: 'p1', question: 'Proceed?' }, seq: 16 })
      expect(useProjectStore.getState().qualityPrompt).toEqual({ id: 'p1', question: 'Proceed?' })
    })

    it('user_prompt without data does not set qualityPrompt', async () => {
      const { fire } = await setupWithSubscribe()
      fire({ type: 'user_prompt', seq: 17 })
      expect(useProjectStore.getState().qualityPrompt).toBeNull()
    })

    it('task_queued triggers loadQueue', async () => {
      vi.useFakeTimers()
      const { fire, client } = await setupWithSubscribe()
      const callsBefore = (client.call as ReturnType<typeof vi.fn>).mock.calls.length
      fire({ type: 'task_queued', seq: 18 })
      // loadQueue is debounced by 500ms
      await vi.advanceTimersByTimeAsync(500)
      const callsAfter = (client.call as ReturnType<typeof vi.fn>).mock.calls.length
      expect(callsAfter).toBeGreaterThan(callsBefore)
      vi.useRealTimers()
    })

    it('task_dequeued triggers loadQueue', async () => {
      vi.useFakeTimers()
      const { fire, client } = await setupWithSubscribe()
      const callsBefore = (client.call as ReturnType<typeof vi.fn>).mock.calls.length
      fire({ type: 'task_dequeued', seq: 19 })
      // loadQueue is debounced by 500ms
      await vi.advanceTimersByTimeAsync(500)
      const callsAfter = (client.call as ReturnType<typeof vi.fn>).mock.calls.length
      expect(callsAfter).toBeGreaterThan(callsBefore)
      vi.useRealTimers()
    })

    it('queue_advancing triggers loadQueue and appends message', async () => {
      const { fire } = await setupWithSubscribe()
      const prevOutputLen = useProjectStore.getState().output.length
      fire({ type: 'queue_advancing', message: 'Loading next...', seq: 20 })
      await Promise.resolve()
      expect(useProjectStore.getState().output.length).toBeGreaterThan(prevOutputLen)
      const last = useProjectStore.getState().output.at(-1)!
      expect(last).toContain('Loading next...')
    })

    it('task_finished appends message and triggers refresh + loadQueue', async () => {
      const { fire } = await setupWithSubscribe()
      const prevOutputLen = useProjectStore.getState().output.length
      fire({ type: 'task_finished', message: 'Done!', seq: 21 })
      await Promise.resolve()
      expect(useProjectStore.getState().output.length).toBeGreaterThan(prevOutputLen)
      const last = useProjectStore.getState().output.at(-1)!
      expect(last).toContain('Done!')
    })

    it('heartbeat is ignored (no state change, no output)', async () => {
      const { fire } = await setupWithSubscribe()
      const stateBefore = useProjectStore.getState().state
      const outputLenBefore = useProjectStore.getState().output.length
      fire({ type: 'heartbeat', seq: 22 })
      expect(useProjectStore.getState().state).toBe(stateBefore)
      expect(useProjectStore.getState().output.length).toBe(outputLenBefore)
    })

    it('event deduplication: seq already seen is skipped', async () => {
      const { fire } = await setupWithSubscribe()
      // Send event with seq=50
      fire({ type: 'state_changed', state: 'planned', seq: 50 })
      expect(useProjectStore.getState().state).toBe('planned')

      // Reset state manually
      useProjectStore.setState({ state: 'none' })

      // Send same seq again — should be ignored
      fire({ type: 'state_changed', state: 'implementing', seq: 50 })
      expect(useProjectStore.getState().state).toBe('none')
    })

    it('lastSeq updates to highest seq seen', async () => {
      const { fire } = await setupWithSubscribe()
      fire({ type: 'heartbeat', seq: 100 })
      expect(useProjectStore.getState().lastSeq).toBe(100)
      // 99 < 100 — deduplicated, lastSeq stays 100
      fire({ type: 'heartbeat', seq: 99 })
      expect(useProjectStore.getState().lastSeq).toBe(100)
    })

    it('events without seq are always processed', async () => {
      const { fire } = await setupWithSubscribe()
      const prevOutputLen = useProjectStore.getState().output.length
      // No seq field — should not be deduplicated
      fire({ type: 'job_output', content: 'no seq line' })
      expect(useProjectStore.getState().output.length).toBeGreaterThan(prevOutputLen)
    })
  })
})
