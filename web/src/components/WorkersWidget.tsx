import { useGlobalStore } from '../stores/globalStore'
import { useEffect } from 'react'

interface WorkersWidgetProps {
  embedded?: boolean
}

export function WorkersWidget({ embedded = false }: WorkersWidgetProps) {
  const { workers, workerStats, loadWorkers, loadWorkerStats, connected } = useGlobalStore()

  // Refresh workers periodically
  useEffect(() => {
    if (!connected) return

    const interval = setInterval(() => {
      void loadWorkers()
      void loadWorkerStats()
    }, 5000)

    return () => clearInterval(interval)
  }, [connected, loadWorkers, loadWorkerStats])

  const content = (
    <div>
      {/* Stats Summary */}
      {workerStats && (
        <div className="grid grid-cols-2 gap-2 mb-4">
          <div className="stat bg-base-300 rounded-lg p-3">
            <div className="stat-title text-xs">Workers</div>
            <div className="stat-value text-lg">
              {workerStats.available_workers}/{workerStats.total_workers}
            </div>
            <div className="stat-desc text-xs">available</div>
          </div>
          <div className="stat bg-base-300 rounded-lg p-3">
            <div className="stat-title text-xs">Jobs</div>
            <div className="stat-value text-lg">
              {workerStats.in_progress_jobs}
            </div>
            <div className="stat-desc text-xs">
              {workerStats.queued_jobs} queued
            </div>
          </div>
        </div>
      )}

      {/* Worker List */}
      {workers.length === 0 ? (
        <div className="text-center py-6">
          <svg aria-hidden="true" className="w-8 h-8 mx-auto mb-2 text-base-content/30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
          <p className="text-base-content/60 text-sm">No workers registered</p>
        </div>
      ) : (
        <div className="space-y-2 max-h-[300px] overflow-auto">
          {workers.map((worker) => (
            <div
              key={worker.id}
              className="p-3 rounded-lg bg-base-300 border border-transparent"
            >
              <div className="flex items-center gap-3">
                {/* Status indicator */}
                <div aria-hidden="true" className={`w-2 h-2 rounded-full ${
                  worker.status === 'available' ? 'bg-success' :
                  worker.status === 'working' ? 'bg-warning animate-pulse' :
                  'bg-error'
                }`} />

                <div className="flex-1 min-w-0">
                  <div className="font-medium text-sm text-base-content">
                    {worker.agent_name}
                    {worker.is_default && (
                      <span className="ml-2 badge badge-xs badge-primary">default</span>
                    )}
                  </div>
                  <div className="text-xs text-base-content/60">
                    {worker.status === 'working' && worker.current_job ? (
                      <span className="font-mono">Job: {worker.current_job.slice(0, 8)}...</span>
                    ) : (
                      <span className="capitalize">{worker.status}</span>
                    )}
                  </div>
                </div>

                {/* Status badge */}
                <span className={`badge badge-sm ${
                  worker.status === 'available' ? 'badge-success' :
                  worker.status === 'working' ? 'badge-warning' :
                  'badge-error'
                }`}>
                  {worker.status}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Job Stats Footer */}
      {workerStats && (workerStats.completed_jobs > 0 || workerStats.failed_jobs > 0) && (
        <div className="flex gap-4 pt-4 mt-4 border-t border-base-300 text-xs text-base-content/60">
          <span className="flex items-center gap-1">
            <svg aria-hidden="true" className="w-3 h-3 text-success" fill="currentColor" viewBox="0 0 20 20">
              <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
            </svg>
            {workerStats.completed_jobs} completed
          </span>
          {workerStats.failed_jobs > 0 && (
            <span className="flex items-center gap-1">
              <svg aria-hidden="true" className="w-3 h-3 text-error" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
              </svg>
              {workerStats.failed_jobs} failed
            </span>
          )}
        </div>
      )}
    </div>
  )

  if (embedded) {
    return content
  }

  return (
    <section className="card bg-base-200">
      <div className="card-body">
        <h2 className="card-title text-base-content flex items-center gap-2">
          <svg aria-hidden="true" className="w-5 h-5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
          Workers
          {workerStats && (
            <span className="badge badge-sm badge-ghost">
              {workerStats.total_workers}
            </span>
          )}
        </h2>
        <div className="mt-4">
          {content}
        </div>
      </div>
    </section>
  )
}
