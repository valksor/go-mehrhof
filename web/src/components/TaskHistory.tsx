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

    if (!query.trim()) {
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
        const result = await client.call<{ tasks: SearchTask[] | null }>('task.search', {
          query: query.trim(),
          limit: 20,
        })
        setSearchResults(result.tasks || [])
      } catch {
        setSearchResults([])
      } finally {
        setSearching(false)
      }
    }, 300)
  }, [client])

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (searchTimerRef.current) clearTimeout(searchTimerRef.current)
    }
  }, [])

  if (tasks === null && !searchQuery) {
    return <p className="text-xs text-base-content/50">Loading history...</p>
  }

  const displayTasks = searchQuery.trim() ? searchResults : tasks
  const isSearchMode = !!searchQuery.trim()

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
