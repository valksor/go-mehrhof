import { describe, it, expect, vi, beforeEach } from 'vitest'
import { workflowCommands } from './chatCommandsWorkflow'

const mockProjectState: Record<string, unknown> = {
  state: 'none',
  quickStart: vi.fn(),
  plan: vi.fn(),
  implement: vi.fn(),
  simplify: vi.fn(),
  optimize: vi.fn(),
  review: vi.fn(),
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
  const cmd = workflowCommands.find(c => c.name === name)
  if (!cmd) throw new Error(`Command "${name}" not found in workflowCommands`)
  return cmd
}

beforeEach(() => {
  vi.clearAllMocks()
  setState({ state: 'none' })
})

// ---------------------------------------------------------------------------
// Structure
// ---------------------------------------------------------------------------

describe('workflowCommands structure', () => {
  it('exports a non-empty array of commands', () => {
    expect(workflowCommands.length).toBeGreaterThan(0)
  })

  it('every command has name, description, isAvailable, and execute', () => {
    for (const cmd of workflowCommands) {
      expect(cmd.name).toBeTruthy()
      expect(cmd.description).toBeTruthy()
      expect(typeof cmd.isAvailable).toBe('function')
      expect(typeof cmd.execute).toBe('function')
    }
  })

  it('has no duplicate command names', () => {
    const names = workflowCommands.map(c => c.name)
    expect(new Set(names).size).toBe(names.length)
  })
})

// ---------------------------------------------------------------------------
// Availability
// ---------------------------------------------------------------------------

describe('workflowCommands availability', () => {
  it('/quick is available only when state=none', () => {
    const cmd = findCmd('/quick')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/plan is available only when state=loaded', () => {
    const cmd = findCmd('/plan')
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/plan! is available only when state=planned', () => {
    const cmd = findCmd('/plan!')
    setState({ state: 'planned' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/implement is available only when state=planned', () => {
    const cmd = findCmd('/implement')
    setState({ state: 'planned' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/implement! is available only when state=implemented', () => {
    const cmd = findCmd('/implement!')
    setState({ state: 'implemented' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'planned' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/simplify is available only when state=implemented', () => {
    const cmd = findCmd('/simplify')
    setState({ state: 'implemented' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/optimize is available only when state=implemented', () => {
    const cmd = findCmd('/optimize')
    setState({ state: 'implemented' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/review is available only when state=implemented', () => {
    const cmd = findCmd('/review')
    setState({ state: 'implemented' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/review fix is available only when state=implemented', () => {
    const cmd = findCmd('/review fix')
    setState({ state: 'implemented' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'planned' })
    expect(cmd.isAvailable()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

describe('workflowCommands execution', () => {
  it('/quick returns usage when no args', async () => {
    const cmd = findCmd('/quick')
    const result = await cmd.execute('')
    expect(result).toContain('Usage')
  })

  it('/quick calls quickStart with source', async () => {
    const cmd = findCmd('/quick')
    await cmd.execute('github:owner/repo#1')
    expect(mockProjectState.quickStart).toHaveBeenCalledWith('github:owner/repo#1')
  })

  it('/quick returns success message', async () => {
    const cmd = findCmd('/quick')
    const result = await cmd.execute('github:owner/repo#1')
    expect(result).toContain('Quick fix started')
  })

  it('/plan calls store plan', async () => {
    const cmd = findCmd('/plan')
    await expect(cmd.execute('')).resolves.toBe('Planning started.')
    expect(mockProjectState.plan).toHaveBeenCalledOnce()
  })

  it('/plan! calls store plan for re-planning', async () => {
    const cmd = findCmd('/plan!')
    await expect(cmd.execute('')).resolves.toBe('Re-planning started.')
    expect(mockProjectState.plan).toHaveBeenCalledOnce()
  })

  it('/implement calls store implement', async () => {
    const cmd = findCmd('/implement')
    await expect(cmd.execute('')).resolves.toBe('Implementation started.')
    expect(mockProjectState.implement).toHaveBeenCalledOnce()
  })

  it('/implement! calls store implement for re-implementation', async () => {
    const cmd = findCmd('/implement!')
    await expect(cmd.execute('')).resolves.toBe('Re-implementation started.')
    expect(mockProjectState.implement).toHaveBeenCalledOnce()
  })

  it('/simplify calls store simplify', async () => {
    const cmd = findCmd('/simplify')
    await expect(cmd.execute('')).resolves.toBe('Simplification started.')
    expect(mockProjectState.simplify).toHaveBeenCalledOnce()
  })

  it('/optimize calls store optimize', async () => {
    const cmd = findCmd('/optimize')
    await expect(cmd.execute('')).resolves.toBe('Optimization started.')
    expect(mockProjectState.optimize).toHaveBeenCalledOnce()
  })

  it('/review calls store review with approve=true', async () => {
    const cmd = findCmd('/review')
    await expect(cmd.execute('')).resolves.toBe('Review started.')
    expect(mockProjectState.review).toHaveBeenCalledWith({ approve: true })
  })

  it('/review fix calls store review with fix=true', async () => {
    const cmd = findCmd('/review fix')
    await expect(cmd.execute('')).resolves.toBe('Review with fixes started.')
    expect(mockProjectState.review).toHaveBeenCalledWith({ fix: true })
  })
})
