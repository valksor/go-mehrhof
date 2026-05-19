import { describe, it, expect, vi, beforeEach } from 'vitest'
import { controlCommands } from './chatCommandsControl'

const mockClientCall = vi.fn()

const mockProjectState: Record<string, unknown> = {
  state: 'none',
  checkpoints: [],
  redoStack: [],
  client: null,
  undo: vi.fn(),
  redo: vi.fn(),
  stop: vi.fn(),
  abort: vi.fn(),
  reset: vi.fn(),
  retry: vi.fn(),
  update: vi.fn(),
}

vi.mock('../stores/projectStore', () => ({
  useProjectStore: { getState: () => mockProjectState },
}))

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: { getState: () => ({}) },
}))

function setState(overrides: Record<string, unknown>) {
  Object.assign(mockProjectState, overrides)
}

function findCmd(name: string) {
  const cmd = controlCommands.find((c) => c.name === name)
  if (!cmd) throw new Error(`Command "${name}" not found in controlCommands`)
  return cmd
}

beforeEach(() => {
  vi.clearAllMocks()
  mockClientCall.mockReset()
  setState({
    state: 'none',
    checkpoints: [],
    redoStack: [],
    client: null,
  })
})

// ---------------------------------------------------------------------------
// Structure
// ---------------------------------------------------------------------------

describe('controlCommands structure', () => {
  it('exports a non-empty array of commands', () => {
    expect(controlCommands.length).toBeGreaterThan(0)
  })

  it('every command has name, description, isAvailable, and execute', () => {
    for (const cmd of controlCommands) {
      expect(cmd.name).toBeTruthy()
      expect(cmd.description).toBeTruthy()
      expect(typeof cmd.isAvailable).toBe('function')
      expect(typeof cmd.execute).toBe('function')
    }
  })

  it('has no duplicate command names', () => {
    const names = controlCommands.map((c) => c.name)
    expect(new Set(names).size).toBe(names.length)
  })
})

// ---------------------------------------------------------------------------
// Availability
// ---------------------------------------------------------------------------

describe('controlCommands availability', () => {
  it('/undo is available when checkpoints exist', () => {
    const cmd = findCmd('/undo')
    setState({ checkpoints: [] })
    expect(cmd.isAvailable()).toBe(false)
    setState({ checkpoints: ['cp1'] })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/redo is available when redoStack is non-empty', () => {
    const cmd = findCmd('/redo')
    setState({ redoStack: [] })
    expect(cmd.isAvailable()).toBe(false)
    setState({ redoStack: ['cp1'] })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/stop is available during active phases', () => {
    const cmd = findCmd('/stop')
    for (const s of ['planning', 'implementing', 'simplifying', 'optimizing', 'reviewing']) {
      setState({ state: s })
      expect(cmd.isAvailable()).toBe(true)
    }
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/abort is available when active but not in none, loaded, or submitted', () => {
    const cmd = findCmd('/abort')
    for (const s of ['none', 'loaded', 'submitted']) {
      setState({ state: s })
      expect(cmd.isAvailable()).toBe(false)
    }
    setState({ state: 'implementing' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/reset is available when active (state !== none)', () => {
    const cmd = findCmd('/reset')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/retry is available only when state=failed', () => {
    const cmd = findCmd('/retry')
    setState({ state: 'failed' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/update is available in loaded, planned, or implemented', () => {
    const cmd = findCmd('/update')
    for (const s of ['loaded', 'planned', 'implemented']) {
      setState({ state: s })
      expect(cmd.isAvailable()).toBe(true)
    }
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'submitted' })
    expect(cmd.isAvailable()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

describe('controlCommands execution', () => {
  it('/undo calls store undo', async () => {
    const cmd = findCmd('/undo')
    await expect(cmd.execute('')).resolves.toBe('Undone to previous checkpoint.')
    expect(mockProjectState.undo).toHaveBeenCalledOnce()
  })

  it('/redo calls store redo', async () => {
    const cmd = findCmd('/redo')
    await expect(cmd.execute('')).resolves.toBe('Redone to next checkpoint.')
    expect(mockProjectState.redo).toHaveBeenCalledOnce()
  })

  it('/stop calls store stop', async () => {
    const cmd = findCmd('/stop')
    await expect(cmd.execute('')).resolves.toBe('Operation stopped.')
    expect(mockProjectState.stop).toHaveBeenCalledOnce()
  })

  it('/abort calls store abort', async () => {
    const cmd = findCmd('/abort')
    await expect(cmd.execute('')).resolves.toBe('Operation aborted.')
    expect(mockProjectState.abort).toHaveBeenCalledOnce()
  })

  it('/reset calls store reset', async () => {
    const cmd = findCmd('/reset')
    await expect(cmd.execute('')).resolves.toBe('Task reset.')
    expect(mockProjectState.reset).toHaveBeenCalledOnce()
  })

  it('/retry calls store retry', async () => {
    const cmd = findCmd('/retry')
    await expect(cmd.execute('')).resolves.toBe('Retrying failed phase.')
    expect(mockProjectState.retry).toHaveBeenCalledOnce()
  })

  it('/update returns specification-generated message when changed and regenerated', async () => {
    const cmd = findCmd('/update')
    ;(mockProjectState.update as ReturnType<typeof vi.fn>).mockResolvedValue({
      changed: true,
      specification_generated: true,
    })
    await expect(cmd.execute('')).resolves.toBe('Task updated from source — new specification generated.')
  })

  it('/update returns content-updated message when changed without new specification', async () => {
    const cmd = findCmd('/update')
    ;(mockProjectState.update as ReturnType<typeof vi.fn>).mockResolvedValue({
      changed: true,
      specification_generated: false,
    })
    await expect(cmd.execute('')).resolves.toBe('Task content updated from source.')
  })

  it('/update returns already-up-to-date message when nothing changed', async () => {
    const cmd = findCmd('/update')
    ;(mockProjectState.update as ReturnType<typeof vi.fn>).mockResolvedValue({ changed: false })
    await expect(cmd.execute('')).resolves.toBe('Task is already up to date.')
  })
})
