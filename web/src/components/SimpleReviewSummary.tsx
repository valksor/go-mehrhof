import { lazy, Suspense, useState, useEffect, useMemo } from 'react'
import { DiffMethod } from 'react-diff-viewer-continued'
import { useProjectStore } from '../stores/projectStore'
import { useThemeStore } from '../stores/themeStore'
import { parseDiffForFile } from '../lib/diff'

const ReactDiffViewer = lazy(() => import('react-diff-viewer-continued').then((m) => ({ default: m.default })))

const STATUS_ICONS: Record<string, { icon: string; color: string }> = {
  added: { icon: '+', color: 'text-success' },
  modified: { icon: '~', color: 'text-warning' },
  deleted: { icon: '-', color: 'text-error' },
  renamed: { icon: '>', color: 'text-info' },
}

interface SimpleReviewSummaryProps {
  onReview: () => void
  onRequestChanges: () => void
  loading?: boolean
}

export function SimpleReviewSummary({ onReview, onRequestChanges, loading }: SimpleReviewSummaryProps) {
  const fileChanges = useProjectStore((s) => s.fileChanges)
  const getGitDiff = useProjectStore((s) => s.getGitDiff)
  const theme = useThemeStore((s) => s.theme)
  const isDark = theme === 'dark'
  const [expandedFile, setExpandedFile] = useState<string | null>(null)
  const [fullDiff, setFullDiff] = useState<string | null>(null)
  const [diffError, setDiffError] = useState<string | null>(null)

  // Fetch full diff when a file is expanded
  useEffect(() => {
    if (!expandedFile) return
    let cancelled = false
    getGitDiff()
      .then((diff) => {
        if (!cancelled) setFullDiff(diff)
      })
      .catch((err) => {
        if (!cancelled) setDiffError(err instanceof Error ? err.message : 'Failed to load diff')
      })
    return () => {
      cancelled = true
    }
  }, [expandedFile, getGitDiff])

  const counts = useMemo(
    () => ({
      added: fileChanges.filter((f) => f.status === 'added').length,
      modified: fileChanges.filter((f) => f.status === 'modified').length,
      deleted: fileChanges.filter((f) => f.status === 'deleted').length,
    }),
    [fileChanges],
  )

  const fileDiff = expandedFile && fullDiff ? parseDiffForFile(fullDiff, expandedFile) : null

  return (
    <div className="space-y-4">
      {/* Summary stats */}
      <div className="flex gap-4 text-sm">
        {counts.added > 0 && <span className="text-success font-medium">+{counts.added} added</span>}
        {counts.modified > 0 && <span className="text-warning font-medium">~{counts.modified} modified</span>}
        {counts.deleted > 0 && <span className="text-error font-medium">-{counts.deleted} deleted</span>}
        {fileChanges.length === 0 && (
          <span className="text-warning">No file changes detected — use chat to ask the AI to fix the issue</span>
        )}
      </div>

      {/* File list with expandable diffs */}
      {fileChanges.length > 0 && (
        <ul className="space-y-1">
          {fileChanges.map((fc) => {
            const info = STATUS_ICONS[fc.status] ?? { icon: '?', color: 'text-base-content/50' }
            const fileName = fc.path.split('/').pop() || fc.path
            const isExpanded = expandedFile === fc.path
            return (
              <li key={fc.path}>
                <button
                  onClick={() => {
                    const next = isExpanded ? null : fc.path
                    if (next) {
                      setDiffError(null)
                      setFullDiff(null)
                    }
                    setExpandedFile(next)
                  }}
                  className="flex items-center gap-2 text-sm py-1 px-2 w-full text-left rounded hover:bg-base-200 transition-colors cursor-pointer"
                >
                  <span className={`font-mono font-bold w-4 text-center ${info.color}`}>{info.icon}</span>
                  <span className="text-base-content/80 truncate" title={fc.path}>
                    {fileName}
                  </span>
                  <span className="text-base-content/30 text-xs truncate hidden sm:inline flex-1">{fc.path}</span>
                  <span className="text-base-content/30 text-xs ml-auto">{isExpanded ? '▾' : '▸'}</span>
                </button>
                {isExpanded && (
                  <div className="mt-1 mb-2 rounded-lg overflow-hidden border border-base-300 text-xs">
                    {diffError ? (
                      <div className="flex items-center justify-center h-16 text-error text-sm">{diffError}</div>
                    ) : fileDiff ? (
                      <Suspense
                        fallback={
                          <div className="flex items-center justify-center h-16">
                            <span className="loading loading-spinner loading-sm" />
                          </div>
                        }
                      >
                        <ReactDiffViewer
                          oldValue={fileDiff.oldValue}
                          newValue={fileDiff.newValue}
                          splitView={false}
                          compareMethod={DiffMethod.WORDS}
                          useDarkTheme={isDark}
                          hideLineNumbers={false}
                          styles={{
                            variables: {
                              dark: {
                                diffViewerBackground: 'var(--fallback-b1,oklch(var(--b1)/1))',
                                addedBackground: 'oklch(var(--su)/0.2)',
                                removedBackground: 'oklch(var(--er)/0.2)',
                                wordAddedBackground: 'oklch(var(--su)/0.4)',
                                wordRemovedBackground: 'oklch(var(--er)/0.4)',
                                addedGutterBackground: 'oklch(var(--su)/0.3)',
                                removedGutterBackground: 'oklch(var(--er)/0.3)',
                                gutterBackground: 'var(--fallback-b2,oklch(var(--b2)/1))',
                                gutterBackgroundDark: 'var(--fallback-b3,oklch(var(--b3)/1))',
                                codeFoldGutterBackground: 'var(--fallback-b2,oklch(var(--b2)/1))',
                                codeFoldBackground: 'var(--fallback-b2,oklch(var(--b2)/1))',
                              },
                              light: {
                                diffViewerBackground: 'var(--fallback-b1,oklch(var(--b1)/1))',
                                addedBackground: 'oklch(var(--su)/0.15)',
                                removedBackground: 'oklch(var(--er)/0.15)',
                                wordAddedBackground: 'oklch(var(--su)/0.3)',
                                wordRemovedBackground: 'oklch(var(--er)/0.3)',
                                addedGutterBackground: 'oklch(var(--su)/0.2)',
                                removedGutterBackground: 'oklch(var(--er)/0.2)',
                                gutterBackground: 'var(--fallback-b1,oklch(var(--b1)/1))',
                                gutterBackgroundDark: 'var(--fallback-b2,oklch(var(--b2)/1))',
                                codeFoldGutterBackground: 'var(--fallback-b1,oklch(var(--b1)/1))',
                                codeFoldBackground: 'var(--fallback-b1,oklch(var(--b1)/1))',
                              },
                            },
                          }}
                        />
                      </Suspense>
                    ) : fullDiff ? (
                      <div className="flex items-center justify-center h-16 text-base-content/50 text-sm">
                        No diff available for this file
                      </div>
                    ) : (
                      <div className="flex items-center justify-center h-16">
                        <span className="loading loading-spinner loading-sm" />
                      </div>
                    )}
                  </div>
                )}
              </li>
            )
          })}
        </ul>
      )}

      {/* Actions */}
      <div className="flex gap-2">
        <button onClick={onReview} className="btn btn-primary btn-sm" disabled={fileChanges.length === 0 || loading}>
          {loading ? <span className="loading loading-spinner loading-xs"></span> : 'Approve & Submit'}
        </button>
        <button onClick={onRequestChanges} className="btn btn-ghost btn-sm" disabled={loading}>
          Request Changes
        </button>
      </div>
    </div>
  )
}
