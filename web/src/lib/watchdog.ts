/**
 * Monitors browser heap growth rate and triggers a callback if a sustained
 * memory leak is detected.
 *
 * Uses a rolling window to distinguish real leaks (heap grows monotonically
 * with no GC recovery) from legitimate spikes (GC drops it back down).
 *
 * Requires: Chromium-based browser + cross-origin isolation headers
 * (COOP: same-origin, COEP: require-corp). Degrades gracefully otherwise.
 *
 * @param onLeak              - Called with growth MB when a leak is detected
 * @param intervalMs          - Milliseconds between samples
 * @param windowSize          - Number of samples in the rolling window
 * @param growthThresholdMB   - MB growth over the window before triggering
 * @param noiseToleranceMB    - Allowed dip before considering GC recovered memory
 * @returns stop              - Call to cancel the interval
 */
// Chromium-specific Memory Measurement API (behind cross-origin isolation)
interface MemoryMeasurement {
  bytes: number
}

interface PerformanceWithMemory extends Performance {
  measureUserAgentSpecificMemory?: () => Promise<MemoryMeasurement>
}

export function startBrowserLeakWatchdog(
  onLeak: (growthMB: number) => void,
  intervalMs = 15_000,
  windowSize = 8,
  growthThresholdMB = 100,
  noiseToleranceMB = 2,
): () => void {
  const perf = performance as PerformanceWithMemory
  if (typeof perf.measureUserAgentSpecificMemory !== 'function') {
    console.warn(
      'LeakWatchdog: measureUserAgentSpecificMemory not available — ' +
        'cross-origin isolation (COOP/COEP headers) required',
    )
    return () => {}
  }

  const samples: number[] = []

  const id = setInterval(async () => {
    try {
      const result = await perf.measureUserAgentSpecificMemory!()
      const mb: number = result.bytes / 1024 / 1024
      samples.push(mb)
      if (samples.length > windowSize) samples.shift()

      if (samples.length === windowSize) {
        const growth = samples.at(-1)! - samples[0]
        const neverDropped = samples.every((v, i) => i === 0 || v >= samples[i - 1] - noiseToleranceMB)

        if (growth > growthThresholdMB && neverDropped) {
          onLeak(growth)
        }
      }
    } catch {
      // API unavailable or cross-origin isolation not active
    }
  }, intervalMs)

  return () => clearInterval(id)
}
