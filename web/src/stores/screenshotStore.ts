import { create } from 'zustand'
import type { SocketClient } from '../lib/socket'

export interface Screenshot {
  id: string
  task_id: string
  path: string
  filename: string
  timestamp: string
  source: 'agent' | 'user'
  step?: string
  agent?: string
  format: 'png' | 'jpeg'
  width: number
  height: number
  size_bytes: number
}

interface ScreenshotState {
  // Data
  screenshots: Screenshot[]
  loading: boolean
  error: string | null

  // Selection and attachment
  selectedId: string | null
  attachedIds: string[]

  // Full screenshot data (loaded on demand)
  screenshotData: Record<string, string>

  // Actions - accept client parameter to avoid circular dependency with projectStore
  load: (client?: SocketClient | null) => Promise<void>
  add: (screenshot: Screenshot) => void
  remove: (id: string) => void
  select: (id: string | null) => void
  attach: (id: string) => void
  detach: (id: string) => void
  clearAttached: () => void
  captureScreenshot: (client?: SocketClient | null, source?: string, step?: string) => Promise<void>
  deleteScreenshot: (id: string, client?: SocketClient | null) => Promise<void>

  getScreenshot: (id: string, client?: SocketClient | null) => Promise<string | null>

  // Event handlers for WebSocket
  handleScreenshotCaptured: (screenshot: Screenshot) => void
  handleScreenshotDeleted: (id: string) => void
}

export const useScreenshotStore = create<ScreenshotState>((set, get) => ({
  screenshots: [],
  loading: false,
  error: null,
  selectedId: null,
  attachedIds: [],
  screenshotData: {},

  load: async (client) => {
    if (!client) return

    set({ loading: true, error: null })

    try {
      const result = await client.call<{ screenshots: Screenshot[] }>('screenshots.list', {})
      set({ screenshots: result.screenshots || [], loading: false })
    } catch (err) {
      set({
        loading: false,
        error: err instanceof Error ? err.message : 'Failed to load screenshots',
      })
    }
  },

  add: (screenshot: Screenshot) => {
    set((state) => ({
      screenshots: [screenshot, ...state.screenshots],
    }))
  },

  remove: (id: string) => {
    set((state) => ({
      screenshots: state.screenshots.filter((s) => s.id !== id),
      selectedId: state.selectedId === id ? null : state.selectedId,
      attachedIds: state.attachedIds.filter((aid) => aid !== id),
    }))
  },

  select: (id: string | null) => {
    set({ selectedId: id })
  },

  attach: (id: string) => {
    set((state) => {
      if (state.attachedIds.includes(id)) return state
      return { attachedIds: [...state.attachedIds, id] }
    })
  },

  detach: (id: string) => {
    set((state) => ({
      attachedIds: state.attachedIds.filter((aid) => aid !== id),
    }))
  },

  clearAttached: () => {
    set({ attachedIds: [] })
  },

  captureScreenshot: async (client, source = 'user', step) => {
    if (!client) return

    set({ loading: true, error: null })

    try {
      await client.call('screenshots.capture', { source, step })
      // The actual addition will happen via the WebSocket screenshot_captured event
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Failed to capture screenshot' })
    } finally {
      set({ loading: false })
    }
  },

  deleteScreenshot: async (id: string, client) => {
    if (!client) return

    try {
      await client.call('screenshots.delete', { screenshot_id: id })
      // The actual removal will happen via the WebSocket event
    } catch (err) {
      set({ error: err instanceof Error ? err.message : 'Failed to delete screenshot' })
    }
  },

  getScreenshot: async (id: string, client): Promise<string | null> => {
    // Return cached data if already loaded
    const cached = get().screenshotData[id]
    if (cached) return cached

    if (!client) return null

    try {
      const result = await client.call<{ data: string; format: string }>('screenshots.get', { screenshot_id: id })
      const dataUrl = `data:image/${result.format};base64,${result.data}`
      set((state) => ({
        screenshotData: { ...state.screenshotData, [id]: dataUrl },
      }))
      return dataUrl
    } catch (err) {
      if (import.meta.env.DEV) {
        console.warn('Could not fetch screenshot data:', err)
      }
      return null
    }
  },

  handleScreenshotCaptured: (screenshot: Screenshot) => {
    get().add(screenshot)
  },

  handleScreenshotDeleted: (id: string) => {
    get().remove(id)
  },
}))

// Helper to get screenshot by ID
export function getScreenshotById(id: string): Screenshot | undefined {
  return useScreenshotStore.getState().screenshots.find((s) => s.id === id)
}

// Helper to format screenshot reference for chat
export function formatScreenshotRef(id: string): string {
  return `@screenshot-${id}`
}

// Helper to parse screenshot references from text
export function parseScreenshotRefs(text: string): string[] {
  const regex = /@screenshot-([a-zA-Z0-9]+)/g
  const matches = []
  let match
  while ((match = regex.exec(text)) !== null) {
    matches.push(match[1])
  }
  return matches
}
