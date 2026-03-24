import { describe, it, expect, beforeEach } from 'vitest'
import { useDebugStore } from './debugStore'

describe('debugStore', () => {
  beforeEach(() => {
    useDebugStore.setState({ enabled: false, logs: [] })
  })

  it('has correct initial state', () => {
    const { enabled, logs } = useDebugStore.getState()
    expect(enabled).toBe(false)
    expect(logs).toEqual([])
  })

  it('setEnabled(true) enables and clears logs', () => {
    // Add a log first to verify clearing
    useDebugStore.setState({ enabled: true, logs: [{ id: 1, timestamp: new Date(), direction: 'request' as const, data: '{}' }] })
    useDebugStore.getState().setEnabled(true)
    const { enabled, logs } = useDebugStore.getState()
    expect(enabled).toBe(true)
    expect(logs).toEqual([])
  })

  it('setEnabled(false) disables and clears logs', () => {
    useDebugStore.setState({ enabled: true, logs: [{ id: 1, timestamp: new Date(), direction: 'request' as const, data: '{}' }] })
    useDebugStore.getState().setEnabled(false)
    const { enabled, logs } = useDebugStore.getState()
    expect(enabled).toBe(false)
    expect(logs).toEqual([])
  })

  it('addLog adds entry when enabled', () => {
    useDebugStore.getState().setEnabled(true)
    useDebugStore.getState().addLog({ direction: 'request', method: 'task.start', data: '{"source":"file:test.md"}' })
    const { logs } = useDebugStore.getState()
    expect(logs).toHaveLength(1)
    expect(logs[0].direction).toBe('request')
    expect(logs[0].method).toBe('task.start')
    expect(logs[0].data).toBe('{"source":"file:test.md"}')
    expect(logs[0].id).toBeGreaterThan(0)
    expect(logs[0].timestamp).toBeInstanceOf(Date)
  })

  it('addLog is a no-op when disabled', () => {
    useDebugStore.setState({ enabled: false, logs: [] })
    useDebugStore.getState().addLog({ direction: 'request', data: '{}' })
    expect(useDebugStore.getState().logs).toHaveLength(0)
  })

  it('addLog caps entries at 200', () => {
    useDebugStore.getState().setEnabled(true)
    for (let i = 0; i < 210; i++) {
      useDebugStore.getState().addLog({ direction: 'request', data: `{"i":${i}}` })
    }
    expect(useDebugStore.getState().logs).toHaveLength(200)
    // Should keep the last 200 entries (entries 10-209)
    expect(useDebugStore.getState().logs[0].data).toBe('{"i":10}')
  })

  it('addLog auto-increments IDs', () => {
    useDebugStore.getState().setEnabled(true)
    useDebugStore.getState().addLog({ direction: 'request', data: '{}' })
    useDebugStore.getState().addLog({ direction: 'response', data: '{}' })
    const { logs } = useDebugStore.getState()
    expect(logs[1].id).toBeGreaterThan(logs[0].id)
  })

  it('clearLogs empties the log array', () => {
    useDebugStore.getState().setEnabled(true)
    useDebugStore.getState().addLog({ direction: 'request', data: '{}' })
    useDebugStore.getState().clearLogs()
    expect(useDebugStore.getState().logs).toEqual([])
  })
})
