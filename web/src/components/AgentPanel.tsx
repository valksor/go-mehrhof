import { useEffect, useState, useCallback } from 'react'
import { WORKER_POLL_MS, STRATEGY_POLL_MS } from '../lib/constants'
import { useGlobalStore } from '../stores/globalStore'
import { Widget } from './Widget'

interface Worker {
  id: string
  status: string
  agent_name: string
  current_job?: string
  is_default?: boolean
}

interface WorkerStats {
  total_workers: number
  available_workers: number
  working_workers: number
  queued_jobs: number
}

interface WorkersListResult {
  workers: Worker[]
  stats: WorkerStats
}

const AGENT_OPTIONS = [
  { value: 'claude', label: 'Claude' },
  { value: 'codex', label: 'Codex' },
  { value: 'custom', label: 'Custom' },
]

export function AgentPanel() {
  const { client, connected } = useGlobalStore()
  const [workers, setWorkers] = useState<Worker[]>([])
  const [stats, setStats] = useState<WorkerStats | null>(null)
  const [strategies, setStrategies] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showAddForm, setShowAddForm] = useState(false)
  // Default to first available agent type
  const [newAgent, setNewAgent] = useState(AGENT_OPTIONS[0].value)

  const fetchWorkers = useCallback(async () => {
    if (!client || !connected) return
    setLoading(true)
    try {
      const result = await client.call<WorkersListResult>('workers.list')
      setWorkers(result.workers || [])
      setStats(result.stats || null)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch workers')
    } finally {
      setLoading(false)
    }
  }, [client, connected])

  const fetchStrategies = useCallback(async () => {
    if (!client || !connected) return
    try {
      const result = await client.call<string[]>('strategy.list')
      setStrategies(result || [])
    } catch {
      // Non-critical — strategies display is informational
    }
  }, [client, connected])

  useEffect(() => {
    if (!connected) return
    // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
    void fetchWorkers()
    void fetchStrategies()
    const interval = setInterval(fetchWorkers, WORKER_POLL_MS)
    const strategyInterval = setInterval(fetchStrategies, STRATEGY_POLL_MS)
    return () => {
      clearInterval(interval)
      clearInterval(strategyInterval)
    }
  }, [connected, fetchWorkers, fetchStrategies])

  const handleAddWorker = async () => {
    if (!client) return
    try {
      await client.call('workers.add', { agent: newAgent })
      void fetchWorkers()
      setShowAddForm(false)
      setNewAgent(AGENT_OPTIONS[0].value)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add worker')
    }
  }

  const handleRemoveWorker = async (id: string) => {
    if (!client) return
    try {
      await client.call('workers.remove', { id })
      void fetchWorkers()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove worker')
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'available': return 'badge-success'
      case 'working': return 'badge-warning'
      case 'disconnected': return 'badge-error'
      default: return 'badge-ghost'
    }
  }

  return (
    <Widget
      id="agents"
      title="Agents"
      icon={
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
        </svg>
      }
    >
      <div className="space-y-3">
        {/* Stats bar */}
        {stats && (
          <div className="flex gap-2 text-xs flex-wrap">
            <span className="badge badge-sm badge-outline">
              {stats.total_workers} workers
            </span>
            <span className="badge badge-sm badge-success badge-outline">
              {stats.available_workers} available
            </span>
            {stats.working_workers > 0 && (
              <span className="badge badge-sm badge-warning badge-outline">
                {stats.working_workers} working
              </span>
            )}
            {stats.queued_jobs > 0 && (
              <span className="badge badge-sm badge-info badge-outline">
                {stats.queued_jobs} queued
              </span>
            )}
          </div>
        )}

        {/* Available reasoning strategies (read-only, configured per-task via settings) */}
        {strategies.length > 0 && (
          <div className="flex gap-2 text-xs flex-wrap">
            <span className="text-base-content/60" title="Agent reasoning strategies (configure via settings or CLI)">Strategies:</span>
            {strategies.map(s => (
              <span key={s} className="badge badge-sm badge-outline capitalize" title={`${s} strategy`}>{s}</span>
            ))}
          </div>
        )}

        {error && <div className="text-sm text-error">{error}</div>}

        {loading && workers.length === 0 && (
          <div className="flex items-center gap-2 text-sm text-base-content/60">
            <span className="loading loading-spinner loading-xs"></span>
            Loading agents...
          </div>
        )}

        {!loading && workers.length === 0 && !error && (
          <div className="text-center py-4">
            <div className="text-3xl mb-2">🤖</div>
            <p className="text-sm text-base-content/60">No workers running</p>
          </div>
        )}

        {/* Worker list */}
        {workers.length > 0 && (
          <div className="space-y-2">
            {workers.map(worker => (
              <div
                key={worker.id}
                className="p-2 rounded-lg bg-base-100 border border-base-300 group"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-lg">🤖</span>
                    <div>
                      <span className="font-mono text-sm">{worker.id}</span>
                      <span className="text-xs text-base-content/60 ml-2 capitalize">
                        {worker.agent_name}
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <span className={`badge badge-sm ${getStatusColor(worker.status)}`}>
                      {worker.status}
                    </span>
                    {!worker.is_default && (
                      <button
                        onClick={() => handleRemoveWorker(worker.id)}
                        className="btn btn-ghost btn-xs text-error opacity-0 group-hover:opacity-100 transition-opacity"
                        title="Remove worker"
                        disabled={worker.status === 'working'}
                      >
                        <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    )}
                  </div>
                </div>
                {worker.current_job && (
                  <div className="text-xs text-base-content/60 mt-1">
                    Job: <code className="text-primary">{worker.current_job}</code>
                  </div>
                )}
                {worker.status === 'working' && (
                  <progress className="progress progress-primary w-full h-1 mt-1" />
                )}
              </div>
            ))}
          </div>
        )}

        {/* Add worker */}
        {showAddForm ? (
          <div className="p-3 bg-base-200 rounded-lg space-y-3">
            <select
              value={newAgent}
              onChange={(e) => setNewAgent(e.target.value)}
              className="select select-sm select-bordered w-full"
            >
              {AGENT_OPTIONS.map(agent => (
                <option key={agent.value} value={agent.value}>
                  {agent.label}
                </option>
              ))}
            </select>
            <div className="flex gap-2">
              <button
                onClick={handleAddWorker}
                className="btn btn-sm btn-primary flex-1"
                disabled={!connected}
              >
                Add
              </button>
              <button
                onClick={() => setShowAddForm(false)}
                className="btn btn-sm btn-ghost"
              >
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <button
            onClick={() => setShowAddForm(true)}
            className="btn btn-sm btn-outline btn-block"
            disabled={!connected}
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            Add Worker
          </button>
        )}
      </div>
    </Widget>
  )
}
