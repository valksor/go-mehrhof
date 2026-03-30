import { useState, useCallback, useEffect } from 'react'
import { useGlobalStore, type MemoryResult, type DocumentOutcome } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface MemoryPanelProps {
  isOpen: boolean
  onClose: () => void
}

const TYPE_LABELS: Record<string, string> = {
  task: 'Task',
  specification: 'Specification',
  plan: 'Plan',
  note: 'Note',
  snippet: 'Snippet',
}

const TYPE_COLORS: Record<string, string> = {
  task: 'badge-primary',
  specification: 'badge-secondary',
  plan: 'badge-accent',
  note: 'badge-info',
  snippet: 'badge-warning',
}

function OutcomeBadges({ outcome }: { outcome?: DocumentOutcome }) {
  if (!outcome) return null

  return (
    <div className="flex items-center gap-1 flex-wrap">
      {outcome.success === true && (
        <span className="badge badge-xs badge-success" title="Task succeeded">pass</span>
      )}
      {outcome.success === false && (
        <span className="badge badge-xs badge-error" title="Task failed">fail</span>
      )}
      {outcome.pr_merged && (
        <span className="badge badge-xs badge-info" title="PR was merged">merged</span>
      )}
      {outcome.ci_passed_first_try && (
        <span className="badge badge-xs badge-success badge-outline" title="CI passed on first try">CI ok</span>
      )}
      {outcome.human_changes_needed && (
        <span className="badge badge-xs badge-warning" title="Required human changes">human edit</span>
      )}
    </div>
  )
}

export function MemoryPanel({ isOpen, onClose }: MemoryPanelProps) {
  const { searchMemory, loadMemoryStats, clearMemory, memoryStats, connected } = useGlobalStore()

  const [query, setQuery] = useState('')
  const [results, setResults] = useState<MemoryResult[]>([])
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [clearConfirm, setClearConfirm] = useState(false)

  // Load stats when panel opens
  useEffect(() => {
    if (isOpen && connected) {
      void loadMemoryStats()
    }
  }, [isOpen, connected, loadMemoryStats])

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return

    setLoading(true)
    setError(null)
    setSearched(true)

    try {
      const r = await searchMemory(query.trim(), 20)
      setResults(r)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed')
      setResults([])
    } finally {
      setLoading(false)
    }
  }, [query, searchMemory])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      void handleSearch()
    }
  }

  const handleClear = async () => {
    if (!clearConfirm) {
      setClearConfirm(true)
      return
    }
    setClearConfirm(false)
    await clearMemory()
    setResults([])
    setSearched(false)
    await loadMemoryStats()
  }

  const formatScore = (score: number) => `${Math.round(score * 100)}%`

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
  }

  const formatDate = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
        year: 'numeric'
      })
    } catch {
      return dateStr
    }
  }

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Memory" size="2xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Stats row */}
        {memoryStats && (
          <div className="grid grid-cols-3 gap-2 mb-4">
            <div className="bg-base-200 rounded-lg p-2.5 text-center">
              <div className="text-lg font-semibold text-base-content">{memoryStats.total_entries}</div>
              <div className="text-xs text-base-content/60">Entries</div>
            </div>
            <div className="bg-base-200 rounded-lg p-2.5 text-center">
              <div className="text-lg font-semibold text-base-content">{formatBytes(memoryStats.total_size_bytes)}</div>
              <div className="text-xs text-base-content/60">Size</div>
            </div>
            <div className="bg-base-200 rounded-lg p-2.5 text-center">
              <div className={`text-lg font-semibold ${memoryStats.index_ready ? 'text-success' : 'text-warning'}`}>
                {memoryStats.index_ready ? 'Ready' : 'Not ready'}
              </div>
              <div className="text-xs text-base-content/60">Index</div>
            </div>
          </div>
        )}

        {/* Clear memory button */}
        <div className="flex items-center gap-2 mb-4 justify-end">
          <button
            onClick={handleClear}
            disabled={!connected}
            className={`btn btn-sm ${clearConfirm ? 'btn-error' : 'btn-ghost'}`}
            aria-label={clearConfirm ? 'Confirm clear all memory entries' : 'Clear all memory entries'}
            onBlur={() => setClearConfirm(false)}
          >
            <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            {clearConfirm ? 'Confirm Clear' : 'Clear Memory'}
          </button>
        </div>

        {/* Search input */}
        <div className="flex gap-2 mb-4">
          <input
            type="text"
            value={query}
            onChange={e => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search memory..."
            aria-label="Search memory"
            className="input input-bordered flex-1"
          />
          <button
            onClick={handleSearch}
            disabled={loading || !query.trim()}
            className="btn btn-primary"
          >
            {loading ? (
              <span className="loading loading-spinner loading-sm"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            )}
            Search
          </button>
        </div>

        {/* Error */}
        {error && (
          <div className="alert alert-error py-2 mb-4">
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Results */}
        <div className="flex-1 overflow-y-auto">
          {!searched ? (
            <div className="text-center py-12 text-base-content/50">
              <svg aria-hidden="true" className="w-12 h-12 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
              </svg>
              <p>Enter a search query to explore agent memory</p>
              <p className="text-xs mt-2 text-base-content/40">Memory is populated when tasks complete</p>
            </div>
          ) : loading ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : results.length === 0 ? (
            <div className="text-center py-12 text-base-content/50">
              <svg aria-hidden="true" className="w-10 h-10 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <p>No results found for "{query}"</p>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-xs text-base-content/50">{results.length} result{results.length !== 1 ? 's' : ''} found</p>
              {results.map((r) => (
                <div key={r.id} className="p-4 rounded-lg bg-base-200 border border-base-300">
                  <div className="flex items-center justify-between gap-2 mb-2">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className={`badge badge-sm ${TYPE_COLORS[r.type] || 'badge-ghost'}`}>
                        {TYPE_LABELS[r.type] || r.type}
                      </span>
                      {r.task_id && (
                        <span className="text-xs text-base-content/50 font-mono">
                          task: {r.task_id.slice(0, 8)}
                        </span>
                      )}
                      <OutcomeBadges outcome={r.outcome} />
                    </div>
                    <div className="flex items-center gap-2 flex-shrink-0">
                      <span
                        className={`text-xs font-semibold ${
                          r.score >= 0.8 ? 'text-success' :
                          r.score >= 0.5 ? 'text-warning' :
                          'text-base-content/50'
                        }`}
                        title="Relevance score"
                      >
                        {formatScore(r.score)}
                      </span>
                      <span className="text-xs text-base-content/40">{formatDate(r.created_at)}</span>
                    </div>
                  </div>
                  <p className="text-sm text-base-content/80 leading-relaxed line-clamp-4 whitespace-pre-wrap">
                    {r.content}
                  </p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
