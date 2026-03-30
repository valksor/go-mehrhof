import { useState, useCallback, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface EventLogPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface EventLogEntry {
  timestamp: string
  type: string
  phase: string
  message: string
  data?: Record<string, unknown>
}

const EVENT_TYPE_COLORS: Record<string, string> = {
  phase_started: 'badge-info',
  phase_completed: 'badge-success',
  phase_failed: 'badge-error',
  checkpoint_created: 'badge-accent',
  finding_detected: 'badge-warning',
  task_loaded: 'badge-primary',
  task_finished: 'badge-success',
  router_decision: 'badge-ghost',
  spec_changed: 'badge-secondary',
  guardrail_checked: 'badge-ghost',
}

export function EventLogPanel({ isOpen, onClose }: EventLogPanelProps) {
  const { client, connected } = useProjectStore()

  const [entries, setEntries] = useState<EventLogEntry[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [typeFilter, setTypeFilter] = useState('')
  const [phaseFilter, setPhaseFilter] = useState('')

  const loadEvents = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      const params: Record<string, unknown> = { per_page: 100 }
      if (typeFilter) params.type = typeFilter
      if (phaseFilter) params.phase = phaseFilter

      const result = await client.call<{ entries: EventLogEntry[]; total: number }>('eventlog.query', params)
      setEntries(result.entries || [])
      setTotal(result.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load event log')
      setEntries([])
    } finally {
      setLoading(false)
    }
  }, [client, connected, typeFilter, phaseFilter])

  useEffect(() => {
    if (isOpen && connected) {
      void loadEvents()
    }
  }, [isOpen, connected, loadEvents])

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Event Log" size="3xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Toolbar */}
        <div className="flex items-center gap-2 mb-4 flex-wrap">
          <select
            className="select select-bordered select-sm"
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            aria-label="Filter by event type"
          >
            <option value="">All types</option>
            <option value="phase_started">Phase Started</option>
            <option value="phase_completed">Phase Completed</option>
            <option value="phase_failed">Phase Failed</option>
            <option value="checkpoint_created">Checkpoint</option>
            <option value="finding_detected">Finding</option>
            <option value="router_decision">Router Decision</option>
            <option value="spec_changed">Spec Changed</option>
            <option value="guardrail_checked">Guardrail Checked</option>
            <option value="task_loaded">Task Loaded</option>
            <option value="task_finished">Task Finished</option>
          </select>

          <select
            className="select select-bordered select-sm"
            value={phaseFilter}
            onChange={(e) => setPhaseFilter(e.target.value)}
            aria-label="Filter by phase"
          >
            <option value="">All phases</option>
            <option value="plan">Plan</option>
            <option value="implement">Implement</option>
            <option value="simplify">Simplify</option>
            <option value="optimize">Optimize</option>
            <option value="review">Review</option>
          </select>

          <div className="flex-1" />

          <button
            onClick={loadEvents}
            disabled={loading || !connected}
            className="btn btn-ghost btn-sm"
            aria-label="Refresh event log"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            )}
          </button>
        </div>

        {/* Error */}
        {error && (
          <div className="alert alert-error mb-4">
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Content */}
        <div className="overflow-auto flex-1">
          {entries.length === 0 && !loading ? (
            <div className="flex items-center justify-center py-12 text-base-content/50">
              <p className="text-sm">No lifecycle events recorded</p>
            </div>
          ) : (
            <div className="space-y-1">
              {entries.map((entry, idx) => (
                <div key={`${entry.timestamp}-${entry.type}-${idx}`} className="flex items-start gap-3 px-2 py-1.5 hover:bg-base-200 rounded">
                  <span className="text-xs text-base-content/50 whitespace-nowrap font-mono mt-0.5">
                    {new Date(entry.timestamp).toLocaleTimeString()}
                  </span>
                  <span className={`badge badge-xs ${EVENT_TYPE_COLORS[entry.type] || 'badge-ghost'} whitespace-nowrap mt-0.5`}>
                    {entry.type.replace(/_/g, ' ')}
                  </span>
                  {entry.phase && (
                    <span className="text-xs text-base-content/60 whitespace-nowrap mt-0.5">
                      [{entry.phase}]
                    </span>
                  )}
                  <span className="text-sm flex-1 min-w-0 break-words">
                    {entry.message || entry.type}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Footer */}
        {total > 0 && (
          <div className="mt-3 pt-3 border-t border-base-300 text-xs text-base-content/50">
            {entries.length < total
              ? `Showing ${entries.length} of ${total} events`
              : `${total} event${total !== 1 ? 's' : ''} total`}
          </div>
        )}
      </div>
    </AccessibleModal>
  )
}
