import { describe, it, expect, vi, beforeEach } from 'vitest'
import { parseCommand, getAvailableCommands, COMMANDS, MODAL_COMMANDS } from './chatCommands'

// Mock stores so getState() returns controllable values
const mockSendMessage = vi.fn()
const mockClientCall = vi.fn()

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
  reset: vi.fn(),
  retry: vi.fn(),
  update: vi.fn(),
  queueTask: vi.fn(),
  dequeueTask: vi.fn(),
  loadQueue: vi.fn(),
  taskQueue: [],
  createFork: vi.fn(),
  listForks: vi.fn(),
  compareForks: vi.fn(),
  selectFork: vi.fn(),
  forks: [],
  approveRemote: vi.fn(),
  mergeRemote: vi.fn(),
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

beforeEach(() => {
  vi.clearAllMocks()
  mockClientCall.mockReset()
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
  it('/plan starts planning', async () => {
    const plan = COMMANDS.find(c => c.name === '/plan')!

    await expect(plan.execute('')).resolves.toBe('Planning started.')
    expect(mockProjectState.plan).toHaveBeenCalledOnce()
  })

  it('/plan! starts re-planning', async () => {
    const replan = COMMANDS.find(c => c.name === '/plan!')!

    await expect(replan.execute('')).resolves.toBe('Re-planning started.')
    expect(mockProjectState.plan).toHaveBeenCalledOnce()
  })

  it('/implement starts implementation', async () => {
    const implement = COMMANDS.find(c => c.name === '/implement')!

    await expect(implement.execute('')).resolves.toBe('Implementation started.')
    expect(mockProjectState.implement).toHaveBeenCalledOnce()
  })

  it('/implement! starts re-implementation', async () => {
    const reimplement = COMMANDS.find(c => c.name === '/implement!')!

    await expect(reimplement.execute('')).resolves.toBe('Re-implementation started.')
    expect(mockProjectState.implement).toHaveBeenCalledOnce()
  })

  it('/simplify starts simplification', async () => {
    const simplify = COMMANDS.find(c => c.name === '/simplify')!

    await expect(simplify.execute('')).resolves.toBe('Simplification started.')
    expect(mockProjectState.simplify).toHaveBeenCalledOnce()
  })

  it('/optimize starts optimization', async () => {
    const optimize = COMMANDS.find(c => c.name === '/optimize')!

    await expect(optimize.execute('')).resolves.toBe('Optimization started.')
    expect(mockProjectState.optimize).toHaveBeenCalledOnce()
  })

  it('/review starts review with approve=true', async () => {
    const review = COMMANDS.find(c => c.name === '/review')!

    await expect(review.execute('')).resolves.toBe('Review started.')
    expect(mockProjectState.review).toHaveBeenCalledWith({ approve: true })
  })

  it('/review fix starts review with fix=true', async () => {
    const reviewFix = COMMANDS.find(c => c.name === '/review fix')!

    await expect(reviewFix.execute('')).resolves.toBe('Review with fixes started.')
    expect(mockProjectState.review).toHaveBeenCalledWith({ fix: true })
  })

  it('/undo restores the previous checkpoint', async () => {
    const undo = COMMANDS.find(c => c.name === '/undo')!

    await expect(undo.execute('')).resolves.toBe('Undone to previous checkpoint.')
    expect(mockProjectState.undo).toHaveBeenCalledOnce()
  })

  it('/redo restores the next checkpoint', async () => {
    const redo = COMMANDS.find(c => c.name === '/redo')!

    await expect(redo.execute('')).resolves.toBe('Redone to next checkpoint.')
    expect(mockProjectState.redo).toHaveBeenCalledOnce()
  })

  it('/stop stops the current operation', async () => {
    const stop = COMMANDS.find(c => c.name === '/stop')!

    await expect(stop.execute('')).resolves.toBe('Operation stopped.')
    expect(mockProjectState.stop).toHaveBeenCalledOnce()
  })

  it('/abort aborts the current operation', async () => {
    const abort = COMMANDS.find(c => c.name === '/abort')!

    await expect(abort.execute('')).resolves.toBe('Operation aborted.')
    expect(mockProjectState.abort).toHaveBeenCalledOnce()
  })

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

  it('/update returns generated-specification message when content changes and regenerates', async () => {
    const update = COMMANDS.find(c => c.name === '/update')!
    vi.mocked(mockProjectState.update as () => Promise<unknown>).mockResolvedValue({ changed: true, specification_generated: true })

    await expect(update.execute('')).resolves.toBe('Task updated from source — new specification generated.')
  })

  it('/update returns content-updated message when content changes without new specification', async () => {
    const update = COMMANDS.find(c => c.name === '/update')!
    vi.mocked(mockProjectState.update as () => Promise<unknown>).mockResolvedValue({ changed: true, specification_generated: false })

    await expect(update.execute('')).resolves.toBe('Task content updated from source.')
  })

  it('/update returns already-up-to-date message when nothing changed', async () => {
    const update = COMMANDS.find(c => c.name === '/update')!
    vi.mocked(mockProjectState.update as () => Promise<unknown>).mockResolvedValue({ changed: false, specification_generated: false })

    await expect(update.execute('')).resolves.toBe('Task is already up to date.')
  })

  it('/explain sends a follow-up chat message with the active worktree id', async () => {
    const explain = COMMANDS.find(c => c.name === '/explain')!
    setState({ worktreeId: 'wt-123' })

    await expect(explain.execute('')).resolves.toBe('')
    expect(mockSendMessage).toHaveBeenCalledWith(
      'Explain what you did in the last action, why you made those choices, and any assumptions or constraints you encountered.',
      'wt-123'
    )
  })

  it('/explain sends undefined worktree id when there is no active worktree id', async () => {
    const explain = COMMANDS.find(c => c.name === '/explain')!
    setState({ worktreeId: null })

    await explain.execute('')
    expect(mockSendMessage).toHaveBeenCalledWith(
      'Explain what you did in the last action, why you made those choices, and any assumptions or constraints you encountered.',
      undefined
    )
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

  it('/tag add calls the RPC client when connected', async () => {
    const tagAdd = COMMANDS.find(c => c.name === '/tag add')!
    setState({ client: { call: mockClientCall } })

    await expect(tagAdd.execute('urgent')).resolves.toBe('Tag "urgent" added.')
    expect(mockClientCall).toHaveBeenCalledWith('task.tag', { action: 'add', tag: 'urgent' })
  })

  it('/tag remove returns usage when no args', async () => {
    const tagRemove = COMMANDS.find(c => c.name === '/tag remove')!

    await expect(tagRemove.execute('')).resolves.toBe('Usage: /tag remove <name>')
  })

  it('/tag remove returns not connected when no client', async () => {
    const tagRemove = COMMANDS.find(c => c.name === '/tag remove')!
    setState({ client: null })

    await expect(tagRemove.execute('urgent')).resolves.toBe('Not connected.')
  })

  it('/tag remove calls the RPC client when connected', async () => {
    const tagRemove = COMMANDS.find(c => c.name === '/tag remove')!
    setState({ client: { call: mockClientCall } })

    await expect(tagRemove.execute('urgent')).resolves.toBe('Tag "urgent" removed.')
    expect(mockClientCall).toHaveBeenCalledWith('task.tag', { action: 'remove', tag: 'urgent' })
  })

  it('/tags returns not connected when no client', async () => {
    const tags = COMMANDS.find(c => c.name === '/tags')!
    setState({ client: null })

    await expect(tags.execute('')).resolves.toBe('Not connected.')
  })

  it('/tags returns a comma-separated tag list when tags exist', async () => {
    const tags = COMMANDS.find(c => c.name === '/tags')!
    mockClientCall.mockResolvedValue({ tags: ['urgent', 'frontend'] })
    setState({ client: { call: mockClientCall } })

    await expect(tags.execute('')).resolves.toBe('Tags: urgent, frontend')
    expect(mockClientCall).toHaveBeenCalledWith('task.tag', { action: 'list' })
  })

  it('/tags returns no-tags message when the list is empty', async () => {
    const tags = COMMANDS.find(c => c.name === '/tags')!
    mockClientCall.mockResolvedValue({ tags: [] })
    setState({ client: { call: mockClientCall } })

    await expect(tags.execute('')).resolves.toBe('No tags.')
  })
})

// ---------------------------------------------------------------------------
// New commands — parsing
// ---------------------------------------------------------------------------

describe('new command parsing', () => {
  it('parses /reset as action', () => {
    const result = parseCommand('/reset')
    expect(result).toMatchObject({ type: 'action' })
    expect(result!.command!.name).toBe('/reset')
  })

  it('parses /retry as action', () => {
    const result = parseCommand('/retry')
    expect(result).toMatchObject({ type: 'action' })
    expect(result!.command!.name).toBe('/retry')
  })

  it('parses /checkpoints goto with sha', () => {
    const result = parseCommand('/checkpoints goto abc1234')
    expect(result!.command!.name).toBe('/checkpoints goto')
    expect(result!.args).toBe('abc1234')
  })

  it('parses /checkpoints alone', () => {
    const result = parseCommand('/checkpoints')
    expect(result!.command!.name).toBe('/checkpoints')
  })

  it('parses /diff', () => {
    const result = parseCommand('/diff')
    expect(result!.command!.name).toBe('/diff')
  })

  it('parses /show spec', () => {
    const result = parseCommand('/show spec')
    expect(result!.command!.name).toBe('/show spec')
  })

  it('parses /queue add with source', () => {
    const result = parseCommand('/queue add github:repo#1')
    expect(result!.command!.name).toBe('/queue add')
    expect(result!.args).toBe('github:repo#1')
  })

  it('parses /fork create with label', () => {
    const result = parseCommand('/fork create experiment')
    expect(result!.command!.name).toBe('/fork create')
    expect(result!.args).toBe('experiment')
  })

  it('parses /changelog with refs', () => {
    const result = parseCommand('/changelog v1.0..v2.0')
    expect(result!.command!.name).toBe('/changelog')
    expect(result!.args).toBe('v1.0..v2.0')
  })

  it('parses /changelog full with refs', () => {
    const result = parseCommand('/changelog full v1.0..v2.0')
    expect(result!.command!.name).toBe('/changelog full')
    expect(result!.args).toBe('v1.0..v2.0')
  })

  it('parses /changelog with refs and note', () => {
    const result = parseCommand('/changelog v1.0..v2.0 only frontend changes')
    expect(result!.command!.name).toBe('/changelog')
    expect(result!.args).toBe('v1.0..v2.0 only frontend changes')
  })

  it('parses /changelog full with refs and note', () => {
    const result = parseCommand('/changelog full v1.0..v2.0 my note')
    expect(result!.command!.name).toBe('/changelog full')
    expect(result!.args).toBe('v1.0..v2.0 my note')
  })

  it('parses /git status', () => {
    const result = parseCommand('/git status')
    expect(result!.command!.name).toBe('/git status')
  })

  it('parses /memory search with query', () => {
    const result = parseCommand('/memory search auth tokens')
    expect(result!.command!.name).toBe('/memory search')
    expect(result!.args).toBe('auth tokens')
  })

  it('parses /remote merge', () => {
    const result = parseCommand('/remote merge')
    expect(result!.command!.name).toBe('/remote merge')
  })

  it('parses /security scan', () => {
    const result = parseCommand('/security scan')
    expect(result!.command!.name).toBe('/security scan')
  })
})

// ---------------------------------------------------------------------------
// New commands — availability
// ---------------------------------------------------------------------------

describe('new commands availability', () => {
  it('/reset is available when active', () => {
    const reset = COMMANDS.find(c => c.name === '/reset')!
    setState({ state: 'loaded' })
    expect(reset.isAvailable()).toBe(true)
    setState({ state: 'none' })
    expect(reset.isAvailable()).toBe(false)
  })

  it('/retry is available when state=failed', () => {
    const retry = COMMANDS.find(c => c.name === '/retry')!
    setState({ state: 'failed' })
    expect(retry.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(retry.isAvailable()).toBe(false)
  })

  it('/approve is available when state=waiting', () => {
    const approve = COMMANDS.find(c => c.name === '/approve')!
    setState({ state: 'waiting' })
    expect(approve.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(approve.isAvailable()).toBe(false)
  })

  it('/remote approve is available when state=submitted', () => {
    const cmd = COMMANDS.find(c => c.name === '/remote approve')!
    setState({ state: 'submitted' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/changelog is always available', () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog')!
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/workers is always available', () => {
    const cmd = COMMANDS.find(c => c.name === '/workers')!
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// New commands — execution
// ---------------------------------------------------------------------------

describe('new commands execution', () => {
  it('/reset calls store reset', async () => {
    const reset = COMMANDS.find(c => c.name === '/reset')!
    await expect(reset.execute('')).resolves.toBe('Task reset.')
    expect(mockProjectState.reset).toHaveBeenCalledOnce()
  })

  it('/retry calls store retry', async () => {
    const retry = COMMANDS.find(c => c.name === '/retry')!
    await expect(retry.execute('')).resolves.toBe('Retrying failed phase.')
    expect(mockProjectState.retry).toHaveBeenCalledOnce()
  })

  it('/queue add returns usage when no args', async () => {
    const cmd = COMMANDS.find(c => c.name === '/queue add')!
    expect(await cmd.execute('')).toBe('Usage: /queue add <source>')
  })

  it('/queue add queues a task', async () => {
    const cmd = COMMANDS.find(c => c.name === '/queue add')!
    await cmd.execute('github:repo#1')
    expect(mockProjectState.queueTask).toHaveBeenCalledWith('github:repo#1')
  })

  it('/fork create returns usage when no args', async () => {
    const cmd = COMMANDS.find(c => c.name === '/fork create')!
    setState({ state: 'loaded' })
    expect(await cmd.execute('')).toBe('Usage: /fork create <label>')
  })

  it('/changelog returns usage for bad format', async () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog')!
    expect(await cmd.execute('norange')).toContain('Usage')
    expect(await cmd.execute('')).toContain('Usage')
  })

  it('/checkpoints returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checkpoints')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/checkpoints lists checkpoints', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checkpoints')!
    mockClientCall.mockResolvedValue({ checkpoints: [{ sha: 'abc1234567', message: 'Plan done', timestamp: '2026-01-01' }] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await cmd.execute('')
    expect(result).toContain('abc1234')
    expect(result).toContain('Plan done')
  })

  it('/diff shows changes', async () => {
    const cmd = COMMANDS.find(c => c.name === '/diff')!
    mockClientCall.mockResolvedValue({ diff: '+new line\n-old line' })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('+new line\n-old line')
  })

  it('/remote merge calls store mergeRemote', async () => {
    const cmd = COMMANDS.find(c => c.name === '/remote merge')!
    await cmd.execute('')
    expect(mockProjectState.mergeRemote).toHaveBeenCalledOnce()
  })
})
