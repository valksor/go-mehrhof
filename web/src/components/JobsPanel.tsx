import { useEffect, useState } from 'react'
import { useGlobalStore, type Job } from '../stores/globalStore'
import { EmptyState } from './EmptyState'

export function JobsPanel() {
  const { jobs, loadJobs, loadJob, connected } = useGlobalStore()
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [jobDetail, setJobDetail] = useState<Record<string, Job>>({})
  const [loadingId, setLoadingId] = useState<string | null>(null)

  useEffect(() => {
    if (connected) {
      void loadJobs()
    }
  }, [connected, loadJobs])

  const handleToggleExpand = async (job: Job) => {
    if (expandedId === job.id) {
      setExpandedId(null)
      return
    }

    setExpandedId(job.id)

    if (!jobDetail[job.id]) {
      setLoadingId(job.id)
      const detail = await loadJob(job.id)
      if (detail) {
        setJobDetail(prev => ({ ...prev, [job.id]: detail }))
      }
      setLoadingId(null)
    }
  }

  const formatDate = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      })
    } catch {
      return dateStr
    }
  }

  const statusBadgeClass = (status: string) => {
    switch (status) {
      case 'completed': return 'badge-success'
      case 'running':
      case 'in_progress': return 'badge-warning'
      case 'failed': return 'badge-error'
      case 'queued':
      case 'pending': return 'badge-info'
      default: return 'badge-ghost'
    }
  }

  const statusDotClass = (status: string) => {
    switch (status) {
      case 'completed': return 'bg-success'
      case 'running':
      case 'in_progress': return 'bg-warning animate-pulse'
      case 'failed': return 'bg-error'
      case 'queued':
      case 'pending': return 'bg-info'
      default: return 'bg-base-content/30'
    }
  }

  if (!connected) {
    return (
      <div className="flex items-center justify-center h-full text-base-content/50">
        <p className="text-sm">Not connected</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-base-300">
        <h3 className="font-semibold text-base-content flex items-center gap-2">
          <svg className="w-5 h-5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 10h16M4 14h16M4 18h16" />
          </svg>
          Jobs
          {jobs.length > 0 && (
            <span className="badge badge-sm badge-ghost">{jobs.length}</span>
          )}
        </h3>
        <button
          onClick={loadJobs}
          className="btn btn-ghost btn-xs btn-square"
          title="Refresh jobs"
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
      </div>

      {/* Job list */}
      <div className="flex-1 overflow-auto p-3">
        {jobs.length === 0 ? (
          <EmptyState title="No jobs found" description="Jobs will appear here when tasks are planned or implemented" icon="📝" />
        ) : (
          <div className="space-y-1.5">
            {jobs.map((job) => {
              const isExpanded = expandedId === job.id
              const isLoading = loadingId === job.id
              const detail = jobDetail[job.id]

              return (
                <div
                  key={job.id}
                  className="rounded-lg bg-base-200 border border-transparent overflow-hidden"
                >
                  {/* Row — click to expand */}
                  <button
                    className="w-full px-3 py-2.5 text-left hover:bg-base-300/50 transition-colors"
                    onClick={() => handleToggleExpand(job)}
                  >
                    <div className="flex items-center gap-3">
                      <div className={`w-2 h-2 rounded-full flex-shrink-0 ${statusDotClass(job.status)}`} />

                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-0.5">
                          <span className="font-mono text-xs text-base-content/70 truncate">
                            {job.id.slice(0, 12)}...
                          </span>
                          <span className={`badge badge-xs ${statusBadgeClass(job.status)}`}>
                            {job.status}
                          </span>
                        </div>
                        <div className="flex items-center gap-3 text-xs text-base-content/50">
                          <span className="capitalize">{job.type}</span>
                          {job.worktree_id && (
                            <span className="font-mono truncate max-w-[120px]" title={job.worktree_id}>
                              {job.worktree_id.split('/').pop()}
                            </span>
                          )}
                          <span className="flex-shrink-0">{formatDate(job.created_at)}</span>
                        </div>
                      </div>

                      <svg
                        className={`w-3.5 h-3.5 text-base-content/40 flex-shrink-0 transition-transform duration-150 ${isExpanded ? 'rotate-90' : ''}`}
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                      </svg>
                    </div>
                  </button>

                  {/* Expanded detail */}
                  {isExpanded && (
                    <div className="border-t border-base-300 px-3 pb-3 pt-2">
                      {isLoading ? (
                        <div className="flex items-center justify-center py-4">
                          <span className="loading loading-spinner loading-sm text-primary"></span>
                        </div>
                      ) : detail ? (
                        <div className="space-y-2 text-xs">
                          <div className="grid grid-cols-2 gap-2">
                            <div>
                              <span className="text-base-content/50 uppercase tracking-wide font-semibold">ID</span>
                              <p className="font-mono text-base-content/80 break-all">{detail.id}</p>
                            </div>
                            <div>
                              <span className="text-base-content/50 uppercase tracking-wide font-semibold">Type</span>
                              <p className="text-base-content/80 capitalize">{detail.type}</p>
                            </div>
                            <div>
                              <span className="text-base-content/50 uppercase tracking-wide font-semibold">Status</span>
                              <p className="text-base-content/80">{detail.status}</p>
                            </div>
                            {detail.updated_at && (
                              <div>
                                <span className="text-base-content/50 uppercase tracking-wide font-semibold">Updated</span>
                                <p className="text-base-content/80">{formatDate(detail.updated_at)}</p>
                              </div>
                            )}
                          </div>

                          {detail.worktree_id && (
                            <div>
                              <span className="text-base-content/50 uppercase tracking-wide font-semibold">Worktree</span>
                              <p className="font-mono text-base-content/80 break-all">{detail.worktree_id}</p>
                            </div>
                          )}

                          {detail.error && (
                            <div>
                              <span className="text-base-content/50 uppercase tracking-wide font-semibold text-error">Error</span>
                              <p className="text-error/80 whitespace-pre-wrap">{detail.error}</p>
                            </div>
                          )}

                          {detail.result && Object.keys(detail.result).length > 0 && (
                            <div>
                              <span className="text-base-content/50 uppercase tracking-wide font-semibold">Result</span>
                              <pre className="bg-base-300 rounded p-2 text-xs text-base-content/80 overflow-auto max-h-32 whitespace-pre-wrap break-all">
                                {JSON.stringify(detail.result, null, 2)}
                              </pre>
                            </div>
                          )}
                        </div>
                      ) : (
                        <p className="text-xs text-base-content/50 italic">Could not load job details.</p>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
