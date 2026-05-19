import { useState } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { EmptyState } from './EmptyState'

interface CheckpointsWidgetProps {
  embedded?: boolean
}

export function CheckpointsWidget({ embedded = false }: CheckpointsWidgetProps) {
  const { checkpoints, redoStack, goToCheckpoint, undo, redo, loading } = useProjectStore()

  const hasCheckpoints = checkpoints.length > 0 || redoStack.length > 0

  const content = (
    <div>
      {!hasCheckpoints ? (
        <EmptyState
          title="No checkpoints yet"
          description="Checkpoints are created during planning and implementation"
          icon="🕐"
        />
      ) : (
        <>
          {/* Checkpoint Timeline */}
          <div className="space-y-2 mb-4 max-h-[300px] overflow-auto">
            {checkpoints.map((cp, i) => (
              <CheckpointEntry
                key={cp.sha}
                checkpoint={cp}
                index={checkpoints.length - i}
                loading={loading}
                onGoTo={() => goToCheckpoint(cp.sha)}
              />
            ))}
          </div>

          {/* Quick Actions */}
          <div className="flex gap-2 pt-4 border-t border-base-300">
            <button
              onClick={() => undo()}
              disabled={checkpoints.length === 0 || loading}
              className="btn btn-ghost flex-1 btn-sm"
            >
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6"
                />
              </svg>
              Undo ({checkpoints.length})
            </button>
            <button
              onClick={() => redo()}
              disabled={redoStack.length === 0 || loading}
              className="btn btn-ghost flex-1 btn-sm"
            >
              Redo ({redoStack.length})
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M21 10h-10a8 8 0 00-8 8v2m18-10l-6 6m6-6l-6-6"
                />
              </svg>
            </button>
          </div>
        </>
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
              d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          Checkpoints
        </h2>
        <div className="mt-4">{content}</div>
      </div>
    </section>
  )
}

function CheckpointEntry({
  checkpoint,
  index,
  loading,
  onGoTo,
}: {
  checkpoint: { sha: string; message: string; timestamp: string; state?: string }
  index: number
  loading: boolean
  onGoTo: () => void
}) {
  const [previewDiff, setPreviewDiff] = useState<string | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const previewCheckpoint = useProjectStore((s) => s.previewCheckpoint)

  const handlePreview = async (e: React.MouseEvent) => {
    e.stopPropagation()
    if (previewDiff !== null) {
      setPreviewDiff(null)
      return
    }
    setPreviewLoading(true)
    const result = await previewCheckpoint(checkpoint.sha)
    setPreviewDiff(result?.diff || 'No differences')
    setPreviewLoading(false)
  }

  return (
    <div className="rounded-lg bg-base-300 border border-transparent hover:border-primary/30 transition-all duration-150">
      <div className="flex items-center gap-3 p-3">
        <div className="w-6 h-6 rounded-full bg-primary/20 flex items-center justify-center text-primary text-xs font-semibold">
          {index}
        </div>
        <button
          onClick={onGoTo}
          disabled={loading}
          aria-label={`Go to checkpoint ${index}: ${checkpoint.message || checkpoint.sha.slice(0, 8)}`}
          className="flex-1 min-w-0 text-left group"
        >
          <div className="font-mono text-sm text-base-content/80 group-hover:text-base-content transition-colors">
            {checkpoint.sha.slice(0, 8)}
            {checkpoint.state && <span className="ml-2 badge badge-xs badge-ghost">{checkpoint.state}</span>}
          </div>
          {checkpoint.message && <div className="text-xs text-base-content/60 truncate">{checkpoint.message}</div>}
        </button>
        <button
          onClick={handlePreview}
          disabled={previewLoading}
          className="btn btn-ghost btn-xs"
          title="Preview diff"
          aria-label="Preview diff"
        >
          {previewLoading ? (
            <span className="loading loading-spinner loading-xs" />
          ) : (
            <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
              />
            </svg>
          )}
        </button>
      </div>
      {previewDiff !== null && (
        <div className="border-t border-base-200 px-3 pb-3">
          <pre className="text-xs font-mono whitespace-pre-wrap max-h-48 overflow-auto mt-2 text-base-content/70">
            {previewDiff}
          </pre>
        </div>
      )}
    </div>
  )
}
