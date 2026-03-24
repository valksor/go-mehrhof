import { describe, it, expect, vi, beforeEach } from 'vitest'
import { parseCommand, getAvailableCommands, COMMANDS, MODAL_COMMANDS } from './chatCommands'

// Mock stores so getState() returns controllable values
const mockProjectState: Record<string, unknown> = {
  state: 'none',
  checkpoints: [],
  redoStack: [],
  client: null,
  worktreeId: null,
  task: null,
  quickStart: vi.fn(),
  plan: vi.fn(),
  implement: vi.fn(),
  simplify: vi.fn(),
  optimize: vi.fn(),
  review: vi.fn(),
  undo: vi.fn(),
  redo: vi.fn(),
  stop: vi.fn(),
  abort: vi.fn(),
  update: vi.fn(),
}

vi.mock('../stores/projectStore', () => ({
  useProjectStore: { getState: () => mockProjectState },
}))

vi.mock('../stores/chatStore', () => ({
  useChatStore: { getState: () => ({ sendMessage: vi.fn() }) },
}))

function setState(overrides: Record<string, unknown>) {
  Object.assign(mockProjectState, overrides)
}

beforeEach(() => {
  vi.clearAllMocks()
  setState({
    state: 'none',
    checkpoints: [],
    redoStack: [],
    client: null,
    worktreeId: null,
    task: null,
  })
})

// ---------------------------------------------------------------------------
// parseCommand
// ---------------------------------------------------------------------------

describe('parseCommand', () => {
  it('returns null for non-slash input', () => {
    expect(parseCommand('hello')).toBeNull()
    expect(parseCommand('')).toBeNull()
    expect(parseCommand('no slash here')).toBeNull()
  })

  it('parses modal commands', () => {
    const result = parseCommand('/submit')
    expect(result).toMatchObject({ type: 'modal', input: '/submit', args: '' })
    expect(result!.modalCommand!.modal).toBe('submit')
  })

  it('parses modal command with args', () => {
    const result = parseCommand('/abandon some reason')
    expect(result).toMatchObject({ type: 'modal', args: 'some reason' })
    expect(result!.modalCommand!.modal).toBe('abandon')
  })

  it('parses action commands', () => {
    const result = parseCommand('/plan')
    expect(result).toMatchObject({ type: 'action', args: '' })
    expect(result!.command!.name).toBe('/plan')
  })

  it('parses /quick with args', () => {
    const result = parseCommand('/quick github:owner/repo#123')
    expect(result).toMatchObject({ type: 'action', args: 'github:owner/repo#123' })
    expect(result!.command!.name).toBe('/quick')
  })

  it('uses longest-match: /review fix beats /review', () => {
    const result = parseCommand('/review fix')
    expect(result!.command!.name).toBe('/review fix')
  })

  it('parses /review alone', () => {
    const result = parseCommand('/review')
    expect(result!.command!.name).toBe('/review')
  })

  it('returns unknown for unrecognized slash command', () => {
    const result = parseCommand('/nonexistent')
    expect(result).toMatchObject({ type: 'unknown', args: '', input: '/nonexistent' })
  })

  it('parses /implement! as re-run variant', () => {
    const result = parseCommand('/implement!')
    expect(result!.command!.name).toBe('/implement!')
  })

  it('parses /tag add with tag name', () => {
    const result = parseCommand('/tag add urgent')
    expect(result).toMatchObject({ type: 'action', args: 'urgent' })
    expect(result!.command!.name).toBe('/tag add')
  })

  it('parses /tags as action', () => {
    const result = parseCommand('/tags')
    expect(result).toMatchObject({ type: 'action', args: '' })
    expect(result!.command!.name).toBe('/tags')
  })
})

// ---------------------------------------------------------------------------
// COMMANDS availability
// ---------------------------------------------------------------------------

describe('COMMANDS availability', () => {
  it('/quick is available only when state=none', () => {
    const quick = COMMANDS.find(c => c.name === '/quick')!
    setState({ state: 'none' })
    expect(quick.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(quick.isAvailable()).toBe(false)
  })

  it('/plan is available only when state=loaded', () => {
    const plan = COMMANDS.find(c => c.name === '/plan')!
    setState({ state: 'loaded' })
    expect(plan.isAvailable()).toBe(true)
    setState({ state: 'none' })
    expect(plan.isAvailable()).toBe(false)
  })

  it('/implement is available only when state=planned', () => {
    const impl = COMMANDS.find(c => c.name === '/implement')!
    setState({ state: 'planned' })
    expect(impl.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(impl.isAvailable()).toBe(false)
  })

  it('/stop is available during active phases', () => {
    const stop = COMMANDS.find(c => c.name === '/stop')!
    for (const s of ['planning', 'implementing', 'simplifying', 'optimizing', 'reviewing']) {
      setState({ state: s })
      expect(stop.isAvailable()).toBe(true)
    }
    setState({ state: 'none' })
    expect(stop.isAvailable()).toBe(false)
  })

  it('/undo is available when checkpoints exist', () => {
    const undo = COMMANDS.find(c => c.name === '/undo')!
    setState({ checkpoints: [] })
    expect(undo.isAvailable()).toBe(false)
    setState({ checkpoints: ['cp1'] })
    expect(undo.isAvailable()).toBe(true)
  })

  it('/redo is available when redoStack is non-empty', () => {
    const redo = COMMANDS.find(c => c.name === '/redo')!
    setState({ redoStack: [] })
    expect(redo.isAvailable()).toBe(false)
    setState({ redoStack: ['cp1'] })
    expect(redo.isAvailable()).toBe(true)
  })

  it('/status is always available', () => {
    const status = COMMANDS.find(c => c.name === '/status')!
    setState({ state: 'none' })
    expect(status.isAvailable()).toBe(true)
    setState({ state: 'implementing' })
    expect(status.isAvailable()).toBe(true)
  })

  it('/update is available in loaded, planned, or implemented', () => {
    const update = COMMANDS.find(c => c.name === '/update')!
    for (const s of ['loaded', 'planned', 'implemented']) {
      setState({ state: s })
      expect(update.isAvailable()).toBe(true)
    }
    setState({ state: 'none' })
    expect(update.isAvailable()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// MODAL_COMMANDS availability
// ---------------------------------------------------------------------------

describe('MODAL_COMMANDS availability', () => {
  it('/submit is available only when state=reviewing', () => {
    const submit = MODAL_COMMANDS.find(c => c.name === '/submit')!
    setState({ state: 'reviewing' })
    expect(submit.isAvailable()).toBe(true)
    setState({ state: 'implemented' })
    expect(submit.isAvailable()).toBe(false)
  })

  it('/finish is available only when state=submitted', () => {
    const finish = MODAL_COMMANDS.find(c => c.name === '/finish')!
    setState({ state: 'submitted' })
    expect(finish.isAvailable()).toBe(true)
    setState({ state: 'none' })
    expect(finish.isAvailable()).toBe(false)
  })

  it('/abandon is available when active', () => {
    const abandon = MODAL_COMMANDS.find(c => c.name === '/abandon')!
    setState({ state: 'loaded' })
    expect(abandon.isAvailable()).toBe(true)
    setState({ state: 'none' })
    expect(abandon.isAvailable()).toBe(false)
  })

  it('/delete is available when active', () => {
    const del = MODAL_COMMANDS.find(c => c.name === '/delete')!
    setState({ state: 'implementing' })
    expect(del.isAvailable()).toBe(true)
    setState({ state: 'none' })
    expect(del.isAvailable()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// getAvailableCommands
// ---------------------------------------------------------------------------

describe('getAvailableCommands', () => {
  it('returns commands that are available in current state', () => {
    setState({ state: 'none' })
    const cmds = getAvailableCommands('')
    const names = cmds.map(c => c.name)
    expect(names).toContain('/quick')
    expect(names).toContain('/status')
    expect(names).not.toContain('/plan')
  })

  it('filters by name query', () => {
    setState({ state: 'implemented' })
    const cmds = getAvailableCommands('simp')
    expect(cmds.length).toBeGreaterThan(0)
    expect(cmds.every(c => c.name.includes('simp') || c.description.toLowerCase().includes('simp'))).toBe(true)
  })

  it('filters by description query', () => {
    setState({ state: 'loaded' })
    const cmds = getAvailableCommands('planning')
    expect(cmds.some(c => c.name === '/plan')).toBe(true)
  })

  it('returns all available when filter is empty', () => {
    setState({ state: 'loaded' })
    const cmds = getAvailableCommands('')
    expect(cmds.length).toBeGreaterThan(3)
  })
})

// ---------------------------------------------------------------------------
// Command execution
// ---------------------------------------------------------------------------

describe('command execution', () => {
  it('/status returns current state', async () => {
    const status = COMMANDS.find(c => c.name === '/status')!
    setState({ state: 'none' })
    expect(await status.execute('')).toBe('No active task.')

    setState({ state: 'implementing', task: { title: 'Fix bug' } })
    expect(await status.execute('')).toBe('Current state: implementing — Fix bug')
  })

  it('/quick returns usage when no args', async () => {
    const quick = COMMANDS.find(c => c.name === '/quick')!
    const result = await quick.execute('')
    expect(result).toContain('Usage')
  })

  it('/quick calls quickStart with source', async () => {
    const quick = COMMANDS.find(c => c.name === '/quick')!
    await quick.execute('github:owner/repo#1')
    expect(mockProjectState.quickStart).toHaveBeenCalledWith('github:owner/repo#1')
  })

  it('/tag add returns usage when no args', async () => {
    const tagAdd = COMMANDS.find(c => c.name === '/tag add')!
    setState({ state: 'loaded' })
    expect(await tagAdd.execute('')).toBe('Usage: /tag add <name>')
  })

  it('/tag add returns not connected when no client', async () => {
    const tagAdd = COMMANDS.find(c => c.name === '/tag add')!
    setState({ state: 'loaded', client: null })
    expect(await tagAdd.execute('urgent')).toBe('Not connected.')
  })
})
