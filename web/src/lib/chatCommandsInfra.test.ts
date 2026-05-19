import { describe, it, expect, vi, beforeEach } from 'vitest'
import { infraCommands } from './chatCommandsInfra'

const mockClientCall = vi.fn()

const mockProjectState: Record<string, unknown> = {
  state: 'none',
  client: null,
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

function setState(overrides: Record<string, unknown>) {
  Object.assign(mockProjectState, overrides)
}

function findCmd(name: string) {
  const cmd = infraCommands.find((c) => c.name === name)
  if (!cmd) throw new Error(`Command "${name}" not found in infraCommands`)
  return cmd
}

beforeEach(() => {
  vi.clearAllMocks()
  mockClientCall.mockReset()
  setState({ state: 'none', client: null })
  mockGlobalState.client = null
})

// ---------------------------------------------------------------------------
// Structure
// ---------------------------------------------------------------------------

describe('infraCommands structure', () => {
  it('exports a non-empty array of commands', () => {
    expect(infraCommands.length).toBeGreaterThan(0)
  })

  it('every command has name, description, isAvailable, and execute', () => {
    for (const cmd of infraCommands) {
      expect(cmd.name).toBeTruthy()
      expect(cmd.description).toBeTruthy()
      expect(typeof cmd.isAvailable).toBe('function')
      expect(typeof cmd.execute).toBe('function')
    }
  })

  it('has no duplicate command names', () => {
    const names = infraCommands.map((c) => c.name)
    expect(new Set(names).size).toBe(names.length)
  })
})

// ---------------------------------------------------------------------------
// Governance — availability
// ---------------------------------------------------------------------------

describe('governance commands availability', () => {
  it('/approve is available only when state=waiting', () => {
    const cmd = findCmd('/approve')
    setState({ state: 'waiting' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(false)
  })

  it('/checklist is available when active', () => {
    const cmd = findCmd('/checklist')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/quality is available when active', () => {
    const cmd = findCmd('/quality')
    setState({ state: 'none' })
    expect(cmd.isAvailable()).toBe(false)
    setState({ state: 'implementing' })
    expect(cmd.isAvailable()).toBe(true)
  })

  it('/remote approve is available only when state=submitted', () => {
    const cmd = findCmd('/remote approve')
    setState({ state: 'submitted' })
    expect(cmd.isAvailable()).toBe(true)
    setState({ state: 'loaded' })
    expect(cmd.isAvailable()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Governance — execution
// ---------------------------------------------------------------------------

describe('governance commands execution', () => {
  // /approve
  it('/approve returns usage when no event', async () => {
    setState({ state: 'waiting' })
    expect(await findCmd('/approve').execute('')).toBe('Usage: /approve <event> (e.g. submit, implement)')
  })

  it('/approve returns not connected when no client', async () => {
    setState({ state: 'waiting', client: null })
    expect(await findCmd('/approve').execute('submit')).toBe('Not connected.')
  })

  it('/approve calls approve RPC', async () => {
    mockClientCall.mockResolvedValue({})
    setState({ state: 'waiting', client: { call: mockClientCall } })
    expect(await findCmd('/approve').execute('submit')).toBe('Approved: submit')
    expect(mockClientCall).toHaveBeenCalledWith('approve', { event: 'submit' })
  })

  // /checklist check
  it('/checklist check returns usage when no item', async () => {
    setState({ state: 'loaded' })
    expect(await findCmd('/checklist check').execute('')).toBe('Usage: /checklist check <item-name>')
  })

  it('/checklist check returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/checklist check').execute('tests-pass')).toBe('Not connected.')
  })

  it('/checklist check calls review.checklist.check', async () => {
    mockClientCall.mockResolvedValue({})
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/checklist check').execute('tests-pass')).toBe('Checked: tests-pass')
    expect(mockClientCall).toHaveBeenCalledWith('review.checklist.check', { item: 'tests-pass' })
  })

  // /checklist uncheck
  it('/checklist uncheck returns usage when no item', async () => {
    setState({ state: 'loaded' })
    expect(await findCmd('/checklist uncheck').execute('')).toBe('Usage: /checklist uncheck <item-name>')
  })

  it('/checklist uncheck calls review.checklist.uncheck', async () => {
    mockClientCall.mockResolvedValue({})
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/checklist uncheck').execute('tests-pass')).toBe('Unchecked: tests-pass')
  })

  // /checklist
  it('/checklist returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/checklist').execute('')).toBe('Not connected.')
  })

  it('/checklist returns no items when empty', async () => {
    mockClientCall.mockResolvedValue({ required: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/checklist').execute('')).toBe('No checklist items.')
  })

  it('/checklist returns formatted checked/unchecked items', async () => {
    mockClientCall.mockResolvedValue({ required: ['Tests pass', 'Docs updated'], checked: ['Tests pass'] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await findCmd('/checklist').execute('')
    expect(result).toContain('✓ 1. Tests pass')
    expect(result).toContain('☐ 2. Docs updated')
  })

  // /quality
  it('/quality returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/quality').execute('')).toBe('Not connected.')
  })

  it('/quality returns status when no findings', async () => {
    mockClientCall.mockResolvedValue({ status: 'passed', findings: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/quality').execute('')).toBe('Quality: passed')
  })

  it('/quality returns status with findings', async () => {
    mockClientCall.mockResolvedValue({
      status: 'failed',
      findings: [{ message: 'Unused import', severity: 'warning' }],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/quality').execute('')).toBe('Quality: failed\n[warning] Unused import')
  })

  // /ci
  it('/ci returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/ci').execute('')).toBe('Not connected.')
  })

  it('/ci returns status without checks', async () => {
    mockClientCall.mockResolvedValue({ status: 'running' })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/ci').execute('')).toBe('CI: running')
  })

  it('/ci returns status with url and checks', async () => {
    mockClientCall.mockResolvedValue({
      status: 'completed',
      url: 'https://ci.example.com/123',
      checks: [
        { name: 'lint', status: 'passed' },
        { name: 'test', status: 'failed' },
      ],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await findCmd('/ci').execute('')
    expect(result).toContain('CI: completed (https://ci.example.com/123)')
    expect(result).toContain('✓ lint')
    expect(result).toContain('✗ test')
  })

  // /policy
  it('/policy returns not connected when no client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/policy').execute('')).toBe('Not connected.')
  })

  it('/policy returns compliant when no violations', async () => {
    mockClientCall.mockResolvedValue({ compliant: true })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/policy').execute('')).toBe('Policy: compliant.')
  })

  it('/policy returns violations when not compliant', async () => {
    mockClientCall.mockResolvedValue({
      compliant: false,
      violations: [{ rule: 'no-force-push', message: 'Not allowed' }],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await findCmd('/policy').execute('')
    expect(result).toContain('Policy violations:')
    expect(result).toContain('no-force-push: Not allowed')
  })

  // /remote approve
  it('/remote approve calls approveRemote', async () => {
    expect(await findCmd('/remote approve').execute('')).toBe('PR approved.')
    expect(mockProjectState.approveRemote).toHaveBeenCalledOnce()
  })

  // /remote merge
  it('/remote merge calls mergeRemote', async () => {
    expect(await findCmd('/remote merge').execute('')).toBe('PR merged.')
    expect(mockProjectState.mergeRemote).toHaveBeenCalledOnce()
  })
})

// ---------------------------------------------------------------------------
// File commands — execution
// ---------------------------------------------------------------------------

describe('file commands execution', () => {
  // /files search
  it('/files search returns usage when no pattern', async () => {
    expect(await findCmd('/files search').execute('')).toBe('Usage: /files search <pattern>')
  })

  it('/files search returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/files search').execute('*.ts')).toBe('Not connected.')
  })

  it('/files search returns matching files', async () => {
    mockClientCall.mockResolvedValue({ files: ['src/main.ts', 'src/utils.ts'] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/files search').execute('*.ts')).toBe('src/main.ts\nsrc/utils.ts')
  })

  it('/files search returns no matching when empty', async () => {
    mockClientCall.mockResolvedValue({ files: [] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/files search').execute('*.xyz')).toBe('No matching files.')
  })

  // /files
  it('/files returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/files').execute('')).toBe('Not connected.')
  })

  it('/files defaults to current directory', async () => {
    mockClientCall.mockResolvedValue({ files: ['README.md'] })
    setState({ client: { call: mockClientCall } })
    await findCmd('/files').execute('')
    expect(mockClientCall).toHaveBeenCalledWith('files.list', { path: '.' })
  })

  it('/files passes path argument', async () => {
    mockClientCall.mockResolvedValue({ files: ['src/main.ts'] })
    setState({ client: { call: mockClientCall } })
    await findCmd('/files').execute('src')
    expect(mockClientCall).toHaveBeenCalledWith('files.list', { path: 'src' })
  })

  // /git status
  it('/git status returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/git status').execute('')).toBe('Not connected.')
  })

  it('/git status returns branch with summary', async () => {
    mockClientCall.mockResolvedValue({ branch: 'main', has_changes: true, summary: 'M src/main.ts' })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/git status').execute('')).toBe('Branch: main\nM src/main.ts')
  })

  it('/git status returns clean branch', async () => {
    mockClientCall.mockResolvedValue({ branch: 'main', has_changes: false })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/git status').execute('')).toBe('Branch: main (clean)')
  })

  it('/git status returns has-changes flag when no summary', async () => {
    mockClientCall.mockResolvedValue({ branch: 'feature', has_changes: true })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/git status').execute('')).toBe('Branch: feature (has changes)')
  })

  // /git log
  it('/git log returns formatted commits', async () => {
    mockClientCall.mockResolvedValue({ entries: [{ sha: 'abc1234567890', message: 'Fix bug' }] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/git log').execute('')).toBe('abc1234 Fix bug')
  })

  it('/git log returns no commits when empty', async () => {
    mockClientCall.mockResolvedValue({ entries: [] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/git log').execute('')).toBe('No commits.')
  })

  // /codegraph search
  it('/codegraph search returns usage when no name', async () => {
    expect(await findCmd('/codegraph search').execute('')).toBe('Usage: /codegraph search <symbol>')
  })

  it('/codegraph search returns formatted symbols', async () => {
    mockClientCall.mockResolvedValue({ symbols: [{ name: 'MyClass', kind: 'class', file: 'src/main.ts', line: 42 }] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/codegraph search').execute('MyClass')).toBe('class MyClass — src/main.ts:42')
  })

  it('/codegraph search returns no symbols when empty', async () => {
    mockClientCall.mockResolvedValue({ symbols: [] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/codegraph search').execute('NonExistent')).toBe('No symbols found.')
  })

  // /codegraph callers
  it('/codegraph callers returns usage when no name', async () => {
    expect(await findCmd('/codegraph callers').execute('')).toBe('Usage: /codegraph callers <symbol>')
  })

  it('/codegraph callers returns formatted callers', async () => {
    mockClientCall.mockResolvedValue({ callers: [{ name: 'main', file: 'src/app.ts', line: 10 }] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/codegraph callers').execute('MyFunc')).toBe('main — src/app.ts:10')
  })

  it('/codegraph callers returns no callers when empty', async () => {
    mockClientCall.mockResolvedValue({ callers: [] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/codegraph callers').execute('Orphan')).toBe('No callers found.')
  })

  // /codegraph deps
  it('/codegraph deps returns usage when no name', async () => {
    expect(await findCmd('/codegraph deps').execute('')).toBe('Usage: /codegraph deps <symbol>')
  })

  it('/codegraph deps returns formatted deps', async () => {
    mockClientCall.mockResolvedValue({ deps: [{ name: 'Logger', kind: 'class', file: 'src/logger.ts' }] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/codegraph deps').execute('MyClass')).toBe('class Logger — src/logger.ts')
  })

  it('/codegraph deps returns no dependencies when empty', async () => {
    mockClientCall.mockResolvedValue({ deps: [] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/codegraph deps').execute('Leaf')).toBe('No dependencies found.')
  })

  // /codegraph index
  it('/codegraph index returns symbol count', async () => {
    mockClientCall.mockResolvedValue({ symbols: 150 })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/codegraph index').execute('')).toBe('Indexed 150 symbols.')
  })

  // /codegraph stats
  it('/codegraph stats returns JSON', async () => {
    mockClientCall.mockResolvedValue({ files: 10, symbols: 200 })
    setState({ client: { call: mockClientCall } })
    const result = await findCmd('/codegraph stats').execute('')
    expect(JSON.parse(result)).toEqual({ files: 10, symbols: 200 })
  })
})

// ---------------------------------------------------------------------------
// Memory commands — execution
// ---------------------------------------------------------------------------

describe('memory commands execution', () => {
  // /memory search
  it('/memory search returns usage when no query', async () => {
    expect(await findCmd('/memory search').execute('')).toBe('Usage: /memory search <query>')
  })

  it('/memory search returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/memory search').execute('auth')).toBe('Not connected to global socket.')
  })

  it('/memory search returns formatted results', async () => {
    mockClientCall.mockResolvedValue({ results: [{ content: 'Auth tokens', score: 0.95 }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/memory search').execute('auth')).toBe('1. (0.95) Auth tokens')
  })

  it('/memory search returns no results when empty', async () => {
    mockClientCall.mockResolvedValue({ results: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/memory search').execute('xyz')).toBe('No results.')
  })

  // /memory stats
  it('/memory stats returns JSON', async () => {
    mockClientCall.mockResolvedValue({ entries: 42 })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/memory stats').execute('')
    expect(JSON.parse(result)).toEqual({ entries: 42 })
  })

  // /memory clear
  it('/memory clear returns cleared', async () => {
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/memory clear').execute('')).toBe('Memory cleared.')
    expect(mockClientCall).toHaveBeenCalledWith('memory.clear', {})
  })

  // /cache stats
  it('/cache stats returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/cache stats').execute('')).toBe('Not connected.')
  })

  it('/cache stats returns JSON', async () => {
    mockClientCall.mockResolvedValue({ hits: 10, misses: 5 })
    setState({ client: { call: mockClientCall } })
    const result = await findCmd('/cache stats').execute('')
    expect(JSON.parse(result)).toEqual({ hits: 10, misses: 5 })
  })

  // /cache clear
  it('/cache clear returns cleared', async () => {
    mockClientCall.mockResolvedValue({})
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/cache clear').execute('')).toBe('Cache cleared.')
  })
})

// ---------------------------------------------------------------------------
// Utility commands — execution
// ---------------------------------------------------------------------------

describe('utility commands execution', () => {
  // /audit
  it('/audit returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/audit').execute('')).toBe('Not connected.')
  })

  it('/audit returns formatted entries', async () => {
    mockClientCall.mockResolvedValue({
      entries: [{ action: 'plan', timestamp: '2026-01-01', details: 'spec written' }],
    })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/audit').execute('')).toBe('[2026-01-01] plan — spec written')
  })

  it('/audit returns no entries when empty', async () => {
    mockClientCall.mockResolvedValue({ entries: [] })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/audit').execute('')).toBe('No audit entries.')
  })

  // /report
  it('/report returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/report').execute('')).toBe('Not connected to global socket.')
  })

  it('/report returns report content', async () => {
    mockClientCall.mockResolvedValue({ report: '## Report\nAll good.' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/report').execute('')).toBe('## Report\nAll good.')
  })

  it('/report returns fallback when empty', async () => {
    mockClientCall.mockResolvedValue({ report: '' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/report').execute('')).toBe('Report generated.')
  })

  // /backup
  it('/backup returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/backup').execute('')).toBe('Not connected to global socket.')
  })

  it('/backup returns backup path', async () => {
    mockClientCall.mockResolvedValue({ path: '/tmp/backup.tar.gz' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/backup').execute('')).toBe('Backup created: /tmp/backup.tar.gz')
  })

  // /export
  it('/export returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/export').execute('')).toBe('Not connected.')
  })

  it('/export defaults to json format', async () => {
    mockClientCall.mockResolvedValue({ data: '{"tasks":[]}' })
    setState({ client: { call: mockClientCall } })
    await findCmd('/export').execute('')
    expect(mockClientCall).toHaveBeenCalledWith('task.export', { format: 'json' })
  })

  it('/export returns path when available', async () => {
    mockClientCall.mockResolvedValue({ path: '/tmp/export.json', data: '' })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/export').execute('csv')).toBe('Exported to /tmp/export.json')
  })

  // /changelog
  it('/changelog returns usage for bad format', async () => {
    expect(await findCmd('/changelog').execute('')).toContain('Usage')
    expect(await findCmd('/changelog').execute('norange')).toContain('Usage')
  })

  it('/changelog with valid refs calls changelog.generate', async () => {
    mockClientCall.mockResolvedValue({ markdown: '## v2.0\n- Feature A' })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/changelog').execute('v1.0..v2.0')).toBe('## v2.0\n- Feature A')
    expect(mockClientCall).toHaveBeenCalledWith('changelog.generate', { source: 'v1.0', target: 'v2.0' })
  })

  it('/changelog passes note when provided', async () => {
    mockClientCall.mockResolvedValue({ markdown: 'changes' })
    setState({ client: { call: mockClientCall } })
    await findCmd('/changelog').execute('v1.0..v2.0 only frontend')
    expect(mockClientCall).toHaveBeenCalledWith('changelog.generate', {
      source: 'v1.0',
      target: 'v2.0',
      note: 'only frontend',
    })
  })

  it('/changelog returns fallback when no commits', async () => {
    mockClientCall.mockResolvedValue({ markdown: '' })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/changelog').execute('v1.0..v2.0')).toBe('No commits between v1.0 and v2.0')
  })

  // /changelog full
  it('/changelog full returns usage for bad format', async () => {
    expect(await findCmd('/changelog full').execute('')).toContain('Usage')
  })

  it('/changelog full calls with full:true', async () => {
    mockClientCall.mockResolvedValue({ markdown: '## Full' })
    setState({ client: { call: mockClientCall } })
    await findCmd('/changelog full').execute('v1.0..v2.0')
    expect(mockClientCall).toHaveBeenCalledWith('changelog.generate', { source: 'v1.0', target: 'v2.0', full: true })
  })

  // /workers
  it('/workers returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/workers').execute('')).toBe('Not connected to global socket.')
  })

  it('/workers returns formatted list', async () => {
    mockClientCall.mockResolvedValue({ workers: [{ name: 'w-1', state: 'idle' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/workers').execute('')).toBe('w-1 [idle]')
  })

  it('/workers returns no workers when empty', async () => {
    mockClientCall.mockResolvedValue({ workers: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/workers').execute('')).toBe('No workers.')
  })

  // /workers add
  it('/workers add returns usage when no name', async () => {
    expect(await findCmd('/workers add').execute('')).toBe('Usage: /workers add <agent-name>')
  })

  it('/workers add calls workers.add', async () => {
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/workers add').execute('claude')).toBe('Worker "claude" added.')
    expect(mockClientCall).toHaveBeenCalledWith('workers.add', { agent: 'claude' })
  })

  // /workers remove
  it('/workers remove returns usage when no name', async () => {
    expect(await findCmd('/workers remove').execute('')).toBe('Usage: /workers remove <worker-name>')
  })

  it('/workers remove calls workers.remove', async () => {
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/workers remove').execute('worker-1')).toBe('Worker "worker-1" removed.')
  })

  // /discover
  it('/discover returns not connected when no client', async () => {
    setState({ client: null })
    expect(await findCmd('/discover').execute('')).toBe('Not connected.')
  })

  it('/discover returns formatted commands', async () => {
    mockClientCall.mockResolvedValue({ commands: ['make build', 'bun run dev'], count: 2 })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/discover').execute('')).toBe('Discovered commands (2)\n\n  make build\n  bun run dev')
  })

  it('/discover returns no commands when empty', async () => {
    mockClientCall.mockResolvedValue({ commands: [], count: 0 })
    setState({ client: { call: mockClientCall } })
    expect(await findCmd('/discover').execute('')).toBe('No project commands found.')
  })

  // /diagnose
  it('/diagnose returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/diagnose').execute('')).toBe('Not connected to global socket.')
  })

  it('/diagnose returns formatted checks', async () => {
    mockClientCall.mockResolvedValue({
      checks: [
        { name: 'socket', status: 'passed' },
        { name: 'disk', status: 'failed', detail: 'low space' },
      ],
    })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/diagnose').execute('')
    expect(result).toContain('✓ socket')
    expect(result).toContain('✗ disk: low space')
  })

  it('/diagnose returns OK when no checks', async () => {
    mockClientCall.mockResolvedValue({ checks: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/diagnose').execute('')).toBe('Diagnostics: OK')
  })

  // /security scan
  it('/security scan returns no issues when clean', async () => {
    mockClientCall.mockResolvedValue({ issues: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/security scan').execute('')).toBe('No security issues found.')
  })

  it('/security scan returns formatted issues', async () => {
    mockClientCall.mockResolvedValue({ issues: [{ severity: 'high', message: 'Secret found' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/security scan').execute('')).toBe('[high] Secret found')
  })

  // /config check
  it('/config check returns no drift when clean', async () => {
    mockClientCall.mockResolvedValue({ drifts: [], count: 0 })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/config check').execute('')).toBe('Configuration: no drift detected.')
  })

  it('/config check returns drift details', async () => {
    mockClientCall.mockResolvedValue({
      drifts: [{ path: 'agent.model', expected: 'claude-4', actual: 'claude-3' }],
      count: 1,
    })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/config check').execute('')
    expect(result).toContain('Configuration drift:')
    expect(result).toContain('agent.model: expected=claude-4, actual=claude-3')
  })

  // /config show
  it('/config show returns JSON config', async () => {
    mockClientCall.mockResolvedValue({ effective: { agent: 'claude' } })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/config show').execute('')
    expect(JSON.parse(result)).toEqual({ agent: 'claude' })
  })

  // /config validate
  it('/config validate returns valid config', async () => {
    mockClientCall.mockResolvedValue({ valid: true, checks: [{ name: 'agent', status: 'ok' }] })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/config validate').execute('')
    expect(result).toContain('Configuration valid')
    expect(result).toContain('[PASS] agent')
  })

  it('/config validate returns invalid config with fix hint', async () => {
    mockClientCall.mockResolvedValue({
      valid: false,
      checks: [{ name: 'token', status: 'error', detail: 'Missing', fix: 'Run kvelmo config init' }],
    })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/config validate').execute('')
    expect(result).toContain('Configuration INVALID')
    expect(result).toContain('[FAIL] token')
    expect(result).toContain('Fix: Run kvelmo config init')
  })

  // /strategy
  it('/strategy returns strategies', async () => {
    mockClientCall.mockResolvedValue(['chain-of-thought', 'tree-of-thought'])
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/strategy').execute('')
    expect(result).toContain('chain-of-thought')
    expect(result).toContain('tree-of-thought')
  })

  it('/strategy returns no strategies when empty', async () => {
    mockClientCall.mockResolvedValue([])
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/strategy').execute('')).toBe('No strategies registered.')
  })

  // /restore
  it('/restore returns usage when no path', async () => {
    expect(await findCmd('/restore').execute('')).toBe('Usage: /restore <archive-path>')
  })

  it('/restore calls backup.restore', async () => {
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/restore').execute('/tmp/backup.tar.gz')).toBe('Restored from /tmp/backup.tar.gz.')
    expect(mockClientCall).toHaveBeenCalledWith('backup.restore', { archive_path: '/tmp/backup.tar.gz' })
  })

  // /catalog list
  it('/catalog list returns templates', async () => {
    mockClientCall.mockResolvedValue({ templates: [{ name: 'bug-fix', description: 'Fix a bug' }] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/catalog list').execute('')).toContain('bug-fix')
  })

  it('/catalog list returns no templates when empty', async () => {
    mockClientCall.mockResolvedValue({ templates: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/catalog list').execute('')).toBe('No templates in catalog.')
  })

  // /catalog use
  it('/catalog use returns usage when no name', async () => {
    expect(await findCmd('/catalog use').execute('')).toBe('Usage: /catalog use <template-name>')
  })

  it('/catalog use returns message when template has no source', async () => {
    mockClientCall.mockResolvedValue({ name: 'empty', source: '' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/catalog use').execute('empty')).toBe('Template "empty" has no source configured.')
  })

  // /rpc-log
  it('/rpc-log returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/rpc-log').execute('')).toBe('Not connected to global socket.')
  })

  it('/rpc-log returns formatted entries', async () => {
    mockClientCall.mockResolvedValue({
      entries: [{ timestamp: '2026-01-01', level: 'INFO', message: 'start called', method: 'start' }],
    })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/rpc-log').execute('')
    expect(result).toContain('[2026-01-01]')
    expect(result).toContain('start called')
  })

  it('/rpc-log returns no entries when empty', async () => {
    mockClientCall.mockResolvedValue({ entries: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/rpc-log').execute('')).toBe('No log entries.')
  })
})

// ---------------------------------------------------------------------------
// Parity additions — surfaces CLI-only commands in chat
// ---------------------------------------------------------------------------

describe('parity commands execution', () => {
  // /agent
  it('/agent returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/agent').execute('')).toBe('Not connected to global socket.')
  })

  it('/agent reports available agent with checks (all three OK statuses)', async () => {
    mockClientCall.mockResolvedValue({
      agent_available: true,
      simulation_mode: false,
      checks: [
        { name: 'claude', status: 'ok', detail: 'v4.6' },
        { name: 'git', status: 'pass' },
        { name: 'ollama', status: 'passed', detail: 'reachable' },
      ],
    })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/agent').execute('')
    expect(result).toContain('Agent: available')
    expect(result).toContain('claude: OK (v4.6)')
    expect(result).toContain('git: OK')
    expect(result).toContain('ollama: OK (reachable)')
    expect(mockClientCall).toHaveBeenCalledWith('agent.status', {})
  })

  it('/agent reports simulation mode when no agent', async () => {
    mockClientCall.mockResolvedValue({
      agent_available: false,
      simulation_mode: true,
      checks: [],
    })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/agent').execute('')).toBe('Agent: simulation mode')
  })

  it('/agent reports not available when neither flag set', async () => {
    mockClientCall.mockResolvedValue({
      agent_available: false,
      simulation_mode: false,
      checks: [{ name: 'probe', status: 'error', detail: 'timeout' }],
    })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/agent').execute('')
    expect(result).toContain('Agent: not available')
    expect(result).toContain('probe: error (timeout)')
  })

  // /projects
  it('/projects returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/projects').execute('')).toBe('Not connected to global socket.')
  })

  it('/projects returns no projects when empty', async () => {
    mockClientCall.mockResolvedValue({ projects: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/projects').execute('')).toBe('No projects registered.')
  })

  it('/projects returns formatted list with name preferred over path', async () => {
    mockClientCall.mockResolvedValue({
      projects: [
        { id: 'abcdef1234567890', name: 'kvelmo', path: '/workspace/kvelmo' },
        { id: 'fedcba0987654321', path: '/workspace/other' },
      ],
    })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/projects').execute('')
    expect(result).toContain('abcdef12 kvelmo')
    expect(result).toContain('fedcba09 /workspace/other')
    expect(mockClientCall).toHaveBeenCalledWith('projects.list', {})
  })

  // /projects unregister
  it('/projects unregister returns usage when no id', async () => {
    expect(await findCmd('/projects unregister').execute('')).toBe('Usage: /projects unregister <id>')
  })

  it('/projects unregister returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/projects unregister').execute('abc123')).toBe('Not connected to global socket.')
  })

  it('/projects unregister calls projects.unregister', async () => {
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/projects unregister').execute('abc123')).toBe('Project abc123 unregistered.')
    expect(mockClientCall).toHaveBeenCalledWith('projects.unregister', { id: 'abc123' })
  })

  // /hooks (worktree-scoped, requires active state)
  it('/hooks is unavailable when not active', () => {
    setState({ state: 'none' })
    expect(findCmd('/hooks').isAvailable()).toBe(false)
  })

  it('/hooks is available when active', () => {
    setState({ state: 'loaded' })
    expect(findCmd('/hooks').isAvailable()).toBe(true)
  })

  it('/hooks returns not connected when no worktree client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/hooks').execute('')).toBe('Not connected.')
  })

  it('/hooks returns no hooks when empty', async () => {
    mockClientCall.mockResolvedValue({ hooks: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/hooks').execute('')).toBe('No hooks configured.')
  })

  it('/hooks returns formatted hooks by event', async () => {
    mockClientCall.mockResolvedValue({
      hooks: [
        { event: 'pre-plan', command: 'echo planning' },
        { event: 'post-submit', command: 'notify slack' },
      ],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await findCmd('/hooks').execute('')
    expect(result).toContain('pre-plan: echo planning')
    expect(result).toContain('post-submit: notify slack')
  })

  it('/hooks falls back to name when event missing', async () => {
    mockClientCall.mockResolvedValue({ hooks: [{ name: 'my-hook', command: 'run' }] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/hooks').execute('')).toBe('my-hook: run')
  })

  // /recordings
  it('/recordings returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/recordings').execute('')).toBe('Not connected to global socket.')
  })

  it('/recordings returns no recordings when empty', async () => {
    mockClientCall.mockResolvedValue({ recordings: [] })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/recordings').execute('')).toBe('No recordings.')
  })

  it('/recordings returns formatted list with id, path, and timestamp', async () => {
    mockClientCall.mockResolvedValue({
      recordings: [
        { id: 'rec11111234567890', path: '/rec/a.jsonl', created_at: '2026-01-15T10:00:00Z' },
        { path: '/rec/b.jsonl' },
      ],
    })
    mockGlobalState.client = { call: mockClientCall }
    const result = await findCmd('/recordings').execute('')
    expect(result).toContain('rec11111')
    expect(result).toContain('/rec/a.jsonl')
    expect(result).toContain('(2026-01-15T10:00:00Z)')
    expect(result).toContain('/rec/b.jsonl')
  })

  // /screenshots (worktree-scoped, requires active state)
  it('/screenshots is unavailable when not active', () => {
    setState({ state: 'none' })
    expect(findCmd('/screenshots').isAvailable()).toBe(false)
  })

  it('/screenshots is available when active', () => {
    setState({ state: 'loaded' })
    expect(findCmd('/screenshots').isAvailable()).toBe(true)
  })

  it('/screenshots returns not connected when no worktree client', async () => {
    setState({ state: 'loaded', client: null })
    expect(await findCmd('/screenshots').execute('')).toBe('Not connected.')
  })

  it('/screenshots returns no screenshots when empty', async () => {
    mockClientCall.mockResolvedValue({ screenshots: [] })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    expect(await findCmd('/screenshots').execute('')).toBe('No screenshots.')
  })

  it('/screenshots returns formatted list', async () => {
    mockClientCall.mockResolvedValue({
      screenshots: [{ id: 'shot1234', path: '/shots/a.png' }, { path: '/shots/b.png' }],
    })
    setState({ state: 'loaded', client: { call: mockClientCall } })
    const result = await findCmd('/screenshots').execute('')
    expect(result).toContain('shot1234')
    expect(result).toContain('/shots/a.png')
    expect(result).toContain('/shots/b.png')
  })

  // /notify test
  it('/notify test returns not connected when no global client', async () => {
    mockGlobalState.client = null
    expect(await findCmd('/notify test').execute('')).toBe('Not connected to global socket.')
  })

  it('/notify test reports delivered count when sent', async () => {
    mockClientCall.mockResolvedValue({ sent: 2 })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/notify test').execute('')).toBe('Notification test sent (2 delivered).')
    expect(mockClientCall).toHaveBeenCalledWith('notify.test', {})
  })

  it('/notify test reports message when not sent', async () => {
    mockClientCall.mockResolvedValue({ sent: 0, message: 'no webhooks configured' })
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/notify test').execute('')).toBe('Notification test: no webhooks configured')
  })

  it('/notify test reports generic complete when no detail', async () => {
    mockClientCall.mockResolvedValue({})
    mockGlobalState.client = { call: mockClientCall }
    expect(await findCmd('/notify test').execute('')).toBe('Notification test complete.')
  })
})
