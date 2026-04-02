// Timing constants used across the frontend.

/** Delay before resetting "Copied!" feedback to normal state. */
export const COPY_FEEDBACK_MS = 1500

/** Interval for polling worker status from the backend. */
export const WORKER_POLL_MS = 3000

/** Interval for polling worker stats (WorkersWidget). */
export const WORKER_STATS_POLL_MS = 5000

/** Interval for polling available agent strategies. */
export const STRATEGY_POLL_MS = 30000

/** Debounce delay for search inputs. */
export const SEARCH_DEBOUNCE_MS = 300

/** Debounce delay for status refresh on rapid event streams. */
export const REFRESH_DEBOUNCE_MS = 300

/** Debounce delay for store refresh operations (tasks, queue). */
export const STORE_DEBOUNCE_MS = 500

/** Delay before triggering context search in TaskWidget. */
export const CONTEXT_SEARCH_DELAY_MS = 150
