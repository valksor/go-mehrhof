import { create } from 'zustand'
import { useGlobalStore } from './globalStore'

export interface BrowserStatus {
  installed: boolean
  runtime_dir: string
  binary_path: string
  version?: string
  version_error?: string
  config?: BrowserConfig
  config_error?: string
}

export interface BrowserConfig {
  headless: boolean
  browser: 'chromium' | 'firefox' | 'webkit'
  profile: string
  timeout: number
}

export interface BrowserResult {
  success?: boolean
  url?: string
  title?: string
  selector?: string
  value?: string
  path?: string
  base64?: string
  message?: string
  error?: string
}

interface BrowserState {
  // Status
  status: BrowserStatus | null
  loading: boolean
  error: string | null
  lastResult: BrowserResult | null

  // Current page info
  currentUrl: string
  currentTitle: string

  // Actions
  checkStatus: () => Promise<void>
  install: () => Promise<void>
  getConfig: () => Promise<BrowserConfig | null>
  setConfig: (key: string, value: string) => Promise<void>

  // Navigation
  navigate: (url: string) => Promise<BrowserResult>
  back: () => Promise<BrowserResult>
  forward: () => Promise<BrowserResult>
  reload: () => Promise<BrowserResult>

  // Interactions
  click: (selector: string) => Promise<BrowserResult>
  type: (selector: string, text: string) => Promise<BrowserResult>
  fill: (selector: string, value: string) => Promise<BrowserResult>
  select: (selector: string, values: string[]) => Promise<BrowserResult>
  hover: (selector: string) => Promise<BrowserResult>
  focus: (selector: string) => Promise<BrowserResult>
  press: (key: string, selector?: string) => Promise<BrowserResult>
  scroll: (direction: string, amount?: number, selector?: string) => Promise<BrowserResult>
  wait: (selector: string, timeoutMs?: number) => Promise<BrowserResult>

  // Dialogs & Files
  dialog: (action: 'accept' | 'dismiss', text?: string) => Promise<BrowserResult>
  upload: (selector: string, files: string[]) => Promise<BrowserResult>

  // Capture
  screenshot: (options?: { path?: string; fullPage?: boolean }) => Promise<BrowserResult>
  snapshot: () => Promise<{ snapshot: string }>
  pdf: (options?: { path?: string; format?: string; landscape?: boolean }) => Promise<BrowserResult>

  // Evaluation
  eval: (js: string) => Promise<{ result: string; error?: string }>

  // Console & Network
  console: () => Promise<{ messages: ConsoleMessage[] }>
  network: () => Promise<{ requests: NetworkRequest[] }>

  // Clear state
  clearError: () => void
  clearResult: () => void
}

export interface ConsoleMessage {
  type: string
  text: string
  timestamp?: string
}

export interface NetworkRequest {
  method: string
  url: string
  status?: number
  resource_type?: string
}

export const useBrowserStore = create<BrowserState>((set, get) => ({
  status: null,
  loading: false,
  error: null,
  lastResult: null,
  currentUrl: '',
  currentTitle: '',

  checkStatus: async () => {
    const client = useGlobalStore.getState().client
    if (!client) return

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserStatus>('browser.status', {})
      set({ status: result, loading: false })
    } catch (err) {
      set({
        loading: false,
        error: err instanceof Error ? err.message : 'Failed to check browser status',
      })
    }
  },

  install: async () => {
    const client = useGlobalStore.getState().client
    if (!client) return

    set({ loading: true, error: null })
    try {
      await client.call('browser.install', {})
      await get().checkStatus()
    } catch (err) {
      set({
        loading: false,
        error: err instanceof Error ? err.message : 'Failed to install browser',
      })
    }
  },

  getConfig: async () => {
    const client = useGlobalStore.getState().client
    if (!client) return null

    try {
      const result = await client.call<BrowserConfig>('browser.config.get', {})
      return result
    } catch {
      return null
    }
  },

  setConfig: async (key: string, value: string) => {
    const client = useGlobalStore.getState().client
    if (!client) return

    set({ loading: true, error: null })
    try {
      await client.call('browser.config.set', { key, value })
      await get().checkStatus()
    } catch (err) {
      set({
        loading: false,
        error: err instanceof Error ? err.message : 'Failed to set config',
      })
    }
  },

  navigate: async (url: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.navigate', { url })
      set({
        loading: false,
        lastResult: result,
        currentUrl: result.url || url,
        currentTitle: result.title || '',
      })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Navigation failed'
      set({ loading: false, error })
      throw err
    }
  },

  back: async () => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.back', {})
      set({ loading: false, lastResult: result, currentTitle: result.title || '' })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Back navigation failed'
      set({ loading: false, error })
      throw err
    }
  },

  forward: async () => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.forward', {})
      set({ loading: false, lastResult: result, currentTitle: result.title || '' })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Forward navigation failed'
      set({ loading: false, error })
      throw err
    }
  },

  reload: async () => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.reload', {})
      set({ loading: false, lastResult: result, currentTitle: result.title || '' })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Reload failed'
      set({ loading: false, error })
      throw err
    }
  },

  click: async (selector: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.click', { selector })
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Click failed'
      set({ loading: false, error })
      throw err
    }
  },

  type: async (selector: string, text: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.type', { selector, text })
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Type failed'
      set({ loading: false, error })
      throw err
    }
  },

  fill: async (selector: string, value: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.fill', { selector, value })
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Fill failed'
      set({ loading: false, error })
      throw err
    }
  },

  select: async (selector: string, values: string[]) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.select', { selector, values })
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Select failed'
      set({ loading: false, error })
      throw err
    }
  },

  hover: async (selector: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.hover', { selector })
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Hover failed'
      set({ loading: false, error })
      throw err
    }
  },

  focus: async (selector: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.focus', { selector })
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Focus failed'
      set({ loading: false, error })
      throw err
    }
  },

  press: async (key: string, selector?: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const params: Record<string, unknown> = { key }
      if (selector) params.selector = selector
      const result = await client.call<BrowserResult>('browser.press', params)
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Press failed'
      set({ loading: false, error })
      throw err
    }
  },

  scroll: async (direction: string, amount?: number, selector?: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const params: Record<string, unknown> = { direction }
      if (amount) params.amount = amount
      if (selector) params.selector = selector
      const result = await client.call<BrowserResult>('browser.scroll', params)
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Scroll failed'
      set({ loading: false, error })
      throw err
    }
  },

  wait: async (selector: string, timeoutMs?: number) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const params: Record<string, unknown> = { selector }
      if (timeoutMs) params.timeout_ms = timeoutMs
      const result = await client.call<BrowserResult>('browser.wait', params)
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Wait failed'
      set({ loading: false, error })
      throw err
    }
  },

  dialog: async (action: 'accept' | 'dismiss', text?: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const params: Record<string, unknown> = { action }
      if (text) params.text = text
      const result = await client.call<BrowserResult>('browser.dialog', params)
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Dialog handling failed'
      set({ loading: false, error })
      throw err
    }
  },

  upload: async (selector: string, files: string[]) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<BrowserResult>('browser.upload', { selector, files })
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Upload failed'
      set({ loading: false, error })
      throw err
    }
  },

  screenshot: async (options?: { path?: string; fullPage?: boolean }) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const params: Record<string, unknown> = {}
      if (options?.path) params.path = options.path
      if (options?.fullPage) params.full_page = options.fullPage
      const result = await client.call<BrowserResult>('browser.screenshot', params)
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Screenshot failed'
      set({ loading: false, error })
      throw err
    }
  },

  snapshot: async () => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<{ snapshot: string }>('browser.snapshot', {})
      set({ loading: false })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Snapshot failed'
      set({ loading: false, error })
      throw err
    }
  },

  pdf: async (options?: { path?: string; format?: string; landscape?: boolean }) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const params: Record<string, unknown> = {}
      if (options?.path) params.path = options.path
      if (options?.format) params.format = options.format
      if (options?.landscape) params.landscape = options.landscape
      const result = await client.call<BrowserResult>('browser.pdf', params)
      set({ loading: false, lastResult: result })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'PDF generation failed'
      set({ loading: false, error })
      throw err
    }
  },

  eval: async (js: string) => {
    const client = useGlobalStore.getState().client
    if (!client) throw new Error('Not connected')

    set({ loading: true, error: null })
    try {
      const result = await client.call<{ result: string; error?: string }>('browser.eval', { js })
      set({ loading: false })
      return result
    } catch (err) {
      const error = err instanceof Error ? err.message : 'Eval failed'
      set({ loading: false, error })
      throw err
    }
  },

  console: async () => {
    const client = useGlobalStore.getState().client
    if (!client) return { messages: [] }

    return client.call<{ messages: ConsoleMessage[] }>('browser.console', {})
  },

  network: async () => {
    const client = useGlobalStore.getState().client
    if (!client) return { requests: [] }

    return client.call<{ requests: NetworkRequest[] }>('browser.network', {})
  },

  clearError: () => set({ error: null }),
  clearResult: () => set({ lastResult: null }),
}))
