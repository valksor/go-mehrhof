import { useState, useCallback, useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface BackupPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface BackupResult {
  path: string
  size: number
  files: number
}

interface RestoreResult {
  target: string
  files: number
  dirs: number
  skipped: number
}

interface BackupInfo {
  name: string
  path: string
  size: number
  created_at: string
}

export function BackupPanel({ isOpen, onClose }: BackupPanelProps) {
  const { client, connected } = useGlobalStore()

  const [backups, setBackups] = useState<BackupInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [restoring, setRestoring] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [lastResult, setLastResult] = useState<BackupResult | null>(null)
  const [restoreResult, setRestoreResult] = useState<RestoreResult | null>(null)

  const loadBackups = useCallback(async () => {
    if (!client) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<{ backups: BackupInfo[] }>('backup.list')
      setBackups(result.backups || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to list backups')
    } finally {
      setLoading(false)
    }
  }, [client])

  // Load backups when panel opens
  useEffect(() => {
    if (isOpen && connected) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
      void loadBackups()
    }
  }, [isOpen, connected, loadBackups])

  const handleCreate = useCallback(async () => {
    if (!client) return

    setCreating(true)
    setError(null)
    setLastResult(null)

    try {
      const result = await client.call<BackupResult>('backup.create')
      setLastResult(result)
      // Refresh the list
      await loadBackups()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create backup')
    } finally {
      setCreating(false)
    }
  }, [client, loadBackups])

  const handleRestore = useCallback(async (archivePath: string) => {
    if (!client) return

    setRestoring(archivePath)
    setError(null)
    setRestoreResult(null)

    try {
      const result = await client.call<RestoreResult>('backup.restore', {
        archive_path: archivePath,
      })
      setRestoreResult(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to restore backup')
    } finally {
      setRestoring(null)
    }
  }, [client])

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
  }

  const formatDate = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    } catch {
      return dateStr
    }
  }

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Backup" size="2xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Create backup button */}
        <div className="flex items-center justify-between mb-4">
          <p className="text-sm text-base-content/60">
            Create and manage backups of kvelmo state
          </p>
          <button
            onClick={handleCreate}
            disabled={creating || !connected}
            className="btn btn-primary btn-sm"
          >
            {creating ? (
              <span className="loading loading-spinner loading-sm"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
              </svg>
            )}
            Create Backup
          </button>
        </div>

        {/* Last backup result */}
        {lastResult && (
          <div className="alert alert-success py-2 mb-4">
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
            <div className="text-sm">
              <p className="font-medium">Backup created successfully</p>
              <p className="text-xs opacity-80 font-mono mt-1">{lastResult.path}</p>
              <p className="text-xs opacity-80">{formatBytes(lastResult.size)} ({lastResult.files} files)</p>
            </div>
          </div>
        )}

        {/* Restore result */}
        {restoreResult && (
          <div className="alert alert-success py-2 mb-4">
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
            <div className="text-sm">
              <p className="font-medium">Backup restored successfully</p>
              <p className="text-xs opacity-80 font-mono mt-1">{restoreResult.target}</p>
              <p className="text-xs opacity-80">{restoreResult.files} files, {restoreResult.dirs} directories{restoreResult.skipped > 0 ? `, ${restoreResult.skipped} skipped` : ''}</p>
            </div>
          </div>
        )}

        {/* Error */}
        {error && (
          <div className="alert alert-error py-2 mb-4">
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Backups list */}
        <div className="flex-1 overflow-y-auto">
          <h3 className="text-sm font-medium text-base-content mb-2">Existing Backups</h3>
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : backups.length === 0 ? (
            <div className="text-center py-8 text-base-content/50">
              <svg aria-hidden="true" className="w-10 h-10 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
              </svg>
              <p>No backups found</p>
              <p className="text-xs mt-2 text-base-content/40">Click "Create Backup" to create your first backup</p>
            </div>
          ) : (
            <div className="space-y-2">
              {backups.map((b) => (
                <div key={b.path} className="p-3 rounded-lg bg-base-200 border border-base-300">
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-sm text-base-content truncate">{b.name}</span>
                    <div className="flex items-center gap-2 flex-shrink-0">
                      <span className="text-xs text-base-content/50">{formatBytes(b.size)}</span>
                      <span className="text-xs text-base-content/40">{formatDate(b.created_at)}</span>
                      <button
                        onClick={() => handleRestore(b.path)}
                        disabled={restoring !== null || !connected}
                        className="btn btn-xs btn-outline btn-warning"
                        title={`Restore from ${b.name}`}
                      >
                        {restoring === b.path ? (
                          <span className="loading loading-spinner loading-xs"></span>
                        ) : (
                          <svg aria-hidden="true" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                          </svg>
                        )}
                        Restore
                      </button>
                    </div>
                  </div>
                  <p className="text-xs text-base-content/40 font-mono truncate mt-1">{b.path}</p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
