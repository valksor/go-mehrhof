import { useEffect, useRef, useState } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useGlobalStore } from '../stores/globalStore'

interface LogsPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface HistoryMessage {
  id: string
  role: string
  content: string
  timestamp?: string
  job_id?: string
}

type LogTab = 'output' | 'history'

export function LogsPanel({ isOpen, onClose }: LogsPanelProps) {
  const { output, clearOutput, worktreeId } = useProjectStore()
  const { client } = useGlobalStore()
  const logsEndRef = useRef<HTMLDivElement>(null)

  const [tab, setTab] = useState<LogTab>('output')
  const [history, setHistory] = useState<HistoryMessage[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)

  // Auto-scroll to bottom when new output arrives
  useEffect(() => {
    if (isOpen && tab === 'output' && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [output, isOpen, tab])

  // Load chat history when switching to history tab
  useEffect(() => {
    if (!isOpen || tab !== 'history' || !client || !worktreeId) return

    let cancelled = false

    async function fetchHistory() {
      if (!client) return
      setHistoryLoading(true)
      try {
        const result = await client.call<{ messages: HistoryMessage[]; task_id: string }>('chat.history', {
          worktree_id: worktreeId
        })
        if (!cancelled) setHistory(result.messages || [])
      } catch {
        if (!cancelled) setHistory([])
      } finally {
        if (!cancelled) setHistoryLoading(false)
      }
    }

    fetchHistory()
    return () => { cancelled = true }
  }, [isOpen, tab, client, worktreeId])

  if (!isOpen) return null

  const roleBadge = (role: string) => {
    switch (role) {
      case 'user': return 'badge-primary'
      case 'assistant': return 'badge-secondary'
      case 'system': return 'badge-ghost'
      default: return 'badge-ghost'
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-base-100 rounded-2xl shadow-2xl max-w-3xl w-full max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-base-300">
          <div className="flex items-center gap-2">
            <svg aria-hidden="true" className="w-5 h-5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h7" />
            </svg>
            <h2 className="text-lg font-semibold text-base-content">Logs</h2>
          </div>
          <div className="flex items-center gap-2">
            {tab === 'output' && (
              <button
                onClick={clearOutput}
                className="btn btn-ghost btn-sm"
                disabled={output.length === 0}
              >
                Clear
              </button>
            )}
            <button
              onClick={onClose}
              className="btn btn-ghost btn-sm btn-square"
              aria-label="Close"
            >
              <svg aria-hidden="true" className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* Tab bar */}
        <div role="tablist" className="tabs tabs-bordered px-4 pt-2">
          <button
            role="tab"
            aria-selected={tab === 'output'}
            className={`tab gap-1.5 ${tab === 'output' ? 'tab-active' : ''}`}
            onClick={() => setTab('output')}
          >
            Live Output
            <span className="badge badge-sm badge-ghost">{output.length}</span>
          </button>
          <button
            role="tab"
            aria-selected={tab === 'history'}
            className={`tab gap-1.5 ${tab === 'history' ? 'tab-active' : ''}`}
            onClick={() => setTab('history')}
          >
            Chat History
            {history.length > 0 && <span className="badge badge-sm badge-ghost">{history.length}</span>}
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-auto p-4 bg-base-200" role="tabpanel" aria-label={tab === 'output' ? 'Live Output' : 'Chat History'}>
          {tab === 'output' ? (
            // Live output (existing behavior)
            output.length === 0 ? (
              <div className="text-center py-8 text-base-content/50">
                <svg aria-hidden="true" className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 6h16M4 12h16M4 18h7" />
                </svg>
                <p>No output yet</p>
                <p className="text-sm mt-1">Logs will appear here when tasks run</p>
              </div>
            ) : (
              <pre className="font-mono text-sm text-base-content whitespace-pre-wrap break-words">
                {output.map((line, i) => (
                  <div
                    key={i}
                    className={`py-0.5 ${
                      line.toLowerCase().includes('error') || line.toLowerCase().includes('failed')
                        ? 'text-error'
                        : line.toLowerCase().includes('warning')
                        ? 'text-warning'
                        : line.toLowerCase().includes('completed') || line.toLowerCase().includes('success')
                        ? 'text-success'
                        : ''
                    }`}
                  >
                    <span className="text-base-content/40 select-none mr-2">{String(i + 1).padStart(3)}</span>
                    {line}
                  </div>
                ))}
                <div ref={logsEndRef} />
              </pre>
            )
          ) : (
            // Chat history (matches CLI `logs` command data source)
            historyLoading ? (
              <div className="flex items-center justify-center py-12">
                <span className="loading loading-spinner loading-lg text-primary"></span>
              </div>
            ) : history.length === 0 ? (
              <div className="text-center py-8 text-base-content/50">
                <svg aria-hidden="true" className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                </svg>
                <p>No chat history</p>
                <p className="text-sm mt-1">History appears after agent interactions</p>
              </div>
            ) : (
              <div className="space-y-3">
                {history.map((msg) => (
                  <div key={msg.id} className="rounded-lg bg-base-100 p-3 border border-base-300">
                    <div className="flex items-center gap-2 mb-1.5">
                      <span className={`badge badge-xs ${roleBadge(msg.role)}`}>{msg.role}</span>
                      {msg.timestamp && (
                        <span className="text-xs text-base-content/40">
                          {new Date(msg.timestamp).toLocaleTimeString()}
                        </span>
                      )}
                      {msg.job_id && (
                        <span className="text-xs text-base-content/30 font-mono">job:{msg.job_id.slice(0, 8)}</span>
                      )}
                    </div>
                    <p className="text-sm text-base-content/80 whitespace-pre-wrap leading-relaxed">
                      {msg.content}
                    </p>
                  </div>
                ))}
              </div>
            )
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between p-3 border-t border-base-300 bg-base-100">
          <span className="text-xs text-base-content/50">
            {tab === 'output' ? 'Real-time output from current session' : 'Full chat history for current task'}
          </span>
          <button
            onClick={onClose}
            className="btn btn-primary btn-sm"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  )
}
