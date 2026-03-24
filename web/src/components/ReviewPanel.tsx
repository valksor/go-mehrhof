import { useProjectStore, Review, ReviewDetail } from '../stores/projectStore'
import { useState, useEffect, useCallback, useRef } from 'react'

interface ReviewPanelProps {
  data?: Record<string, unknown>
}

export function ReviewPanel({ data }: ReviewPanelProps) {
  const reviews = useProjectStore((state) => state.reviews)
  const loadReview = useProjectStore((state) => state.loadReview)
  const pendingNodeApprovals = useProjectStore((state) => state.pendingNodeApprovals)
  const approveNode = useProjectStore((state) => state.approveNode)
  const rejectNode = useProjectStore((state) => state.rejectNode)
  const [selectedReview, setSelectedReview] = useState<ReviewDetail | null>(null)
  const [selectedSummary, setSelectedSummary] = useState<Review | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Track the last loaded review number to avoid re-loading
  const lastLoadedNumber = useRef<number | undefined>(undefined)

  // Use data from props if available (with runtime check), otherwise from store
  const displayReviews = Array.isArray(data?.reviews)
    ? (data.reviews as Review[])
    : reviews

  // Stable identifier for latest review
  const latestNumber = displayReviews.length > 0
    ? displayReviews[displayReviews.length - 1].number
    : undefined

  // Load review details with error handling
  const handleLoadReview = useCallback((review: Review) => {
    setLoading(true)
    setError(null)
    setSelectedSummary(review)
    lastLoadedNumber.current = review.number
    loadReview(review.number)
      .then((detail) => {
        setSelectedReview(detail)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to load review')
        setSelectedReview(null)
      })
      .finally(() => {
        setLoading(false)
      })
  }, [loadReview])

  // Load the latest review details when latestNumber changes
  useEffect(() => {
    if (latestNumber !== undefined && displayReviews.length > 0) {
      // Only load if we haven't loaded this review yet (use ref to avoid dependency cycle)
      if (lastLoadedNumber.current !== latestNumber) {
        const latest = displayReviews[displayReviews.length - 1]
        // Schedule outside effect to avoid synchronous setState warning
        queueMicrotask(() => handleLoadReview(latest))
      }
    }
  }, [latestNumber, displayReviews, handleLoadReview])

  if (displayReviews.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-base-content/50">
        <div className="text-center">
          <svg className="w-12 h-12 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p className="text-sm">No reviews yet</p>
        </div>
      </div>
    )
  }

  // Use selected summary for header display, fall back to latest
  const displaySummary = selectedSummary ?? displayReviews[displayReviews.length - 1]

  return (
    <div className="h-full overflow-auto p-4">
      <div className="space-y-4">
        {/* Status Header - uses selected review */}
        <div className="flex items-center gap-3">
          <StatusBadge approved={displaySummary.approved} />
          <div className="flex-1">
            <h2 className="text-lg font-semibold">
              Review #{displaySummary.number}
            </h2>
            <p className="text-xs text-base-content/60">
              {new Date(displaySummary.timestamp).toLocaleString()}
            </p>
          </div>
        </div>

        {/* Summary Message - uses selected review */}
        <div className="bg-base-200 rounded-lg p-4">
          <h3 className="text-sm font-medium mb-2 text-base-content/70">Summary</h3>
          <p className="text-sm text-base-content/80">{displaySummary.message}</p>
        </div>

        {/* Error state */}
        {error && (
          <div className="bg-error/10 border border-error/20 rounded-lg p-4 text-center">
            <p className="text-sm text-error">{error}</p>
          </div>
        )}

        {/* Findings (from detail) */}
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <span className="loading loading-spinner loading-md"></span>
          </div>
        ) : selectedReview?.findings && selectedReview.findings.length > 0 ? (
          <div className="space-y-2">
            <h3 className="text-sm font-medium text-base-content/70">
              Findings ({selectedReview.findings.length})
            </h3>
            <div className="space-y-2">
              {selectedReview.findings.map((finding, index) => (
                <FindingItem key={index} finding={finding} index={index} />
              ))}
            </div>
          </div>
        ) : !error ? (
          <div className="bg-success/10 border border-success/20 rounded-lg p-4 text-center">
            <svg className="w-8 h-8 mx-auto mb-2 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <p className="text-sm text-success">No issues found</p>
          </div>
        ) : null}

        {/* Pending Node Approvals */}
        {pendingNodeApprovals.length > 0 && (
          <div className="space-y-2">
            <h3 className="text-sm font-medium text-base-content/70">
              Pending Approvals ({pendingNodeApprovals.length})
            </h3>
            <div className="space-y-2">
              {pendingNodeApprovals.map((node) => (
                <div key={node.nodeId} className="bg-warning/10 border border-warning/20 rounded-lg p-3">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium truncate">{node.nodeId}</p>
                      {node.message && (
                        <p className="text-xs text-base-content/60 mt-0.5">{node.message}</p>
                      )}
                    </div>
                    <div className="flex gap-1.5 shrink-0">
                      <button
                        className="btn btn-success btn-xs"
                        onClick={() => approveNode(node.nodeId)}
                        aria-label={`Approve node ${node.nodeId}`}
                      >
                        Approve
                      </button>
                      <button
                        className="btn btn-error btn-xs btn-outline"
                        onClick={() => rejectNode(node.nodeId)}
                        aria-label={`Reject node ${node.nodeId}`}
                      >
                        Reject
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Review History */}
        {displayReviews.length > 1 && (
          <div className="space-y-2">
            <h3 className="text-sm font-medium text-base-content/70">History</h3>
            <div className="space-y-1">
              {displayReviews.slice(0, -1).reverse().map((review) => (
                <button
                  key={review.number}
                  className={`w-full flex items-center gap-2 p-2 rounded-lg hover:bg-base-200 transition-colors text-left ${
                    selectedSummary?.number === review.number ? 'bg-base-200' : ''
                  }`}
                  onClick={() => handleLoadReview(review)}
                >
                  <span className={`w-2 h-2 rounded-full ${review.approved ? 'bg-success' : 'bg-error'}`} />
                  <span className="text-sm flex-1">Review #{review.number}</span>
                  <span className="text-xs text-base-content/50">
                    {new Date(review.timestamp).toLocaleDateString()}
                  </span>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function StatusBadge({ approved }: { approved: boolean }) {
  if (approved) {
    return (
      <div className="badge badge-success gap-1 font-medium">
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
        Approved
      </div>
    )
  }

  return (
    <div className="badge badge-error gap-1 font-medium">
      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
      </svg>
      Changes Requested
    </div>
  )
}

function FindingItem({ finding, index }: { finding: string; index: number }) {
  // Try to parse severity from finding (e.g., "[HIGH] message" or "[warning] message")
  const severityMatch = finding.match(/^\[(high|medium|low|critical|warning|info)\]/i)
  const severity = severityMatch ? severityMatch[1].toLowerCase() : 'info'
  const message = severityMatch ? finding.slice(severityMatch[0].length).trim() : finding

  // Try to parse classification from finding (e.g., "{flaky}" or "{genuine}")
  const classMatch = message.match(/\{(flaky|genuine|intermittent)\}/i)
  const classification = classMatch ? classMatch[1].toLowerCase() : null
  const cleanMessage = classMatch ? message.replace(classMatch[0], '').trim() : message

  const severityConfig: Record<string, { color: string; bg: string }> = {
    critical: { color: 'text-error', bg: 'bg-error/10' },
    high: { color: 'text-error', bg: 'bg-error/10' },
    medium: { color: 'text-warning', bg: 'bg-warning/10' },
    warning: { color: 'text-warning', bg: 'bg-warning/10' },
    low: { color: 'text-info', bg: 'bg-info/10' },
    info: { color: 'text-base-content/70', bg: 'bg-base-200' },
  }

  const { color, bg } = severityConfig[severity] || severityConfig.info

  return (
    <div className={`${bg} rounded-lg p-3`}>
      <div className="flex items-start gap-2">
        <span className={`text-xs font-medium uppercase ${color}`}>
          {severity}
        </span>
        <span className="text-xs text-base-content/50">#{index + 1}</span>
        {classification && <ClassificationBadge classification={classification} />}
      </div>
      <p className="text-sm mt-1">{cleanMessage}</p>
    </div>
  )
}

function ClassificationBadge({ classification }: { classification: string }) {
  const config: Record<string, string> = {
    flaky: 'badge-warning',
    genuine: 'badge-error',
    intermittent: 'badge-accent',
  }

  const badgeClass = config[classification] || 'badge-ghost'

  return (
    <span className={`badge badge-xs ${badgeClass}`}>
      {classification}
    </span>
  )
}
