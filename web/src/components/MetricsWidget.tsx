import { useState } from 'react'
import { useGlobalStore, type TimedSnapshot } from '../stores/globalStore'

function Sparkline({ data, color = 'currentColor' }: { data: number[]; color?: string }) {
  if (data.length < 2) return null
  const max = Math.max(...data, 1)
  const w = 120
  const h = 28
  const points = data.map((v, i) => `${(i / (data.length - 1)) * w},${h - (v / max) * h}`).join(' ')

  return (
    <svg width={w} height={h} className="inline-block ml-2 opacity-70">
      <polyline points={points} fill="none" stroke={color} strokeWidth="1.5" />
    </svg>
  )
}

function HistoryView({ entries }: { entries: TimedSnapshot[] }) {
  if (entries.length === 0) {
    return <p className="text-sm opacity-60">No history data yet</p>
  }

  const series = [
    { label: 'Jobs Submitted', key: 'jobs_submitted' as const, color: 'oklch(var(--p))' },
    { label: 'Jobs Completed', key: 'jobs_completed' as const, color: 'oklch(var(--su))' },
    { label: 'Jobs Failed', key: 'jobs_failed' as const, color: 'oklch(var(--er))' },
    { label: 'RPC Requests', key: 'rpc_requests' as const, color: 'oklch(var(--in))' },
  ]

  return (
    <div className="grid grid-cols-2 gap-2 text-sm">
      {series.map(s => (
        <div key={s.key}>
          <span className="opacity-60">{s.label}</span>
          <div className="flex items-center">
            <span className="font-mono">{entries[entries.length - 1][s.key]}</span>
            <Sparkline data={entries.map(e => e[s.key])} color={s.color} />
          </div>
        </div>
      ))}
    </div>
  )
}

export function MetricsWidget() {
  const metrics = useGlobalStore(s => s.metrics)
  const metricsHistory = useGlobalStore(s => s.metricsHistory)
  const loadMetrics = useGlobalStore(s => s.loadMetrics)
  const loadMetricsHistory = useGlobalStore(s => s.loadMetricsHistory)
  const [showHistory, setShowHistory] = useState(false)

  if (!metrics) {
    return (
      <div className="card bg-base-200 p-4">
        <h3 className="font-semibold mb-2">System Metrics</h3>
        <p className="text-sm opacity-60">No metrics available</p>
        <button className="btn btn-xs btn-ghost mt-2" onClick={loadMetrics}>Refresh</button>
      </div>
    )
  }

  const handleToggle = () => {
    const next = !showHistory
    setShowHistory(next)
    if (next && metricsHistory.length === 0) {
      loadMetricsHistory()
    }
  }

  return (
    <div className="card bg-base-200 p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-semibold">System Metrics</h3>
        <div className="flex gap-1">
          <button
            className={`btn btn-xs ${showHistory ? 'btn-primary' : 'btn-ghost'}`}
            onClick={handleToggle}
          >
            History
          </button>
          <button
            className="btn btn-xs btn-ghost"
            onClick={() => { loadMetrics(); if (showHistory) loadMetricsHistory() }}
          >
            Refresh
          </button>
        </div>
      </div>

      {showHistory ? (
        <HistoryView entries={metricsHistory} />
      ) : (
        <div className="grid grid-cols-2 gap-2 text-sm">
          <div>
            <span className="opacity-60">Jobs Submitted</span>
            <span className="block font-mono">{metrics.jobs_submitted}</span>
          </div>
          <div>
            <span className="opacity-60">Jobs Completed</span>
            <span className="block font-mono">{metrics.jobs_completed}</span>
          </div>
          <div>
            <span className="opacity-60">Jobs Failed</span>
            <span className="block font-mono text-error">{metrics.jobs_failed || 0}</span>
          </div>
          <div>
            <span className="opacity-60">In Progress</span>
            <span className="block font-mono">{metrics.jobs_in_progress}</span>
          </div>
          <div>
            <span className="opacity-60">RPC Requests</span>
            <span className="block font-mono">{metrics.rpc_requests}</span>
          </div>
          <div>
            <span className="opacity-60">P99 Latency</span>
            <span className="block font-mono">{(metrics.p99_latency_ms ?? 0).toFixed(1)}ms</span>
          </div>
          <div>
            <span className="opacity-60">Agent Connects</span>
            <span className="block font-mono">{metrics.agent_connects}</span>
          </div>
          <div>
            <span className="opacity-60">Events Dropped</span>
            <span className="block font-mono">{metrics.events_dropped || 0}</span>
          </div>
        </div>
      )}

      {/* Per-agent performance breakdown */}
      {metrics.agents && Object.keys(metrics.agents).length > 0 && (
        <div className="mt-3 pt-3 border-t border-base-300">
          <span className="text-xs font-semibold opacity-70">Agent Performance</span>
          <div className="overflow-x-auto mt-1">
            <table className="table table-xs">
              <thead>
                <tr>
                  <th>Agent</th>
                  <th className="text-right">Requests</th>
                  <th className="text-right">Tokens</th>
                  <th className="text-right">Avg Latency</th>
                  <th className="text-right">Errors</th>
                </tr>
              </thead>
              <tbody>
                {Object.entries(metrics.agents)
                  .sort(([, a], [, b]) => (b.tokens ?? 0) - (a.tokens ?? 0))
                  .map(([name, stats]) => (
                    <tr key={name}>
                      <td className="font-mono">{name}</td>
                      <td className="text-right font-mono">{stats.requests ?? 0}</td>
                      <td className="text-right font-mono">{(stats.tokens ?? 0).toLocaleString()}</td>
                      <td className="text-right font-mono">{(stats.avg_latency_ms ?? 0).toFixed(0)}ms</td>
                      <td className="text-right font-mono">{(stats.errors ?? 0) > 0 ? <span className="text-error">{stats.errors}</span> : '0'}</td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
