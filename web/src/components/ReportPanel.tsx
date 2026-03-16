import { useState } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface ReportPanelProps {
  isOpen: boolean
  onClose: () => void
}

type SinceRange = '7d' | '30d' | '90d'
type ReportFormat = 'md' | 'json'

const SINCE_LABELS: Record<SinceRange, string> = {
  '7d': 'Last 7 days',
  '30d': 'Last 30 days',
  '90d': 'Last 90 days',
}

export function ReportPanel({ isOpen, onClose }: ReportPanelProps) {
  const { client, connected } = useGlobalStore()

  const [since, setSince] = useState<SinceRange>('30d')
  const [format, setFormat] = useState<ReportFormat>('md')
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const generate = async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)
    setContent('')

    try {
      const result = await client.call<{ markdown: string; report: object }>(
        'report.generate',
        { format, since }
      )
      if (format === 'json') {
        setContent(JSON.stringify(result.report, null, 2))
      } else {
        setContent(result.markdown)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate report')
    } finally {
      setLoading(false)
    }
  }

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Compliance Report" size="4xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Controls */}
        <div className="flex flex-wrap items-center gap-2 mb-4">
          <select
            value={since}
            onChange={e => setSince(e.target.value as SinceRange)}
            className="select select-bordered select-sm"
            aria-label="Time range"
          >
            {(Object.keys(SINCE_LABELS) as SinceRange[]).map(key => (
              <option key={key} value={key}>{SINCE_LABELS[key]}</option>
            ))}
          </select>

          <div role="tablist" className="tabs tabs-boxed w-fit">
            <button
              role="tab"
              aria-selected={format === 'md'}
              className={`tab tab-sm ${format === 'md' ? 'tab-active' : ''}`}
              onClick={() => setFormat('md')}
            >
              Markdown
            </button>
            <button
              role="tab"
              aria-selected={format === 'json'}
              className={`tab tab-sm ${format === 'json' ? 'tab-active' : ''}`}
              onClick={() => setFormat('json')}
            >
              JSON
            </button>
          </div>

          <button
            className="btn btn-sm btn-primary"
            onClick={generate}
            disabled={loading || !connected}
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
            )}
            Generate
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
          {loading && !content ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : content ? (
            <pre className="text-xs whitespace-pre-wrap font-mono bg-base-200 p-3 rounded-lg">
              {content}
            </pre>
          ) : (
            <div className="text-center py-12 text-base-content/50">
              <svg aria-hidden="true" className="w-10 h-10 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <p>Select a time range and click Generate.</p>
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
