import { useState, useCallback, useEffect, useRef } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface ActivityPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface ActivityEntry {
  timestamp: string
  method: string
  correlation_id: string
  duration_ms: number
  error: string
  params_size: number
  user_id?: string
  task_id?: string
  agent_model?: string
  description?: string // Present in timeline mode
}

interface AuditTask {
  id: string
  path: string
  state: string
}

type ViewMode = 'activity' | 'timeline' | 'audit'
type TimeRange = '1h' | '6h' | '24h' | '7d'

const VIEW_MODE_TITLES: Record<ViewMode, string> = {
  activity: 'Activity Log',
  timeline: 'Timeline',
  audit: 'Compliance Audit',
}

const TIME_RANGE_LABELS: Record<TimeRange, string> = {
  '1h': 'Last hour',
  '6h': 'Last 6 hours',
  '24h': 'Last 24 hours',
  '7d': 'Last 7 days',
}

export function ActivityPanel({ isOpen, onClose }: ActivityPanelProps) {
  const { client, connected } = useGlobalStore()

  const [entries, setEntries] = useState<ActivityEntry[]>([])
  const [count, setCount] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [viewMode, setViewMode] = useState<ViewMode>('activity')
  const [auditTasks, setAuditTasks] = useState<AuditTask[]>([])
  const [timeRange, setTimeRange] = useState<TimeRange>('1h')
  const [errorsOnly, setErrorsOnly] = useState(false)
  const [methodFilter, setMethodFilter] = useState('')
  const [exporting, setExporting] = useState(false)
  const downloadRef = useRef<HTMLAnchorElement>(null)

  const loadActivity = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      if (viewMode === 'audit') {
        // Audit view uses the export RPC for compliance-focused data
        const result = await client.call<{
          tasks: AuditTask[]
          activity: ActivityEntry[]
        }>('export', {
          format: 'json',
          since: timeRange,
          include: 'tasks,activity',
        })

        setAuditTasks(result.tasks || [])
        let filtered = result.activity || []

        if (errorsOnly) {
          filtered = filtered.filter((e) => e.error !== '')
        }
        if (methodFilter.trim()) {
          const search = methodFilter.trim().toLowerCase()
          filtered = filtered.filter((e) => e.method.toLowerCase().includes(search))
        }

        setEntries(filtered)
        setCount(filtered.length)
      } else {
        const isTimeline = viewMode === 'timeline'
        const result = await client.call<{ entries: ActivityEntry[]; count: number; enabled: boolean }>(
          'activity.query',
          { since: timeRange, limit: 100, timeline: isTimeline },
        )
        let filtered = result.entries || []

        if (errorsOnly) {
          filtered = filtered.filter((e) => e.error !== '')
        }
        if (methodFilter.trim()) {
          const filterText = methodFilter.trim().toLowerCase()
          filtered = filtered.filter(
            (e) =>
              e.method.toLowerCase().includes(filterText) ||
              (isTimeline && e.description?.toLowerCase().includes(filterText)),
          )
        }

        setEntries(filtered)
        setCount(result.count)
        setAuditTasks([])
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load activity')
      setEntries([])
    } finally {
      setLoading(false)
    }
  }, [client, connected, timeRange, errorsOnly, methodFilter, viewMode])

  const handleExport = useCallback(async () => {
    if (!client || !connected) return
    setExporting(true)
    try {
      const result = await client.call<Record<string, unknown>>('export', {
        format: 'json',
        since: timeRange,
        include: 'tasks,activity',
      })
      const json = JSON.stringify(result, null, 2)
      const blob = new Blob([json], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = downloadRef.current ?? document.createElement('a')
      a.href = url
      a.download = `kvelmo-export-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.json`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Export failed')
    } finally {
      setExporting(false)
    }
  }, [client, connected, timeRange])

  // Auto-load when panel opens or filters change
  useEffect(() => {
    if (isOpen && connected) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
      void loadActivity()
    }
  }, [isOpen, connected, loadActivity])

  const formatTimestamp = (ts: string) => {
    try {
      return new Date(ts).toLocaleTimeString(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    } catch {
      return ts
    }
  }

  const formatDuration = (ms: number) => {
    if (ms < 1) return '<1ms'
    if (ms < 1000) return `${Math.round(ms)}ms`
    return `${(ms / 1000).toFixed(1)}s`
  }

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title={VIEW_MODE_TITLES[viewMode]} size="4xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* View mode toggle */}
        <div role="tablist" className="tabs tabs-boxed mb-4 w-fit">
          <button
            role="tab"
            aria-selected={viewMode === 'activity'}
            className={`tab tab-sm ${viewMode === 'activity' ? 'tab-active' : ''}`}
            onClick={() => setViewMode('activity')}
          >
            Activity
          </button>
          <button
            role="tab"
            aria-selected={viewMode === 'timeline'}
            className={`tab tab-sm ${viewMode === 'timeline' ? 'tab-active' : ''}`}
            onClick={() => setViewMode('timeline')}
          >
            Timeline
          </button>
          <button
            role="tab"
            aria-selected={viewMode === 'audit'}
            className={`tab tab-sm ${viewMode === 'audit' ? 'tab-active' : ''}`}
            onClick={() => setViewMode('audit')}
          >
            Audit
          </button>
        </div>

        {/* Filter controls */}
        <div className="flex flex-wrap items-center gap-2 mb-4">
          <select
            value={timeRange}
            onChange={(e) => setTimeRange(e.target.value as TimeRange)}
            className="select select-bordered select-sm"
            aria-label="Time range"
          >
            {(Object.keys(TIME_RANGE_LABELS) as TimeRange[]).map((key) => (
              <option key={key} value={key}>
                {TIME_RANGE_LABELS[key]}
              </option>
            ))}
          </select>

          <label className="flex items-center gap-1.5 cursor-pointer">
            <input
              type="checkbox"
              checked={errorsOnly}
              onChange={(e) => setErrorsOnly(e.target.checked)}
              className="checkbox checkbox-sm checkbox-error"
            />
            <span className="text-sm">Errors only</span>
          </label>

          <input
            type="text"
            value={methodFilter}
            onChange={(e) => setMethodFilter(e.target.value)}
            placeholder="Filter by method..."
            aria-label="Filter by method"
            className="input input-bordered input-sm flex-1 min-w-[140px]"
          />

          <button
            onClick={loadActivity}
            disabled={loading || !connected}
            className="btn btn-ghost btn-sm"
            aria-label="Refresh activity log"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
            )}
            Refresh
          </button>

          <button
            onClick={handleExport}
            disabled={exporting || !connected}
            className="btn btn-ghost btn-sm"
            aria-label="Export activity data"
          >
            {exporting ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                />
              </svg>
            )}
            Export
          </button>
          <a ref={downloadRef} className="hidden" href="data:" download aria-hidden="true" tabIndex={-1}>
            download
          </a>
        </div>

        {/* Error */}
        {error && (
          <div className="alert alert-error py-2 mb-4">
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Content */}
        <div className="flex-1 overflow-y-auto">
          {loading && entries.length === 0 ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : entries.length === 0 ? (
            <div className="text-center py-12 text-base-content/50">
              <svg
                aria-hidden="true"
                className="w-10 h-10 mx-auto mb-3 opacity-30"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                />
              </svg>
              <p>No activity entries</p>
            </div>
          ) : (
            <>
              {/* Audit tasks summary */}
              {viewMode === 'audit' && auditTasks.length > 0 && (
                <div className="mb-4 p-3 rounded-lg bg-base-200 border border-base-300">
                  <h4 className="text-sm font-medium mb-2">Active Tasks ({auditTasks.length})</h4>
                  <div className="space-y-1">
                    {auditTasks.map((t) => (
                      <div key={t.id} className="flex items-center justify-between text-xs">
                        <span className="font-mono truncate flex-1">{t.id}</span>
                        <span className="badge badge-xs badge-ghost ml-2">{t.state}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <p className="text-xs text-base-content/50 mb-2">
                Showing {entries.length} of {count} entries
              </p>
              <div className="overflow-x-auto">
                <table className="table table-sm table-zebra w-full">
                  <thead>
                    <tr>
                      <th>Time</th>
                      {viewMode === 'audit' && <th>User</th>}
                      <th>{viewMode === 'timeline' ? 'Event' : 'Method'}</th>
                      <th className="text-right">Duration</th>
                      <th>Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {entries.map((entry, i) => {
                      const hasError = entry.error !== ''
                      return (
                        <tr key={`${entry.correlation_id}-${i}`} className={hasError ? 'text-error' : ''}>
                          <td className="font-mono text-xs whitespace-nowrap">{formatTimestamp(entry.timestamp)}</td>
                          {viewMode === 'audit' && (
                            <td className="text-xs whitespace-nowrap">{entry.user_id || '-'}</td>
                          )}
                          <td className={`text-xs ${viewMode === 'timeline' ? '' : 'font-mono'}`}>
                            {viewMode === 'timeline' ? entry.description || entry.method : entry.method}
                          </td>
                          <td className="text-right text-xs whitespace-nowrap">{formatDuration(entry.duration_ms)}</td>
                          <td>
                            {hasError ? (
                              <span className="badge badge-sm badge-error" title={entry.error}>
                                ERR
                              </span>
                            ) : (
                              <span className="badge badge-sm badge-success">OK</span>
                            )}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
