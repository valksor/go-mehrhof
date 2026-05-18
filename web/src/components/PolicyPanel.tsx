import { useState, useCallback, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface PolicyPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface PolicyViolation {
  severity: 'error' | 'warning'
  rule: string
  message: string
}

interface PolicyCheckResult {
  violations: PolicyViolation[]
  blocking: boolean
}

const SEVERITY_BADGE: Record<PolicyViolation['severity'], string> = {
  error: 'badge-error',
  warning: 'badge-warning',
}

export function PolicyPanel({ isOpen, onClose }: PolicyPanelProps) {
  const { client, connected } = useProjectStore()

  const [result, setResult] = useState<PolicyCheckResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const checkPolicies = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      const data = await client.call<PolicyCheckResult>('policy.check', {})
      setResult({
        violations: data?.violations ?? [],
        blocking: data?.blocking ?? false,
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to check policies')
      setResult(null)
    } finally {
      setLoading(false)
    }
  }, [client, connected])

  useEffect(() => {
    if (isOpen && connected) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
      void checkPolicies()
    }
  }, [isOpen, connected, checkPolicies])

  const violations = result?.violations ?? []

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Policy Checks" size="2xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Toolbar */}
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            {result && (
              <span className={`badge ${result.blocking ? 'badge-error' : 'badge-success'}`}>
                {result.blocking ? 'Blocking' : 'Passing'}
              </span>
            )}
          </div>
          <button
            onClick={checkPolicies}
            disabled={loading || !connected}
            className="btn btn-ghost btn-sm"
            aria-label="Check policies"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
            )}
            Check Policies
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
          {loading && !result ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : violations.length === 0 ? (
            <div className="text-center py-12 text-base-content/50">
              <svg aria-hidden="true" className="w-10 h-10 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
              <p>{result ? 'All policies pass' : 'No policy data available'}</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="table table-sm table-zebra w-full">
                <thead>
                  <tr>
                    <th>Severity</th>
                    <th>Rule</th>
                    <th>Message</th>
                  </tr>
                </thead>
                <tbody>
                  {violations.map((v, i) => (
                    <tr key={`${v.rule}-${i}`}>
                      <td>
                        <span className={`badge badge-sm ${SEVERITY_BADGE[v.severity] || 'badge-ghost'}`}>
                          {v.severity}
                        </span>
                      </td>
                      <td className="font-mono text-xs">{v.rule}</td>
                      <td className="text-xs text-base-content/70">{v.message}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
