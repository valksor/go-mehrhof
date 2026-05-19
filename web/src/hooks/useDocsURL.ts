import { useState, useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'

interface DocsURLResponse {
  url: string
  version: string
}

/**
 * Hook to fetch the documentation URL from the backend.
 * Returns version-aware URL: /latest for stable releases, /nightly for dev builds.
 * Caches the result for the session (version doesn't change during runtime).
 */
export function useDocsURL() {
  const { client, connected } = useGlobalStore()
  const [data, setData] = useState<DocsURLResponse | null>(null)

  useEffect(() => {
    if (!connected || !client || data) return

    client
      .call<DocsURLResponse>('system.docsURL', {})
      .then(setData)
      .catch((err) => console.error('[useDocsURL] Error fetching docs URL:', err))
  }, [connected, client, data])

  return data
}
