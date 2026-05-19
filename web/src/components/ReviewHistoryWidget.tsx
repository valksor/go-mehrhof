import { useEffect, useState } from 'react'
import { useProjectStore, type Review, type ReviewDetail } from '../stores/projectStore'

interface ReviewHistoryWidgetProps {
  embedded?: boolean
}

export function ReviewHistoryWidget({ embedded = false }: ReviewHistoryWidgetProps) {
  const reviews = useProjectStore((s) => s.reviews)
  const loadReviews = useProjectStore((s) => s.loadReviews)
  const loadReview = useProjectStore((s) => s.loadReview)
  const reviewDetails = useProjectStore((s) => s.reviewDetails)
  const connected = useProjectStore((s) => s.connected)

  const [expandedNumber, setExpandedNumber] = useState<number | null>(null)
  const [loadingNumber, setLoadingNumber] = useState<number | null>(null)

  useEffect(() => {
    if (connected) {
      void loadReviews()
    }
  }, [connected, loadReviews])

  const handleToggleExpand = async (r: Review) => {
    if (expandedNumber === r.number) {
      setExpandedNumber(null)
      return
    }

    setExpandedNumber(r.number)

    if (!reviewDetails[r.number]) {
      setLoadingNumber(r.number)
      await loadReview(r.number)
      setLoadingNumber(null)
    }
  }

  const formatTimestamp = (ts: string) => {
    try {
      return new Date(ts).toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    } catch {
      return ts
    }
  }

  const content = (
    <div>
      {reviews.length === 0 ? (
        <div className="text-center py-6">
          <svg
            aria-hidden="true"
            className="w-8 h-8 mx-auto mb-2 text-base-content/30"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
            />
          </svg>
          <p className="text-base-content/60 text-sm">No reviews yet</p>
        </div>
      ) : (
        <div className="space-y-2 max-h-[400px] overflow-auto">
          {reviews.map((r: Review) => {
            const isExpanded = expandedNumber === r.number
            const isLoading = loadingNumber === r.number
            const detail: ReviewDetail | undefined = reviewDetails[r.number]

            return (
              <div key={r.number} className="rounded-lg bg-base-300 border border-transparent overflow-hidden">
                {/* Row header — always visible, clickable to expand */}
                <button
                  className="w-full p-3 text-left hover:bg-base-200/50 transition-colors"
                  onClick={() => handleToggleExpand(r)}
                  aria-expanded={isExpanded}
                  aria-label={`${r.approved ? 'Approved' : 'Rejected'} review #${r.number} — ${isExpanded ? 'collapse' : 'expand'}`}
                >
                  <div className="flex items-start gap-3">
                    {/* Approve/Reject icon */}
                    <div
                      className={`flex-shrink-0 w-6 h-6 rounded-full flex items-center justify-center mt-0.5 ${
                        r.approved ? 'bg-success/20 text-success' : 'bg-error/20 text-error'
                      }`}
                    >
                      {r.approved ? (
                        <svg
                          aria-hidden="true"
                          className="w-3.5 h-3.5"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
                        </svg>
                      ) : (
                        <svg
                          aria-hidden="true"
                          className="w-3.5 h-3.5"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2.5}
                            d="M6 18L18 6M6 6l12 12"
                          />
                        </svg>
                      )}
                    </div>

                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between gap-2 mb-1">
                        <span className={`text-xs font-semibold ${r.approved ? 'text-success' : 'text-error'}`}>
                          {r.approved ? 'Approved' : 'Rejected'} #{r.number}
                        </span>
                        <div className="flex items-center gap-1.5 flex-shrink-0">
                          <span className="text-xs text-base-content/50">{formatTimestamp(r.timestamp)}</span>
                          <svg
                            aria-hidden="true"
                            className={`w-3.5 h-3.5 text-base-content/40 transition-transform duration-150 ${isExpanded ? 'rotate-90' : ''}`}
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                          </svg>
                        </div>
                      </div>
                      {r.message && (
                        <p
                          className={`text-sm text-base-content/80 leading-relaxed ${isExpanded ? '' : 'line-clamp-2'}`}
                        >
                          {r.message}
                        </p>
                      )}
                    </div>
                  </div>
                </button>

                {/* Expanded detail section */}
                {isExpanded && (
                  <div className="border-t border-base-200 px-3 pb-3 pt-2">
                    {isLoading ? (
                      <div className="flex items-center justify-center py-4">
                        <span className="loading loading-spinner loading-sm text-primary"></span>
                      </div>
                    ) : detail ? (
                      <div className="space-y-3">
                        {detail.content && (
                          <div>
                            <p className="text-xs font-semibold text-base-content/50 uppercase tracking-wide mb-1">
                              Full Review
                            </p>
                            <p className="text-sm text-base-content/80 leading-relaxed whitespace-pre-wrap">
                              {detail.content}
                            </p>
                          </div>
                        )}
                        {detail.findings && detail.findings.length > 0 && (
                          <div>
                            <p className="text-xs font-semibold text-base-content/50 uppercase tracking-wide mb-1">
                              Findings ({detail.findings.length})
                            </p>
                            <ul className="space-y-1">
                              {detail.findings.map((finding, i) => (
                                <li key={i} className="text-sm text-base-content/80 flex items-start gap-1.5">
                                  <span className="text-base-content/40 mt-0.5">•</span>
                                  <span>{finding}</span>
                                </li>
                              ))}
                            </ul>
                          </div>
                        )}
                        {!detail.content && (!detail.findings || detail.findings.length === 0) && (
                          <p className="text-sm text-base-content/50 italic">No additional detail available.</p>
                        )}
                      </div>
                    ) : (
                      <p className="text-sm text-base-content/50 italic">Could not load review detail.</p>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )

  if (embedded) {
    return content
  }

  return (
    <section className="card bg-base-200">
      <div className="card-body">
        <h2 className="card-title text-base-content flex items-center gap-2">
          <svg
            aria-hidden="true"
            className="w-5 h-5 text-primary"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
            />
          </svg>
          Review History
          {reviews.length > 0 && <span className="badge badge-sm badge-ghost">{reviews.length}</span>}
        </h2>
        <div className="mt-4">{content}</div>
      </div>
    </section>
  )
}
