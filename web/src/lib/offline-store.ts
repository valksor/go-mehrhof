/**
 * IndexedDB-backed offline cache for instant UI loading on reconnect.
 *
 * All operations are non-blocking and wrapped in try/catch so that a
 * cache failure (e.g. private browsing, quota exceeded) never prevents
 * the UI from working.
 */

const DB_NAME = 'kvelmo-offline'
const DB_VERSION = 1
const STORE_NAME = 'cache'
const PREFIX = 'kvelmo:'
const CACHE_VERSION = 1

interface CachedState<T> {
  key: string
  data: T
  timestamp: number
  version: number
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onerror = () => reject(request.error)
    request.onsuccess = () => resolve(request.result)
    request.onupgradeneeded = (event) => {
      const db = (event.target as IDBOpenDBRequest).result
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME, { keyPath: 'key' })
      }
    }
  })
}

async function putItem<T>(key: string, data: T): Promise<void> {
  const db = await openDB()
  const tx = db.transaction(STORE_NAME, 'readwrite')
  const store = tx.objectStore(STORE_NAME)
  const entry: CachedState<T> = {
    key,
    data,
    timestamp: Date.now(),
    version: CACHE_VERSION,
  }
  store.put(entry)
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => { db.close(); resolve() }
    tx.onerror = () => { db.close(); reject(tx.error) }
  })
}

async function getItem<T>(key: string): Promise<CachedState<T> | null> {
  const db = await openDB()
  const tx = db.transaction(STORE_NAME, 'readonly')
  const store = tx.objectStore(STORE_NAME)
  return new Promise((resolve, reject) => {
    const request = store.get(key)
    request.onsuccess = () => { db.close(); resolve(request.result ?? null) }
    request.onerror = () => { db.close(); reject(request.error) }
  })
}

async function deleteItem(key: string): Promise<void> {
  const db = await openDB()
  const tx = db.transaction(STORE_NAME, 'readwrite')
  const store = tx.objectStore(STORE_NAME)
  store.delete(key)
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => { db.close(); resolve() }
    tx.onerror = () => { db.close(); reject(tx.error) }
  })
}

async function getAllKeys(): Promise<string[]> {
  const db = await openDB()
  const tx = db.transaction(STORE_NAME, 'readonly')
  const store = tx.objectStore(STORE_NAME)
  return new Promise((resolve, reject) => {
    const request = store.getAllKeys()
    request.onsuccess = () => { db.close(); resolve(request.result as string[]) }
    request.onerror = () => { db.close(); reject(request.error) }
  })
}

async function getAllEntries<T>(): Promise<CachedState<T>[]> {
  const db = await openDB()
  const tx = db.transaction(STORE_NAME, 'readonly')
  const store = tx.objectStore(STORE_NAME)
  return new Promise((resolve, reject) => {
    const request = store.getAll()
    request.onsuccess = () => { db.close(); resolve(request.result) }
    request.onerror = () => { db.close(); reject(request.error) }
  })
}

// ---------------------------------------------------------------------------
// Staleness and expiry
// ---------------------------------------------------------------------------

const MAX_AGE_MS = 24 * 60 * 60 * 1000 // 24 hours — discard after this
const STALE_AFTER_MS = 5 * 60 * 1000    // 5 minutes — mark as stale

function evaluateCache<T>(
  cached: CachedState<T> | null,
): { data: T; stale: boolean } | null {
  if (!cached || cached.version !== CACHE_VERSION) return null
  const age = Date.now() - cached.timestamp
  if (age > MAX_AGE_MS) return null
  return {
    data: cached.data,
    stale: age > STALE_AFTER_MS,
  }
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Cache global state (project list, workers, agent status).
 */
export async function cacheGlobalState(state: unknown): Promise<void> {
  try {
    await putItem(`${PREFIX}global`, state)
  } catch {
    // Cache write failed — non-critical, ignore silently
  }
}

/**
 * Get cached global state. Returns null if missing, expired, or version mismatch.
 */
export async function getCachedGlobalState<T>(): Promise<{ data: T; stale: boolean } | null> {
  try {
    const cached = await getItem<T>(`${PREFIX}global`)
    return evaluateCache(cached)
  } catch {
    return null
  }
}

/**
 * Cache project state for instant loading on reconnect.
 */
export async function cacheProjectState(projectId: string, state: unknown): Promise<void> {
  try {
    await putItem(`${PREFIX}project:${projectId}`, state)
  } catch {
    // Cache write failed — non-critical
  }
}

/**
 * Get cached project state. Returns null if not found or expired.
 */
export async function getCachedProjectState<T>(
  projectId: string,
): Promise<{ data: T; stale: boolean } | null> {
  try {
    const cached = await getItem<T>(`${PREFIX}project:${projectId}`)
    const result = evaluateCache(cached)
    if (!result) {
      // Clean up expired entry
      await deleteItem(`${PREFIX}project:${projectId}`).catch(() => {})
    }
    return result
  } catch {
    return null
  }
}

/**
 * Clear all cached state.
 */
export async function clearCache(): Promise<void> {
  try {
    const keys = await getAllKeys()
    const db = await openDB()
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    for (const key of keys) {
      store.delete(key)
    }
    await new Promise<void>((resolve, reject) => {
      tx.oncomplete = () => { db.close(); resolve() }
      tx.onerror = () => { db.close(); reject(tx.error) }
    })
  } catch {
    // Clear failed — non-critical
  }
}

/**
 * Get cache statistics (number of entries, age of oldest entry).
 */
export async function getCacheStats(): Promise<{ entries: number; oldestMs: number }> {
  try {
    const all = await getAllEntries<unknown>()
    if (all.length === 0) return { entries: 0, oldestMs: 0 }
    const now = Date.now()
    const oldest = all.reduce((min, e) => Math.min(min, e.timestamp), all[0].timestamp)
    return {
      entries: all.length,
      oldestMs: now - oldest,
    }
  } catch {
    return { entries: 0, oldestMs: 0 }
  }
}
