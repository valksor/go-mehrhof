import { useState } from 'react'
import type { ReactNode } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useLayoutStore } from '../stores/layoutStore'

interface FileChange {
  path: string
  status: 'added' | 'modified' | 'deleted' | 'renamed'
}

interface FileChangesPanelProps {
  data?: Record<string, unknown>
}

export function FileChangesPanel({ data }: FileChangesPanelProps) {
  const fileChanges = useProjectStore((state) => state.fileChanges)
  const getGitDiffAgainst = useProjectStore((state) => state.getGitDiffAgainst)
  const openTab = useLayoutStore((state) => state.openTab)

  const [diffStat, setDiffStat] = useState<string | null>(null)
  const [diffLoading, setDiffLoading] = useState(false)

  // Use data from props if available (with runtime check), otherwise from store
  const displayChanges = Array.isArray(data?.fileChanges) ? (data.fileChanges as FileChange[]) : fileChanges

  const handleDiffAgainstBase = async () => {
    setDiffLoading(true)
    try {
      const stat = await getGitDiffAgainst('HEAD~1', true)
      setDiffStat(stat || 'No differences')
    } catch {
      setDiffStat('Could not compare')
    } finally {
      setDiffLoading(false)
    }
  }

  if (displayChanges.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-base-content/50">
        <div className="text-center">
          <svg className="w-12 h-12 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
            />
          </svg>
          <p className="text-sm">No file changes</p>
        </div>
      </div>
    )
  }

  // Group files by status
  const grouped = {
    added: displayChanges.filter((f) => f.status === 'added'),
    modified: displayChanges.filter((f) => f.status === 'modified'),
    deleted: displayChanges.filter((f) => f.status === 'deleted'),
    renamed: displayChanges.filter((f) => f.status === 'renamed'),
  }

  const handleFileClick = (fc: FileChange) => {
    const fileName = fc.path.split('/').pop() || fc.path
    openTab({
      id: `diff-${fc.path}`,
      type: 'diff',
      title: fileName,
      data: { path: fc.path, status: fc.status },
      closeable: true,
    })
  }

  return (
    <div className="h-full overflow-auto p-4">
      <div className="space-y-4">
        {/* Summary Header */}
        <div className="flex items-center gap-4 pb-3 border-b border-base-300">
          <h2 className="text-lg font-semibold">{displayChanges.length} Files Changed</h2>
          <div className="flex gap-2">
            {grouped.added.length > 0 && <span className="badge badge-success badge-sm">+{grouped.added.length}</span>}
            {grouped.modified.length > 0 && (
              <span className="badge badge-warning badge-sm">~{grouped.modified.length}</span>
            )}
            {grouped.deleted.length > 0 && (
              <span className="badge badge-error badge-sm">-{grouped.deleted.length}</span>
            )}
            {grouped.renamed.length > 0 && (
              <span className="badge badge-info badge-sm">{grouped.renamed.length} renamed</span>
            )}
          </div>
          <div className="ml-auto">
            <button
              onClick={handleDiffAgainstBase}
              disabled={diffLoading}
              className="btn btn-ghost btn-xs"
              aria-label="Compare against base commit"
            >
              {diffLoading ? (
                <span className="loading loading-spinner loading-xs"></span>
              ) : (
                <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
                  />
                </svg>
              )}
              Diff stat
            </button>
          </div>
        </div>

        {/* Diff stat output */}
        {diffStat && (
          <div className="bg-base-200 rounded-lg p-3 border border-base-300">
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs font-semibold text-base-content/50 uppercase">Diff stat</span>
              <button
                onClick={() => setDiffStat(null)}
                className="btn btn-ghost btn-xs btn-square"
                aria-label="Close diff stat"
              >
                <svg aria-hidden="true" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <pre className="text-xs font-mono text-base-content/80 whitespace-pre-wrap">{diffStat}</pre>
          </div>
        )}

        {/* File List by Status */}
        {grouped.added.length > 0 && <FileGroup title="Added" files={grouped.added} onFileClick={handleFileClick} />}
        {grouped.modified.length > 0 && (
          <FileGroup title="Modified" files={grouped.modified} onFileClick={handleFileClick} />
        )}
        {grouped.deleted.length > 0 && (
          <FileGroup title="Deleted" files={grouped.deleted} onFileClick={handleFileClick} />
        )}
        {grouped.renamed.length > 0 && (
          <FileGroup title="Renamed" files={grouped.renamed} onFileClick={handleFileClick} />
        )}
      </div>
    </div>
  )
}

function FileGroup({
  title,
  files,
  onFileClick,
}: {
  title: string
  files: FileChange[]
  onFileClick: (fc: FileChange) => void
}) {
  return (
    <div className="space-y-1">
      <h3 className="text-sm font-medium text-base-content/70">{title}</h3>
      <div className="space-y-0.5">
        {files.map((fc) => (
          <FileItem key={fc.path} file={fc} onClick={() => onFileClick(fc)} />
        ))}
      </div>
    </div>
  )
}

function FileItem({ file, onClick }: { file: FileChange; onClick: () => void }) {
  const statusConfig: Record<string, { color: string; icon: ReactNode }> = {
    added: {
      color: 'text-success',
      icon: (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
        </svg>
      ),
    },
    modified: {
      color: 'text-warning',
      icon: (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
          />
        </svg>
      ),
    },
    deleted: {
      color: 'text-error',
      icon: (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
          />
        </svg>
      ),
    },
    renamed: {
      color: 'text-info',
      icon: (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
          />
        </svg>
      ),
    },
  }

  const { color, icon } = statusConfig[file.status] || statusConfig.modified

  // Split path into directory and filename
  const parts = file.path.split('/')
  const fileName = parts.pop() || file.path
  const directory = parts.join('/')

  return (
    <button
      className="w-full flex items-center gap-2 p-2 rounded-lg hover:bg-base-200 transition-colors text-left group"
      onClick={onClick}
    >
      <span className={color}>{icon}</span>
      <span className="flex-1 min-w-0">
        <span className="font-medium">{fileName}</span>
        {directory && <span className="text-base-content/50 text-xs ml-2">{directory}/</span>}
      </span>
      <svg
        className="w-4 h-4 text-base-content/30 group-hover:text-base-content/60 transition-colors"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
      </svg>
    </button>
  )
}
