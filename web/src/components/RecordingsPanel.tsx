import { useState, useCallback, useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface RecordingsPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface RecordingInfo {
  path: string
  job_id: string
  agent: string
  model?: string
  started_at: string
  lines: number
}

interface RecordingHeader {
  job_id: string
  agent: string
  model?: string
  work_dir?: string
  started_at: string
}

interface RecordingRecord {
  timestamp: string
  job_id: string
  direction: 'in' | 'out'
  type?: string
  event: unknown
}

interface RecordingViewResult {
  header: RecordingHeader
  records: RecordingRecord[]
}

export function RecordingsPanel({ isOpen, onClose }: RecordingsPanelProps) {
  const { client, connected } = useGlobalStore()

  const [recordings, setRecordings] = useState<RecordingInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [jobFilter, setJobFilter] = useState('')
  const [selectedRecording, setSelectedRecording] = useState<RecordingViewResult | null>(null)
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [viewLoading, setViewLoading] = useState(false)

  const loadRecordings = useCallback(async () => {
    if (!client) return

    setLoading(true)
    setError(null)

    try {
      const params: Record<string, string> = {}
      if (jobFilter.trim()) {
        params.job = jobFilter.trim()
      }
      const result = await client.call<{ recordings: RecordingInfo[] }>('recordings.list', params)
      setRecordings(result.recordings || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load recordings')
      setRecordings([])
    } finally {
      setLoading(false)
    }
  }, [client, jobFilter])

  // Load recordings when panel opens
  useEffect(() => {
    if (isOpen && connected) {
      void loadRecordings()
    }
  }, [isOpen, connected, loadRecordings])

  const handleView = useCallback(async (filePath: string) => {
    if (!client) return

    // Toggle: clicking the same recording closes it
    if (selectedFile === filePath) {
      setSelectedRecording(null)
      setSelectedFile(null)
      return
    }

    setViewLoading(true)
    setError(null)

    try {
      const result = await client.call<RecordingViewResult>('recordings.view', { file: filePath })
      setSelectedRecording(result)
      setSelectedFile(filePath)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to view recording')
    } finally {
      setViewLoading(false)
    }
  }, [client, selectedFile])

  const handleFilterKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      void loadRecordings()
    }
  }

  const formatDate = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    } catch {
      return dateStr
    }
  }

  const formatTime = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleTimeString(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    } catch {
      return dateStr
    }
  }

  const fileName = (path: string) => path.split('/').pop() ?? path

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Recordings" size="4xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Filter input */}
        <div className="flex gap-2 mb-4">
          <input
            type="text"
            value={jobFilter}
            onChange={e => setJobFilter(e.target.value)}
            onKeyDown={handleFilterKeyDown}
            placeholder="Filter by job ID..."
            aria-label="Filter recordings by job ID"
            className="input input-bordered flex-1"
          />
          <button
            onClick={loadRecordings}
            disabled={loading || !connected}
            className="btn btn-primary"
          >
            {loading ? (
              <span className="loading loading-spinner loading-sm"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            )}
            Filter
          </button>
        </div>

        {/* Error */}
        {error && (
          <div className="alert alert-error py-2 mb-4">
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Recordings list */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : recordings.length === 0 ? (
            <div className="text-center py-12 text-base-content/50">
              <svg aria-hidden="true" className="w-12 h-12 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 01-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 011.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 00-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 01-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-1.5a3.375 3.375 0 00-3.375-3.375H9.75" />
              </svg>
              <p>No recordings found</p>
              <p className="text-xs mt-2 text-base-content/40">Recordings are created when agents run tasks</p>
            </div>
          ) : (
            <div className="space-y-2">
              <p className="text-xs text-base-content/50 mb-2">{recordings.length} recording{recordings.length !== 1 ? 's' : ''}</p>
              {recordings.map((rec) => (
                <div key={rec.path}>
                  <button
                    type="button"
                    onClick={() => handleView(rec.path)}
                    className={`w-full text-left p-3 rounded-lg transition-all duration-150 border ${
                      selectedFile === rec.path
                        ? 'bg-primary/10 border-primary/30'
                        : 'bg-base-200 border-base-300 hover:bg-base-300'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2 mb-1">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-sm text-base-content truncate">
                          {rec.job_id.length > 16 ? rec.job_id.slice(0, 16) + '...' : rec.job_id}
                        </span>
                        <span className="badge badge-sm badge-ghost">{rec.agent}</span>
                        {rec.model && (
                          <span className="badge badge-sm badge-outline">{rec.model}</span>
                        )}
                      </div>
                      <div className="flex items-center gap-2 flex-shrink-0">
                        <span className="text-xs text-base-content/50">{rec.lines} lines</span>
                        <span className="text-xs text-base-content/40">{formatDate(rec.started_at)}</span>
                      </div>
                    </div>
                    <p className="text-xs text-base-content/40 font-mono truncate">{fileName(rec.path)}</p>
                  </button>

                  {/* Expanded recording view */}
                  {selectedFile === rec.path && selectedRecording && (
                    <div className="mt-1 p-3 rounded-lg bg-base-200 border border-base-300 max-h-80 overflow-y-auto">
                      {/* Header info */}
                      {selectedRecording.header && (
                        <div className="mb-3 pb-2 border-b border-base-300">
                          <div className="flex items-center gap-3 text-xs text-base-content/60">
                            <span>Job: <span className="font-mono">{selectedRecording.header.job_id}</span></span>
                            <span>Agent: {selectedRecording.header.agent}</span>
                            {selectedRecording.header.model && <span>Model: {selectedRecording.header.model}</span>}
                            {selectedRecording.header.work_dir && (
                              <span className="truncate">Dir: {selectedRecording.header.work_dir}</span>
                            )}
                          </div>
                        </div>
                      )}

                      {/* Records */}
                      <div className="space-y-1">
                        {selectedRecording.records.map((record, idx) => (
                          <div key={idx} className="flex items-start gap-2 text-xs font-mono">
                            <span className="text-base-content/40 flex-shrink-0">{formatTime(record.timestamp)}</span>
                            <span className={`flex-shrink-0 ${record.direction === 'in' ? 'text-info' : 'text-success'}`}>
                              {record.direction === 'in' ? '<-' : '->'}
                            </span>
                            {record.type && (
                              <span className="badge badge-xs badge-ghost flex-shrink-0">{record.type}</span>
                            )}
                            <span className="text-base-content/70 truncate break-all">
                              {typeof record.event === 'string'
                                ? record.event
                                : JSON.stringify(record.event).slice(0, 200)}
                            </span>
                          </div>
                        ))}
                        {selectedRecording.records.length === 0 && (
                          <p className="text-center text-base-content/40 py-4">No records in this recording</p>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Loading state for view */}
                  {selectedFile === rec.path && viewLoading && (
                    <div className="mt-1 p-4 rounded-lg bg-base-200 border border-base-300 flex items-center justify-center">
                      <span className="loading loading-spinner loading-sm"></span>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
