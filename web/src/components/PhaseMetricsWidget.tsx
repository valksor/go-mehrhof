import { useProjectStore } from '../stores/projectStore'
import { EmptyState } from './EmptyState'

interface PhaseMetricsWidgetProps {
  embedded?: boolean
}

/** Format nanosecond duration to human-readable string. */
function formatDuration(ns: number): string {
  const ms = ns / 1_000_000
  if (ms < 1000) return `${Math.round(ms)}ms`
  const seconds = ms / 1000
  if (seconds < 60) return `${seconds.toFixed(1)}s`
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = Math.round(seconds % 60)
  return `${minutes}m ${remainingSeconds}s`
}

/** Format USD cost. */
function formatCost(usd: number): string {
  if (usd < 0.01) return '<$0.01'
  return `$${usd.toFixed(2)}`
}

/** Ordered phases for display. */
const PHASE_ORDER = ['plan', 'implement', 'simplify', 'optimize', 'review', 'submit']

export function PhaseMetricsWidget({ embedded = false }: PhaseMetricsWidgetProps) {
  const phaseMetrics = useProjectStore(s => s.phaseMetrics)

  if (!phaseMetrics || Object.keys(phaseMetrics).length === 0) {
    const empty = <EmptyState title="No phase metrics" description="Metrics are recorded as phases complete" icon="--" />
    if (embedded) return empty
    return (
      <section className="card bg-base-200">
        <div className="card-body">
          <h2 className="card-title text-base-content text-base">Phase Metrics</h2>
          {empty}
        </div>
      </section>
    )
  }

  // Sort phases by the canonical order, then any extras alphabetically
  const phases = Object.keys(phaseMetrics).sort((a, b) => {
    const ai = PHASE_ORDER.indexOf(a)
    const bi = PHASE_ORDER.indexOf(b)
    if (ai !== -1 && bi !== -1) return ai - bi
    if (ai !== -1) return -1
    if (bi !== -1) return 1
    return a.localeCompare(b)
  })

  const content = (
    <div className="overflow-x-auto">
      <table className="table table-xs w-full">
        <thead>
          <tr className="text-base-content/60">
            <th>Phase</th>
            <th>Duration</th>
            <th>Agent</th>
            <th className="text-right">Tokens</th>
            <th className="text-right">Cost</th>
          </tr>
        </thead>
        <tbody>
          {phases.map(phase => {
            const m = phaseMetrics[phase]
            if (!m) return null
            return (
              <tr key={phase} className="hover">
                <td className="font-medium">{phase}</td>
                <td className="font-mono text-xs">{formatDuration(m.duration)}</td>
                <td className="text-xs">
                  {m.agent ? (
                    <span className="badge badge-ghost badge-xs">{m.agent}</span>
                  ) : (
                    <span className="text-base-content/40">--</span>
                  )}
                </td>
                <td className="font-mono text-xs text-right">
                  {m.total_tokens ? m.total_tokens.toLocaleString() : '--'}
                </td>
                <td className="font-mono text-xs text-right">
                  {m.est_cost_usd ? formatCost(m.est_cost_usd) : '--'}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )

  if (embedded) return content

  return (
    <section className="card bg-base-200">
      <div className="card-body">
        <h2 className="card-title text-base-content flex items-center gap-2 text-base">
          <svg aria-hidden="true" className="w-4 h-4 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          Phase Metrics
        </h2>
        {content}
      </div>
    </section>
  )
}
