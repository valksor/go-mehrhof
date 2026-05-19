import { useState, useCallback, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface CIStatusPanelProps {
  isOpen: boolean
  onClose: () => void
}

type CIState = 'passing' | 'failed' | 'pending' | 'unknown'

interface CIStatusResult {
  state: CIState
  message?: string
  pr_id?: string
}

const STATE_BADGE: Record<CIState, string> = {
  passing: 'badge-success',
  failed: 'badge-error',
  pending: 'badge-warning',
  unknown: 'badge-ghost',
}

const STATE_LABEL: Record<CIState, string> = {
  passing: 'Passing',
  failed: 'Failed',
  pending: 'Pending',
  unknown: 'Unknown',
}

const STATE_ALERT: Record<CIState, string> = {
  passing: 'alert-success',
  failed: 'alert-error',
  pending: 'alert-warning',
  unknown: '',
}

export function CIStatusPanel({ isOpen, onClose }: CIStatusPanelProps) {
  const { client, connected } = useProjectStore()

  const [ciStatus, setCIStatus] = useState<CIStatusResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadCIStatus = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<CIStatusResult>('ci.status', {})
      setCIStatus(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load CI status')
      setCIStatus(null)
    } finally {
      setLoading(false)
    }
  }, [client, connected])

  useEffect(() => {
    if (isOpen && connected) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
      void loadCIStatus()
    }
  }, [isOpen, connected, loadCIStatus])

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="CI Status" size="2xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Toolbar */}
        <div className="flex items-center justify-end mb-4">
          <button
            onClick={loadCIStatus}
            disabled={loading || !connected}
            className="btn btn-ghost btn-sm"
            aria-label="Refresh CI status"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
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
          {loading && !ciStatus ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : !ciStatus ? (
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
                  d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2z"
                />
              </svg>
              <p>No CI data available</p>
            </div>
          ) : (
            <div className="space-y-4">
              {/* Overall status */}
              <div className="flex items-center gap-3">
                <span className="text-sm font-medium">Pipeline Status</span>
                <span className={`badge ${STATE_BADGE[ciStatus.state] || 'badge-ghost'}`}>
                  {STATE_LABEL[ciStatus.state] || ciStatus.state}
                </span>
              </div>

              {/* Message */}
              {ciStatus.message && (
                <div className={`alert py-2 ${STATE_ALERT[ciStatus.state] || ''}`}>
                  <svg
                    aria-hidden="true"
                    className="w-4 h-4 shrink-0"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <span className="text-sm">{ciStatus.message}</span>
                </div>
              )}

              {/* PR ID */}
              {ciStatus.pr_id && (
                <div className="flex items-center gap-2 text-sm">
                  <span className="opacity-60">PR:</span>
                  <span className="font-mono">{ciStatus.pr_id}</span>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
