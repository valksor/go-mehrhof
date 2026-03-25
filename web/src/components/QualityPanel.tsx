import { useState, useCallback, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface QualityPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface AutoFixStatus {
  active: boolean
  attempt: number
  max_attempts: number
  last_error?: string
}

interface FailclassStats {
  total: number
  flaky: number
  genuine: number
  intermittent: number
  unclassified: number
}

interface QualityFinding {
  severity: string
  category: string
  rule: string
  file: string
  line: number
  message: string
  origin: string
  classification: string
}

const SEVERITY_BADGE: Record<string, string> = {
  critical: 'badge-error',
  high: 'badge-error',
  medium: 'badge-warning',
  low: 'badge-info',
  info: 'badge-ghost',
}

const CLASS_BADGE: Record<string, string> = {
  flaky: 'badge-warning',
  genuine: 'badge-error',
  intermittent: 'badge-info',
}

export function QualityPanel({ isOpen, onClose }: QualityPanelProps) {
  const { client, connected, qualityPrompt } = useProjectStore()

  const [autofix, setAutofix] = useState<AutoFixStatus | null>(null)
  const [failclass, setFailclass] = useState<FailclassStats | null>(null)
  const [findings, setFindings] = useState<QualityFinding[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      const [autofixResult, failclassResult] = await Promise.all([
        client.call<AutoFixStatus>('autofix.status', {}).catch(() => null),
        client.call<FailclassStats>('failclass.stats', {}).catch(() => null),
      ])

      setAutofix(autofixResult)
      setFailclass(failclassResult)

      // Try to get quality findings from a quality run
      try {
        const qResult = await client.call<{ findings?: QualityFinding[]; status: string }>('quality.respond', { action: 'status' })
        setFindings(qResult.findings || [])
      } catch {
        // quality.respond may not have findings in status mode — that's ok
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load quality data')
    } finally {
      setLoading(false)
    }
  }, [client, connected])

  useEffect(() => {
    if (isOpen && connected) load()
  }, [isOpen, connected, load])

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Quality Gates" size="4xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Toolbar */}
        <div className="flex items-center justify-end mb-4">
          <button
            onClick={load}
            disabled={loading || !connected}
            className="btn btn-ghost btn-sm"
            aria-label="Refresh quality data"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            )}
            Refresh
          </button>
        </div>

        {/* Error */}
        {error && (
          <div className="alert alert-error py-2 mb-4">
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Content */}
        <div className="flex-1 overflow-y-auto">
          {loading && !autofix && !failclass ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : (
            <div className="space-y-6">
              {/* Quality Prompt (if active) */}
              {qualityPrompt && (
                <div className="alert alert-warning">
                  <svg aria-hidden="true" className="w-5 h-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                  <div>
                    <p className="font-medium text-sm">Quality gate requires input</p>
                    <p className="text-xs opacity-80">{qualityPrompt.question}</p>
                  </div>
                </div>
              )}

              {/* AutoFix Status */}
              {autofix && (
                <div>
                  <h3 className="text-sm font-semibold mb-2 opacity-60">Auto-Fix</h3>
                  <div className="stats stats-vertical sm:stats-horizontal shadow w-full">
                    <div className="stat py-3 px-4">
                      <div className="stat-title text-xs">Status</div>
                      <div className="stat-value text-lg">
                        {autofix.active ? (
                          <span className="text-warning">Running</span>
                        ) : (
                          <span className="text-base-content/50">Idle</span>
                        )}
                      </div>
                    </div>
                    {autofix.active && (
                      <>
                        <div className="stat py-3 px-4">
                          <div className="stat-title text-xs">Attempt</div>
                          <div className="stat-value text-lg">{autofix.attempt}/{autofix.max_attempts}</div>
                        </div>
                        {autofix.last_error && (
                          <div className="stat py-3 px-4">
                            <div className="stat-title text-xs">Last Error</div>
                            <div className="stat-desc text-error truncate max-w-xs">{autofix.last_error}</div>
                          </div>
                        )}
                      </>
                    )}
                  </div>
                </div>
              )}

              {/* Failure Classification */}
              {failclass && failclass.total > 0 && (
                <div>
                  <h3 className="text-sm font-semibold mb-2 opacity-60">Failure Classification</h3>
                  <div className="stats stats-vertical sm:stats-horizontal shadow w-full">
                    <div className="stat py-3 px-4">
                      <div className="stat-title text-xs">Total</div>
                      <div className="stat-value text-2xl">{failclass.total}</div>
                    </div>
                    <div className="stat py-3 px-4">
                      <div className="stat-title text-xs">Genuine</div>
                      <div className="stat-value text-2xl text-error">{failclass.genuine}</div>
                    </div>
                    <div className="stat py-3 px-4">
                      <div className="stat-title text-xs">Flaky</div>
                      <div className="stat-value text-2xl text-warning">{failclass.flaky}</div>
                    </div>
                    <div className="stat py-3 px-4">
                      <div className="stat-title text-xs">Intermittent</div>
                      <div className="stat-value text-2xl text-info">{failclass.intermittent}</div>
                    </div>
                  </div>
                </div>
              )}

              {/* Findings Table */}
              {findings.length > 0 && (
                <div>
                  <h3 className="text-sm font-semibold mb-2 opacity-60">Findings ({findings.length})</h3>
                  <div className="overflow-x-auto">
                    <table className="table table-sm w-full">
                      <thead>
                        <tr>
                          <th>Severity</th>
                          <th>Rule</th>
                          <th>File</th>
                          <th>Message</th>
                          <th>Class</th>
                        </tr>
                      </thead>
                      <tbody>
                        {findings.map((f) => (
                          <tr key={`${f.file}:${f.line}:${f.rule}`}>
                            <td>
                              <span className={`badge badge-sm ${SEVERITY_BADGE[f.severity] || 'badge-ghost'}`}>
                                {f.severity}
                              </span>
                            </td>
                            <td className="font-mono text-xs">{f.rule}</td>
                            <td className="font-mono text-xs truncate max-w-[200px]">
                              {f.file}{f.line > 0 ? `:${f.line}` : ''}
                            </td>
                            <td className="text-xs max-w-[300px] truncate">{f.message}</td>
                            <td>
                              {f.classification && (
                                <span className={`badge badge-sm ${CLASS_BADGE[f.classification] || 'badge-ghost'}`}>
                                  {f.classification}
                                </span>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              {/* Empty state */}
              {!autofix?.active && (!failclass || failclass.total === 0) && findings.length === 0 && !qualityPrompt && (
                <div className="text-center py-12 text-base-content/50">
                  <svg aria-hidden="true" className="w-10 h-10 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <p>No quality gate data</p>
                  <p className="text-xs mt-1">Quality gates run during the Review phase</p>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
