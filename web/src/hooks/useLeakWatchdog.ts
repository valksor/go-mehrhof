import { useEffect, useRef } from 'react'
import { startBrowserLeakWatchdog } from '../lib/watchdog'

/**
 * Starts the memory leak watchdog on mount and stops it on unmount.
 *
 * Uses a ref so the latest onLeak callback is always called without
 * requiring the effect to re-run when the handler changes.
 */
export function useLeakWatchdog(onLeak: (growthMB: number) => void): void {
  const onLeakRef = useRef(onLeak)

  // Keep ref in sync with latest callback without causing watchdog to restart
  useEffect(() => {
    onLeakRef.current = onLeak
  }, [onLeak])

  useEffect(() => {
    return startBrowserLeakWatchdog((growthMB) => onLeakRef.current(growthMB))
  }, [])
}
