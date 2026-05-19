import { useState, useEffect, useCallback } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface DiagnosePanelProps {
  isOpen: boolean
  onClose: () => void
}

interface CheckResult {
  name: string
  status: string
  detail?: string
  fix?: string
}

interface ProviderResult {
  name: string
  configured: boolean
}

interface DiagnoseData {
  checks: CheckResult[]
  global_socket: string
  providers: ProviderResult[]
  issues?: string[]
}

const STATUS_BADGE: Record<string, string> = {
  passed: 'badge-success',
  failed: 'badge-error',
  warning: 'badge-warning',
}

const STATUS_LABEL: Record<string, string> = {
  passed: 'OK',
  failed: 'Failed',
  warning: 'Warning',
}

const CHECK_DISPLAY_NAME: Record<string, string> = {
  git: 'Git',
  claude: 'Claude CLI',
  'claude-auth': 'Claude Auth',
  codex: 'Codex CLI',
}

export function DiagnosePanel({ isOpen, onClose }: DiagnosePanelProps) {
  const { connected } = useGlobalStore()
  const [data, setData] = useState<DiagnoseData | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const runDiagnose = useCallback(async () => {
    const client = useGlobalStore.getState().client
    if (!client) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<DiagnoseData>('system.diagnose', {})
      setData(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Diagnosis failed')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (isOpen && connected) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
      void runDiagnose()
    }
  }, [isOpen, connected, runDiagnose])

  const allPassed = data && !data.issues?.length

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="System Diagnostics" size="lg">
      <div className="max-h-[70vh] flex flex-col">
        {loading && (
          <div className="flex items-center justify-center py-12">
            <span className="loading loading-spinner loading-lg text-primary"></span>
          </div>
        )}

        {error && (
          <div className="alert alert-error py-2 mb-4">
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span className="text-sm">{error}</span>
          </div>
        )}

        {data && !loading && (
          <>
            {/* Overall status */}
            {allPassed ? (
              <div className="alert alert-success py-2 mb-4">
                <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                <span className="text-sm">All checks passed</span>
              </div>
            ) : (
              <div className="alert alert-warning py-2 mb-4">
                <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <span className="text-sm">
                  {data.issues?.length} issue{data.issues && data.issues.length !== 1 ? 's' : ''} found
                </span>
              </div>
            )}

            {/* Preflight checks */}
            <h3 className="text-sm font-semibold text-base-content mb-2">System Checks</h3>
            <div className="space-y-2 mb-4">
              {data.checks.map((c) => (
                <div key={c.name} className="flex items-center justify-between p-2.5 rounded-lg bg-base-200">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-base-content">
                        {CHECK_DISPLAY_NAME[c.name] || c.name}
                      </span>
                      <span className={`badge badge-sm ${STATUS_BADGE[c.status] || 'badge-ghost'}`}>
                        {STATUS_LABEL[c.status] || c.status}
                      </span>
                    </div>
                    {c.detail && c.status === 'passed' && (
                      <p className="text-xs text-base-content/50 mt-0.5">{c.detail}</p>
                    )}
                    {c.fix && <p className="text-xs text-warning mt-0.5">{c.fix}</p>}
                  </div>
                </div>
              ))}

              {/* Global socket (always running if we got here) */}
              <div className="flex items-center justify-between p-2.5 rounded-lg bg-base-200">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-base-content">Global Socket</span>
                  <span className="badge badge-sm badge-success">
                    {data.global_socket === 'running' ? 'Running' : data.global_socket}
                  </span>
                </div>
              </div>
            </div>

            {/* Provider tokens */}
            <h3 className="text-sm font-semibold text-base-content mb-2">Provider Tokens</h3>
            <div className="space-y-2 mb-4">
              {data.providers.map((p) => (
                <div key={p.name} className="flex items-center justify-between p-2.5 rounded-lg bg-base-200">
                  <span className="text-sm font-medium text-base-content">{p.name}</span>
                  <span className={`badge badge-sm ${p.configured ? 'badge-success' : 'badge-ghost'}`}>
                    {p.configured ? 'Configured' : 'Not configured'}
                  </span>
                </div>
              ))}
            </div>

            {/* Issues / next steps */}
            {data.issues && data.issues.length > 0 && (
              <>
                <h3 className="text-sm font-semibold text-base-content mb-2">Next Steps</h3>
                <ul className="space-y-1 mb-4">
                  {data.issues.map((issue, i) => (
                    <li key={i} className="text-xs text-base-content/70 flex items-start gap-2">
                      <span className="text-warning mt-0.5 flex-shrink-0">&#8226;</span>
                      <span className="font-mono">{issue}</span>
                    </li>
                  ))}
                </ul>
              </>
            )}

            {/* Re-run button */}
            <div className="flex justify-end">
              <button onClick={runDiagnose} disabled={loading} className="btn btn-ghost btn-sm">
                <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                  />
                </svg>
                Re-run
              </button>
            </div>
          </>
        )}
      </div>
    </AccessibleModal>
  )
}
