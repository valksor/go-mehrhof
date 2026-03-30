import { Screenshot, useScreenshotStore } from '../stores/screenshotStore'
import { useProjectStore } from '../stores/projectStore'

interface ScreenshotThumbnailProps {
  screenshot: Screenshot
  isSelected: boolean
  onClick: () => void
  onSelect?: () => void
}

export function ScreenshotThumbnail({
  screenshot,
  isSelected,
  onClick,
}: ScreenshotThumbnailProps) {
  const { attach, attachedIds, detach, deleteScreenshot, getScreenshot, screenshotData } = useScreenshotStore()
  const client = useProjectStore(s => s.client)

  const isAttached = attachedIds.includes(screenshot.id)
  const timestamp = new Date(screenshot.timestamp)
  const timeStr = timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  const dateStr = timestamp.toLocaleDateString([], { month: 'short', day: 'numeric' })

  const handleAttach = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (isAttached) {
      detach(screenshot.id)
    } else {
      attach(screenshot.id)
    }
  }

  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (confirm('Delete this screenshot?')) {
      void deleteScreenshot(screenshot.id, client)
    }
  }

  // Use loaded full-res data if available, otherwise fall back to static URL
  const loadedData = screenshotData[screenshot.id]
  const imageUrl = loadedData || `/api/screenshots/${screenshot.task_id}/${screenshot.filename}`

  const handleClick = async () => {
    // Load full screenshot data on first click
    if (!loadedData) {
      await getScreenshot(screenshot.id, client)
    }
    onClick()
  }

  return (
    <div
      role="button"
      tabIndex={0}
      className={`
        group relative rounded-lg overflow-hidden cursor-pointer
        border-2 transition-all duration-150
        ${isSelected ? 'border-primary ring-2 ring-primary/30' : 'border-transparent hover:border-base-300'}
        ${isAttached ? 'ring-2 ring-success/50' : ''}
      `}
      onClick={handleClick}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); void handleClick() } }}
    >
      {/* Thumbnail image */}
      <div className="aspect-video bg-base-300 relative">
        <img
          src={imageUrl}
          alt={screenshot.filename}
          className="w-full h-full object-cover"
          loading="lazy"
          onError={(e) => {
            // Fallback to placeholder on error
            (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="%23666" stroke-width="1"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg>'
          }}
        />

        {/* Overlay with actions (visible on hover) */}
        <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-2">
          <button
            onClick={handleAttach}
            className={`btn btn-sm ${isAttached ? 'btn-success' : 'btn-ghost text-white hover:btn-primary'}`}
            title={isAttached ? 'Remove from message' : 'Attach to message'}
          >
            {isAttached ? (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
            ) : (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
              </svg>
            )}
          </button>
          <button
            onClick={handleDelete}
            className="btn btn-sm btn-ghost text-white hover:btn-error"
            title="Delete screenshot"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </button>
        </div>

        {/* Source badge */}
        <div className="absolute top-2 left-2">
          <span className={`badge badge-sm ${screenshot.source === 'agent' ? 'badge-primary' : 'badge-secondary'}`}>
            {screenshot.source === 'agent' ? '🤖' : '👤'}
          </span>
        </div>

        {/* Attached indicator */}
        {isAttached && (
          <div className="absolute top-2 right-2">
            <span className="badge badge-sm badge-success">📎</span>
          </div>
        )}
      </div>

      {/* Info bar */}
      <div className="p-2 bg-base-200">
        <div className="flex items-center justify-between text-xs text-base-content/70">
          <span className="font-mono truncate max-w-[60%]" title={screenshot.filename}>
            {screenshot.id}
          </span>
          <span>{timeStr}</span>
        </div>
        <div className="text-xs text-base-content/50 mt-1">
          {dateStr} • {formatBytes(screenshot.size_bytes)}
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
