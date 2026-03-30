import { useState, useEffect, useCallback } from 'react'
import { useProjectStore } from '../stores/projectStore'

interface FileEntry {
  name: string
  path: string
  is_dir: boolean
}

interface BrowseResponse {
  path: string
  parent: string
  entries: FileEntry[]
  error?: string
}

interface FilePickerProps {
  isOpen: boolean
  onClose: () => void
  onSelect: (path: string) => void
  startPath?: string
}

export function FilePicker({ isOpen, onClose, onSelect, startPath }: FilePickerProps) {
  const { client } = useProjectStore()
  const [currentPath, setCurrentPath] = useState('')
  const [parentPath, setParentPath] = useState('')
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const browse = useCallback(async (path?: string) => {
    if (import.meta.env.DEV) {
      console.log('[FilePicker] browse called, client:', !!client, 'path:', path)
    }
    if (!client) {
      if (import.meta.env.DEV) {
        console.log('[FilePicker] No client, showing not connected')
      }
      setError('Not connected')
      return
    }

    setLoading(true)
    setError(null)
    try {
      const data = await client.call<BrowseResponse>('browse', {
        path: path || '',
        files: true
      })

      if (data.error) {
        setError(data.error)
        return
      }

      setCurrentPath(data.path)
      setParentPath(data.parent)
      setEntries(data.entries || [])
    } catch (err) {
      if (import.meta.env.DEV) {
        console.error('[FilePicker] browse error:', err)
      }
      setError('Failed to browse directory')
    } finally {
      setLoading(false)
    }
  }, [client])

  useEffect(() => {
    if (isOpen) {
      void browse(startPath)
    }
  }, [isOpen, browse, startPath])

  const handleSelect = (entry: FileEntry) => {
    if (entry.is_dir) {
      void browse(entry.path)
    } else {
      onSelect(entry.path)
      onClose()
    }
  }

  const handleGoUp = () => {
    if (parentPath && parentPath !== currentPath) {
      void browse(parentPath)
    }
  }

  if (!isOpen) return null

  // Separate directories and files
  const dirs = entries.filter(e => e.is_dir)
  const files = entries.filter(e => !e.is_dir)

  return (
    <div className="modal modal-open">
      <div className="modal-box bg-base-100 max-w-lg">
        {/* Header */}
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-base-content">Select Task File</h2>
          <button onClick={onClose} className="btn btn-ghost btn-sm btn-circle">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
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
            title="Go up"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
            </svg>
          </button>
          <div className="flex-1 font-mono text-sm text-base-content/80 truncate" title={currentPath}>
            {currentPath || '...'}
          </div>
        </div>

        {/* Entry List */}
        <div className="h-[300px] overflow-auto bg-base-200 rounded-lg">
          {loading ? (
            <div className="flex items-center justify-center h-full">
              <span className="loading loading-spinner loading-md text-primary"></span>
            </div>
          ) : error ? (
            <div className="flex items-center justify-center h-full text-error">
              {error}
            </div>
          ) : entries.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-base-content/60 p-4 text-center">
              <svg className="w-8 h-8 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <span className="font-medium mb-1">No task files found</span>
              <span className="text-xs mb-2">
                in <code className="bg-base-300 px-1 rounded">{currentPath || 'project root'}</code>
              </span>
              <span className="text-xs">
                Create a <code className="bg-base-300 px-1 rounded">.md</code> file to define a task,
                or use <strong>Quick Task</strong> to start without a file.
              </span>
            </div>
          ) : (
            <ul className="p-2 space-y-1">
              {/* Directories first */}
              {dirs.map((entry) => (
                <li key={entry.path}>
                  <button
                    onClick={() => handleSelect(entry)}
                    className="w-full flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-base-300 transition-colors text-left group"
                  >
                    <svg className="w-5 h-5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                    </svg>
                    <span className="text-base-content/80 group-hover:text-base-content transition-colors truncate">
                      {entry.name}
                    </span>
                    <svg className="w-4 h-4 text-base-content/40 group-hover:text-base-content/60 ml-auto transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                    </svg>
                  </button>
                </li>
              ))}

              {/* Files */}
              {files.map((entry) => (
                <li key={entry.path}>
                  <button
                    onClick={() => handleSelect(entry)}
                    className="w-full flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-primary/20 transition-colors text-left group border border-transparent hover:border-primary/30"
                  >
                    <svg className="w-5 h-5 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                    </svg>
                    <span className="text-base-content group-hover:text-base-content transition-colors truncate font-medium">
                      {entry.name}
                    </span>
                    <span className="text-xs text-base-content/40 ml-auto">.md</span>
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
        </div>
      </div>
      <button type="button" className="modal-backdrop bg-black/60" onClick={onClose} aria-label="Close dialog" tabIndex={-1} />
    </div>
  )
}
