import { useState } from 'react'
import { COPY_FEEDBACK_MS } from '../lib/constants'
import { useProjectStore } from '../stores/projectStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface ChangelogPanelProps {
  isOpen: boolean
  onClose: () => void
}

export function ChangelogPanel({ isOpen, onClose }: ChangelogPanelProps) {
  const { client, connected } = useProjectStore()

  const [source, setSource] = useState('')
  const [target, setTarget] = useState('')
  const [full, setFull] = useState(false)
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const generate = async () => {
    if (!client || !connected || !source || !target) return

    setLoading(true)
    setError(null)
    setContent('')

    try {
      const result = await client.call<{ markdown: string; entries: unknown[] }>('changelog.generate', {
        source,
        target,
        full,
      })
      setContent(result.markdown || '')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate changelog')
    } finally {
      setLoading(false)
    }
  }

  const copyToClipboard = async () => {
    try {
      await navigator.clipboard.writeText(content)
      setCopied(true)
      setTimeout(() => setCopied(false), COPY_FEEDBACK_MS)
    } catch (err) {
      console.error('Failed to copy to clipboard:', err)
    }
  }

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Changelog" size="4xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Controls */}
        <div className="flex flex-wrap items-center gap-2 mb-4">
          <input
            type="text"
            value={source}
            onChange={(e) => setSource(e.target.value)}
            placeholder="Source (tag, branch, SHA)"
            className="input input-bordered input-sm w-48"
            aria-label="Source ref"
          />
          <span className="text-base-content/50 text-sm">..</span>
          <input
            type="text"
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            placeholder="Target (tag, branch, SHA)"
            className="input input-bordered input-sm w-48"
            aria-label="Target ref"
          />

          <label className="label cursor-pointer gap-1.5">
            <input
              type="checkbox"
              checked={full}
              onChange={(e) => setFull(e.target.checked)}
              className="checkbox checkbox-sm"
            />
            <span className="label-text text-xs">Full descriptions</span>
          </label>

          <button
            className="btn btn-sm btn-primary"
            onClick={generate}
            disabled={loading || !connected || !source || !target}
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
                />
              </svg>
            )}
            Generate
          </button>

          {content && (
            <button className="btn btn-sm btn-ghost" onClick={copyToClipboard} aria-label="Copy to clipboard">
              {copied ? (
                <svg
                  aria-hidden="true"
                  className="w-4 h-4 text-success"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
              ) : (
                <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                  />
                </svg>
              )}
              {copied ? 'Copied!' : 'Copy'}
            </button>
          )}
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
            <pre className="text-xs whitespace-pre-wrap font-mono bg-base-200 p-3 rounded-lg">{content}</pre>
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
                  d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                />
              </svg>
              <p>Enter source and target refs, then click Generate.</p>
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
