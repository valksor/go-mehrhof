interface AsyncActionOptions {
  loadingKey?: string
  errorKey?: string
  showToast?: boolean
  toastMessage?: string
}

/**
 * Wraps an async store action with automatic loading/error state management.
 *
 * Usage:
 * ```ts
 * asyncAction(set, async () => {
 *   const result = await rpc('task.start', params)
 *   set({ task: result })
 * }, { loadingKey: 'isStarting' })
 * ```
 */
export async function asyncAction(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Zustand set has store-specific types; we only pass plain objects
  set: (partial: any) => void,
  fn: () => Promise<void>,
  opts: AsyncActionOptions = {}
): Promise<void> {
  const loadingKey = opts.loadingKey || 'isLoading'
  const errorKey = opts.errorKey || 'error'

  set({ [loadingKey]: true, [errorKey]: null })
  try {
    await fn()
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    set({ [errorKey]: message })
    if (opts.showToast) {
      // Log to console for now; toast integration can be added later
      console.error(`[store action] ${opts.toastMessage || message}`)
    }
    throw err // Re-throw so callers can handle
  } finally {
    set({ [loadingKey]: false })
  }
}
