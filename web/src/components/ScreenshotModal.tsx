import { useEffect } from 'react'
import { type Screenshot, useScreenshotStore } from '../stores/screenshotStore'
import { useProjectStore } from '../stores/projectStore'

interface ScreenshotModalProps {
  screenshot: Screenshot
  onClose: () => void
}

export function ScreenshotModal({ screenshot, onClose }: ScreenshotModalProps) {
  const { attach, detach, attachedIds, deleteScreenshot } = useScreenshotStore()
  const client = useProjectStore(s => s.client)

  const isAttached = attachedIds.includes(screenshot.id)
  const timestamp = new Date(screenshot.timestamp)
  const imageUrl = `/api/screenshots/${screenshot.task_id}/${screenshot.filename}`

  // Close on escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  const handleAttach = () => {
    if (isAttached) {
      detach(screenshot.id)
    } else {
      attach(screenshot.id)
    }
  }

  const handleDelete = () => {
    if (confirm('Delete this screenshot?')) {
      void deleteScreenshot(screenshot.id, client)
      onClose()
    }
  }

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  return (
    // Backdrop: click-to-dismiss is a convenience; main close actions are Escape key and close button
    // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
      onClick={handleBackdropClick}
    >
      <div className="relative max-w-[90vw] max-h-[90vh] bg-base-100 rounded-lg overflow-hidden shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-base-300">
          <div className="flex items-center gap-3">
            <span className={`badge ${screenshot.source === 'agent' ? 'badge-primary' : 'badge-secondary'}`}>
              {screenshot.source === 'agent' ? '🤖 Agent' : '👤 User'}
            </span>
            <span className="text-sm text-base-content/70">
              {timestamp.toLocaleString()}
            </span>
            {screenshot.step && (
              <span className="badge badge-ghost badge-sm">{screenshot.step}</span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={handleAttach}
              className={`btn btn-sm ${isAttached ? 'btn-success' : 'btn-ghost'}`}
            >
              {isAttached ? '✓ Attached' : '📎 Attach'}
            </button>
            <button onClick={handleDelete} className="btn btn-sm btn-ghost text-error">
              🗑️ Delete
            </button>
            <button onClick={onClose} className="btn btn-sm btn-circle btn-ghost">
              ✕
            </button>
          </div>
        </div>

        {/* Image */}
        <div className="overflow-auto max-h-[calc(90vh-120px)]">
          <img
            src={imageUrl}
            alt={screenshot.filename}
            className="max-w-full h-auto"
          />
        </div>

        {/* Footer with metadata */}
        <div className="flex items-center justify-between p-3 bg-base-200 text-xs text-base-content/70">
          <span className="font-mono">{screenshot.filename}</span>
          <div className="flex items-center gap-4">
            <span>{screenshot.width} × {screenshot.height}</span>
            <span>{formatBytes(screenshot.size_bytes)}</span>
            <span className="uppercase">{screenshot.format}</span>
          </div>
        </div>
      </div>
    </div>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}
