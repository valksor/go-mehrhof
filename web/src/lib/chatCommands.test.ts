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
    const update = COMMANDS.find(c => c.name === '/update')!;
    (mockProjectState.update as ReturnType<typeof vi.fn>).mockResolvedValue({ changed: true, specification_generated: true })

    await expect(update.execute('')).resolves.toBe('Task updated from source — new specification generated.')
  })

  it('/update returns content-updated message when content changes without new specification', async () => {
    const update = COMMANDS.find(c => c.name === '/update')!;
    (mockProjectState.update as ReturnType<typeof vi.fn>).mockResolvedValue({ changed: true, specification_generated: false })

    await expect(update.execute('')).resolves.toBe('Task content updated from source.')
  })

  it('/update returns already-up-to-date message when nothing changed', async () => {
    const update = COMMANDS.find(c => c.name === '/update')!;
    (mockProjectState.update as ReturnType<typeof vi.fn>).mockResolvedValue({ changed: false, specification_generated: false })

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

// ---------------------------------------------------------------------------
// Info commands — execution
// ---------------------------------------------------------------------------

describe('info commands execution', () => {
  // /checkpoints goto
  it('/checkpoints goto returns usage when no sha', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checkpoints goto')!
    expect(await cmd.execute('')).toBe('Usage: /checkpoints goto <sha>')
  })

  it('/checkpoints goto returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checkpoints goto')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('abc1234')).toBe('Not connected.')
  })

  it('/checkpoints goto calls checkpoint.goto', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checkpoints goto')!
    setState({ state: 'loaded', client: { call: mockClientCall } })
    mockClientCall.mockResolvedValue({})
    expect(await cmd.execute('abc1234567')).toBe('Jumped to checkpoint abc1234.')
    expect(mockClientCall).toHaveBeenCalledWith('checkpoint.goto', { sha: 'abc1234567' })
  })

  // /recap
  it('/recap returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/recap')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/recap returns recap string', async () => {
    const cmd = COMMANDS.find(c => c.name === '/recap')!
    mockClientCall.mockResolvedValue({ recap: 'Task is halfway through implementation.' })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('Task is halfway through implementation.')
  })

  it('/recap returns fallback when recap is empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/recap')!
    mockClientCall.mockResolvedValue({ recap: '' })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('No recap available.')
  })

  // /show spec
  it('/show spec returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/show spec')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/show spec returns spec content', async () => {
    const cmd = COMMANDS.find(c => c.name === '/show spec')!
    mockClientCall.mockResolvedValue({ specifications: [{ content: 'Spec A' }, { content: 'Spec B' }] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('Spec A\n---\n\nSpec B')
    expect(mockClientCall).toHaveBeenCalledWith('show.spec', {})
  })

  it('/show spec returns no specification when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/show spec')!
    mockClientCall.mockResolvedValue({ specifications: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('No specification available.')
  })

  // /show plan
  it('/show plan returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/show plan')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/show plan returns plan content', async () => {
    const cmd = COMMANDS.find(c => c.name === '/show plan')!
    mockClientCall.mockResolvedValue({ plans: [{ content: 'Plan step 1' }] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('Plan step 1')
    expect(mockClientCall).toHaveBeenCalledWith('show.plan', {})
  })

  it('/show plan returns no plan when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/show plan')!
    mockClientCall.mockResolvedValue({ plans: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('No plan available.')
  })

  // /list search
  it('/list search returns usage when no query', async () => {
    const cmd = COMMANDS.find(c => c.name === '/list search')!
    expect(await cmd.execute('')).toBe('Usage: /list search <query>')
  })

  it('/list search returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/list search')!
    setState({ client: null })
    expect(await cmd.execute('auth')).toBe('Not connected.')
  })

  it('/list search returns formatted tasks', async () => {
    const cmd = COMMANDS.find(c => c.name === '/list search')!
    mockClientCall.mockResolvedValue({ tasks: [{ id: 'abcdef1234567890', title: 'Fix auth', state: 'implemented' }] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('auth')).toBe('abcdef12 [implemented] Fix auth')
    expect(mockClientCall).toHaveBeenCalledWith('task.search', { query: 'auth' })
  })

  it('/list search returns no matching when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/list search')!
    mockClientCall.mockResolvedValue({ tasks: [] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('xyz')).toBe('No matching tasks.')
  })

  // /list
  it('/list returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/list')!
    setState({ client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/list returns formatted task history', async () => {
    const cmd = COMMANDS.find(c => c.name === '/list')!
    mockClientCall.mockResolvedValue({ tasks: [{ id: 'abcdef1234567890', title: 'Task 1', state: 'submitted' }] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('abcdef12 [submitted] Task 1')
    expect(mockClientCall).toHaveBeenCalledWith('task.history', {})
  })

  it('/list returns no history when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/list')!
    mockClientCall.mockResolvedValue({ tasks: [] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('No task history.')
  })

  // /eventlog
  it('/eventlog returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/eventlog')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/eventlog returns formatted events', async () => {
    const cmd = COMMANDS.find(c => c.name === '/eventlog')!
    mockClientCall.mockResolvedValue({ events: [{ type: 'phase.start', timestamp: '2026-01-01T00:00:00Z', message: 'Planning' }] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('[2026-01-01T00:00:00Z] phase.start: Planning')
    expect(mockClientCall).toHaveBeenCalledWith('eventlog.query', { limit: 20 })
  })

  it('/eventlog returns no events when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/eventlog')!
    mockClientCall.mockResolvedValue({ events: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('No events.')
  })

  it('/eventlog formats events without message', async () => {
    const cmd = COMMANDS.find(c => c.name === '/eventlog')!
    mockClientCall.mockResolvedValue({ events: [{ type: 'checkpoint', timestamp: '2026-01-01' }] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('[2026-01-01] checkpoint')
  })

  // /jobs
  it('/jobs returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/jobs')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/jobs returns formatted jobs', async () => {
    const cmd = COMMANDS.find(c => c.name === '/jobs')!
    mockClientCall.mockResolvedValue({ jobs: [{ id: 'job12345678', type: 'implement', status: 'running' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('job12345 [running] implement')
    expect(mockClientCall).toHaveBeenCalledWith('jobs.list', {})
  })

  it('/jobs returns no jobs when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/jobs')!
    mockClientCall.mockResolvedValue({ jobs: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('No jobs.')
  })

  // /stats
  it('/stats returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/stats')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/stats returns JSON metrics', async () => {
    const cmd = COMMANDS.find(c => c.name === '/stats')!
    mockClientCall.mockResolvedValue({ tasks_completed: 5, uptime: 3600 })
    mockGlobalState.client = { call: mockClientCall }
    const result = await cmd.execute('')
    expect(JSON.parse(result)).toEqual({ tasks_completed: 5, uptime: 3600 })
    expect(mockClientCall).toHaveBeenCalledWith('metrics', {})
  })
})

// ---------------------------------------------------------------------------
// Org commands — execution
// ---------------------------------------------------------------------------

describe('org commands execution', () => {
  // /queue remove
  it('/queue remove returns usage when no id', async () => {
    const cmd = COMMANDS.find(c => c.name === '/queue remove')!
    expect(await cmd.execute('')).toBe('Usage: /queue remove <id>')
  })

  it('/queue remove calls dequeueTask', async () => {
    const cmd = COMMANDS.find(c => c.name === '/queue remove')!
    await cmd.execute('task-123')
    expect(mockProjectState.dequeueTask).toHaveBeenCalledWith('task-123')
  })

  // /queue list and /queue
  it('/queue list calls loadQueue and returns formatted list', async () => {
    const cmd = COMMANDS.find(c => c.name === '/queue list')!
    mockProjectState.taskQueue = [{ id: 'abcdef1234567890', title: 'Fix bug', source: 'github:repo#1' }]
    await cmd.execute('')
    expect(mockProjectState.loadQueue).toHaveBeenCalledOnce()
  })

  it('/queue returns empty when no tasks', async () => {
    const cmd = COMMANDS.find(c => c.name === '/queue')!
    mockProjectState.taskQueue = []
    expect(await cmd.execute('')).toBe('Queue is empty.')
  })

  it('/queue returns formatted list when tasks exist', async () => {
    const cmd = COMMANDS.find(c => c.name === '/queue')!
    mockProjectState.taskQueue = [{ id: 'abcdef1234567890', title: 'Fix bug', source: 'github:repo#1' }]
    const result = await cmd.execute('')
    expect(result).toContain('abcdef12')
    expect(result).toContain('Fix bug')
  })

  // /fork list
  it('/fork list returns no active forks when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/fork list')!
    setState({ state: 'loaded' })
    mockProjectState.forks = []
    expect(await cmd.execute('')).toBe('No active forks.')
  })

  it('/fork list returns formatted forks', async () => {
    const cmd = COMMANDS.find(c => c.name === '/fork list')!
    setState({ state: 'loaded' })
    mockProjectState.forks = [{ id: 'fork12345678', label: 'experiment', state: 'active' }]
    const result = await cmd.execute('')
    expect(result).toContain('fork1234')
    expect(result).toContain('experiment')
    expect(mockProjectState.listForks).toHaveBeenCalledOnce()
  })

  // /fork compare
  it('/fork compare returns JSON result', async () => {
    const cmd = COMMANDS.find(c => c.name === '/fork compare')!
    setState({ state: 'loaded' });
    (mockProjectState.compareForks as ReturnType<typeof vi.fn>).mockResolvedValue({ winner: 'fork-a' })
    const result = await cmd.execute('')
    expect(JSON.parse(result)).toEqual({ winner: 'fork-a' })
  })

  // /fork select
  it('/fork select returns usage when no id', async () => {
    const cmd = COMMANDS.find(c => c.name === '/fork select')!
    setState({ state: 'loaded' })
    expect(await cmd.execute('')).toBe('Usage: /fork select <fork-id>')
  })

  it('/fork select calls selectFork', async () => {
    const cmd = COMMANDS.find(c => c.name === '/fork select')!
    setState({ state: 'loaded' })
    await cmd.execute('fork12345678')
    expect(mockProjectState.selectFork).toHaveBeenCalledWith('fork12345678')
  })

  it('/fork select truncates id in response', async () => {
    const cmd = COMMANDS.find(c => c.name === '/fork select')!
    setState({ state: 'loaded' })
    expect(await cmd.execute('fork12345678')).toBe('Switched to fork fork1234.')
  })

  // /group create
  it('/group create returns usage when no label', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group create')!
    expect(await cmd.execute('')).toBe('Usage: /group create <label>')
  })

  it('/group create returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group create')!
    mockGlobalState.client = null
    expect(await cmd.execute('my-group')).toBe('Not connected to global socket.')
  })

  it('/group create calls taskgroup.create', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group create')!
    mockClientCall.mockResolvedValue({ id: 'grp-abc' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('my-group')).toBe('Group created: grp-abc')
    expect(mockClientCall).toHaveBeenCalledWith('taskgroup.create', { label: 'my-group' })
  })

  // /group list
  it('/group list returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group list')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/group list returns formatted groups', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group list')!
    mockClientCall.mockResolvedValue({ groups: [{ id: 'grp1234567890', label: 'frontend' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('grp12345 — frontend')
  })

  it('/group list returns no groups when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group list')!
    mockClientCall.mockResolvedValue({ groups: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('No groups.')
  })

  // /group status
  it('/group status returns usage when no id', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group status')!
    expect(await cmd.execute('')).toBe('Usage: /group status <group-id>')
  })

  it('/group status returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group status')!
    mockGlobalState.client = null
    expect(await cmd.execute('grp-1')).toBe('Not connected to global socket.')
  })

  it('/group status returns JSON status', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group status')!
    mockClientCall.mockResolvedValue({ id: 'grp-1', tasks: 3, completed: 1 })
    mockGlobalState.client = { call: mockClientCall }
    const result = await cmd.execute('grp-1')
    expect(JSON.parse(result)).toEqual({ id: 'grp-1', tasks: 3, completed: 1 })
    expect(mockClientCall).toHaveBeenCalledWith('taskgroup.status', { id: 'grp-1' })
  })

  // /group add
  it('/group add returns usage when insufficient args', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group add')!
    expect(await cmd.execute('only-one')).toBe('Usage: /group add <group-id> <task-id>')
    expect(await cmd.execute('')).toBe('Usage: /group add <group-id> <task-id>')
  })

  it('/group add returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group add')!
    mockGlobalState.client = null
    expect(await cmd.execute('grp-1 task-1')).toBe('Not connected to global socket.')
  })

  it('/group add calls taskgroup.add', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group add')!
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('grp12345678 task-1')).toBe('Task added to group grp12345.')
    expect(mockClientCall).toHaveBeenCalledWith('taskgroup.add', { id: 'grp12345678', task_id: 'task-1' })
  })

  // /group submit
  it('/group submit returns usage when no id', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group submit')!
    expect(await cmd.execute('')).toBe('Usage: /group submit <group-id>')
  })

  it('/group submit returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group submit')!
    mockGlobalState.client = null
    expect(await cmd.execute('grp-1')).toBe('Not connected to global socket.')
  })

  it('/group submit calls taskgroup.submit', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group submit')!
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('grp12345678')).toBe('Group grp12345 submitted.')
    expect(mockClientCall).toHaveBeenCalledWith('taskgroup.submit', { id: 'grp12345678' })
  })

  // /group remove
  it('/group remove returns usage when no id', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group remove')!
    expect(await cmd.execute('')).toBe('Usage: /group remove <group-id>')
  })

  it('/group remove returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group remove')!
    mockGlobalState.client = null
    expect(await cmd.execute('grp-1')).toBe('Not connected to global socket.')
  })

  it('/group remove calls taskgroup.remove', async () => {
    const cmd = COMMANDS.find(c => c.name === '/group remove')!
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('grp12345678')).toBe('Group grp12345 removed.')
    expect(mockClientCall).toHaveBeenCalledWith('taskgroup.remove', { id: 'grp12345678' })
  })

  // /batch
  it('/batch returns usage when no action', async () => {
    const cmd = COMMANDS.find(c => c.name === '/batch')!
    expect(await cmd.execute('')).toContain('Usage')
  })

  it('/batch calls batchAction and formats result', async () => {
    const cmd = COMMANDS.find(c => c.name === '/batch')!
    const mockBatchAction = vi.fn().mockResolvedValue({ succeeded: 3, total: 5 })
    mockGlobalState.batchAction = mockBatchAction
    expect(await cmd.execute('plan')).toBe('Batch plan: 3/5 succeeded.')
    expect(mockBatchAction).toHaveBeenCalledWith('plan')
  })

  // /activity
  it('/activity returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/activity')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/activity returns formatted entries', async () => {
    const cmd = COMMANDS.find(c => c.name === '/activity')!
    mockClientCall.mockResolvedValue({ entries: [{ method: 'plan', timestamp: '2026-01-01', duration_ms: 42 }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('[2026-01-01] plan (42ms)')
    expect(mockClientCall).toHaveBeenCalledWith('activity.query', { limit: 20 })
  })

  it('/activity returns no activity when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/activity')!
    mockClientCall.mockResolvedValue({ entries: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('No activity.')
  })

  // /audit
  it('/audit returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/audit')!
    setState({ client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/audit returns JSON audit trail', async () => {
    const cmd = COMMANDS.find(c => c.name === '/audit')!
    mockClientCall.mockResolvedValue({ entries: [{ action: 'plan', timestamp: '2026-01-01' }] })
    setState({ client: { call: mockClientCall } })
    const result = await cmd.execute('')
    expect(JSON.parse(result)).toEqual({ entries: [{ action: 'plan', timestamp: '2026-01-01' }] })
    expect(mockClientCall).toHaveBeenCalledWith('task.export', { format: 'audit' })
  })

  // /report
  it('/report returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/report')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/report returns report content', async () => {
    const cmd = COMMANDS.find(c => c.name === '/report')!
    mockClientCall.mockResolvedValue({ report: '## Compliance Report\nAll checks passed.' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('## Compliance Report\nAll checks passed.')
    expect(mockClientCall).toHaveBeenCalledWith('report.generate', {})
  })

  it('/report returns fallback when report is empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/report')!
    mockClientCall.mockResolvedValue({ report: '' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('Report generated.')
  })

  // /backup
  it('/backup returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/backup')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/backup returns backup path', async () => {
    const cmd = COMMANDS.find(c => c.name === '/backup')!
    mockClientCall.mockResolvedValue({ path: '/tmp/backup-2026.tar.gz' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('Backup created: /tmp/backup-2026.tar.gz')
    expect(mockClientCall).toHaveBeenCalledWith('backup.create', {})
  })

  // /access
  it('/access returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/access')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/access returns formatted tokens', async () => {
    const cmd = COMMANDS.find(c => c.name === '/access')!
    mockClientCall.mockResolvedValue({ tokens: [{ id: 'tok12345678', name: 'dev-token', created: '2026-01-01' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('tok12345 — dev-token (2026-01-01)')
    expect(mockClientCall).toHaveBeenCalledWith('access.token.list', {})
  })

  it('/access returns no tokens when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/access')!
    mockClientCall.mockResolvedValue({ tokens: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('No access tokens.')
  })
})

// ---------------------------------------------------------------------------
// Governance commands — execution
// ---------------------------------------------------------------------------

describe('governance commands execution', () => {
  // /approve
  it('/approve returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/approve')!
    setState({ state: 'waiting', client: null })
    expect(await cmd.execute('submit')).toBe('Not connected.')
  })

  it('/approve returns usage when no event', async () => {
    const cmd = COMMANDS.find(c => c.name === '/approve')!
    setState({ state: 'waiting' })
    expect(await cmd.execute('')).toBe('Usage: /approve <event> (e.g. submit, implement)')
  })

  it('/approve calls approve RPC with event', async () => {
    const cmd = COMMANDS.find(c => c.name === '/approve')!
    mockClientCall.mockResolvedValue({})
    setState({ state: 'waiting', client: { call: mockClientCall } })
    expect(await cmd.execute('submit')).toBe('Approved: submit')
    expect(mockClientCall).toHaveBeenCalledWith('approve', { event: 'submit' })
  })

  // /checklist check
  it('/checklist check returns usage when no item', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checklist check')!
    setState({ state: 'loaded' })
    expect(await cmd.execute('')).toBe('Usage: /checklist check <item-name>')
  })

  it('/checklist check returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checklist check')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('tests-pass')).toBe('Not connected.')
  })

  it('/checklist check calls review.checklist.check with item', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checklist check')!
    mockClientCall.mockResolvedValue({})
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('tests-pass')).toBe('Checked: tests-pass')
    expect(mockClientCall).toHaveBeenCalledWith('review.checklist.check', { item: 'tests-pass' })
  })

  // /checklist uncheck
  it('/checklist uncheck returns usage when no item', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checklist uncheck')!
    setState({ state: 'loaded' })
    expect(await cmd.execute('')).toBe('Usage: /checklist uncheck <item-name>')
  })

  it('/checklist uncheck returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checklist uncheck')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('tests-pass')).toBe('Not connected.')
  })

  it('/checklist uncheck calls review.checklist.uncheck with item', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checklist uncheck')!
    mockClientCall.mockResolvedValue({})
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('tests-pass')).toBe('Unchecked: tests-pass')
    expect(mockClientCall).toHaveBeenCalledWith('review.checklist.uncheck', { item: 'tests-pass' })
  })

  // /checklist
  it('/checklist returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checklist')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/checklist returns no items when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checklist')!
    mockClientCall.mockResolvedValue({ required: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('No checklist items.')
  })

  it('/checklist returns formatted checked/unchecked items', async () => {
    const cmd = COMMANDS.find(c => c.name === '/checklist')!
    mockClientCall.mockResolvedValue({
      required: ['Tests pass', 'Docs updated'],
      checked: ['Tests pass'],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await cmd.execute('')
    expect(result).toContain('✓ 1. Tests pass')
    expect(result).toContain('☐ 2. Docs updated')
    expect(mockClientCall).toHaveBeenCalledWith('review.checklist.get', {})
  })

  // /quality
  it('/quality returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/quality')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/quality returns status when no findings', async () => {
    const cmd = COMMANDS.find(c => c.name === '/quality')!
    mockClientCall.mockResolvedValue({ status: 'passed', findings: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('Quality: passed')
    expect(mockClientCall).toHaveBeenCalledWith('quality.respond', { action: 'run' })
  })

  it('/quality returns status with findings', async () => {
    const cmd = COMMANDS.find(c => c.name === '/quality')!
    mockClientCall.mockResolvedValue({
      status: 'failed',
      findings: [{ message: 'Unused import', severity: 'warning' }],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await cmd.execute('')
    expect(result).toBe('Quality: failed\n[warning] Unused import')
  })

  // /ci
  it('/ci returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/ci')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/ci returns status without checks', async () => {
    const cmd = COMMANDS.find(c => c.name === '/ci')!
    mockClientCall.mockResolvedValue({ status: 'running' })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('CI: running')
    expect(mockClientCall).toHaveBeenCalledWith('ci.status', {})
  })

  it('/ci returns status with url and checks', async () => {
    const cmd = COMMANDS.find(c => c.name === '/ci')!
    mockClientCall.mockResolvedValue({
      status: 'completed',
      url: 'https://ci.example.com/123',
      checks: [
        { name: 'lint', status: 'passed' },
        { name: 'test', status: 'failed' },
      ],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await cmd.execute('')
    expect(result).toContain('CI: completed (https://ci.example.com/123)')
    expect(result).toContain('lint')
    expect(result).toContain('test')
  })

  // /policy
  it('/policy returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/policy')!
    setState({ state: 'loaded', client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/policy returns compliant when no violations', async () => {
    const cmd = COMMANDS.find(c => c.name === '/policy')!
    mockClientCall.mockResolvedValue({ compliant: true })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('Policy: compliant.')
    expect(mockClientCall).toHaveBeenCalledWith('policy.check', {})
  })

  it('/policy returns violations when not compliant', async () => {
    const cmd = COMMANDS.find(c => c.name === '/policy')!
    mockClientCall.mockResolvedValue({
      compliant: false,
      violations: [{ rule: 'no-force-push', message: 'Force push is not allowed' }],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await cmd.execute('')
    expect(result).toContain('Policy violations:')
    expect(result).toContain('no-force-push: Force push is not allowed')
  })
})

// ---------------------------------------------------------------------------
// File commands — execution
// ---------------------------------------------------------------------------

describe('file commands execution', () => {
  // /files search
  it('/files search returns usage when no pattern', async () => {
    const cmd = COMMANDS.find(c => c.name === '/files search')!
    expect(await cmd.execute('')).toBe('Usage: /files search <pattern>')
  })

  it('/files search returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/files search')!
    setState({ client: null })
    expect(await cmd.execute('*.ts')).toBe('Not connected.')
  })

  it('/files search returns matching files', async () => {
    const cmd = COMMANDS.find(c => c.name === '/files search')!
    mockClientCall.mockResolvedValue({ files: ['src/main.ts', 'src/utils.ts'] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('*.ts')).toBe('src/main.ts\nsrc/utils.ts')
    expect(mockClientCall).toHaveBeenCalledWith('files.search', { pattern: '*.ts' })
  })

  it('/files search returns no matching when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/files search')!
    mockClientCall.mockResolvedValue({ files: [] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('*.xyz')).toBe('No matching files.')
  })

  // /files
  it('/files returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/files')!
    setState({ client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/files returns file list', async () => {
    const cmd = COMMANDS.find(c => c.name === '/files')!
    mockClientCall.mockResolvedValue({ files: ['README.md', 'package.json'] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('README.md\npackage.json')
    expect(mockClientCall).toHaveBeenCalledWith('files.list', { path: '.' })
  })

  it('/files passes path argument', async () => {
    const cmd = COMMANDS.find(c => c.name === '/files')!
    mockClientCall.mockResolvedValue({ files: ['src/main.ts'] })
    setState({ client: { call: mockClientCall } })
    await cmd.execute('src')
    expect(mockClientCall).toHaveBeenCalledWith('files.list', { path: 'src' })
  })

  it('/files returns no files when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/files')!
    mockClientCall.mockResolvedValue({ files: [] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('No files.')
  })

  // /git status
  it('/git status returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/git status')!
    setState({ client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/git status returns branch with summary', async () => {
    const cmd = COMMANDS.find(c => c.name === '/git status')!
    mockClientCall.mockResolvedValue({ branch: 'main', has_changes: true, summary: 'M src/main.ts' })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('Branch: main\nM src/main.ts')
    expect(mockClientCall).toHaveBeenCalledWith('git.status', {})
  })

  it('/git status returns branch with has_changes flag', async () => {
    const cmd = COMMANDS.find(c => c.name === '/git status')!
    mockClientCall.mockResolvedValue({ branch: 'feature', has_changes: true })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('Branch: feature (has changes)')
  })

  it('/git status returns clean branch', async () => {
    const cmd = COMMANDS.find(c => c.name === '/git status')!
    mockClientCall.mockResolvedValue({ branch: 'main', has_changes: false })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('Branch: main (clean)')
  })

  // /git log
  it('/git log returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/git log')!
    setState({ client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/git log returns formatted commits', async () => {
    const cmd = COMMANDS.find(c => c.name === '/git log')!
    mockClientCall.mockResolvedValue({ entries: [{ sha: 'abc1234567890', message: 'Fix bug' }] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('abc1234 Fix bug')
    expect(mockClientCall).toHaveBeenCalledWith('git.log', { limit: 10 })
  })

  it('/git log returns no commits when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/git log')!
    mockClientCall.mockResolvedValue({ entries: [] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('No commits.')
  })

  // /codegraph search
  it('/codegraph search returns usage when no name', async () => {
    const cmd = COMMANDS.find(c => c.name === '/codegraph search')!
    expect(await cmd.execute('')).toBe('Usage: /codegraph search <symbol>')
  })

  it('/codegraph search returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/codegraph search')!
    setState({ client: null })
    expect(await cmd.execute('MyClass')).toBe('Not connected.')
  })

  it('/codegraph search returns formatted symbols', async () => {
    const cmd = COMMANDS.find(c => c.name === '/codegraph search')!
    mockClientCall.mockResolvedValue({ symbols: [{ name: 'MyClass', kind: 'class', file: 'src/main.ts', line: 42 }] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('MyClass')).toBe('class MyClass — src/main.ts:42')
    expect(mockClientCall).toHaveBeenCalledWith('codegraph.search', { name: 'MyClass' })
  })

  it('/codegraph search returns no symbols when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/codegraph search')!
    mockClientCall.mockResolvedValue({ symbols: [] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('NonExistent')).toBe('No symbols found.')
  })
})

// ---------------------------------------------------------------------------
// Memory commands — execution
// ---------------------------------------------------------------------------

describe('memory commands execution', () => {
  // /memory search
  it('/memory search returns usage when no query', async () => {
    const cmd = COMMANDS.find(c => c.name === '/memory search')!
    expect(await cmd.execute('')).toBe('Usage: /memory search <query>')
  })

  it('/memory search returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/memory search')!
    mockGlobalState.client = null
    expect(await cmd.execute('auth')).toBe('Not connected to global socket.')
  })

  it('/memory search returns formatted results', async () => {
    const cmd = COMMANDS.find(c => c.name === '/memory search')!
    mockClientCall.mockResolvedValue({ results: [{ content: 'Auth tokens usage', score: 0.95 }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('auth')).toBe('1. (0.95) Auth tokens usage')
    expect(mockClientCall).toHaveBeenCalledWith('memory.search', { query: 'auth', limit: 5 })
  })

  it('/memory search returns no results when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/memory search')!
    mockClientCall.mockResolvedValue({ results: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('xyz')).toBe('No results.')
  })

  // /memory stats
  it('/memory stats returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/memory stats')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/memory stats returns JSON stats', async () => {
    const cmd = COMMANDS.find(c => c.name === '/memory stats')!
    mockClientCall.mockResolvedValue({ entries: 42, size_mb: 1.5 })
    mockGlobalState.client = { call: mockClientCall }
    const result = await cmd.execute('')
    expect(JSON.parse(result)).toEqual({ entries: 42, size_mb: 1.5 })
    expect(mockClientCall).toHaveBeenCalledWith('memory.stats', {})
  })

  // /cache stats
  it('/cache stats returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/cache stats')!
    setState({ client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/cache stats returns JSON stats', async () => {
    const cmd = COMMANDS.find(c => c.name === '/cache stats')!
    mockClientCall.mockResolvedValue({ hits: 10, misses: 5 })
    setState({ client: { call: mockClientCall } })
    const result = await cmd.execute('')
    expect(JSON.parse(result)).toEqual({ hits: 10, misses: 5 })
    expect(mockClientCall).toHaveBeenCalledWith('cache.stats', {})
  })

  // /cache clear
  it('/cache clear returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/cache clear')!
    setState({ client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/cache clear calls cache.clear', async () => {
    const cmd = COMMANDS.find(c => c.name === '/cache clear')!
    mockClientCall.mockResolvedValue({})
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('Cache cleared.')
    expect(mockClientCall).toHaveBeenCalledWith('cache.clear', {})
  })
})

// ---------------------------------------------------------------------------
// Infra commands — execution
// ---------------------------------------------------------------------------

describe('infra commands execution', () => {
  // /changelog with valid refs
  it('/changelog with valid refs calls changelog.generate', async () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog')!
    mockClientCall.mockResolvedValue({ markdown: '## v2.0\n- Feature A' })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('v1.0..v2.0')).toBe('## v2.0\n- Feature A')
    expect(mockClientCall).toHaveBeenCalledWith('changelog.generate', { source: 'v1.0', target: 'v2.0' })
  })

  it('/changelog with refs and note passes note', async () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog')!
    mockClientCall.mockResolvedValue({ markdown: '## Frontend changes' })
    setState({ client: { call: mockClientCall } })
    await cmd.execute('v1.0..v2.0 only frontend')
    expect(mockClientCall).toHaveBeenCalledWith('changelog.generate', { source: 'v1.0', target: 'v2.0', note: 'only frontend' })
  })

  it('/changelog returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog')!
    setState({ client: null })
    expect(await cmd.execute('v1.0..v2.0')).toBe('Not connected.')
  })

  it('/changelog returns fallback when no commits', async () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog')!
    mockClientCall.mockResolvedValue({ markdown: '' })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('v1.0..v2.0')).toBe('No commits between v1.0 and v2.0')
  })

  // /changelog full
  it('/changelog full returns usage for bad format', async () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog full')!
    expect(await cmd.execute('')).toContain('Usage')
    expect(await cmd.execute('norange')).toContain('Usage')
  })

  it('/changelog full with valid refs calls with full:true', async () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog full')!
    mockClientCall.mockResolvedValue({ markdown: '## Full changelog' })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('v1.0..v2.0')).toBe('## Full changelog')
    expect(mockClientCall).toHaveBeenCalledWith('changelog.generate', { source: 'v1.0', target: 'v2.0', full: true })
  })

  it('/changelog full with note passes note and full', async () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog full')!
    mockClientCall.mockResolvedValue({ markdown: 'changes' })
    setState({ client: { call: mockClientCall } })
    await cmd.execute('v1.0..v2.0 my note')
    expect(mockClientCall).toHaveBeenCalledWith('changelog.generate', { source: 'v1.0', target: 'v2.0', full: true, note: 'my note' })
  })

  it('/changelog full returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/changelog full')!
    setState({ client: null })
    expect(await cmd.execute('v1.0..v2.0')).toBe('Not connected.')
  })

  // /workers
  it('/workers returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/workers')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/workers returns formatted worker list', async () => {
    const cmd = COMMANDS.find(c => c.name === '/workers')!
    mockClientCall.mockResolvedValue({ workers: [{ name: 'worker-1', state: 'idle' }, { name: 'worker-2', state: 'busy' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('worker-1 [idle]\nworker-2 [busy]')
    expect(mockClientCall).toHaveBeenCalledWith('workers.list', {})
  })

  it('/workers returns no workers when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/workers')!
    mockClientCall.mockResolvedValue({ workers: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('No workers.')
  })

  // /discover
  it('/discover returns not connected when no client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/discover')!
    setState({ client: null })
    expect(await cmd.execute('')).toBe('Not connected.')
  })

  it('/discover returns formatted commands', async () => {
    const cmd = COMMANDS.find(c => c.name === '/discover')!
    mockClientCall.mockResolvedValue({ commands: [{ name: 'make build', source: 'Makefile' }] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('[Makefile] make build')
    expect(mockClientCall).toHaveBeenCalledWith('discovery.scan', {})
  })

  it('/discover returns no commands when empty', async () => {
    const cmd = COMMANDS.find(c => c.name === '/discover')!
    mockClientCall.mockResolvedValue({ commands: [] })
    setState({ client: { call: mockClientCall } })
    expect(await cmd.execute('')).toBe('No project commands found.')
  })

  // /diagnose
  it('/diagnose returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/diagnose')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/diagnose returns formatted checks', async () => {
    const cmd = COMMANDS.find(c => c.name === '/diagnose')!
    mockClientCall.mockResolvedValue({ checks: [{ name: 'socket', status: 'passed' }, { name: 'disk', status: 'failed', detail: 'low space' }] })
    mockGlobalState.client = { call: mockClientCall }
    const result = await cmd.execute('')
    expect(result).toContain('socket')
    expect(result).toContain('disk: low space')
    expect(mockClientCall).toHaveBeenCalledWith('system.diagnose', {})
  })

  it('/diagnose returns OK when no checks', async () => {
    const cmd = COMMANDS.find(c => c.name === '/diagnose')!
    mockClientCall.mockResolvedValue({ checks: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('Diagnostics: OK')
  })

  // /security scan
  it('/security scan returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/security scan')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/security scan returns no issues when clean', async () => {
    const cmd = COMMANDS.find(c => c.name === '/security scan')!
    mockClientCall.mockResolvedValue({ issues: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('No security issues found.')
    expect(mockClientCall).toHaveBeenCalledWith('security.scan', {})
  })

  it('/security scan returns formatted issues', async () => {
    const cmd = COMMANDS.find(c => c.name === '/security scan')!
    mockClientCall.mockResolvedValue({ issues: [{ severity: 'high', message: 'Hardcoded secret found' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('[high] Hardcoded secret found')
  })

  // /remote approve
  it('/remote approve calls approveRemote', async () => {
    const cmd = COMMANDS.find(c => c.name === '/remote approve')!
    await cmd.execute('')
    expect(mockProjectState.approveRemote).toHaveBeenCalledOnce()
  })

  it('/remote approve returns PR approved', async () => {
    const cmd = COMMANDS.find(c => c.name === '/remote approve')!
    expect(await cmd.execute('')).toBe('PR approved.')
  })

  // /config check
  it('/config check returns not connected when no global client', async () => {
    const cmd = COMMANDS.find(c => c.name === '/config check')!
    mockGlobalState.client = null
    expect(await cmd.execute('')).toBe('Not connected to global socket.')
  })

  it('/config check returns no drift when clean', async () => {
    const cmd = COMMANDS.find(c => c.name === '/config check')!
    mockClientCall.mockResolvedValue({ drifts: [], count: 0 })
    mockGlobalState.client = { call: mockClientCall }
    expect(await cmd.execute('')).toBe('Configuration: no drift detected.')
    expect(mockClientCall).toHaveBeenCalledWith('config.check', {})
  })

  it('/config check returns drift details', async () => {
    const cmd = COMMANDS.find(c => c.name === '/config check')!
    mockClientCall.mockResolvedValue({
      drifts: [{ path: 'agent.model', expected: 'claude-4', actual: 'claude-3' }],
      count: 1,
    })
    mockGlobalState.client = { call: mockClientCall }
    const result = await cmd.execute('')
    expect(result).toContain('Configuration drift:')
    expect(result).toContain('agent.model: expected=claude-4, actual=claude-3')
  })
})
