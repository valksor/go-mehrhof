import { useState, useCallback, useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface MetricsPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface MetricsData {
  jobs_submitted: number
  jobs_completed: number
  jobs_failed: number
  jobs_in_progress: number
  rpc_requests: number
  rpc_errors: number
  avg_latency_ms: number
  p99_latency_ms: number
  agent_connects: number
  agent_disconnects: number
  events_dropped: number
  permissions_approved: number
  permissions_denied: number
  tokens_consumed: number
  agents?: Record<string, {
    tokens: number
    requests: number
    errors: number
    avg_latency_ms: number
  }>
}

function pct(num: number, den: number): string {
  if (den === 0) return '—'
  return `${((num / den) * 100).toFixed(1)}%`
}

export function MetricsPanel({ isOpen, onClose }: MetricsPanelProps) {
  const { client, connected } = useGlobalStore()

  const [metrics, setMetrics] = useState<MetricsData | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<MetricsData>('metrics', {})
      setMetrics(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load metrics')
      setMetrics(null)
    } finally {
      setLoading(false)
    }
  }, [client, connected])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
    if (isOpen && connected) void load()
  }, [isOpen, connected, load])

  const agents = metrics?.agents ? Object.entries(metrics.agents) : []

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Metrics" size="4xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Toolbar */}
        <div className="flex items-center justify-end mb-4">
          <button
            onClick={load}
            disabled={loading || !connected}
            className="btn btn-ghost btn-sm"
            aria-label="Refresh metrics"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            )}
            Refresh
          </button>
        </div>

        {/* Error */}
        {error && (
          <div className="alert alert-error py-2 mb-4">
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Content */}
        <div className="flex-1 overflow-y-auto">
          {loading && !metrics ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : !metrics ? (
            <div className="text-center py-12 text-base-content/50">
              <p>No metrics available</p>
            </div>
          ) : (
            <div className="space-y-6">
              {/* Job Stats */}
              <div>
                <h3 className="text-sm font-semibold mb-2 opacity-60">Jobs</h3>
                <div className="stats stats-vertical sm:stats-horizontal shadow w-full">
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Submitted</div>
                    <div className="stat-value text-2xl">{metrics.jobs_submitted}</div>
                  </div>
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Completed</div>
                    <div className="stat-value text-2xl text-success">{metrics.jobs_completed}</div>
                  </div>
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Failed</div>
                    <div className="stat-value text-2xl text-error">{metrics.jobs_failed}</div>
                  </div>
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">In Progress</div>
                    <div className="stat-value text-2xl text-info">{metrics.jobs_in_progress}</div>
                  </div>
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Success Rate</div>
                    <div className="stat-value text-2xl">{pct(metrics.jobs_completed, metrics.jobs_completed + metrics.jobs_failed)}</div>
                  </div>
                </div>
              </div>

              {/* RPC Stats */}
              <div>
                <h3 className="text-sm font-semibold mb-2 opacity-60">RPC</h3>
                <div className="stats stats-vertical sm:stats-horizontal shadow w-full">
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Requests</div>
                    <div className="stat-value text-2xl">{metrics.rpc_requests}</div>
                  </div>
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Errors</div>
                    <div className="stat-value text-2xl text-error">{metrics.rpc_errors}</div>
                    <div className="stat-desc">{pct(metrics.rpc_errors, metrics.rpc_requests)} error rate</div>
                  </div>
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Avg Latency</div>
                    <div className="stat-value text-2xl">{(metrics.avg_latency_ms ?? 0).toFixed(1)}ms</div>
                  </div>
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">P99 Latency</div>
                    <div className="stat-value text-2xl">{(metrics.p99_latency_ms ?? 0).toFixed(1)}ms</div>
                  </div>
                </div>
              </div>

              {/* Agent Stats */}
              <div>
                <h3 className="text-sm font-semibold mb-2 opacity-60">Agent</h3>
                <div className="stats stats-vertical sm:stats-horizontal shadow w-full">
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Tokens</div>
                    <div className="stat-value text-2xl">{metrics.tokens_consumed.toLocaleString()}</div>
                  </div>
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Permissions</div>
                    <div className="stat-value text-2xl">{metrics.permissions_approved + metrics.permissions_denied}</div>
                    <div className="stat-desc">{metrics.permissions_approved} approved, {metrics.permissions_denied} denied</div>
                  </div>
                  <div className="stat py-3 px-4">
                    <div className="stat-title text-xs">Events Dropped</div>
                    <div className="stat-value text-2xl">{metrics.events_dropped}</div>
                  </div>
                </div>
              </div>

              {/* Per-Agent Table */}
              {agents.length > 0 && (
                <div>
                  <h3 className="text-sm font-semibold mb-2 opacity-60">Per-Agent Breakdown</h3>
                  <div className="overflow-x-auto">
                    <table className="table table-sm w-full">
                      <thead>
                        <tr>
                          <th>Agent</th>
                          <th className="text-right">Tokens</th>
                          <th className="text-right">Requests</th>
                          <th className="text-right">Errors</th>
                          <th className="text-right">Avg Latency</th>
                        </tr>
                      </thead>
                      <tbody>
                        {agents.map(([name, data]) => (
                          <tr key={name}>
                            <td className="font-mono text-xs">{name}</td>
                            <td className="text-right">{data.tokens.toLocaleString()}</td>
                            <td className="text-right">{data.requests}</td>
                            <td className="text-right">{data.errors > 0 ? <span className="text-error">{data.errors}</span> : '0'}</td>
                            <td className="text-right">{data.avg_latency_ms}ms</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
