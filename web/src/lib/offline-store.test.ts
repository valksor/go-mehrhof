import 'fake-indexeddb/auto'
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import {
  cacheGlobalState,
  getCachedGlobalState,
  cacheProjectState,
  getCachedProjectState,
  clearCache,
  getCacheStats,
} from './offline-store'

describe('offline-store', () => {
  beforeEach(async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    await clearCache()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('cacheGlobalState + getCachedGlobalState', () => {
    it('round-trips data', async () => {
      const data = { projects: [{ id: 'p1' }], activeTasks: [] }
      await cacheGlobalState(data)
      const result = await getCachedGlobalState<typeof data>()
      expect(result).not.toBeNull()
      expect(result!.data).toEqual(data)
    })

    it('returns stale: false for fresh data', async () => {
      await cacheGlobalState({ test: true })
      const result = await getCachedGlobalState()
      expect(result!.stale).toBe(false)
    })

    it('returns stale: true after 5 minutes', async () => {
      await cacheGlobalState({ test: true })
      vi.advanceTimersByTime(5 * 60 * 1000 + 1)
      const result = await getCachedGlobalState()
      expect(result!.stale).toBe(true)
    })

    it('returns null after 24 hours', async () => {
      await cacheGlobalState({ test: true })
      vi.advanceTimersByTime(24 * 60 * 60 * 1000 + 1)
      const result = await getCachedGlobalState()
      expect(result).toBeNull()
    })

    it('returns null when no data cached', async () => {
      const result = await getCachedGlobalState()
      expect(result).toBeNull()
    })
  })

  describe('cacheProjectState + getCachedProjectState', () => {
    it('round-trips project data with id', async () => {
      const data = { state: 'implementing', task: { title: 'Fix bug' } }
      await cacheProjectState('proj-1', data)
      const result = await getCachedProjectState<typeof data>('proj-1')
      expect(result).not.toBeNull()
      expect(result!.data).toEqual(data)
    })

    it('different project ids are independent', async () => {
      await cacheProjectState('proj-1', { a: 1 })
      await cacheProjectState('proj-2', { b: 2 })
      const r1 = await getCachedProjectState<{ a: number }>('proj-1')
      const r2 = await getCachedProjectState<{ b: number }>('proj-2')
      expect(r1!.data.a).toBe(1)
      expect(r2!.data.b).toBe(2)
    })

    it('returns null for non-existent project', async () => {
      const result = await getCachedProjectState('missing')
      expect(result).toBeNull()
    })

    it('returns null for expired project entries', async () => {
      await cacheProjectState('proj-old', { old: true })
      vi.advanceTimersByTime(25 * 60 * 60 * 1000) // 25 hours
      const result = await getCachedProjectState('proj-old')
      expect(result).toBeNull()
    })
  })

  describe('clearCache', () => {
    it('removes all cached entries', async () => {
      await cacheGlobalState({ data: 'global' })
      await cacheProjectState('proj-1', { data: 'project' })
      await clearCache()
      expect(await getCachedGlobalState()).toBeNull()
      expect(await getCachedProjectState('proj-1')).toBeNull()
    })

    it('is safe to call on empty cache', async () => {
      await expect(clearCache()).resolves.not.toThrow()
    })
  })

  describe('getCacheStats', () => {
    it('returns zero entries for empty cache', async () => {
      const stats = await getCacheStats()
      expect(stats.entries).toBe(0)
      expect(stats.oldestMs).toBe(0)
    })

    it('counts entries correctly', async () => {
      await cacheGlobalState({ g: 1 })
      await cacheProjectState('p1', { p: 1 })
      const stats = await getCacheStats()
      expect(stats.entries).toBe(2)
    })

    it('reports age of oldest entry', async () => {
      await cacheGlobalState({ g: 1 })
      vi.advanceTimersByTime(10000) // 10 seconds later
      await cacheProjectState('p1', { p: 1 })
      const stats = await getCacheStats()
      expect(stats.oldestMs).toBeGreaterThanOrEqual(10000)
    })
  })
})
