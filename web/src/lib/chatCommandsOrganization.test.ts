import { describe, it, expect, vi, beforeEach } from 'vitest'
import { organizationCommands } from './chatCommandsOrganization'

const mockClientCall = vi.fn()

const mockProjectState: Record<string, unknown> = {
  state: 'none',
  client: null,
  taskQueue: [],
  forks: [],
  queueTask: vi.fn(),
  dequeueTask: vi.fn(),
  loadQueue: vi.fn(),
  reorderQueue: vi.fn(),
  createFork: vi.fn(),
  listForks: vi.fn(),
  compareForks: vi.fn(),
  selectFork: vi.fn(),
}

const mockGlobalState: Record<string, unknown> = {
  client: null,
  batchAction: vi.fn(),
}

vi.mock('../stores/projectStore', () => ({
  useProjectStore: { getState: () => mockProjectState },
}))

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: { getState: () => mockGlobalState },
}))

function setState(overrides: Record<string, unknown>) {
  Object.assign(mockProjectState, overrides)
}

function findCmd(name: string) {
  const cmd = organizationCommands.find(c => c.name === name)
  if (!cmd) throw new Error(`Command "${name}" not found in organizationCommands`)
  return cmd
}

beforeEach(() => {
  vi.clearAllMocks()
  mockClientCall.mockReset()
  setState({
    state: 'none',
    client: null,
    taskQueue: [],
    forks: [],
  })
  mockGlobalState.client = null
})

// ---------------------------------------------------------------------------
// Structure
// ---------------------------------------------------------------------------

describe('organizationCommands structure', () => {
  it('exports a non-empty array of commands', () => {
    expect(organizationCommands.length).toBeGreaterThan(0)
  })

  it('every command has name, description, isAvailable, and execute', () => {
    for (const cmd of organizationCommands) {
      expect(cmd.name).toBeTruthy()
      expect(cmd.description).toBeTruthy()
      expect(typeof cmd.isAvailable).toBe('function')
      expect(typeof cmd.execute).toBe('function')
    }
  })

  it('has no duplicate command names', () => {
    const names = organizationCommands.map(c => c.name)
    expect(new Set(names).size).toBe(names.length)
  })
})

// ---------------------------------------------------------------------------
// Availability
// ---------------------------------------------------------------------------

describe('organizationCommands availability', () => {
  it('/tag add is available when active', () => {
    const cmd = findCmd('/tag add')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/tags is available when active', () => {
    const cmd = findCmd('/tags')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'implementing' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/queue add is always available', () => {
    const cmd = findCmd('/queue add')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/fork create is available when active', () => {
    const cmd = findCmd('/fork create')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/group create is always available', () => {
    const cmd = findCmd('/group create')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/batch is always available', () => {
    const cmd = findCmd('/batch')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Tag commands execution
// ---------------------------------------------------------------------------

describe('tag commands execution', () => {
  it('/tag add returns usage when no args', async () => {
    setState({ state: 'loaded' })
    expect(await findCmd('/tag add').execute('')).toBe('Usage: /tag add <name>')
  })

  it('/tag add returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/tag add').execute('urgent')).toBe('Not connected.')
  })

  it('/tag add calls RPC client', async () => {
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/tag add').execute('urgent')).toBe('Tag "urgent" added.')
    expect(mockClientCall).toHaveBeenCalledWith('task.tag', { action: 'add', tag: 'urgent' })
  })

  it('/tag remove returns usage when no args', async () => {
    expect(await findCmd('/tag remove').execute('')).toBe('Usage: /tag remove <name>')
  })

  it('/tag remove returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/tag remove').execute('urgent')).toBe('Not connected.')
  })

  it('/tag remove calls RPC client', async () => {
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/tag remove').execute('urgent')).toBe('Tag "urgent" removed.')
    expect(mockClientCall).toHaveBeenCalledWith('task.tag', { action: 'remove', tag: 'urgent' })
  })

  it('/tags returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/tags').execute('')).toBe('Not connected.')
  })

  it('/tags returns comma-separated tags', async () => {
    mockClientCall.mockResolvedValue({ tags: ['urgent', 'frontend'] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/tags').execute('')).toBe('Tags: urgent, frontend')
  })

  it('/tags returns no tags when empty', async () => {
    mockClientCall.mockResolvedValue({ tags: [] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/tags').execute('')).toBe('No tags.')
  })
})

// ---------------------------------------------------------------------------
// Queue commands execution
// ---------------------------------------------------------------------------

describe('queue commands execution', () => {
  it('/queue add returns usage when no args', async () => {
    expect(await findCmd('/queue add').execute('')).toBe('Usage: /queue add <source>')
  })

  it('/queue add queues a task', async () => {
    await findCmd('/queue add').execute('github:repo#1')
    expect(mockProjectState.queueTask).toHaveBeenCalledWith('github:repo#1')
  })

  it('/queue remove returns usage when no id', async () => {
    expect(await findCmd('/queue remove').execute('')).toBe('Usage: /queue remove <id>')
  })

  it('/queue remove calls dequeueTask', async () => {
    await findCmd('/queue remove').execute('task-123')
    expect(mockProjectState.dequeueTask).toHaveBeenCalledWith('task-123')
  })

  it('/queue returns empty when no tasks', async () => {
    mockProjectState.taskQueue = []
    expect(await findCmd('/queue').execute('')).toBe('Queue is empty.')
  })

  it('/queue returns formatted list when tasks exist', async () => {
    mockProjectState.taskQueue = [{ id: 'abcdef1234567890', title: 'Fix bug', source: 'github:repo#1' }]
    const result = await findCmd('/queue').execute('')
    expect(result).toContain('abcdef12')
    expect(result).toContain('Fix bug')
  })

  it('/queue list calls loadQueue and returns list', async () => {
    mockProjectState.taskQueue = [{ id: 'abcdef1234567890', title: '', source: 'file:task.md' }]
    const result = await findCmd('/queue list').execute('')
    expect(mockProjectState.loadQueue).toHaveBeenCalledOnce()
    expect(result).toContain('file:task.md')
  })

  it('/queue reorder returns usage when args are wrong', async () => {
    expect(await findCmd('/queue reorder').execute('only-one')).toBe('Usage: /queue reorder <id> <position>')
    expect(await findCmd('/queue reorder').execute('')).toBe('Usage: /queue reorder <id> <position>')
  })

  it('/queue reorder returns error for non-positive position', async () => {
    expect(await findCmd('/queue reorder').execute('task-1 0')).toBe('Position must be a positive number.')
    expect(await findCmd('/queue reorder').execute('task-1 abc')).toBe('Position must be a positive number.')
  })

  it('/queue reorder calls reorderQueue', async () => {
    await findCmd('/queue reorder').execute('task-1 3')
    expect(mockProjectState.reorderQueue).toHaveBeenCalledWith('task-1', 3)
  })
})

// ---------------------------------------------------------------------------
// Fork commands execution
// ---------------------------------------------------------------------------

describe('fork commands execution', () => {
  it('/fork create returns usage when no label', async () => {
    setState({ state: 'loaded' })
    expect(await findCmd('/fork create').execute('')).toBe('Usage: /fork create <label>')
  })

  it('/fork create calls createFork', async () => {
    await findCmd('/fork create').execute('experiment')
    expect(mockProjectState.createFork).toHaveBeenCalledWith('experiment')
  })

  it('/fork list returns no active forks when empty', async () => {
    setState({ state: 'loaded' })
    mockProjectState.forks = []
    expect(await findCmd('/fork list').execute('')).toBe('No active forks.')
  })

  it('/fork list returns formatted forks', async () => {
    setState({ state: 'loaded' })
    mockProjectState.forks = [{ id: 'fork12345678', label: 'experiment', state: 'active' }]
    const result = await findCmd('/fork list').execute('')
    expect(result).toContain('fork1234')
    expect(result).toContain('experiment')
    expect(mockProjectState.listForks).toHaveBeenCalledOnce()
  })

  it('/fork compare returns JSON when result is an object', async () => {
    setState({ state: 'loaded' });
    (mockProjectState.compareForks as ReturnType<typeof vi.fn>).mockResolvedValue({ winner: 'fork-a' })
    const result = await findCmd('/fork compare').execute('')
    expect(JSON.parse(result)).toEqual({ winner: 'fork-a' })
  })

  it('/fork compare formats array results', async () => {
    setState({ state: 'loaded' });
    (mockProjectState.compareForks as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 'fork12345678', label: 'A', state: 'active', summary: 'Good approach' },
    ])
    const result = await findCmd('/fork compare').execute('')
    expect(result).toContain('fork1234')
    expect(result).toContain('Good approach')
  })

  it('/fork compare returns no comparison data when null', async () => {
    setState({ state: 'loaded' });
    (mockProjectState.compareForks as ReturnType<typeof vi.fn>).mockResolvedValue(null)
    expect(await findCmd('/fork compare').execute('')).toBe('No comparison data.')
  })

  it('/fork select returns usage when no id', async () => {
    setState({ state: 'loaded' })
    expect(await findCmd('/fork select').execute('')).toBe('Usage: /fork select <fork-id>')
  })

  it('/fork select calls selectFork and truncates id', async () => {
    setState({ state: 'loaded' })
    expect(await findCmd('/fork select').execute('fork12345678')).toBe('Switched to fork fork1234.')
    expect(mockProjectState.selectFork).toHaveBeenCalledWith('fork12345678')
  })
})

// ---------------------------------------------------------------------------
// Group commands execution
// ---------------------------------------------------------------------------

describe('group commands execution', () => {
  it('/group create returns usage when no label', async () => {
    expect(await findCmd('/group create').execute('')).toBe('Usage: /group create <label>')
  })

  it('/group create returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/group create').execute('my-group')).toBe('Not connected to global socket.')
  })

  it('/group create calls taskgroup.create', async () => {
    mockClientCall.mockResolvedValue({ id: 'grp-abc' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/group create').execute('my-group')).toBe('Group created: grp-abc')
    expect(mockClientCall).toHaveBeenCalledWith('taskgroup.create', { label: 'my-group' })
  })

  it('/group list returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/group list').execute('')).toBe('Not connected to global socket.')
  })

  it('/group list returns formatted groups', async () => {
    mockClientCall.mockResolvedValue({ groups: [{ id: 'grp1234567890', label: 'frontend' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/group list').execute('')).toBe('grp12345 — frontend')
  })

  it('/group list returns no groups when empty', async () => {
    mockClientCall.mockResolvedValue({ groups: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/group list').execute('')).toBe('No groups.')
  })

  it('/group status returns usage when no id', async () => {
    expect(await findCmd('/group status').execute('')).toBe('Usage: /group status <group-id>')
  })

  it('/group status returns JSON', async () => {
    mockClientCall.mockResolvedValue({ id: 'grp-1', tasks: 3 })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/group status').execute('grp-1')
    expect(JSON.parse(result)).toEqual({ id: 'grp-1', tasks: 3 })
  })

  it('/group add returns usage when insufficient args', async () => {
    expect(await findCmd('/group add').execute('only-one')).toBe('Usage: /group add <group-id> <task-id>')
    expect(await findCmd('/group add').execute('')).toBe('Usage: /group add <group-id> <task-id>')
  })

  it('/group add calls taskgroup.add', async () => {
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/group add').execute('grp12345678 task-1')).toBe('Task added to group grp12345.')
    expect(mockClientCall).toHaveBeenCalledWith('taskgroup.add', { id: 'grp12345678', task_id: 'task-1' })
  })

  it('/group submit returns usage when no id', async () => {
    expect(await findCmd('/group submit').execute('')).toBe('Usage: /group submit <group-id>')
  })

  it('/group submit calls taskgroup.submit', async () => {
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/group submit').execute('grp12345678')).toBe('Group grp12345 submitted.')
  })

  it('/group remove returns usage when no id', async () => {
    expect(await findCmd('/group remove').execute('')).toBe('Usage: /group remove <group-id>')
  })

  it('/group remove calls taskgroup.remove', async () => {
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/group remove').execute('grp12345678')).toBe('Group grp12345 removed.')
  })
})

// ---------------------------------------------------------------------------
// Batch command execution
// ---------------------------------------------------------------------------

describe('batch command execution', () => {
  it('/batch returns usage when no action', async () => {
    expect(await findCmd('/batch').execute('')).toContain('Usage')
  })

  it('/batch calls batchAction and formats result', async () => {
    const mockBatchAction = vi.fn().mockResolvedValue({ succeeded: 3, total: 5 })
    mockGlobalState.batchAction = mockBatchAction
    expect(await findCmd('/batch').execute('plan')).toBe('Batch plan: 3/5 succeeded.')
    expect(mockBatchAction).toHaveBeenCalledWith('plan')
  })
})
