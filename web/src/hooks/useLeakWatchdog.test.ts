import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useLeakWatchdog } from './useLeakWatchdog'

const mockCleanup = vi.fn()
vi.mock('../lib/watchdog', () => ({
  startBrowserLeakWatchdog: vi.fn(() => mockCleanup),
}))

describe('useLeakWatchdog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls startBrowserLeakWatchdog on mount', async () => {
    const { startBrowserLeakWatchdog } = await import('../lib/watchdog')
    renderHook(() => useLeakWatchdog(vi.fn()))
    expect(startBrowserLeakWatchdog).toHaveBeenCalled()
  })

  it('calls cleanup on unmount', async () => {
    const { unmount } = renderHook(() => useLeakWatchdog(vi.fn()))
    unmount()
    expect(mockCleanup).toHaveBeenCalled()
  })

  it('passes growth callback through ref', async () => {
    const { startBrowserLeakWatchdog } = await import('../lib/watchdog')
    const onLeak = vi.fn()
    renderHook(() => useLeakWatchdog(onLeak))
    // Extract the callback passed to startBrowserLeakWatchdog
    const registeredCallback = (startBrowserLeakWatchdog as ReturnType<typeof vi.fn>).mock.calls[0][0]
    registeredCallback(100)
    expect(onLeak).toHaveBeenCalledWith(100)
  })
})
