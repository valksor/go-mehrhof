import { useEffect, useState, useCallback, useRef } from 'react'
import { useProjectStore } from '../stores/projectStore'

interface ArchivedTask {
  id: string
  title: string
  branch: string
  source: string
  final_state: string
  started_at: string
  completed_at: string
}

interface SearchTask {
  id: string
  title: string
  state: string
  source: string
  completed_at: string
}

export function TaskHistory() {
  const { connected } = useProjectStore()
  const client = useProjectStore(s => s.client)
  const [tasks, setTasks] = useState<ArchivedTask[] | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<SearchTask[] | null>(null)
  const [searching, setSearching] = useState(false)
  const searchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [showFilters, setShowFilters] = useState(false)
  const [fileFilter, setFileFilter] = useState('')
  const [stateFilter, setStateFilter] = useState('')
  const [tagFilter, setTagFilter] = useState('')
  const [sinceFilter, setSinceFilter] = useState('')

  useEffect(() => {
    if (!connected || !client) return
    let cancelled = false
    client.call<{ tasks: ArchivedTask[] | null }>('task.history', {})
      .then(result => { if (!cancelled) setTasks(result.tasks || []) })
      .catch(() => { if (!cancelled) setTasks([]) })
    return () => { cancelled = true }
  }, [connected, client])

  const handleSearch = useCallback((query: string) => {
    setSearchQuery(query)

    if (searchTimerRef.current) {
      clearTimeout(searchTimerRef.current)
    }

    const hasFilters = fileFilter || stateFilter || tagFilter || sinceFilter
    if (!query.trim() && !hasFilters) {
      setSearchResults(null)
      setSearching(false)
      return
    }

    setSearching(true)
    searchTimerRef.current = setTimeout(async () => {
      if (!client) {
        setSearching(false)
        return
      }
      try {
        const params: Record<string, unknown> = { query: query.trim(), limit: 20 }
        if (fileFilter) params.file = fileFilter
        if (stateFilter) params.state = stateFilter
        if (tagFilter) params.tag = tagFilter
        if (sinceFilter) {
          const [y, m, d] = sinceFilter.split('-').map(Number)
          params.since = new Date(y, m - 1, d).toISOString()
        }
        const result = await client.call<{ tasks: SearchTask[] | null }>('task.search', params)
        setSearchResults(result.tasks || [])
      } catch {
        setSearchResults([])
      } finally {
        setSearching(false)
      }
    }, 300)
  }, [client, fileFilter, stateFilter, tagFilter, sinceFilter])

  // Trigger search when filters change.
  useEffect(() => {
    handleSearch(searchQuery)
  }, [handleSearch, searchQuery])

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (searchTimerRef.current) clearTimeout(searchTimerRef.current)
    }
  }, [])

  if (tasks === null && !searchQuery) {
    return <p className="text-xs text-base-content/50">Loading history...</p>
  }

  const hasActiveFilters = !!(fileFilter || stateFilter || tagFilter || sinceFilter)
  const isSearchMode = !!searchQuery.trim() || hasActiveFilters
  const displayTasks = isSearchMode ? searchResults : tasks

  return (
    <div className="space-y-2">
      {/* Search input */}
      <div className="relative">
        <input
          type="search"
          placeholder="Search past tasks..."
          value={searchQuery}
          onChange={(e) => handleSearch(e.target.value)}
          className="input input-xs input-bordered w-full pr-8"
          aria-label="Search tasks"
        />
        {searching && (
          <span className="absolute right-2 top-1/2 -translate-y-1/2">
            <span className="loading loading-spinner loading-xs"></span>
          </span>
        )}
      </div>

      {/* Filter toggle */}
      <button
        className="btn btn-ghost btn-xs text-base-content/50"
        onClick={() => setShowFilters(f => !f)}
      >
        {showFilters ? '▾ Filters' : '▸ Filters'}
        {(fileFilter || stateFilter || tagFilter || sinceFilter) && (
          <span className="badge badge-xs badge-primary ml-1">active</span>
        )}
      </button>

      {/* Advanced filters */}
      {showFilters && (
        <div className="grid grid-cols-2 gap-1.5">
          <input
            type="text"
            placeholder="File path..."
            value={fileFilter}
            onChange={e => setFileFilter(e.target.value)}
            className="input input-xs input-bordered"
            aria-label="Filter by file"
          />
          <select
            value={stateFilter}
            onChange={e => setStateFilter(e.target.value)}
            className="select select-xs select-bordered"
            aria-label="Filter by state"
          >
            <option value="">Any state</option>
            <option value="finished">Finished</option>
            <option value="abandoned">Abandoned</option>
            <option value="failed">Failed</option>
          </select>
          <input
            type="text"
            placeholder="Tag..."
            value={tagFilter}
            onChange={e => setTagFilter(e.target.value)}
            className="input input-xs input-bordered"
            aria-label="Filter by tag"
          />
          <input
            type="date"
            value={sinceFilter}
            onChange={e => setSinceFilter(e.target.value)}
            className="input input-xs input-bordered"
            aria-label="Since date"
          />
        </div>
      )}

      {/* Results */}
      {displayTasks === null ? (
        <p className="text-xs text-base-content/50">Loading...</p>
      ) : displayTasks.length === 0 ? (
        <p className="text-xs text-base-content/50">
          {isSearchMode ? `No tasks matching "${searchQuery}"` : 'No completed tasks yet'}
        </p>
      ) : (
        <ul className="space-y-1.5">
          {(isSearchMode ? displayTasks : displayTasks.slice(0, 10)).map(task => {
            const state = 'final_state' in task ? (task as ArchivedTask).final_state : (task as SearchTask).state
            return (
              <li key={task.id} className="p-2 bg-base-300 rounded text-xs">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium truncate">{task.title || task.id}</span>
                  <span className={`badge badge-xs ${
                    state === 'finished' ? 'badge-success' :
                    state === 'abandoned' ? 'badge-warning' :
                    'badge-ghost'
                  }`}>
                    {state}
                  </span>
                </div>
                {task.source && (
                  <p className="text-base-content/50 font-mono text-[10px] truncate mt-0.5">{task.source}</p>
                )}
                {task.completed_at && (
                  <p className="text-base-content/40 text-[10px] mt-0.5">
                    {new Date(task.completed_at).toLocaleDateString()}
                  </p>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
