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
  const [callers, setCallers] = useState<{ name: string; symbols: CodegraphSymbol[] } | null>(null)
  const [deps, setDeps] = useState<{ pkg: string; dependencies: string[] } | null>(null)
  const [drillLoading, setDrillLoading] = useState(false)

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

  const loadCallers = useCallback(async (name: string) => {
    if (!client || !connected) return
    setDrillLoading(true)
    setError(null)
    setDeps(null)
    try {
      const result = await client.call<{ callers: CodegraphSymbol[] }>('codegraph.callers', { name })
      setCallers({ name, symbols: result.callers || [] })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load callers')
    } finally {
      setDrillLoading(false)
    }
  }, [client, connected])

  const loadDeps = useCallback(async (pkg: string) => {
    if (!client || !connected) return
    setDrillLoading(true)
    setError(null)
    setCallers(null)
    try {
      const result = await client.call<{ dependencies: string[] }>('codegraph.deps', { package: pkg })
      setDeps({ pkg, dependencies: result.dependencies || [] })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load dependencies')
    } finally {
      setDrillLoading(false)
    }
  }, [client, connected])

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      void searchSymbols()
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
                      <th></th>
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
                        <td>
                          <div className="flex gap-1">
                            {(s.kind === 'function' || s.kind === 'method') && (
                              <button
                                className="btn btn-ghost btn-xs"
                                onClick={() => void loadCallers(s.name)}
                                disabled={drillLoading}
                                title="Show callers"
                              >
                                Callers
                              </button>
                            )}
                            {s.package && (
                              <button
                                className="btn btn-ghost btn-xs"
                                onClick={() => void loadDeps(s.package)}
                                disabled={drillLoading}
                                title="Show package dependencies"
                              >
                                Deps
                              </button>
                            )}
                          </div>
                        </td>
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

        {/* Callers drill-down */}
        {callers && (
          <div className="border-t border-base-300 pt-3">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium">
                Callers of <code className="text-xs">{callers.name}</code>
              </span>
              <button className="btn btn-ghost btn-xs" onClick={() => setCallers(null)}>Close</button>
            </div>
            {callers.symbols.length === 0 ? (
              <p className="text-xs text-base-content/50">No callers found</p>
            ) : (
              <ul className="space-y-1">
                {callers.symbols.map(s => (
                  <li key={s.id} className="flex items-center gap-2 text-xs">
                    <span className={`badge badge-xs ${KIND_BADGE[s.kind] || 'badge-ghost'}`}>{s.kind}</span>
                    <span className="font-mono font-medium">{s.name}</span>
                    <span className="font-mono text-base-content/50">{s.file}:{s.line}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        {/* Dependencies drill-down */}
        {deps && (
          <div className="border-t border-base-300 pt-3">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium">
                Dependencies of <code className="text-xs">{deps.pkg}</code>
              </span>
              <button className="btn btn-ghost btn-xs" onClick={() => setDeps(null)}>Close</button>
            </div>
            {deps.dependencies.length === 0 ? (
              <p className="text-xs text-base-content/50">No dependencies found</p>
            ) : (
              <ul className="space-y-0.5">
                {deps.dependencies.map(d => (
                  <li key={d} className="font-mono text-xs">{d}</li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>
    </AccessibleModal>
  )
}
