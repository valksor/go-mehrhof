import { useState } from 'react'
import { useScreenshotStore, type Screenshot } from '../stores/screenshotStore'
import { useProjectStore } from '../stores/projectStore'
import { ScreenshotThumbnail } from './ScreenshotThumbnail'
import { ScreenshotModal } from './ScreenshotModal'

type FilterSource = 'all' | 'agent' | 'user'

export function ScreenshotsPanel() {
  const { screenshots, loading, error, selectedId, select, captureScreenshot } = useScreenshotStore()
  const client = useProjectStore((s) => s.client)
  const [filter, setFilter] = useState<FilterSource>('all')
  const [modalScreenshot, setModalScreenshot] = useState<Screenshot | null>(null)

  const filteredScreenshots = screenshots.filter((s) => {
    if (filter === 'all') return true
    return s.source === filter
  })

  const handleThumbnailClick = (screenshot: Screenshot) => {
    setModalScreenshot(screenshot)
  }

  const handleCloseModal = () => {
    setModalScreenshot(null)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <span className="loading loading-spinner loading-lg text-primary"></span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-error">
        <svg className="w-12 h-12 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
        <p className="text-sm">{error}</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header with filter */}
      <div className="flex items-center justify-between p-4 border-b border-base-300">
        <h3 className="font-semibold text-base-content flex items-center gap-2">
          <svg className="w-5 h-5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
            />
          </svg>
          Screenshots ({filteredScreenshots.length})
        </h3>
        <div className="flex items-center gap-2">
          <button
            onClick={() => captureScreenshot(client)}
            disabled={loading}
            className="btn btn-xs btn-primary"
            aria-label="Capture screenshot"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"
                />
              </svg>
            )}
            Capture
          </button>
          <div className="join">
            <button
              className={`join-item btn btn-xs ${filter === 'all' ? 'btn-primary' : 'btn-ghost'}`}
              onClick={() => setFilter('all')}
            >
              All
            </button>
            <button
              className={`join-item btn btn-xs ${filter === 'agent' ? 'btn-primary' : 'btn-ghost'}`}
              onClick={() => setFilter('agent')}
            >
              Agent
            </button>
            <button
              className={`join-item btn btn-xs ${filter === 'user' ? 'btn-primary' : 'btn-ghost'}`}
              onClick={() => setFilter('user')}
            >
              User
            </button>
          </div>
        </div>
      </div>

      {/* Gallery grid */}
      <div className="flex-1 overflow-auto p-4">
        {filteredScreenshots.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-base-content/60">
            <svg className="w-16 h-16 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={1.5}
                d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
              />
            </svg>
            <p className="text-sm">No screenshots yet</p>
            <p className="text-xs mt-1">Screenshots will appear here when captured</p>
          </div>
        ) : (
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
            {filteredScreenshots.map((screenshot) => (
              <ScreenshotThumbnail
                key={screenshot.id}
                screenshot={screenshot}
                isSelected={selectedId === screenshot.id}
                onClick={() => handleThumbnailClick(screenshot)}
                onSelect={() => select(screenshot.id)}
              />
            ))}
          </div>
        )}
      </div>

      {/* Modal for full-size view */}
      {modalScreenshot && <ScreenshotModal screenshot={modalScreenshot} onClose={handleCloseModal} />}
    </div>
  )
}
