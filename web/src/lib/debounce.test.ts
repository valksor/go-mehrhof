import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { debounce } from './debounce'

describe('debounce', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('does not call fn immediately', () => {
    const fn = vi.fn()
    const debounced = debounce(fn, 100)
    debounced()
    expect(fn).not.toHaveBeenCalled()
  })

  it('calls fn after the delay', () => {
    const fn = vi.fn()
    const debounced = debounce(fn, 100)
    debounced()
    vi.advanceTimersByTime(100)
    expect(fn).toHaveBeenCalledOnce()
  })

  it('passes arguments through to fn', () => {
    const fn = vi.fn()
    const debounced = debounce(fn, 100)
    debounced('a', 'b')
    vi.advanceTimersByTime(100)
    expect(fn).toHaveBeenCalledWith('a', 'b')
  })

  it('resets the timer on subsequent calls', () => {
    const fn = vi.fn()
    const debounced = debounce(fn, 100)
    debounced()
    vi.advanceTimersByTime(50)
    debounced()
    vi.advanceTimersByTime(50)
    expect(fn).not.toHaveBeenCalled()
    vi.advanceTimersByTime(50)
    expect(fn).toHaveBeenCalledOnce()
  })

  it('uses arguments from the last call', () => {
    const fn = vi.fn()
    const debounced = debounce(fn, 100)
    debounced('first')
    debounced('second')
    vi.advanceTimersByTime(100)
    expect(fn).toHaveBeenCalledWith('second')
  })

  it('cancel() prevents the pending call', () => {
    const fn = vi.fn()
    const debounced = debounce(fn, 100)
    debounced()
    debounced.cancel()
    vi.advanceTimersByTime(200)
    expect(fn).not.toHaveBeenCalled()
  })

  it('cancel() is a no-op when nothing is pending', () => {
    const fn = vi.fn()
    const debounced = debounce(fn, 100)
    expect(() => debounced.cancel()).not.toThrow()
  })

  it('can be called again after cancel', () => {
    const fn = vi.fn()
    const debounced = debounce(fn, 100)
    debounced()
    debounced.cancel()
    debounced()
    vi.advanceTimersByTime(100)
    expect(fn).toHaveBeenCalledOnce()
  })

  it('handles zero delay', () => {
    const fn = vi.fn()
    const debounced = debounce(fn, 0)
    debounced()
    vi.advanceTimersByTime(0)
    expect(fn).toHaveBeenCalledOnce()
  })
})
