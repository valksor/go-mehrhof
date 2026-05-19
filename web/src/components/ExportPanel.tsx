import { useState, useCallback } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { useProjectStore } from '../stores/projectStore'
import { AccessibleModal } from './ui/AccessibleModal'
import { downloadJSON } from '../lib/export'

interface ExportPanelProps {
  isOpen: boolean
  onClose: () => void
}

export function ExportPanel({ isOpen, onClose }: ExportPanelProps) {
  const [exporting, setExporting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [format] = useState<'json' | 'csv'>('json')
  const [since, setSince] = useState('7d')
  const [include, setInclude] = useState('tasks,metrics,activity')

  const taskAvailable = !!useProjectStore((s) => s.task)

  const handleGlobalExport = useCallback(async () => {
    const client = useGlobalStore.getState().client
    if (!client) return
    setExporting(true)
    setError(null)
    try {
      const result = await client.call<Record<string, unknown>>('export', { format, since, include })
      downloadJSON(result, `kvelmo-export-${new Date().toISOString().slice(0, 10)}.json`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Export failed')
    } finally {
      setExporting(false)
    }
  }, [format, since, include])

  const handleTaskExport = useCallback(async () => {
    const client = useProjectStore.getState().client
    if (!client) return
    setExporting(true)
    setError(null)
    try {
      const result = await client.call<Record<string, unknown>>('task.export', { format: 'json' })
      downloadJSON(result, `kvelmo-task-export-${new Date().toISOString().slice(0, 10)}.json`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Task export failed')
    } finally {
      setExporting(false)
    }
  }, [])

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Export Data" size="md">
      <div className="space-y-4">
        {error && (
          <div className="alert alert-error text-sm">
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span>{error}</span>
          </div>
        )}

        {/* Global Export Section */}
        <div className="card bg-base-200">
          <div className="card-body p-4">
            <h3 className="card-title text-sm">Global Export</h3>
            <p className="text-xs text-base-content/70">
              Export task history, metrics, and activity log across all projects.
            </p>
            <div className="flex gap-2 flex-wrap mt-2">
              <select
                className="select select-xs select-bordered"
                value={since}
                onChange={(e) => setSince(e.target.value)}
                aria-label="Time range"
              >
                <option value="7d">Last 7 days</option>
                <option value="30d">Last 30 days</option>
                <option value="90d">Last 90 days</option>
              </select>
              <select
                className="select select-xs select-bordered"
                value={include}
                onChange={(e) => setInclude(e.target.value)}
                aria-label="Data to include"
              >
                <option value="tasks,metrics,activity">All data</option>
                <option value="tasks">Tasks only</option>
                <option value="metrics">Metrics only</option>
                <option value="activity">Activity only</option>
              </select>
            </div>
            <button className="btn btn-sm btn-primary mt-2" onClick={handleGlobalExport} disabled={exporting}>
              {exporting ? <span className="loading loading-spinner loading-xs" /> : 'Download JSON'}
            </button>
          </div>
        </div>

        {/* Task Export Section */}
        <div className="card bg-base-200">
          <div className="card-body p-4">
            <h3 className="card-title text-sm">Current Task Export</h3>
            <p className="text-xs text-base-content/70">
              Export full context for the active task: metadata, spec, chat, diff, checkpoints.
            </p>
            <button
              className="btn btn-sm btn-primary mt-2"
              onClick={handleTaskExport}
              disabled={exporting || !taskAvailable}
            >
              {!taskAvailable ? (
                'No active task'
              ) : exporting ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                'Download Task JSON'
              )}
            </button>
          </div>
        </div>
      </div>
    </AccessibleModal>
  )
}
