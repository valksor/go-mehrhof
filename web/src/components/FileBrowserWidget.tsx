import { useEffect, useState, useCallback, useRef, useMemo } from 'react'
import { useProjectStore, type BrowseEntry } from '../stores/projectStore'
import { useGlobalStore } from '../stores/globalStore'
import { useLayoutStore } from '../stores/layoutStore'
import { debounce } from '../lib/debounce'

interface SearchEntry {
  name: string
  path: string
  rel_path: string
  is_dir: boolean
}

export function FileBrowserWidget() {
  const { browseFiles, connected, task } = useProjectStore()
  const { client: globalClient, connected: globalConnected } = useGlobalStore()
  const { openTab, setActiveTab } = useLayoutStore()

  const [currentPath, setCurrentPath] = useState<string | undefined>(undefined)
  const [entries, setEntries] = useState<BrowseEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [pathHistory, setPathHistory] = useState<Array<string | undefined>>([undefined])
  const initialLoadDone = useRef(false)

  // Search state
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<SearchEntry[]>([])
  const [searching, setSearching] = useState(false)
  const searchRequestId = useRef(0)
  const hasSearchQuery = searchQuery.trim().length > 0

  const performSearch = useCallback(async (query: string) => {
    if (!globalClient || !globalConnected || !query.trim()) {
      setSearchResults([])
      setSearching(false)
      return
    }
    const requestId = ++searchRequestId.current
    setSearching(true)
    try {
      const result = await globalClient.call<{ entries: SearchEntry[] }>('files.search', {
        query: query.trim(),
        path: task?.worktreePath || currentPath,
        max_results: 30,
      })
      if (requestId === searchRequestId.current) {
        setSearchResults(result.entries || [])
      }
    } catch (err) {
      console.error('File search failed:', err)
      if (requestId === searchRequestId.current) {
        setSearchResults([])
      }
    } finally {
      if (requestId === searchRequestId.current) {
        setSearching(false)
      }
    }
  }, [globalClient, globalConnected, task?.worktreePath, currentPath])

  const debouncedSearch = useMemo(
    () => debounce((query: string) => performSearch(query), 300),
    [performSearch]
  )

  useEffect(() => () => debouncedSearch.cancel(), [debouncedSearch])

  const handleSearchChange = (value: string) => {
    setSearchQuery(value)
    if (value.trim()) {
      debouncedSearch(value)
    } else {
      searchRequestId.current++
      debouncedSearch.cancel()
      setSearching(false)
      setSearchResults([])
    }
  }

  const loadEntries = useCallback(async (path?: string) => {
    if (!connected) return
    setLoading(true)
    setError(null)
    try {
      const result = await browseFiles(path)
      setEntries(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load directory')
    } finally {
      setLoading(false)
    }
  }, [connected, browseFiles])

  // Initial load when connected (only once per connection)
  useEffect(() => {
    if (connected && !initialLoadDone.current) {
      initialLoadDone.current = true
      void loadEntries(currentPath)
    }
    if (!connected) {
      initialLoadDone.current = false
    }
  }, [connected, currentPath, loadEntries])

  const handleEntryClick = async (entry: BrowseEntry) => {
    if (entry.is_dir) {
      setPathHistory(prev => [...prev, entry.path])
      setCurrentPath(entry.path)
      await loadEntries(entry.path)
    } else {
      // Open file in a tab
      const tabId = `file-${entry.path}`
      openTab({
        id: tabId,
        type: 'file',
        title: entry.name,
        closeable: true,
        data: { path: entry.path }
      })
      setActiveTab(tabId)
    }
  }

  const handleNavigateUp = async () => {
    if (pathHistory.length <= 1) return
    const newHistory = pathHistory.slice(0, -1)
    const parentPath = newHistory[newHistory.length - 1]
    setPathHistory(newHistory)
    setCurrentPath(parentPath)
    await loadEntries(parentPath)
  }

  const handleRefresh = () => {
    void loadEntries(currentPath)
  }

  const handleSearchEntryClick = async (entry: SearchEntry) => {
    if (entry.is_dir) {
      setSearchQuery('')
      setSearchResults([])
      setPathHistory(prev => [...prev, entry.path])
      setCurrentPath(entry.path)
      await loadEntries(entry.path)
    } else {
      setSearchQuery('')
      setSearchResults([])
      const tabId = `file-${entry.path}`
      openTab({
        id: tabId,
        type: 'file',
        title: entry.name,
        closeable: true,
        data: { path: entry.path }
      })
      setActiveTab(tabId)
    }
  }

  const formatBytes = (bytes?: number) => {
    if (!bytes) return ''
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
  }

  // Build breadcrumb segments
  const breadcrumbPath = currentPath || (task?.worktreePath ? task.worktreePath : '/')
  const segments = breadcrumbPath.replace(/^\//, '').split('/').filter(Boolean)

  if (!connected) {
    return (
      <div className="flex items-center justify-center h-full text-base-content/50">
        <p className="text-sm">Not connected</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header / breadcrumb */}
      <div className="flex items-center gap-2 px-3 py-2 border-b border-base-300 bg-base-200/50">
        <button
          onClick={handleNavigateUp}
          disabled={pathHistory.length <= 1}
          className="btn btn-ghost btn-xs btn-square"
          aria-label="Go to parent directory"
        >
          <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
        </button>

        <div className="flex-1 min-w-0 flex items-center gap-1 text-xs text-base-content/60 font-mono overflow-hidden">
          <button
            onClick={() => { setPathHistory([undefined]); setCurrentPath(undefined); void loadEntries(undefined) }}
            className="hover:text-base-content transition-colors flex-shrink-0"
          >
            /
          </button>
          {segments.map((seg, i) => {
            const segPath = '/' + segments.slice(0, i + 1).join('/')
            const isCurrent = i === segments.length - 1
            return (
              <span key={i} className="flex items-center gap-1 min-w-0">
                <span className="text-base-content/30" aria-hidden="true">/</span>
                <button
                  onClick={async () => {
                    const newHistory = pathHistory.slice(0, i + 2)
                    setPathHistory(newHistory)
                    setCurrentPath(segPath)
                    await loadEntries(segPath)
                  }}
                  className="hover:text-base-content transition-colors truncate max-w-[80px]"
                  aria-current={isCurrent ? 'page' : undefined}
                >
                  {seg}
                </button>
              </span>
            )
          })}
        </div>

        <button
          onClick={handleRefresh}
          disabled={loading}
          className="btn btn-ghost btn-xs btn-square"
          aria-label="Refresh current directory"
        >
          {loading ? (
            <span className="loading loading-spinner loading-xs" aria-hidden="true"></span>
          ) : (
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          )}
        </button>
      </div>

      {/* Search input */}
      <div className="px-3 py-1.5 border-b border-base-300">
        <div className="relative">
          <svg
            aria-hidden="true"
            className="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-base-content/40"
            fill="none" viewBox="0 0 24 24" stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="text"
            value={searchQuery}
            onChange={e => handleSearchChange(e.target.value)}
            placeholder="Search files..."
            className="input input-xs input-bordered w-full pl-7 pr-7 text-sm"
            aria-label="Search files in project"
          />
          {searchQuery && (
            <button
              onClick={() => handleSearchChange('')}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-base-content/40 hover:text-base-content"
              aria-label="Clear search"
            >
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          )}
        </div>
      </div>

      {/* Entry list (or search results) */}
      <div className="flex-1 overflow-auto">
        {hasSearchQuery ? (
          /* Search results mode */
          searching ? (
            <div className="flex items-center justify-center py-8 text-base-content/50">
              <span className="loading loading-spinner loading-xs mr-2"></span>
              <span className="text-sm">Searching...</span>
            </div>
          ) : searchResults.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-base-content/40 py-8">
              <svg aria-hidden="true" className="w-8 h-8 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <p className="text-sm">No files found</p>
            </div>
          ) : (
            <div>
              {searchResults.map(entry => (
                <button
                  key={entry.path}
                  className="w-full flex items-center gap-2.5 px-3 py-1.5 hover:bg-base-200 transition-colors text-left group"
                  onClick={() => handleSearchEntryClick(entry)}
                >
                  <span className="flex-shrink-0 w-4 h-4 text-base-content/50">
                    {entry.is_dir ? (
                      <svg aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" className="w-4 h-4 text-warning/70">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                      </svg>
                    ) : (
                      <svg aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" className="w-4 h-4">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                      </svg>
                    )}
                  </span>
                  <span className="flex-1 min-w-0 text-sm text-base-content/80 truncate group-hover:text-base-content transition-colors font-mono">
                    {entry.rel_path}
                  </span>
                </button>
              ))}
            </div>
          )
        ) : error ? (
          <div className="flex flex-col items-center justify-center h-full text-error py-8">
            <svg aria-hidden="true" className="w-8 h-8 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <p className="text-sm">{error}</p>
          </div>
        ) : entries.length === 0 && !loading ? (
          <div className="flex flex-col items-center justify-center h-full text-base-content/40 py-8">
            <svg aria-hidden="true" className="w-10 h-10 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
            </svg>
            <p className="text-sm">Empty directory</p>
          </div>
        ) : (
          <div>
            {/* Directories first, then files */}
            {[...entries.filter(e => e.is_dir), ...entries.filter(e => !e.is_dir)].map((entry) => (
              <button
                key={entry.path}
                className="w-full flex items-center gap-2.5 px-3 py-1.5 hover:bg-base-200 transition-colors text-left group"
                onClick={() => handleEntryClick(entry)}
              >
                {/* Icon */}
                <span className="flex-shrink-0 w-4 h-4 text-base-content/50 group-hover:text-base-content/70 transition-colors">
                  {entry.is_dir ? (
                    <svg aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" className="w-4 h-4 text-warning/70">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                    </svg>
                  ) : (
                    <svg aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" className="w-4 h-4">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                    </svg>
                  )}
                </span>

                {/* Name */}
                <span className="flex-1 min-w-0 text-sm text-base-content/80 truncate group-hover:text-base-content transition-colors">
                  {entry.name}
                </span>

                {/* Size (files only) */}
                {!entry.is_dir && entry.size !== undefined && (
                  <span className="flex-shrink-0 text-xs text-base-content/40">
                    {formatBytes(entry.size)}
                  </span>
                )}

                {/* Arrow for dirs */}
                {entry.is_dir && (
                  <svg aria-hidden="true" className="w-3 h-3 text-base-content/30 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                  </svg>
                )}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
