import { useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'

const STATUS_LABELS: Record<string, string> = {
  added: 'A',
  modified: 'M',
  deleted: 'D',
  renamed: 'R',
}

export function RecapWidget() {
  const { recap, loadRecap, connected, state } = useProjectStore()

  useEffect(() => {
    if (connected) {
      void loadRecap()
    }
  }, [connected, state, loadRecap])

  if (!recap || recap.state === 'none') return null

  return (
    <section className="card bg-base-200">
      <div className="card-body gap-3">
        <h2 className="card-title text-base-content text-sm">Recap</h2>

        {/* Task summary */}
        {recap.task && (
          <div className="text-sm">
            <div className="font-medium">{recap.task.title}</div>
            <div className="text-base-content/60 text-xs">
              {recap.task.source}
              {recap.task.branch && <span> &middot; {recap.task.branch}</span>}
            </div>
          </div>
        )}

        {/* State + last activity */}
        <div className="flex flex-wrap gap-2 text-xs">
          <span className="badge badge-sm badge-primary">{recap.state}</span>
          {recap.last_activity && <span className="text-base-content/60">{recap.last_activity}</span>}
        </div>

        {/* Phases completed */}
        {recap.phase_metrics && Object.keys(recap.phase_metrics).length > 0 && (
          <div className="flex flex-wrap gap-1">
            {['plan', 'implement', 'simplify', 'optimize', 'review'].map((phase) =>
              recap.phase_metrics?.[phase] ? (
                <span key={phase} className="badge badge-xs badge-ghost">
                  {phase}
                </span>
              ) : null,
            )}
          </div>
        )}

        {/* Files changed */}
        {recap.files_changed && recap.files_changed.length > 0 && (
          <div className="text-xs">
            <div className="text-base-content/60 mb-1">
              {recap.files_changed.length} file{recap.files_changed.length !== 1 ? 's' : ''} changed
            </div>
            <div className="max-h-24 overflow-auto space-y-0.5">
              {recap.files_changed.slice(0, 8).map((f) => (
                <div key={f.path} className="font-mono text-base-content/70 flex gap-1.5">
                  <span className="text-primary/70">{STATUS_LABELS[f.status] || f.status}</span>
                  <span className="truncate">{f.path}</span>
                </div>
              ))}
              {recap.files_changed.length > 8 && (
                <div className="text-base-content/50">+{recap.files_changed.length - 8} more</div>
              )}
            </div>
          </div>
        )}

        {/* Last checkpoint */}
        {recap.last_checkpoint && (
          <div className="text-xs text-base-content/60">
            Last checkpoint: <span className="font-mono">{recap.last_checkpoint.sha?.slice(0, 8) ?? '—'}</span>
            {recap.last_checkpoint.message && <span> &mdash; {recap.last_checkpoint.message}</span>}
          </div>
        )}

        {/* Error */}
        {recap.last_error && <div className="text-xs text-error">{recap.last_error}</div>}

        {/* Next action */}
        {recap.next_action && (
          <div className="text-xs bg-base-300 rounded-lg px-3 py-2">
            <span className="text-base-content/60">Next:</span>{' '}
            <span className="text-base-content">{recap.next_action}</span>
          </div>
        )}
      </div>
    </section>
  )
}
