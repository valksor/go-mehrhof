import { describe, it, expect, vi, beforeEach } from 'vitest'
import { inspectCommands } from './chatCommandsInspect'

const mockSendMessage = vi.fn()
const mockClientCall = vi.fn()

const mockProjectState: Record<string, unknown> = {
  state: 'none',
  client: null,
  worktreeId: null,
  task: null,
}

const mockGlobalState: Record<string, unknown> = {
  client: null,
}

vi.mock('../stores/projectStore', () => ({
  useProjectStore: { getState: () => mockProjectState },
}))

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: { getState: () => mockGlobalState },
}))

vi.mock('../stores/chatStore', () => ({
  useChatStore: { getState: () => ({ sendMessage: mockSendMessage }) },
}))

function setState(overrides: Record<string, unknown>) {
  Object.assign(mockProjectState, overrides)
}

function findCmd(name: string) {
  const cmd = inspectCommands.find((c) => c.name === name)
  if (!cmd) throw new Error(`Command "${name}" not found in inspectCommands`)
  return cmd
}

beforeEach(() => {
  vi.clearAllMocks()
  mockClientCall.mockReset()
  setState({ state: 'none', client: null, worktreeId: null, task: null })
  mockGlobalState.client = null
})

// ---------------------------------------------------------------------------
// Structure
// ---------------------------------------------------------------------------

describe('inspectCommands structure', () => {
  it('exports a non-empty array of commands', () => {
    expect(inspectCommands.length).toBeGreaterThan(0)
  })

  it('every command has name, description, isAvailable, and execute', () => {
    for (const cmd of inspectCommands) {
      expect(cmd.name).toBeTruthy()
      expect(cmd.description).toBeTruthy()
      expect(typeof cmd.isAvailable).toBe('function')
      expect(typeof cmd.execute).toBe('function')
    }
  })

  it('has no duplicate command names', () => {
    const names = inspectCommands.map((c) => c.name)
    expect(new Set(names).size).toBe(names.length)
  })
})

// ---------------------------------------------------------------------------
// Availability
// ---------------------------------------------------------------------------

describe('inspectCommands availability', () => {
  it('/status is always available', () => {
    const cmd = findCmd('/status')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'implementing' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/explain is available when active and not loaded', () => {
    const cmd = findCmd('/explain')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'implementing' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/checkpoints is available when active', () => {
    const cmd = findCmd('/checkpoints')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/recap is available when active', () => {
    const cmd = findCmd('/recap')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'planned' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/diff is available when active', () => {
    const cmd = findCmd('/diff')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'implemented' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/show spec is available when active', () => {
    const cmd = findCmd('/show spec')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/list is available when active', () => {
    const cmd = findCmd('/list')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/jobs is always available', () => {
    const cmd = findCmd('/jobs')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/stats is always available', () => {
    const cmd = findCmd('/stats')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

describe('inspectCommands execution', () => {
  // /status
  it('/status returns no active task when state=none', async () => {
    setState({ state: 'none' })
    expect(await findCmd('/status').execute('')).toBe('No active task.')
  })

  it('/status returns state with task title', async () => {
    setState({ state: 'implementing', task: { title: 'Fix bug' } })
    expect(await findCmd('/status').execute('')).toBe('Current state: implementing — Fix bug')
  })

  it('/status returns state without title when task has none', async () => {
    setState({ state: 'loaded', task: {} })
    expect(await findCmd('/status').execute('')).toBe('Current state: loaded')
  })

  // /explain
  it('/explain sends a chat message with the worktree id', async () => {
    setState({ worktreeId: 'wt-123' })
    await expect(findCmd('/explain').execute('')).resolves.toBe('')
    expect(mockSendMessage).toHaveBeenCalledWith(expect.stringContaining('Explain'), 'wt-123')
  })

  it('/explain sends undefined worktree id when there is no active worktree', async () => {
    setState({ worktreeId: null })
    await findCmd('/explain').execute('')
    expect(mockSendMessage).toHaveBeenCalledWith(expect.any(String), undefined)
  })

  // /checkpoints
  it('/checkpoints returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/checkpoints').execute('')).toBe('Not connected.')
  })

  it('/checkpoints lists checkpoints', async () => {
    mockClientCall.mockResolvedValue({
      checkpoints: [{ sha: 'abc1234567', message: 'Plan done', timestamp: '2026-01-01' }],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await findCmd('/checkpoints').execute('')
    expect(result).toContain('abc1234')
    expect(result).toContain('Plan done')
  })

  it('/checkpoints returns no checkpoints when empty', async () => {
    mockClientCall.mockResolvedValue({ checkpoints: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/checkpoints').execute('')).toBe('No checkpoints.')
  })

  // /checkpoints goto
  it('/checkpoints goto returns usage when no sha', async () => {
    expect(await findCmd('/checkpoints goto').execute('')).toBe('Usage: /checkpoints goto <sha>')
  })

  it('/checkpoints goto returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/checkpoints goto').execute('abc')).toBe('Not connected.')
  })

  it('/checkpoints goto calls checkpoint.goto', async () => {
    mockClientCall.mockResolvedValue({})
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/checkpoints goto').execute('abc1234567')).toBe('Jumped to checkpoint abc1234.')
    expect(mockClientCall).toHaveBeenCalledWith('checkpoint.goto', { sha: 'abc1234567' })
  })

  // /recap
  it('/recap returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/recap').execute('')).toBe('Not connected.')
  })

  it('/recap returns recap string', async () => {
    mockClientCall.mockResolvedValue({ recap: 'Halfway done.' })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/recap').execute('')).toBe('Halfway done.')
  })

  it('/recap returns fallback when empty', async () => {
    mockClientCall.mockResolvedValue({ recap: '' })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/recap').execute('')).toBe('No recap available.')
  })

  // /diff
  it('/diff returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/diff').execute('')).toBe('Not connected.')
  })

  it('/diff shows changes', async () => {
    mockClientCall.mockResolvedValue({ diff: '+new\n-old' })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/diff').execute('')).toBe('+new\n-old')
  })

  it('/diff returns no changes when empty', async () => {
    mockClientCall.mockResolvedValue({ diff: '' })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/diff').execute('')).toBe('No changes.')
  })

  // /show spec
  it('/show spec returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/show spec').execute('')).toBe('Not connected.')
  })

  it('/show spec returns spec content', async () => {
    mockClientCall.mockResolvedValue({ specifications: [{ content: 'Spec A' }, { content: 'Spec B' }] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/show spec').execute('')).toBe('Spec A\n---\n\nSpec B')
  })

  it('/show spec returns no specification when empty', async () => {
    mockClientCall.mockResolvedValue({ specifications: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/show spec').execute('')).toBe('No specification available.')
  })

  // /show plan
  it('/show plan returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/show plan').execute('')).toBe('Not connected.')
  })

  it('/show plan returns plan content', async () => {
    mockClientCall.mockResolvedValue({ plans: [{ content: 'Step 1' }] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/show plan').execute('')).toBe('Step 1')
  })

  it('/show plan returns no plan when empty', async () => {
    mockClientCall.mockResolvedValue({ plans: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/show plan').execute('')).toBe('No plan available.')
  })

  // /list search
  it('/list search returns usage when no query', async () => {
    expect(await findCmd('/list search').execute('')).toBe('Usage: /list search <query>')
  })

  it('/list search returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/list search').execute('auth')).toBe('Not connected.')
  })

  it('/list search returns formatted tasks', async () => {
    mockClientCall.mockResolvedValue({ tasks: [{ id: 'abcdef1234567890', title: 'Fix auth', state: 'implemented' }] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/list search').execute('auth')).toBe('abcdef12 [implemented] Fix auth')
  })

  it('/list search returns no matching when empty', async () => {
    mockClientCall.mockResolvedValue({ tasks: [] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/list search').execute('xyz')).toBe('No matching tasks.')
  })

  // /list
  it('/list returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/list').execute('')).toBe('Not connected.')
  })

  it('/list returns formatted task history', async () => {
    mockClientCall.mockResolvedValue({ tasks: [{ id: 'abcdef1234567890', title: 'Task 1', state: 'submitted' }] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/list').execute('')).toBe('abcdef12 [submitted] Task 1')
  })

  it('/list returns no history when empty', async () => {
    mockClientCall.mockResolvedValue({ tasks: [] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/list').execute('')).toBe('No task history.')
  })

  // /eventlog
  it('/eventlog returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/eventlog').execute('')).toBe('Not connected.')
  })

  it('/eventlog returns formatted events', async () => {
    mockClientCall.mockResolvedValue({
      events: [{ type: 'phase.start', timestamp: '2026-01-01T00:00:00Z', message: 'Planning' }],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/eventlog').execute('')).toBe('[2026-01-01T00:00:00Z] phase.start: Planning')
  })

  it('/eventlog returns no events when empty', async () => {
    mockClientCall.mockResolvedValue({ events: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/eventlog').execute('')).toBe('No events.')
  })

  // /jobs
  it('/jobs returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/jobs').execute('')).toBe('Not connected to global socket.')
  })

  it('/jobs returns formatted jobs', async () => {
    mockClientCall.mockResolvedValue({ jobs: [{ id: 'job12345678', type: 'implement', status: 'running' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/jobs').execute('')).toBe('job12345 [running] implement')
  })

  it('/jobs returns no jobs when empty', async () => {
    mockClientCall.mockResolvedValue({ jobs: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/jobs').execute('')).toBe('No jobs.')
  })

  // /stats
  it('/stats returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/stats').execute('')).toBe('Not connected to global socket.')
  })

  it('/stats returns JSON metrics', async () => {
    mockClientCall.mockResolvedValue({ tasks_completed: 5 })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/stats').execute('')
    expect(JSON.parse(result)).toEqual({ tasks_completed: 5 })
  })
})
