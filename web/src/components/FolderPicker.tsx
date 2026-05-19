import { useState, useEffect, useCallback } from 'react'
import { useGlobalStore } from '../stores/globalStore'

interface DirEntry {
  name: string
  path: string
}

interface BrowseResponse {
  path: string
  parent: string
  entries: DirEntry[]
  error?: string
}

interface FolderPickerProps {
  isOpen: boolean
  onClose: () => void
  onSelect: (path: string) => void
}

export function FolderPicker({ isOpen, onClose, onSelect }: FolderPickerProps) {
  const { client } = useGlobalStore()
  const [currentPath, setCurrentPath] = useState('')
  const [parentPath, setParentPath] = useState('')
  const [dirs, setDirs] = useState<DirEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const browse = useCallback(
    async (path?: string) => {
      if (!client) {
        setError('Not connected')
        return
      }

      setLoading(true)
      setError(null)
      try {
        const data = await client.call<BrowseResponse>('browse', { path: path || '' })

        if (data.error) {
          setError(data.error)
          return
        }

        setCurrentPath(data.path)
        setParentPath(data.parent)
        setDirs(data.entries || [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to browse directory')
      } finally {
        setLoading(false)
      }
    },
    [client],
  )

  useEffect(() => {
    if (isOpen) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
      void browse()
    }
  }, [isOpen, browse])

  const handleSelect = () => {
    onSelect(currentPath)
    onClose()
  }

  const handleNavigate = (path: string) => {
    void browse(path)
  }

  const handleGoUp = () => {
    if (parentPath && parentPath !== currentPath) {
      void browse(parentPath)
    }
  }

  if (!isOpen) return null

  return (
    <div className="modal modal-open">
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="folder-picker-title"
        className="modal-box bg-base-100 max-w-lg"
      >
        {/* Header */}
        <div className="flex items-center justify-between mb-4">
          <h2 id="folder-picker-title" className="text-lg font-semibold text-base-content">
            Select Project Folder
          </h2>
          <button onClick={onClose} className="btn btn-ghost btn-sm btn-circle" aria-label="Close dialog">
            <svg aria-hidden="true" className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Current Path */}
        <div className="flex items-center gap-2 p-2 bg-base-200 rounded-lg mb-4">
          <button
            onClick={handleGoUp}
            disabled={!parentPath || parentPath === currentPath || loading}
            className="btn btn-ghost btn-sm btn-square"
            aria-label="Go to parent directory"
          >
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
            </svg>
          </button>
          <div className="flex-1 font-mono text-sm text-base-content/80 truncate" title={currentPath}>
            {currentPath || '...'}
          </div>
        </div>

        {/* Directory List */}
        <div className="h-[300px] overflow-auto bg-base-200 rounded-lg">
          {loading ? (
            <div className="flex items-center justify-center h-full">
              <span className="loading loading-spinner loading-md text-primary"></span>
            </div>
          ) : error ? (
            <div className="flex items-center justify-center h-full text-error">{error}</div>
          ) : dirs.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-base-content/60">
              <svg aria-hidden="true" className="w-8 h-8 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"
                />
              </svg>
              <span>No subfolders</span>
            </div>
          ) : (
            <ul className="p-2 space-y-1">
              {dirs.map((dir) => (
                <li key={dir.path}>
                  <button
                    onClick={() => handleNavigate(dir.path)}
                    className="w-full flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-base-300 transition-colors text-left group"
                  >
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
                        d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"
                      />
                    </svg>
                    <span className="text-base-content/80 group-hover:text-base-content transition-colors truncate">
                      {dir.name}
                    </span>
                    <svg
                      aria-hidden="true"
                      className="w-4 h-4 text-base-content/40 group-hover:text-base-content/60 ml-auto transition-colors"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                    </svg>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Footer */}
        <div className="modal-action">
          <button onClick={onClose} className="btn btn-ghost">
            Cancel
          </button>
          <button onClick={handleSelect} disabled={!currentPath || loading} className="btn btn-primary">
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
            Select This Folder
          </button>
        </div>
      </div>
      <button
        type="button"
        className="modal-backdrop bg-black/60"
        onClick={onClose}
        aria-label="Close dialog"
        tabIndex={-1}
      />
    </div>
  )
}
