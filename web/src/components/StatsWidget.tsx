import { useEffect, useState, useMemo } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { useProjectStore } from '../stores/projectStore'

interface HistoryTask {
  id: string
  title: string
  final_state: string
  started_at: string
  completed_at: string
}

interface TaskAnalytics {
  total: number
  byState: Record<string, number>
  successRate: number | null
  avgDuration: string | null
  recent: Array<{ title: string; finalState: string; completedAt: string; duration: string }>
}

function formatDuration(ms: number): string {
  const secs = Math.floor(ms / 1000)
  if (secs < 60) return `${secs}s`
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins}m${secs % 60}s`
  const hours = Math.floor(mins / 60)
  return `${hours}h${mins % 60}m`
}

function computeAnalytics(tasks: HistoryTask[]): TaskAnalytics {
  if (tasks.length === 0) {
    return { total: 0, byState: {}, successRate: null, avgDuration: null, recent: [] }
  }

  const byState: Record<string, number> = {}
  let totalDuration = 0
  let durationCount = 0

  for (const t of tasks) {
    byState[t.final_state] = (byState[t.final_state] || 0) + 1
    const dur = new Date(t.completed_at).getTime() - new Date(t.started_at).getTime()
    if (dur > 0) {
      totalDuration += dur
      durationCount++
    }
  }

  const finished = (byState['finished'] || 0) + (byState['submitted'] || 0)
  const successRate = Math.round((finished / tasks.length) * 1000) / 10

  const avgDuration = durationCount > 0 ? formatDuration(totalDuration / durationCount) : null

  const recent = tasks.slice(0, 5).map((t) => {
    const dur = new Date(t.completed_at).getTime() - new Date(t.started_at).getTime()
    return {
      title: t.title || t.id,
      finalState: t.final_state,
      completedAt: new Date(t.completed_at).toLocaleDateString(),
      duration: Number.isFinite(dur) && dur > 0 ? formatDuration(dur) : '--',
    }
  })

  return { total: tasks.length, byState, successRate, avgDuration, recent }
}

export function StatsWidget() {
  const connected = useGlobalStore((s) => s.connected)
  const metrics = useGlobalStore((s) => s.metrics)
  const activeTasks = useGlobalStore((s) => s.activeTasks)
  const workers = useGlobalStore((s) => s.workers)
  const workerStats = useGlobalStore((s) => s.workerStats)
  const loadMetrics = useGlobalStore((s) => s.loadMetrics)
  const loadActiveTasks = useGlobalStore((s) => s.loadActiveTasks)

  const projectConnected = useProjectStore((s) => s.connected)
  const client = useProjectStore((s) => s.client)
  const cacheStats = useProjectStore((s) => s.cacheStats)
  const loadCacheStats = useProjectStore((s) => s.loadCacheStats)
  const clearCache = useProjectStore((s) => s.clearCache)

  const [historyTasks, setHistoryTasks] = useState<HistoryTask[] | null>(null)

  useEffect(() => {
    if (!connected) return
    void loadMetrics()
    void loadActiveTasks()
  }, [connected, loadMetrics, loadActiveTasks])

  useEffect(() => {
    if (!projectConnected) return
    void loadCacheStats()
  }, [projectConnected, loadCacheStats])

  useEffect(() => {
    if (!projectConnected || !client) return
    let cancelled = false
    client
      .call<{ tasks: HistoryTask[] | null }>('task.history', {})
      .then((result) => {
        if (!cancelled) setHistoryTasks(result.tasks || [])
      })
      .catch(() => {
        if (!cancelled) setHistoryTasks(null)
      })
    return () => {
      cancelled = true
    }
  }, [projectConnected, client])

  const analytics = useMemo(() => (historyTasks ? computeAnalytics(historyTasks) : null), [historyTasks])

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
  const successRate = completed + failed > 0 ? Math.round((completed / (completed + failed)) * 1000) / 10 : null

  // Worker stats
  const totalWorkers = workerStats?.total_workers ?? workers.length
  const activeWorkers = workerStats?.working_workers ?? 0
  const idleWorkers = workerStats?.available_workers ?? totalWorkers - activeWorkers

  const loadHistory = () => {
    if (!projectConnected || !client) return
    client
      .call<{ tasks: HistoryTask[] | null }>('task.history', {})
      .then((result) => setHistoryTasks(result.tasks || []))
      .catch(() => setHistoryTasks(null))
  }

  const handleRefresh = () => {
    void loadMetrics()
    void loadActiveTasks()
    void loadCacheStats()
    loadHistory()
  }

  return (
    <div className="card bg-base-200 p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-semibold flex items-center gap-2">
          <svg className="w-4 h-4 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
            />
          </svg>
          Stats
        </h3>
        <button className="btn btn-xs btn-ghost" onClick={handleRefresh}>
          Refresh
        </button>
      </div>

      <div className="grid grid-cols-2 gap-2 text-sm">
        {/* Job success rate */}
        <div>
          <span className="opacity-60" title="Ratio of completed to failed background jobs">
            Phase Success
          </span>
          <span className="block font-mono">{successRate !== null ? `${successRate}%` : '--'}</span>
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
          <span className="block font-mono">{metrics ? `${(metrics.avg_latency_ms ?? 0).toFixed(1)}ms` : '--'}</span>
        </div>

        {/* Total tokens consumed */}
        {(metrics?.tokens_consumed ?? 0) > 0 && (
          <div>
            <span className="opacity-60">Tokens Used</span>
            <span className="block font-mono">{(metrics?.tokens_consumed ?? 0).toLocaleString()}</span>
          </div>
        )}
      </div>

      {/* Response cache stats */}
      {cacheStats?.enabled && (
        <div className="mt-3 pt-3 border-t border-base-300">
          <div className="flex items-center justify-between">
            <span className="text-xs opacity-60">Response Cache</span>
            {cacheStats.entries > 0 && (
              <button className="btn btn-xs btn-ghost opacity-60" onClick={clearCache}>
                Clear
              </button>
            )}
          </div>
          <div className="grid grid-cols-2 gap-2 text-sm mt-1">
            <div>
              <span className="opacity-60">Hit Rate</span>
              <span className="block font-mono">
                {cacheStats.hits + cacheStats.misses > 0 ? `${(cacheStats.hit_rate * 100).toFixed(1)}%` : '--'}
              </span>
            </div>
            <div>
              <span className="opacity-60">Entries</span>
              <span className="block font-mono">{cacheStats.entries}</span>
            </div>
            <div>
              <span className="opacity-60">Hits / Misses</span>
              <span className="block font-mono">
                {cacheStats.hits} / {cacheStats.misses}
              </span>
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
                    state === 'implementing' ||
                    state === 'planning' ||
                    state === 'reviewing' ||
                    state === 'simplifying' ||
                    state === 'optimizing'
                      ? 'badge-warning'
                      : state === 'implemented'
                        ? 'badge-success'
                        : state === 'failed'
                          ? 'badge-error'
                          : state === 'submitted'
                            ? 'badge-secondary'
                            : state === 'planned'
                              ? 'badge-primary'
                              : state === 'loaded'
                                ? 'badge-info'
                                : 'badge-ghost'
                  }`}
                >
                  {count} {state}
                </span>
              ))}
          </div>
        </div>
      )}

      {/* Task analytics from history */}
      {analytics && analytics.total > 0 && (
        <div className="mt-3 pt-3 border-t border-base-300">
          <span className="text-xs opacity-60">Task History</span>
          <div className="grid grid-cols-2 gap-2 text-sm mt-1">
            <div>
              <span className="opacity-60">Total Tasks</span>
              <span className="block font-mono">{analytics.total}</span>
            </div>
            <div>
              <span className="opacity-60" title="Ratio of finished/submitted tasks to total completed tasks">
                Task Success
              </span>
              <span className="block font-mono">{`${analytics.successRate}%`}</span>
            </div>
            {analytics.avgDuration && (
              <div>
                <span className="opacity-60">Avg Duration</span>
                <span className="block font-mono">{analytics.avgDuration}</span>
              </div>
            )}
          </div>

          {/* By-state breakdown for completed tasks */}
          <div className="flex flex-wrap gap-1.5 mt-2">
            {Object.entries(analytics.byState)
              .sort(([, a], [, b]) => b - a)
              .map(([state, count]) => (
                <span
                  key={state}
                  className={`badge badge-xs ${
                    state === 'finished'
                      ? 'badge-success'
                      : state === 'submitted'
                        ? 'badge-secondary'
                        : state === 'abandoned'
                          ? 'badge-warning'
                          : state === 'failed'
                            ? 'badge-error'
                            : 'badge-ghost'
                  }`}
                >
                  {count} {state}
                </span>
              ))}
          </div>

          {/* Recent completions */}
          {analytics.recent.length > 0 && (
            <div className="mt-2">
              <span className="text-xs opacity-60">Recent</span>
              <ul className="mt-1 space-y-1">
                {analytics.recent.map((t, i) => (
                  <li key={i} className="flex items-center justify-between text-xs gap-2">
                    <span className="truncate">{t.title}</span>
                    <span className="flex items-center gap-1.5 shrink-0">
                      <span
                        className={`badge badge-xs ${
                          t.finalState === 'finished'
                            ? 'badge-success'
                            : t.finalState === 'submitted'
                              ? 'badge-secondary'
                              : t.finalState === 'abandoned'
                                ? 'badge-warning'
                                : t.finalState === 'failed'
                                  ? 'badge-error'
                                  : 'badge-ghost'
                        }`}
                      >
                        {t.finalState}
                      </span>
                      <span className="opacity-50 font-mono">{t.duration}</span>
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
