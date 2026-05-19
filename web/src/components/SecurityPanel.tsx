import { useState, useCallback } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface SecurityPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface SecurityFinding {
  severity: string
  file: string
  line: number
  rule: string
  description: string
}

const SEVERITY_BADGE: Record<string, string> = {
  critical: 'badge-error',
  high: 'badge-warning',
  medium: 'badge-info',
  low: 'badge-ghost',
}

export function SecurityPanel({ isOpen, onClose }: SecurityPanelProps) {
  const { client, connected } = useGlobalStore()

  const [findings, setFindings] = useState<SecurityFinding[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [scanned, setScanned] = useState(false)

  const runScan = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<{ findings: SecurityFinding[] }>('security.scan', { dir: '.' })
      setFindings(result.findings || [])
      setScanned(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to run security scan')
      setFindings([])
    } finally {
      setLoading(false)
    }
  }, [client, connected])

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Security Scan" size="4xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Controls */}
        <div className="flex items-center gap-2 mb-4">
          <button
            onClick={runScan}
            disabled={loading || !connected}
            className="btn btn-primary btn-sm"
            aria-label="Run security scan"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                />
              </svg>
            )}
            Scan
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
          {loading && findings.length === 0 ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : scanned && findings.length === 0 ? (
            <div className="text-center py-12 text-success">
              <svg
                aria-hidden="true"
                className="w-10 h-10 mx-auto mb-3"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
                />
              </svg>
              <p className="font-medium">No security issues found</p>
            </div>
          ) : findings.length > 0 ? (
            <>
              <p className="text-xs text-base-content/50 mb-2">
                {findings.length} finding{findings.length !== 1 ? 's' : ''}
              </p>
              <div className="overflow-x-auto">
                <table className="table table-sm table-zebra w-full">
                  <thead>
                    <tr>
                      <th>Severity</th>
                      <th>File</th>
                      <th>Line</th>
                      <th>Rule</th>
                      <th>Description</th>
                    </tr>
                  </thead>
                  <tbody>
                    {findings.map((f, i) => (
                      <tr key={`${f.file}-${f.line}-${i}`}>
                        <td>
                          <span className={`badge badge-sm ${SEVERITY_BADGE[f.severity] || 'badge-ghost'}`}>
                            {f.severity}
                          </span>
                        </td>
                        <td className="font-mono text-xs">{f.file}</td>
                        <td className="text-xs">{f.line}</td>
                        <td className="font-mono text-xs">{f.rule}</td>
                        <td className="text-xs">{f.description}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          ) : (
            <div className="text-center py-12 text-base-content/50">
              <svg
                aria-hidden="true"
                className="w-10 h-10 mx-auto mb-3 opacity-30"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                />
              </svg>
              <p>Click Scan to check for security issues</p>
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
