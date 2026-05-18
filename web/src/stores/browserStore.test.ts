import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useBrowserStore, type BrowserStatus, type BrowserConfig } from './browserStore'
import { useGlobalStore as mockGlobalStore } from './globalStore'

vi.mock('./globalStore', () => ({
  useGlobalStore: {
    getState: vi.fn().mockReturnValue({ client: null }),
  },
}))

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const makeClient = (overrides: Record<string, ReturnType<typeof vi.fn>> = {}) => ({
  call: vi.fn().mockResolvedValue({}),
  subscribe: vi.fn(),
  connect: vi.fn().mockResolvedValue(undefined),
  close: vi.fn(),
  setOnDisconnect: vi.fn(),
  ...overrides,
})

const setClient = (client: ReturnType<typeof makeClient> | null) => {
  (mockGlobalStore.getState as ReturnType<typeof vi.fn>).mockReturnValue({ client })
}

const makeBrowserStatus = (overrides: Partial<BrowserStatus> = {}): BrowserStatus => ({
  installed: true,
  runtime_dir: '/tmp/browser',
  binary_path: '/usr/bin/chromium',
  version: '120.0',
  ...overrides,
})

const makeBrowserConfig = (overrides: Partial<BrowserConfig> = {}): BrowserConfig => ({
  headless: true,
  browser: 'chromium',
  profile: 'default',
  timeout: 30000,
  ...overrides,
})

const resetState = () => {
  useBrowserStore.setState({
    status: null,
    loading: false,
    error: null,
    lastResult: null,
    currentUrl: '',
    currentTitle: '',
  })
}

describe('browserStore', () => {
  beforeEach(() => {
    resetState()
    setClient(null)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  // ---------------------------------------------------------------------------
  // Initial state
  // ---------------------------------------------------------------------------

  describe('initial state', () => {
    it('has null status', () => {
      expect(useBrowserStore.getState().status).toBeNull()
    })

    it('starts not loading', () => {
      expect(useBrowserStore.getState().loading).toBe(false)
    })

    it('starts with no error', () => {
      expect(useBrowserStore.getState().error).toBeNull()
    })

    it('has null lastResult', () => {
      expect(useBrowserStore.getState().lastResult).toBeNull()
    })

    it('has empty currentUrl', () => {
      expect(useBrowserStore.getState().currentUrl).toBe('')
    })

    it('has empty currentTitle', () => {
      expect(useBrowserStore.getState().currentTitle).toBe('')
    })
  })

  // ---------------------------------------------------------------------------
  // clearError / clearResult
  // ---------------------------------------------------------------------------

  describe('clearError', () => {
    it('clears error state', () => {
      useBrowserStore.setState({ error: 'something went wrong' })
      useBrowserStore.getState().clearError()
      expect(useBrowserStore.getState().error).toBeNull()
    })

    it('does not affect other state', () => {
      useBrowserStore.setState({ error: 'err', currentUrl: 'https://example.com' })
      useBrowserStore.getState().clearError()
      expect(useBrowserStore.getState().currentUrl).toBe('https://example.com')
    })
  })

  describe('clearResult', () => {
    it('clears lastResult', () => {
      useBrowserStore.setState({ lastResult: { url: 'https://x.com', success: true } })
      useBrowserStore.getState().clearResult()
      expect(useBrowserStore.getState().lastResult).toBeNull()
    })

    it('does not affect other state', () => {
      useBrowserStore.setState({ lastResult: { success: true }, currentUrl: 'https://x.com' })
      useBrowserStore.getState().clearResult()
      expect(useBrowserStore.getState().currentUrl).toBe('https://x.com')
    })
  })

  // ---------------------------------------------------------------------------
  // checkStatus
  // ---------------------------------------------------------------------------

  describe('checkStatus', () => {
    it('calls browser.status RPC', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce(makeBrowserStatus())
      setClient(client)
      await useBrowserStore.getState().checkStatus()
      expect(client.call).toHaveBeenCalledWith('browser.status', {})
    })

    it('sets status on success', async () => {
      const status = makeBrowserStatus({ version: '121.0' })
      const client = makeClient()
      client.call.mockResolvedValueOnce(status)
      setClient(client)
      await useBrowserStore.getState().checkStatus()
      expect(useBrowserStore.getState().status).toEqual(status)
      expect(useBrowserStore.getState().loading).toBe(false)
    })

    it('sets error when call rejects', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('browser not available'))
      setClient(client)
      await useBrowserStore.getState().checkStatus()
      expect(useBrowserStore.getState().error).toBe('browser not available')
      expect(useBrowserStore.getState().loading).toBe(false)
    })

    it('does nothing when no client', async () => {
      await useBrowserStore.getState().checkStatus()
      expect(useBrowserStore.getState().status).toBeNull()
      expect(useBrowserStore.getState().error).toBeNull()
    })

    it('includes config in status when present', async () => {
      const status = makeBrowserStatus({ config: makeBrowserConfig({ headless: false }) })
      const client = makeClient()
      client.call.mockResolvedValueOnce(status)
      setClient(client)
      await useBrowserStore.getState().checkStatus()
      expect(useBrowserStore.getState().status?.config?.headless).toBe(false)
    })
  })

  // ---------------------------------------------------------------------------
  // install
  // ---------------------------------------------------------------------------

  describe('install', () => {
    it('calls browser.install then refreshes status', async () => {
      const client = makeClient()
      client.call
        .mockResolvedValueOnce({}) // browser.install
        .mockResolvedValueOnce(makeBrowserStatus({ installed: true })) // browser.status
      setClient(client)

      await useBrowserStore.getState().install()

      expect(client.call).toHaveBeenNthCalledWith(1, 'browser.install', {})
      expect(client.call).toHaveBeenNthCalledWith(2, 'browser.status', {})
      expect(useBrowserStore.getState().status?.installed).toBe(true)
    })

    it('sets error when install fails', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('download failed'))
      setClient(client)

      await useBrowserStore.getState().install()
      expect(useBrowserStore.getState().error).toBe('download failed')
      expect(useBrowserStore.getState().loading).toBe(false)
    })

    it('does nothing when no client', async () => {
      await useBrowserStore.getState().install()
      expect(useBrowserStore.getState().error).toBeNull()
    })
  })

  // ---------------------------------------------------------------------------
  // setConfig
  // ---------------------------------------------------------------------------

  describe('setConfig', () => {
    it('calls browser.config.set with key and value', async () => {
      const client = makeClient()
      client.call
        .mockResolvedValueOnce({}) // config.set
        .mockResolvedValueOnce(makeBrowserStatus()) // status refresh
      setClient(client)

      await useBrowserStore.getState().setConfig('headless', 'false')
      expect(client.call).toHaveBeenCalledWith('browser.config.set', { key: 'headless', value: 'false' })
    })

    it('refreshes status after setting config', async () => {
      const updatedStatus = makeBrowserStatus({ config: makeBrowserConfig({ headless: false }) })
      const client = makeClient()
      client.call
        .mockResolvedValueOnce({})
        .mockResolvedValueOnce(updatedStatus)
      setClient(client)

      await useBrowserStore.getState().setConfig('headless', 'false')
      expect(useBrowserStore.getState().status?.config?.headless).toBe(false)
    })

    it('sets error when call fails', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('invalid key'))
      setClient(client)

      await useBrowserStore.getState().setConfig('bad_key', 'val')
      expect(useBrowserStore.getState().error).toBe('invalid key')
    })

    it('does nothing when no client', async () => {
      await useBrowserStore.getState().setConfig('k', 'v')
      expect(useBrowserStore.getState().error).toBeNull()
    })
  })

  // ---------------------------------------------------------------------------
  // navigate
  // ---------------------------------------------------------------------------

  describe('navigate', () => {
    it('calls browser.navigate with url', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ url: 'https://example.com', title: 'Example', success: true })
      setClient(client)

      await useBrowserStore.getState().navigate('https://example.com')
      expect(client.call).toHaveBeenCalledWith('browser.navigate', { url: 'https://example.com' })
    })

    it('sets currentUrl and currentTitle on success', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ url: 'https://example.com', title: 'Example Domain', success: true })
      setClient(client)

      await useBrowserStore.getState().navigate('https://example.com')
      expect(useBrowserStore.getState().currentUrl).toBe('https://example.com')
      expect(useBrowserStore.getState().currentTitle).toBe('Example Domain')
    })

    it('falls back to input url when result has no url', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ title: 'Redirect Target' })
      setClient(client)

      await useBrowserStore.getState().navigate('https://orig.com')
      expect(useBrowserStore.getState().currentUrl).toBe('https://orig.com')
    })

    it('sets lastResult on success', async () => {
      const result = { url: 'https://x.com', title: 'X', success: true }
      const client = makeClient()
      client.call.mockResolvedValueOnce(result)
      setClient(client)

      await useBrowserStore.getState().navigate('https://x.com')
      expect(useBrowserStore.getState().lastResult).toEqual(result)
    })

    it('returns result from navigate', async () => {
      const result = { url: 'https://x.com', title: 'X', success: true }
      const client = makeClient()
      client.call.mockResolvedValueOnce(result)
      setClient(client)

      const out = await useBrowserStore.getState().navigate('https://x.com')
      expect(out).toEqual(result)
    })

    it('throws and sets error when call fails', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('timeout'))
      setClient(client)

      await expect(useBrowserStore.getState().navigate('https://x.com')).rejects.toThrow('timeout')
      expect(useBrowserStore.getState().error).toBe('timeout')
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().navigate('https://x.com')).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // back / forward / reload
  // ---------------------------------------------------------------------------

  describe('back', () => {
    it('calls browser.back RPC', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ title: 'Previous Page' })
      setClient(client)

      await useBrowserStore.getState().back()
      expect(client.call).toHaveBeenCalledWith('browser.back', {})
    })

    it('updates currentTitle on success', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ title: 'Previous Page' })
      setClient(client)

      await useBrowserStore.getState().back()
      expect(useBrowserStore.getState().currentTitle).toBe('Previous Page')
    })

    it('throws and sets error on failure', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('no history'))
      setClient(client)

      await expect(useBrowserStore.getState().back()).rejects.toThrow('no history')
      expect(useBrowserStore.getState().error).toBe('no history')
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().back()).rejects.toThrow('Not connected')
    })
  })

  describe('forward', () => {
    it('calls browser.forward RPC', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ title: 'Next Page' })
      setClient(client)

      await useBrowserStore.getState().forward()
      expect(client.call).toHaveBeenCalledWith('browser.forward', {})
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().forward()).rejects.toThrow('Not connected')
    })
  })

  describe('reload', () => {
    it('calls browser.reload RPC', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ title: 'Same Page' })
      setClient(client)

      await useBrowserStore.getState().reload()
      expect(client.call).toHaveBeenCalledWith('browser.reload', {})
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().reload()).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // Interaction methods: click / type / fill / select / hover / focus
  // ---------------------------------------------------------------------------

  describe('click', () => {
    it('calls browser.click with selector', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().click('#submit')
      expect(client.call).toHaveBeenCalledWith('browser.click', { selector: '#submit' })
    })

    it('sets lastResult on success', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true, selector: '#btn' })
      setClient(client)

      await useBrowserStore.getState().click('#btn')
      expect(useBrowserStore.getState().lastResult?.selector).toBe('#btn')
    })

    it('throws and sets error on failure', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('element not found'))
      setClient(client)

      await expect(useBrowserStore.getState().click('#missing')).rejects.toThrow('element not found')
      expect(useBrowserStore.getState().error).toBe('element not found')
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().click('#x')).rejects.toThrow('Not connected')
    })
  })

  describe('type', () => {
    it('calls browser.type with selector and text', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().type('#input', 'hello world')
      expect(client.call).toHaveBeenCalledWith('browser.type', { selector: '#input', text: 'hello world' })
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().type('#x', 'y')).rejects.toThrow('Not connected')
    })
  })

  describe('fill', () => {
    it('calls browser.fill with selector and value', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().fill('#email', 'user@example.com')
      expect(client.call).toHaveBeenCalledWith('browser.fill', { selector: '#email', value: 'user@example.com' })
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().fill('#x', 'v')).rejects.toThrow('Not connected')
    })
  })

  describe('select', () => {
    it('calls browser.select with selector and values', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().select('#dropdown', ['opt1', 'opt2'])
      expect(client.call).toHaveBeenCalledWith('browser.select', { selector: '#dropdown', values: ['opt1', 'opt2'] })
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().select('#x', ['v'])).rejects.toThrow('Not connected')
    })
  })

  describe('hover', () => {
    it('calls browser.hover with selector', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().hover('.tooltip-trigger')
      expect(client.call).toHaveBeenCalledWith('browser.hover', { selector: '.tooltip-trigger' })
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().hover('.x')).rejects.toThrow('Not connected')
    })
  })

  describe('focus', () => {
    it('calls browser.focus with selector', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().focus('#search')
      expect(client.call).toHaveBeenCalledWith('browser.focus', { selector: '#search' })
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().focus('#x')).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // press
  // ---------------------------------------------------------------------------

  describe('press', () => {
    it('calls browser.press with key only', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().press('Enter')
      expect(client.call).toHaveBeenCalledWith('browser.press', { key: 'Enter' })
    })

    it('includes selector when provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().press('Tab', '#field')
      expect(client.call).toHaveBeenCalledWith('browser.press', { key: 'Tab', selector: '#field' })
    })

    it('omits selector when not provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().press('Escape')
      const callArg = client.call.mock.calls[0][1] as Record<string, unknown>
      expect(callArg).not.toHaveProperty('selector')
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().press('Enter')).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // scroll
  // ---------------------------------------------------------------------------

  describe('scroll', () => {
    it('calls browser.scroll with direction', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().scroll('down')
      expect(client.call).toHaveBeenCalledWith('browser.scroll', { direction: 'down' })
    })

    it('includes amount when provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().scroll('up', 300)
      const callArg = client.call.mock.calls[0][1] as Record<string, unknown>
      expect(callArg.amount).toBe(300)
    })

    it('includes selector when provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().scroll('down', undefined, '#container')
      const callArg = client.call.mock.calls[0][1] as Record<string, unknown>
      expect(callArg.selector).toBe('#container')
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().scroll('down')).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // wait
  // ---------------------------------------------------------------------------

  describe('wait', () => {
    it('calls browser.wait with selector', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().wait('#loaded')
      expect(client.call).toHaveBeenCalledWith('browser.wait', { selector: '#loaded' })
    })

    it('includes timeout_ms when provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().wait('.spinner', 5000)
      const callArg = client.call.mock.calls[0][1] as Record<string, unknown>
      expect(callArg.timeout_ms).toBe(5000)
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().wait('#x')).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // dialog
  // ---------------------------------------------------------------------------

  describe('dialog', () => {
    it('calls browser.dialog with accept action', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().dialog('accept')
      expect(client.call).toHaveBeenCalledWith('browser.dialog', { action: 'accept' })
    })

    it('calls browser.dialog with dismiss action', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().dialog('dismiss')
      expect(client.call).toHaveBeenCalledWith('browser.dialog', { action: 'dismiss' })
    })

    it('includes text when provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().dialog('accept', 'confirmation text')
      const callArg = client.call.mock.calls[0][1] as Record<string, unknown>
      expect(callArg.text).toBe('confirmation text')
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().dialog('accept')).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // upload
  // ---------------------------------------------------------------------------

  describe('upload', () => {
    it('calls browser.upload with selector and files', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().upload('#file-input', ['/path/to/file.txt'])
      expect(client.call).toHaveBeenCalledWith('browser.upload', {
        selector: '#file-input',
        files: ['/path/to/file.txt'],
      })
    })

    it('supports multiple files', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      const files = ['/a.txt', '/b.txt', '/c.txt']
      await useBrowserStore.getState().upload('#multi', files)
      const callArg = client.call.mock.calls[0][1] as Record<string, unknown>
      expect(callArg.files).toEqual(files)
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().upload('#x', [])).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // screenshot
  // ---------------------------------------------------------------------------

  describe('screenshot', () => {
    it('calls browser.screenshot with no params by default', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ path: '/tmp/ss.png', success: true })
      setClient(client)

      await useBrowserStore.getState().screenshot()
      expect(client.call).toHaveBeenCalledWith('browser.screenshot', {})
    })

    it('includes path when provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ path: '/custom/path.png', success: true })
      setClient(client)

      await useBrowserStore.getState().screenshot({ path: '/custom/path.png' })
      const callArg = client.call.mock.calls[0][1] as Record<string, unknown>
      expect(callArg.path).toBe('/custom/path.png')
    })

    it('includes full_page when provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().screenshot({ fullPage: true })
      const callArg = client.call.mock.calls[0][1] as Record<string, unknown>
      expect(callArg.full_page).toBe(true)
    })

    it('sets lastResult on success', async () => {
      const result = { path: '/tmp/ss.png', success: true }
      const client = makeClient()
      client.call.mockResolvedValueOnce(result)
      setClient(client)

      await useBrowserStore.getState().screenshot()
      expect(useBrowserStore.getState().lastResult).toEqual(result)
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().screenshot()).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // snapshot
  // ---------------------------------------------------------------------------

  describe('snapshot', () => {
    it('calls browser.snapshot RPC', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ snapshot: '<html>...</html>' })
      setClient(client)

      await useBrowserStore.getState().snapshot()
      expect(client.call).toHaveBeenCalledWith('browser.snapshot', {})
    })

    it('returns snapshot string', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ snapshot: '<html>test</html>' })
      setClient(client)

      const result = await useBrowserStore.getState().snapshot()
      expect(result.snapshot).toBe('<html>test</html>')
    })

    it('sets loading false after success', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ snapshot: '' })
      setClient(client)

      await useBrowserStore.getState().snapshot()
      expect(useBrowserStore.getState().loading).toBe(false)
    })

    it('throws and sets error on failure', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('snapshot failed'))
      setClient(client)

      await expect(useBrowserStore.getState().snapshot()).rejects.toThrow('snapshot failed')
      expect(useBrowserStore.getState().error).toBe('snapshot failed')
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().snapshot()).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // pdf
  // ---------------------------------------------------------------------------

  describe('pdf', () => {
    it('calls browser.pdf with no params by default', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ path: '/tmp/out.pdf', success: true })
      setClient(client)

      await useBrowserStore.getState().pdf()
      expect(client.call).toHaveBeenCalledWith('browser.pdf', {})
    })

    it('includes path, format, and landscape when provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ success: true })
      setClient(client)

      await useBrowserStore.getState().pdf({ path: '/out.pdf', format: 'A4', landscape: true })
      expect(client.call).toHaveBeenCalledWith('browser.pdf', {
        path: '/out.pdf',
        format: 'A4',
        landscape: true,
      })
    })

    it('sets lastResult on success', async () => {
      const result = { path: '/tmp/doc.pdf', success: true }
      const client = makeClient()
      client.call.mockResolvedValueOnce(result)
      setClient(client)

      await useBrowserStore.getState().pdf()
      expect(useBrowserStore.getState().lastResult).toEqual(result)
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().pdf()).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // js eval (browser.eval RPC)
  // ---------------------------------------------------------------------------

  describe('js eval via browser.eval RPC', () => {
    it('calls browser.eval with js code', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ result: '42' })
      setClient(client)

      await useBrowserStore.getState().eval('document.title')
      expect(client.call).toHaveBeenCalledWith('browser.eval', { js: 'document.title' })
    })

    it('returns result object', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ result: 'My Page Title' })
      setClient(client)

      const out = await useBrowserStore.getState().eval('document.title')
      expect(out.result).toBe('My Page Title')
    })

    it('returns error field when script has a runtime error', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ result: '', error: 'ReferenceError: x is not defined' })
      setClient(client)

      const out = await useBrowserStore.getState().eval('x.y.z')
      expect(out.error).toBe('ReferenceError: x is not defined')
    })

    it('sets loading false after success', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ result: 'ok' })
      setClient(client)

      await useBrowserStore.getState().eval('1+1')
      expect(useBrowserStore.getState().loading).toBe(false)
    })

    it('throws and sets error on failure', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('eval timeout'))
      setClient(client)

      await expect(useBrowserStore.getState().eval('longRunning()')).rejects.toThrow('eval timeout')
      expect(useBrowserStore.getState().error).toBe('eval timeout')
    })

    it('throws when no client', async () => {
      await expect(useBrowserStore.getState().eval('1+1')).rejects.toThrow('Not connected')
    })
  })

  // ---------------------------------------------------------------------------
  // loading state transitions
  // ---------------------------------------------------------------------------

  describe('loading state transitions', () => {
    it('sets loading true during checkStatus call', async () => {
      let loadingDuringCall = false
      const client = makeClient()
      client.call.mockImplementationOnce(() => {
        loadingDuringCall = useBrowserStore.getState().loading
        return Promise.resolve(makeBrowserStatus())
      })
      setClient(client)

      await useBrowserStore.getState().checkStatus()
      expect(loadingDuringCall).toBe(true)
      expect(useBrowserStore.getState().loading).toBe(false)
    })

    it('sets loading true during navigate call', async () => {
      let loadingDuringCall = false
      const client = makeClient()
      client.call.mockImplementationOnce(() => {
        loadingDuringCall = useBrowserStore.getState().loading
        return Promise.resolve({ url: 'https://x.com', title: 'X' })
      })
      setClient(client)

      await useBrowserStore.getState().navigate('https://x.com')
      expect(loadingDuringCall).toBe(true)
      expect(useBrowserStore.getState().loading).toBe(false)
    })
  })
})
