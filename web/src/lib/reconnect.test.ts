import { describe, it, expect } from 'vitest'
import { reconnectDelay } from './reconnect'

describe('reconnectDelay', () => {
  it('returns a value in expected range for attempt 1', () => {
    // base = 1000, jitter ±20% → [800, 1200]
    for (let i = 0; i < 20; i++) {
      const delay = reconnectDelay(1)
      expect(delay).toBeGreaterThanOrEqual(800)
      expect(delay).toBeLessThanOrEqual(1200)
    }
  })

  it('returns a value in expected range for attempt 2', () => {
    // base = 2000, jitter ±20% → [1600, 2400]
    for (let i = 0; i < 20; i++) {
      const delay = reconnectDelay(2)
      expect(delay).toBeGreaterThanOrEqual(1600)
      expect(delay).toBeLessThanOrEqual(2400)
    }
  })

  it('respects maxDelay cap', () => {
    // attempt 20: 2^19 * 1000 = huge, capped to 30000, then jitter → [24000, 36000]
    for (let i = 0; i < 20; i++) {
      const delay = reconnectDelay(20)
      expect(delay).toBeLessThanOrEqual(36000)
      expect(delay).toBeGreaterThanOrEqual(24000)
    }
  })

  it('supports custom maxDelay', () => {
    for (let i = 0; i < 20; i++) {
      const delay = reconnectDelay(20, 5000)
      expect(delay).toBeLessThanOrEqual(6000)
      expect(delay).toBeGreaterThanOrEqual(4000)
    }
  })

  it('always returns a rounded integer', () => {
    for (let i = 0; i < 20; i++) {
      const delay = reconnectDelay(3)
      expect(Number.isInteger(delay)).toBe(true)
    }
  })
})
