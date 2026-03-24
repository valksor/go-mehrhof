import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'

// Mock the entire globalStore module — useGlobalStore() is called without selector
let mockState = { client: null as { call: ReturnType<typeof vi.fn> } | null, connected: false }

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: () => mockState,
}))

// Import after mock
import { useDocsURL } from './useDocsURL'

describe('useDocsURL', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockState = { client: null, connected: false }
  })

  it('returns null when disconnected', () => {
    mockState = { client: null, connected: false }
    const { result } = renderHook(() => useDocsURL())
    expect(result.current).toBeNull()
  })

  it('fetches and returns URL when connected', async () => {
    const callFn = vi.fn().mockResolvedValue({ url: 'https://docs.example.com/latest', version: '1.0.0' })
    mockState = { client: { call: callFn }, connected: true }

    const { result } = renderHook(() => useDocsURL())

    await waitFor(() => {
      expect(result.current).toEqual({ url: 'https://docs.example.com/latest', version: '1.0.0' })
    })
    expect(callFn).toHaveBeenCalledWith('system.docsURL', {})
  })

  it('returns null when client is null even if connected', () => {
    mockState = { client: null, connected: true }
    const { result } = renderHook(() => useDocsURL())
    expect(result.current).toBeNull()
  })

  it('returns null when client.call rejects', async () => {
    const callFn = vi.fn().mockRejectedValue(new Error('Network error'))
    mockState = { client: { call: callFn }, connected: true }

    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const { result } = renderHook(() => useDocsURL())

    await waitFor(() => {
      expect(callFn).toHaveBeenCalled()
    })
    expect(result.current).toBeNull()
    spy.mockRestore()
  })
})
