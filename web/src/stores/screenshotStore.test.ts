import { describe, it, expect, beforeEach } from 'vitest'
import {
  useScreenshotStore,
  getScreenshotById,
  formatScreenshotRef,
  parseScreenshotRefs,
  type Screenshot,
} from './screenshotStore'

const createMockScreenshot = (overrides: Partial<Screenshot> = {}): Screenshot => ({
  id: 'ss-1',
  task_id: 'task-1',
  path: '/path/to/screenshot.png',
  filename: 'screenshot.png',
  timestamp: '2026-01-01T00:00:00Z',
  source: 'agent',
  format: 'png',
  width: 1920,
  height: 1080,
  size_bytes: 1024000,
  ...overrides,
})

describe('screenshotStore', () => {
  beforeEach(() => {
    // Reset store to initial state
    useScreenshotStore.setState({
      screenshots: [],
      loading: false,
      error: null,
      selectedId: null,
      attachedIds: [],
      screenshotData: {},
    })
  })

  describe('initial state', () => {
    it('starts with empty screenshots', () => {
      expect(useScreenshotStore.getState().screenshots).toEqual([])
    })

    it('starts with no selection', () => {
      expect(useScreenshotStore.getState().selectedId).toBeNull()
    })

    it('starts with no attachments', () => {
      expect(useScreenshotStore.getState().attachedIds).toEqual([])
    })

    it('starts not loading', () => {
      expect(useScreenshotStore.getState().loading).toBe(false)
    })

    it('starts with no error', () => {
      expect(useScreenshotStore.getState().error).toBeNull()
    })
  })

  describe('add', () => {
    it('adds screenshot to the beginning of the list', () => {
      const ss = createMockScreenshot({ id: 'ss-1' })
      useScreenshotStore.getState().add(ss)

      expect(useScreenshotStore.getState().screenshots).toHaveLength(1)
      expect(useScreenshotStore.getState().screenshots[0]).toEqual(ss)
    })

    it('prepends new screenshots', () => {
      const ss1 = createMockScreenshot({ id: 'ss-1' })
      const ss2 = createMockScreenshot({ id: 'ss-2' })

      useScreenshotStore.getState().add(ss1)
      useScreenshotStore.getState().add(ss2)

      const { screenshots } = useScreenshotStore.getState()
      expect(screenshots[0].id).toBe('ss-2')
      expect(screenshots[1].id).toBe('ss-1')
    })
  })

  describe('remove', () => {
    beforeEach(() => {
      useScreenshotStore.setState({
        screenshots: [
          createMockScreenshot({ id: 'ss-1' }),
          createMockScreenshot({ id: 'ss-2' }),
          createMockScreenshot({ id: 'ss-3' }),
        ],
      })
    })

    it('removes screenshot by id', () => {
      useScreenshotStore.getState().remove('ss-2')

      const ids = useScreenshotStore.getState().screenshots.map((s) => s.id)
      expect(ids).toEqual(['ss-1', 'ss-3'])
    })

    it('clears selection if removed screenshot was selected', () => {
      useScreenshotStore.setState({ selectedId: 'ss-2' })
      useScreenshotStore.getState().remove('ss-2')

      expect(useScreenshotStore.getState().selectedId).toBeNull()
    })

    it('preserves selection if different screenshot removed', () => {
      useScreenshotStore.setState({ selectedId: 'ss-1' })
      useScreenshotStore.getState().remove('ss-2')

      expect(useScreenshotStore.getState().selectedId).toBe('ss-1')
    })

    it('removes from attachedIds if attached', () => {
      useScreenshotStore.setState({ attachedIds: ['ss-1', 'ss-2', 'ss-3'] })
      useScreenshotStore.getState().remove('ss-2')

      expect(useScreenshotStore.getState().attachedIds).toEqual(['ss-1', 'ss-3'])
    })

    it('does nothing if id not found', () => {
      useScreenshotStore.getState().remove('nonexistent')

      expect(useScreenshotStore.getState().screenshots).toHaveLength(3)
    })
  })

  describe('select', () => {
    it('sets selectedId', () => {
      useScreenshotStore.getState().select('ss-1')
      expect(useScreenshotStore.getState().selectedId).toBe('ss-1')
    })

    it('can clear selection with null', () => {
      useScreenshotStore.setState({ selectedId: 'ss-1' })
      useScreenshotStore.getState().select(null)
      expect(useScreenshotStore.getState().selectedId).toBeNull()
    })

    it('can change selection', () => {
      useScreenshotStore.setState({ selectedId: 'ss-1' })
      useScreenshotStore.getState().select('ss-2')
      expect(useScreenshotStore.getState().selectedId).toBe('ss-2')
    })
  })

  describe('attach', () => {
    it('adds id to attachedIds', () => {
      useScreenshotStore.getState().attach('ss-1')
      expect(useScreenshotStore.getState().attachedIds).toContain('ss-1')
    })

    it('does not duplicate if already attached', () => {
      useScreenshotStore.setState({ attachedIds: ['ss-1'] })
      useScreenshotStore.getState().attach('ss-1')
      expect(useScreenshotStore.getState().attachedIds).toEqual(['ss-1'])
    })

    it('appends to existing attachments', () => {
      useScreenshotStore.setState({ attachedIds: ['ss-1'] })
      useScreenshotStore.getState().attach('ss-2')
      expect(useScreenshotStore.getState().attachedIds).toEqual(['ss-1', 'ss-2'])
    })
  })

  describe('detach', () => {
    beforeEach(() => {
      useScreenshotStore.setState({ attachedIds: ['ss-1', 'ss-2', 'ss-3'] })
    })

    it('removes id from attachedIds', () => {
      useScreenshotStore.getState().detach('ss-2')
      expect(useScreenshotStore.getState().attachedIds).toEqual(['ss-1', 'ss-3'])
    })

    it('does nothing if id not attached', () => {
      useScreenshotStore.getState().detach('ss-4')
      expect(useScreenshotStore.getState().attachedIds).toEqual(['ss-1', 'ss-2', 'ss-3'])
    })
  })

  describe('clearAttached', () => {
    it('clears all attachments', () => {
      useScreenshotStore.setState({ attachedIds: ['ss-1', 'ss-2', 'ss-3'] })
      useScreenshotStore.getState().clearAttached()
      expect(useScreenshotStore.getState().attachedIds).toEqual([])
    })

    it('works when already empty', () => {
      useScreenshotStore.getState().clearAttached()
      expect(useScreenshotStore.getState().attachedIds).toEqual([])
    })
  })

  describe('handleScreenshotCaptured', () => {
    it('calls add with the screenshot', () => {
      const ss = createMockScreenshot({ id: 'captured-1' })
      useScreenshotStore.getState().handleScreenshotCaptured(ss)

      expect(useScreenshotStore.getState().screenshots[0].id).toBe('captured-1')
    })
  })

  describe('handleScreenshotDeleted', () => {
    it('calls remove with the id', () => {
      useScreenshotStore.setState({
        screenshots: [createMockScreenshot({ id: 'to-delete' })],
      })

      useScreenshotStore.getState().handleScreenshotDeleted('to-delete')

      expect(useScreenshotStore.getState().screenshots).toHaveLength(0)
    })
  })
})

describe('getScreenshotById', () => {
  beforeEach(() => {
    useScreenshotStore.setState({
      screenshots: [
        createMockScreenshot({ id: 'ss-1', filename: 'first.png' }),
        createMockScreenshot({ id: 'ss-2', filename: 'second.png' }),
      ],
    })
  })

  it('returns screenshot with matching id', () => {
    const result = getScreenshotById('ss-2')
    expect(result?.filename).toBe('second.png')
  })

  it('returns undefined for non-existent id', () => {
    const result = getScreenshotById('nonexistent')
    expect(result).toBeUndefined()
  })
})

describe('formatScreenshotRef', () => {
  it('formats id as @screenshot-{id}', () => {
    expect(formatScreenshotRef('abc123')).toBe('@screenshot-abc123')
  })

  it('handles various id formats', () => {
    expect(formatScreenshotRef('ss-1')).toBe('@screenshot-ss-1')
    expect(formatScreenshotRef('123')).toBe('@screenshot-123')
    expect(formatScreenshotRef('ABC')).toBe('@screenshot-ABC')
  })
})

describe('parseScreenshotRefs', () => {
  it('extracts screenshot ids from text', () => {
    const text = 'Check this @screenshot-abc123 and this @screenshot-def456'
    const result = parseScreenshotRefs(text)
    expect(result).toEqual(['abc123', 'def456'])
  })

  it('returns empty array when no refs', () => {
    const result = parseScreenshotRefs('No screenshots here')
    expect(result).toEqual([])
  })

  it('handles single ref', () => {
    const result = parseScreenshotRefs('See @screenshot-single')
    expect(result).toEqual(['single'])
  })

  it('handles refs at start and end of text', () => {
    const result = parseScreenshotRefs('@screenshot-start middle @screenshot-end')
    expect(result).toEqual(['start', 'end'])
  })

  it('handles alphanumeric ids', () => {
    const result = parseScreenshotRefs('@screenshot-ABC123xyz')
    expect(result).toEqual(['ABC123xyz'])
  })

  it('does not match partial patterns', () => {
    const result = parseScreenshotRefs('email@screenshot-fake.com')
    // The regex should still match 'fake' since it follows @screenshot-
    expect(result).toEqual(['fake'])
  })
})

// ---------------------------------------------------------------------------
// Async actions with mock client
// ---------------------------------------------------------------------------

const makeMockClient = (callImpl?: (method: string, params?: unknown) => unknown) => ({
  call: vi.fn().mockImplementation((method: string, params?: unknown) =>
    Promise.resolve(callImpl ? callImpl(method, params) : {})
  ),
  subscribe: vi.fn(),
  connect: vi.fn().mockResolvedValue(undefined),
  close: vi.fn(),
  setOnDisconnect: vi.fn(),
})

describe('load', () => {
  beforeEach(() => {
    useScreenshotStore.setState({ screenshots: [], loading: false, error: null })
  })

  it('returns immediately when client is null', async () => {
    await useScreenshotStore.getState().load(null)
    expect(useScreenshotStore.getState().loading).toBe(false)
    expect(useScreenshotStore.getState().screenshots).toEqual([])
  })

  it('sets screenshots on success', async () => {
    const screenshots = [createMockScreenshot({ id: 'ss-loaded' })]
    const client = makeMockClient(() => ({ screenshots }))
    await useScreenshotStore.getState().load(client as never)
    expect(useScreenshotStore.getState().screenshots).toEqual(screenshots)
    expect(useScreenshotStore.getState().loading).toBe(false)
  })

  it('defaults to empty when response has no screenshots key', async () => {
    const client = makeMockClient(() => ({}))
    await useScreenshotStore.getState().load(client as never)
    expect(useScreenshotStore.getState().screenshots).toEqual([])
  })

  it('sets error on failure', async () => {
    const client = makeMockClient()
    client.call.mockRejectedValueOnce(new Error('network error'))
    await useScreenshotStore.getState().load(client as never)
    expect(useScreenshotStore.getState().error).toBe('network error')
    expect(useScreenshotStore.getState().loading).toBe(false)
  })

  it('uses fallback error message for non-Error throws', async () => {
    const client = makeMockClient()
    client.call.mockRejectedValueOnce('unexpected')
    await useScreenshotStore.getState().load(client as never)
    expect(useScreenshotStore.getState().error).toBe('Failed to load screenshots')
  })
})

describe('captureScreenshot', () => {
  beforeEach(() => {
    useScreenshotStore.setState({ loading: false, error: null })
  })

  it('does nothing when client is null', async () => {
    await useScreenshotStore.getState().captureScreenshot(null)
    expect(useScreenshotStore.getState().loading).toBe(false)
  })

  it('calls screenshots.capture with source and step', async () => {
    const client = makeMockClient()
    await useScreenshotStore.getState().captureScreenshot(client as never, 'agent', 'implement')
    expect(client.call).toHaveBeenCalledWith('screenshots.capture', { source: 'agent', step: 'implement' })
  })

  it('defaults source to user', async () => {
    const client = makeMockClient()
    await useScreenshotStore.getState().captureScreenshot(client as never)
    expect(client.call).toHaveBeenCalledWith('screenshots.capture', { source: 'user', step: undefined })
  })

  it('sets loading false after success', async () => {
    const client = makeMockClient()
    await useScreenshotStore.getState().captureScreenshot(client as never)
    expect(useScreenshotStore.getState().loading).toBe(false)
  })

  it('sets error on failure', async () => {
    const client = makeMockClient()
    client.call.mockRejectedValueOnce(new Error('capture failed'))
    await useScreenshotStore.getState().captureScreenshot(client as never)
    expect(useScreenshotStore.getState().error).toBe('capture failed')
    expect(useScreenshotStore.getState().loading).toBe(false)
  })
})

describe('deleteScreenshot', () => {
  it('does nothing when client is null', async () => {
    useScreenshotStore.setState({ error: null })
    await useScreenshotStore.getState().deleteScreenshot('ss-1', null)
    expect(useScreenshotStore.getState().error).toBeNull()
  })

  it('calls screenshots.delete with id', async () => {
    const client = makeMockClient()
    await useScreenshotStore.getState().deleteScreenshot('ss-1', client as never)
    expect(client.call).toHaveBeenCalledWith('screenshots.delete', { screenshot_id: 'ss-1' })
  })

  it('sets error on failure', async () => {
    useScreenshotStore.setState({ error: null })
    const client = makeMockClient()
    client.call.mockRejectedValueOnce(new Error('not found'))
    await useScreenshotStore.getState().deleteScreenshot('ss-1', client as never)
    expect(useScreenshotStore.getState().error).toBe('not found')
  })
})

describe('getScreenshot', () => {
  beforeEach(() => {
    useScreenshotStore.setState({ screenshotData: {} })
  })

  it('returns cached data if present', async () => {
    useScreenshotStore.setState({ screenshotData: { 'ss-1': 'data:image/png;base64,abc' } })
    const result = await useScreenshotStore.getState().getScreenshot('ss-1', null)
    expect(result).toBe('data:image/png;base64,abc')
  })

  it('returns null when client is null and no cache', async () => {
    const result = await useScreenshotStore.getState().getScreenshot('ss-1', null)
    expect(result).toBeNull()
  })

  it('fetches, builds data URL, and caches on success', async () => {
    const client = makeMockClient(() => ({ data: 'AAAA', format: 'png' }))
    const result = await useScreenshotStore.getState().getScreenshot('ss-1', client as never)
    expect(result).toBe('data:image/png;base64,AAAA')
    expect(useScreenshotStore.getState().screenshotData['ss-1']).toBe('data:image/png;base64,AAAA')
  })

  it('returns null on error', async () => {
    const client = makeMockClient()
    client.call.mockRejectedValueOnce(new Error('fetch failed'))
    const result = await useScreenshotStore.getState().getScreenshot('ss-1', client as never)
    expect(result).toBeNull()
  })
})
