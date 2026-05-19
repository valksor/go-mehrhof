import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { startBrowserLeakWatchdog } from './watchdog'

// Type for performance extended with the memory API
interface PerformanceWithMemory extends Performance {
  measureUserAgentSpecificMemory?: () => Promise<{ bytes: number }>
}

describe('startBrowserLeakWatchdog', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  describe('when measureUserAgentSpecificMemory is not available', () => {
    it('returns a no-op stop function', () => {
      const perf = performance as PerformanceWithMemory
      const original = perf.measureUserAgentSpecificMemory
      delete perf.measureUserAgentSpecificMemory

      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const onLeak = vi.fn()

      const stop = startBrowserLeakWatchdog(onLeak)

      expect(typeof stop).toBe('function')
      expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('measureUserAgentSpecificMemory not available'))

      // Calling stop should not throw
      expect(() => stop()).not.toThrow()

      perf.measureUserAgentSpecificMemory = original
    })

    it('does not call onLeak when API is not available', () => {
      const perf = performance as PerformanceWithMemory
      const original = perf.measureUserAgentSpecificMemory
      delete perf.measureUserAgentSpecificMemory

      vi.spyOn(console, 'warn').mockImplementation(() => {})
      const onLeak = vi.fn()

      startBrowserLeakWatchdog(onLeak)
      vi.runAllTimers()

      expect(onLeak).not.toHaveBeenCalled()

      perf.measureUserAgentSpecificMemory = original
    })
  })

  describe('when measureUserAgentSpecificMemory is available', () => {
    let mockMeasure: ReturnType<typeof vi.fn>
    let sampleIndex: number

    const setupMockMeasure = (samples: number[]) => {
      sampleIndex = 0
      mockMeasure = vi.fn().mockImplementation(async () => {
        const mb = samples[sampleIndex] ?? samples[samples.length - 1]
        sampleIndex++
        return { bytes: mb * 1024 * 1024 }
      })
      ;(performance as PerformanceWithMemory).measureUserAgentSpecificMemory = mockMeasure as unknown as () => Promise<{
        bytes: number
      }>
    }

    afterEach(() => {
      delete (performance as PerformanceWithMemory).measureUserAgentSpecificMemory
    })

    it('returns a stop function', () => {
      setupMockMeasure([100])
      const stop = startBrowserLeakWatchdog(vi.fn())
      expect(typeof stop).toBe('function')
      stop()
    })

    it('stop function cancels the interval', async () => {
      setupMockMeasure([100, 200, 300])
      const onLeak = vi.fn()
      const stop = startBrowserLeakWatchdog(onLeak, 1000, 3, 50)

      stop()

      // Advance time — interval should have been cleared
      await vi.runAllTimersAsync()

      expect(mockMeasure).not.toHaveBeenCalled()
    })

    it('samples memory on each interval tick', async () => {
      setupMockMeasure([100, 110, 120])
      const onLeak = vi.fn()
      startBrowserLeakWatchdog(onLeak, 1000, 8, 200)

      await vi.advanceTimersByTimeAsync(1000)
      expect(mockMeasure).toHaveBeenCalledTimes(1)

      await vi.advanceTimersByTimeAsync(1000)
      expect(mockMeasure).toHaveBeenCalledTimes(2)

      await vi.advanceTimersByTimeAsync(1000)
      expect(mockMeasure).toHaveBeenCalledTimes(3)
    })

    it('does not fire leak callback before window is full', async () => {
      // windowSize=4, only 3 samples so far
      setupMockMeasure([100, 150, 200, 250])
      const onLeak = vi.fn()
      startBrowserLeakWatchdog(onLeak, 1000, 4, 50)

      // Advance 3 ticks (window size is 4, not yet full)
      await vi.advanceTimersByTimeAsync(3000)

      expect(onLeak).not.toHaveBeenCalled()
    })

    it('fires leak callback when window is full and growth exceeds threshold', async () => {
      // Each tick goes up by 30MB, over a window of 4 = total growth 90MB > threshold 50MB
      // and the series is monotonically increasing (no drops)
      setupMockMeasure([100, 130, 160, 190])
      const onLeak = vi.fn()
      startBrowserLeakWatchdog(onLeak, 1000, 4, 50, 2)

      await vi.advanceTimersByTimeAsync(4000)

      expect(onLeak).toHaveBeenCalledTimes(1)
      expect(onLeak).toHaveBeenCalledWith(expect.closeTo(90, 1))
    })

    it('does not fire when growth is below threshold', async () => {
      // Growth = 20MB across window of 4 samples, threshold is 50MB
      setupMockMeasure([100, 105, 110, 120])
      const onLeak = vi.fn()
      startBrowserLeakWatchdog(onLeak, 1000, 4, 50, 2)

      await vi.advanceTimersByTimeAsync(4000)

      expect(onLeak).not.toHaveBeenCalled()
    })

    it('does not fire when memory dropped significantly (GC occurred)', async () => {
      // Has a significant drop mid-series — should not be considered a leak
      // noiseToleranceMB=2, so a 30MB drop disqualifies
      setupMockMeasure([100, 150, 120, 170])
      const onLeak = vi.fn()
      startBrowserLeakWatchdog(onLeak, 1000, 4, 50, 2)

      await vi.advanceTimersByTimeAsync(4000)

      expect(onLeak).not.toHaveBeenCalled()
    })

    it('allows small dips within noise tolerance', async () => {
      // Each sample grows by 30MB with a 1MB dip (within 2MB tolerance)
      // Total growth = 30 + 29 + 30 = 89MB > threshold 50MB
      setupMockMeasure([100, 130, 129, 160])
      const onLeak = vi.fn()
      startBrowserLeakWatchdog(onLeak, 1000, 4, 50, 2)

      await vi.advanceTimersByTimeAsync(4000)

      expect(onLeak).toHaveBeenCalledTimes(1)
    })

    it('uses rolling window — evicts oldest samples', async () => {
      // First 4 samples are stable, next 4 show leak
      const samples = [
        100,
        102,
        101,
        103, // stable — no leak when full at tick 4
        150,
        180,
        210,
        240, // steep growth starting tick 5
      ]
      setupMockMeasure(samples)
      const onLeak = vi.fn()
      // windowSize=4, threshold=50, tolerance=2
      startBrowserLeakWatchdog(onLeak, 1000, 4, 50, 2)

      // First window: [100, 102, 101, 103] — growth ~3MB, no leak
      await vi.advanceTimersByTimeAsync(4000)
      expect(onLeak).not.toHaveBeenCalled()

      // After tick 5: window = [102, 101, 103, 150] — has a dip (102→101), no leak
      await vi.advanceTimersByTimeAsync(1000)
      expect(onLeak).not.toHaveBeenCalled()

      // After tick 6: window = [101, 103, 150, 180] — growth=79MB and monotone, should fire
      await vi.advanceTimersByTimeAsync(1000)
      expect(onLeak).toHaveBeenCalled()
    })

    it('continues monitoring after first leak detection', async () => {
      setupMockMeasure([100, 200, 300, 400, 500, 600])
      const onLeak = vi.fn()
      startBrowserLeakWatchdog(onLeak, 1000, 4, 50, 2)

      // 6 ticks: window cycles through [100,200,300,400] then [200,300,400,500] then [300,400,500,600]
      await vi.advanceTimersByTimeAsync(6000)

      // Should fire multiple times (each window shows growth > 50MB)
      expect(onLeak.mock.calls.length).toBeGreaterThan(1)
    })

    it('uses custom intervalMs', async () => {
      setupMockMeasure([100, 200])
      const onLeak = vi.fn()
      startBrowserLeakWatchdog(onLeak, 5000, 8, 50)

      // After 4999ms — should not have sampled yet
      await vi.advanceTimersByTimeAsync(4999)
      expect(mockMeasure).not.toHaveBeenCalled()

      // After 5000ms — should have sampled once
      await vi.advanceTimersByTimeAsync(1)
      expect(mockMeasure).toHaveBeenCalledTimes(1)
    })

    it('handles API errors gracefully without crashing', async () => {
      mockMeasure = vi.fn().mockRejectedValue(new Error('cross-origin isolation not active'))
      ;(performance as PerformanceWithMemory).measureUserAgentSpecificMemory = mockMeasure as unknown as () => Promise<{
        bytes: number
      }>

      const onLeak = vi.fn()
      const stop = startBrowserLeakWatchdog(onLeak, 1000, 4, 50)

      await vi.advanceTimersByTimeAsync(4000)

      expect(onLeak).not.toHaveBeenCalled()
      stop()
    })
  })
})
