import { useState, useCallback } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface CodegraphPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface CodegraphSymbol {
  id: number
  name: string
  kind: string
  file: string
  line: number
  package: string
}

const KIND_BADGE: Record<string, string> = {
  function: 'badge-primary',
  type: 'badge-secondary',
  interface: 'badge-accent',
  method: 'badge-info',
  const: 'badge-ghost',
  var: 'badge-ghost',
}

export function CodegraphPanel({ isOpen, onClose }: CodegraphPanelProps) {
  const { client, connected } = useProjectStore()

  const [stats, setStats] = useState<Record<string, number> | null>(null)
  const [symbols, setSymbols] = useState<CodegraphSymbol[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [usePattern, setUsePattern] = useState(false)
  const [loading, setLoading] = useState(false)
  const [indexing, setIndexing] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadStats = useCallback(async () => {
    if (!client || !connected) return

    try {
      setError(null)
      const data = await client.call<Record<string, number>>('codegraph.stats', {})
      setStats(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load stats')
    }
  }, [client, connected])

  const runIndex = useCallback(async () => {
    if (!client || !connected) return

    setIndexing(true)
    setError(null)

    try {
      const result = await client.call<{ files: number; symbols: number; edges: number }>('codegraph.index', {})
      setStats(prev => ({ ...(prev ?? {}), files: result.files, symbols: result.symbols, edges: result.edges }))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Indexing failed')
    } finally {
      setIndexing(false)
    }
  }, [client, connected])

  const searchSymbols = useCallback(async () => {
    if (!client || !connected || !searchQuery.trim()) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<{ symbols: CodegraphSymbol[] }>('codegraph.search', {
        name: searchQuery,
        pattern: usePattern,
      })
      setSymbols(result.symbols || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed')
      setSymbols([])
    } finally {
      setLoading(false)
    }
  }, [client, connected, searchQuery, usePattern])

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      searchSymbols()
    }
  }, [searchSymbols])

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Code Graph" size="4xl">
      <div className="max-h-[70vh] flex flex-col gap-4">
        {/* Actions */}
        <div className="flex items-center gap-2 flex-wrap">
          <button
            onClick={runIndex}
            disabled={indexing || !connected}
            className="btn btn-primary btn-sm"
            aria-label="Index project"
          >
            {indexing ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
              </svg>
            )}
            Index
          </button>
          <button
            onClick={loadStats}
            disabled={!connected}
            className="btn btn-ghost btn-sm"
            aria-label="Refresh stats"
          >
            Stats
          </button>

          {stats && (
            <div className="flex items-center gap-3 text-xs text-base-content/60">
              <span>{stats.files ?? 0} files</span>
              <span>{stats.symbols ?? 0} symbols</span>
              <span>{stats.edges ?? 0} edges</span>
            </div>
          )}
        </div>

        {/* Search */}
        <div className="flex items-center gap-2">
          <input
            type="text"
            className="input input-sm input-bordered flex-1"
            placeholder="Search symbol name..."
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            aria-label="Symbol search"
          />
          <label className="label cursor-pointer gap-1 text-xs">
            <input
              type="checkbox"
              className="checkbox checkbox-xs"
              checked={usePattern}
              onChange={e => setUsePattern(e.target.checked)}
            />
            <span>Pattern</span>
          </label>
          <button
            onClick={searchSymbols}
            disabled={loading || !connected || !searchQuery.trim()}
            className="btn btn-sm btn-ghost"
            aria-label="Search"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            )}
          </button>
        </div>

        {/* Error */}
        {error && (
          <div className="alert alert-error py-2">
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Results */}
        <div className="flex-1 overflow-y-auto">
          {symbols.length > 0 ? (
            <>
              <p className="text-xs text-base-content/50 mb-2">
                {symbols.length} symbol{symbols.length !== 1 ? 's' : ''} found
              </p>
              <div className="overflow-x-auto">
                <table className="table table-sm table-zebra w-full">
                  <thead>
                    <tr>
                      <th>Kind</th>
                      <th>Name</th>
                      <th>File</th>
                      <th>Package</th>
                    </tr>
                  </thead>
                  <tbody>
                    {symbols.map(s => (
                      <tr key={s.id}>
                        <td>
                          <span className={`badge badge-sm ${KIND_BADGE[s.kind] || 'badge-ghost'}`}>
                            {s.kind}
                          </span>
                        </td>
                        <td className="font-mono text-xs font-medium">{s.name}</td>
                        <td className="font-mono text-xs">{s.file}:{s.line}</td>
                        <td className="text-xs">{s.package}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          ) : !loading && !error && (
            <div className="text-center py-12 text-base-content/50">
              <svg aria-hidden="true" className="w-10 h-10 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
              </svg>
              <p>Index your project to explore code symbols</p>
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
