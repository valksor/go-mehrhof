import { useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { useProjectStore } from '../stores/projectStore'

export function StatsWidget() {
  const connected = useGlobalStore(s => s.connected)
  const metrics = useGlobalStore(s => s.metrics)
  const activeTasks = useGlobalStore(s => s.activeTasks)
  const workers = useGlobalStore(s => s.workers)
  const workerStats = useGlobalStore(s => s.workerStats)
  const loadMetrics = useGlobalStore(s => s.loadMetrics)
  const loadActiveTasks = useGlobalStore(s => s.loadActiveTasks)

  const projectConnected = useProjectStore(s => s.connected)
  const cacheStats = useProjectStore(s => s.cacheStats)
  const loadCacheStats = useProjectStore(s => s.loadCacheStats)
  const clearCache = useProjectStore(s => s.clearCache)

  useEffect(() => {
    if (!connected) return
    void loadMetrics()
    void loadActiveTasks()
  }, [connected, loadMetrics, loadActiveTasks])

  useEffect(() => {
    if (!projectConnected) return
    void loadCacheStats()
  }, [projectConnected, loadCacheStats])

  // Compute tasks by state
  const tasksByState: Record<string, number> = {}
  for (const t of activeTasks) {
    if (t.state && t.state !== 'none') {
      tasksByState[t.state] = (tasksByState[t.state] || 0) + 1
    }
  }
  const totalActive = Object.values(tasksByState).reduce((a, b) => a + b, 0)

  // Compute success rate from metrics
  const completed = metrics?.jobs_completed ?? 0
  const failed = metrics?.jobs_failed ?? 0
  const successRate = completed + failed > 0
    ? Math.round((completed / (completed + failed)) * 1000) / 10
    : null

  // Worker stats
  const totalWorkers = workerStats?.total_workers ?? workers.length
  const activeWorkers = workerStats?.working_workers ?? 0
  const idleWorkers = workerStats?.available_workers ?? (totalWorkers - activeWorkers)

  const handleRefresh = () => {
    void loadMetrics()
    void loadActiveTasks()
    void loadCacheStats()
  }

  return (
    <div className="card bg-base-200 p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-semibold flex items-center gap-2">
          <svg className="w-4 h-4 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          Stats
        </h3>
        <button className="btn btn-xs btn-ghost" onClick={handleRefresh}>Refresh</button>
      </div>

      <div className="grid grid-cols-2 gap-2 text-sm">
        {/* Job success rate */}
        <div>
          <span className="opacity-60">Success Rate</span>
          <span className="block font-mono">
            {successRate !== null ? `${successRate}%` : '--'}
          </span>
        </div>

        {/* Active tasks */}
        <div>
          <span className="opacity-60">Active Tasks</span>
          <span className="block font-mono">{totalActive}</span>
        </div>

        {/* Workers */}
        <div>
          <span className="opacity-60">Workers</span>
          <span className="block font-mono">
            {activeWorkers} active / {idleWorkers} idle
          </span>
        </div>

        {/* Avg latency */}
        <div>
          <span className="opacity-60">Avg Latency</span>
          <span className="block font-mono">
            {metrics ? `${(metrics.avg_latency_ms ?? 0).toFixed(1)}ms` : '--'}
          </span>
        </div>

        {/* Total tokens consumed */}
        {(metrics?.tokens_consumed ?? 0) > 0 && (
          <div>
            <span className="opacity-60">Tokens Used</span>
            <span className="block font-mono">
              {(metrics?.tokens_consumed ?? 0).toLocaleString()}
            </span>
          </div>
        )}
      </div>

      {/* Response cache stats */}
      {cacheStats?.enabled && (
        <div className="mt-3 pt-3 border-t border-base-300">
          <div className="flex items-center justify-between">
            <span className="text-xs opacity-60">Response Cache</span>
            {cacheStats.entries > 0 && (
              <button
                className="btn btn-xs btn-ghost opacity-60"
                onClick={clearCache}
              >
                Clear
              </button>
            )}
          </div>
          <div className="grid grid-cols-2 gap-2 text-sm mt-1">
            <div>
              <span className="opacity-60">Hit Rate</span>
              <span className="block font-mono">
                {cacheStats.hits + cacheStats.misses > 0
                  ? `${(cacheStats.hit_rate * 100).toFixed(1)}%`
                  : '--'}
              </span>
            </div>
            <div>
              <span className="opacity-60">Entries</span>
              <span className="block font-mono">{cacheStats.entries}</span>
            </div>
            <div>
              <span className="opacity-60">Hits / Misses</span>
              <span className="block font-mono">{cacheStats.hits} / {cacheStats.misses}</span>
            </div>
            <div>
              <span className="opacity-60">Tokens Saved</span>
              <span className="block font-mono">
                {cacheStats.tokens_saved > 0 ? cacheStats.tokens_saved.toLocaleString() : '0'}
              </span>
            </div>
          </div>
        </div>
      )}

      {/* Tasks by state breakdown */}
      {totalActive > 0 && (
        <div className="mt-3 pt-3 border-t border-base-300">
          <span className="text-xs opacity-60">Tasks by State</span>
          <div className="flex flex-wrap gap-1.5 mt-1">
            {Object.entries(tasksByState)
              .sort(([, a], [, b]) => b - a)
              .map(([state, count]) => (
                <span
                  key={state}
                  className={`badge badge-sm ${
                    state === 'implementing' || state === 'planning' || state === 'reviewing' || state === 'simplifying' || state === 'optimizing'
                      ? 'badge-warning'
                      : state === 'implemented' ? 'badge-success'
                      : state === 'failed' ? 'badge-error'
                      : state === 'submitted' ? 'badge-secondary'
                      : state === 'planned' ? 'badge-primary'
                      : state === 'loaded' ? 'badge-info'
                      : 'badge-ghost'
                  }`}
                >
                  {count} {state}
                </span>
              ))}
          </div>
        </div>
      )}
    </div>
  )
}
