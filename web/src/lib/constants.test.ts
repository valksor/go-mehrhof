import { describe, it, expect } from 'vitest'
import {
  COPY_FEEDBACK_MS,
  WORKER_POLL_MS,
  WORKER_STATS_POLL_MS,
  STRATEGY_POLL_MS,
  SEARCH_DEBOUNCE_MS,
  STORE_DEBOUNCE_MS,
  CONTEXT_SEARCH_DELAY_MS,
} from './constants'

describe('constants', () => {
  it('exports COPY_FEEDBACK_MS as a positive number', () => {
    expect(COPY_FEEDBACK_MS).toBe(1500)
  })

  it('exports WORKER_POLL_MS as a positive number', () => {
    expect(WORKER_POLL_MS).toBe(3000)
  })

  it('exports WORKER_STATS_POLL_MS as a positive number', () => {
    expect(WORKER_STATS_POLL_MS).toBe(5000)
  })

  it('exports STRATEGY_POLL_MS as a positive number', () => {
    expect(STRATEGY_POLL_MS).toBe(30000)
  })

  it('exports SEARCH_DEBOUNCE_MS as a positive number', () => {
    expect(SEARCH_DEBOUNCE_MS).toBe(300)
  })

  it('exports STORE_DEBOUNCE_MS as a positive number', () => {
    expect(STORE_DEBOUNCE_MS).toBe(500)
  })

  it('exports CONTEXT_SEARCH_DELAY_MS as a positive number', () => {
    expect(CONTEXT_SEARCH_DELAY_MS).toBe(150)
  })

  it('all constants are finite numbers', () => {
    const all = [
      COPY_FEEDBACK_MS,
      WORKER_POLL_MS,
      WORKER_STATS_POLL_MS,
      STRATEGY_POLL_MS,
      SEARCH_DEBOUNCE_MS,
      STORE_DEBOUNCE_MS,
      CONTEXT_SEARCH_DELAY_MS,
    ]
    for (const val of all) {
      expect(typeof val).toBe('number')
      expect(Number.isFinite(val)).toBe(true)
      expect(val).toBeGreaterThan(0)
    }
  })
})
