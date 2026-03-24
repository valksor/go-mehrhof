import { describe, it, expect, vi, beforeEach } from 'vitest'
import { TypedEventEmitter, worktreeEvents, chatEvents } from './events'

describe('TypedEventEmitter', () => {
  let emitter: TypedEventEmitter

  beforeEach(() => {
    emitter = new TypedEventEmitter()
  })

  it('registers handler and calls it on emit', () => {
    const handler = vi.fn()
    emitter.on('state_changed', handler)
    const event = { type: 'state_changed' as const, state: 'loaded' as const }
    emitter.emit('state_changed', event)
    expect(handler).toHaveBeenCalledWith(event)
  })

  it('does not call handlers for other event types', () => {
    const handler = vi.fn()
    emitter.on('state_changed', handler)
    emitter.emit('heartbeat', { type: 'heartbeat' as const })
    expect(handler).not.toHaveBeenCalled()
  })

  it('supports multiple handlers for the same event', () => {
    const h1 = vi.fn()
    const h2 = vi.fn()
    emitter.on('heartbeat', h1)
    emitter.on('heartbeat', h2)
    emitter.emit('heartbeat', { type: 'heartbeat' as const })
    expect(h1).toHaveBeenCalled()
    expect(h2).toHaveBeenCalled()
  })

  it('on() returns an unsubscribe function', () => {
    const handler = vi.fn()
    const unsub = emitter.on('heartbeat', handler)
    unsub()
    emitter.emit('heartbeat', { type: 'heartbeat' as const })
    expect(handler).not.toHaveBeenCalled()
  })

  it('off() with handler removes specific handler', () => {
    const h1 = vi.fn()
    const h2 = vi.fn()
    emitter.on('heartbeat', h1)
    emitter.on('heartbeat', h2)
    emitter.off('heartbeat', h1)
    emitter.emit('heartbeat', { type: 'heartbeat' as const })
    expect(h1).not.toHaveBeenCalled()
    expect(h2).toHaveBeenCalled()
  })

  it('off() without handler removes all handlers for event', () => {
    const h1 = vi.fn()
    const h2 = vi.fn()
    emitter.on('heartbeat', h1)
    emitter.on('heartbeat', h2)
    emitter.off('heartbeat')
    emitter.emit('heartbeat', { type: 'heartbeat' as const })
    expect(h1).not.toHaveBeenCalled()
    expect(h2).not.toHaveBeenCalled()
  })

  it('clear() removes all handlers', () => {
    const h1 = vi.fn()
    const h2 = vi.fn()
    emitter.on('state_changed', h1)
    emitter.on('heartbeat', h2)
    emitter.clear()
    emitter.emit('state_changed', { type: 'state_changed' as const, state: 'none' as const })
    emitter.emit('heartbeat', { type: 'heartbeat' as const })
    expect(h1).not.toHaveBeenCalled()
    expect(h2).not.toHaveBeenCalled()
  })

  it('dispatch() routes by msg.type', () => {
    const handler = vi.fn()
    emitter.on('job_completed', handler)
    emitter.dispatch({ type: 'job_completed', job_id: '123' })
    expect(handler).toHaveBeenCalledWith({ type: 'job_completed', job_id: '123' })
  })

  it('dispatch() ignores messages without type', () => {
    const handler = vi.fn()
    emitter.on('heartbeat', handler)
    emitter.dispatch({ data: 'no type' })
    expect(handler).not.toHaveBeenCalled()
  })

  it('dispatch() ignores unregistered event types', () => {
    const handler = vi.fn()
    emitter.on('heartbeat', handler)
    emitter.dispatch({ type: 'state_changed', state: 'none' })
    expect(handler).not.toHaveBeenCalled()
  })

  it('emit on event with no handlers does not throw', () => {
    expect(() => emitter.emit('heartbeat', { type: 'heartbeat' as const })).not.toThrow()
  })

  it('off on event with no handlers does not throw', () => {
    expect(() => emitter.off('heartbeat')).not.toThrow()
  })
})

describe('singleton emitters', () => {
  it('worktreeEvents is an instance of TypedEventEmitter', () => {
    expect(worktreeEvents).toBeInstanceOf(TypedEventEmitter)
  })

  it('chatEvents is an instance of TypedEventEmitter', () => {
    expect(chatEvents).toBeInstanceOf(TypedEventEmitter)
  })
})
