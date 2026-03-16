import { lazy, Suspense, useState, useEffect } from 'react'
import { useLayoutStore } from '../stores/layoutStore'

// Import enum for type safety; the heavy component is lazy-loaded
import { DiffMethod } from 'react-diff-viewer-continued'

// Lazy-load heavy diff viewer component (only needed when viewing diffs)
const ReactDiffViewer = lazy(() => import('react-diff-viewer-continued').then(m => ({ default: m.default })))
import { useProjectStore } from '../stores/projectStore'
import { OutputWidget } from './OutputWidget'
import { ScreenshotsPanel } from './ScreenshotsPanel'
import { JobsPanel } from './JobsPanel'
import { FileBrowserWidget } from './FileBrowserWidget'
import { TaskPanel } from './TaskPanel'
import { FileChangesPanel } from './FileChangesPanel'

// Lazy-loaded heavy tab panels
const ChatWidget = lazy(() => import('./ChatWidget').then(m => ({ default: m.ChatWidget })))
const BrowserPanel = lazy(() => import('./BrowserPanel').then(m => ({ default: m.BrowserPanel })))
const ReviewPanel = lazy(() => import('./ReviewPanel').then(m => ({ default: m.ReviewPanel })))

const LazyFallback = <div className="flex items-center justify-center h-32"><span className="loading loading-spinner loading-sm" /></div>

interface TabPanelProps {
  className?: string
}

export function TabPanel({ className = '' }: TabPanelProps) {
  const { tabs, activeTabId } = useLayoutStore()
  const activeTab = tabs.find((t) => t.id === activeTabId)

  if (!activeTab) {
    return (
      <div className={`flex items-center justify-center h-full text-base-content/50 ${className}`}>
        <div className="text-center">
          <svg className="w-12 h-12 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <p className="text-sm">No tab selected</p>
        </div>
      </div>
    )
  }

  return (
    <div className={`h-full overflow-hidden ${className}`}>
      {activeTab.type === 'output' && <OutputContent />}
      {activeTab.type === 'file' && <FileContent data={activeTab.data} />}
      {activeTab.type === 'diff' && <DiffContent data={activeTab.data} />}
      {activeTab.type === 'spec' && <SpecContent data={activeTab.data} />}
      {activeTab.type === 'agent' && <AgentContent data={activeTab.data} />}
      {activeTab.type === 'chat' && <ChatContent />}
      {activeTab.type === 'screenshots' && <ScreenshotsContent />}
      {activeTab.type === 'jobs' && <JobsContent />}
      {activeTab.type === 'files' && <FilesContent />}
      {activeTab.type === 'browser' && <BrowserContent />}
      {activeTab.type === 'task' && <TaskContent data={activeTab.data} />}
      {activeTab.type === 'review' && <ReviewContent data={activeTab.data} />}
      {activeTab.type === 'filechanges' && <FileChangesContent data={activeTab.data} />}
    </div>
  )
}

// Tab content components
function OutputContent() {
  return (
    <div className="h-full">
      <OutputWidget embedded />
    </div>
  )
}

function FileContent({ data }: { data?: Record<string, unknown> }) {
  const filePath = data?.path as string | undefined

  return (
    <div className="h-full p-4 overflow-auto">
      <div className="font-mono text-sm">
        {filePath ? (
          <div>
            <div className="text-base-content/60 mb-2">{filePath}</div>
            <pre className="bg-neutral text-neutral-content p-4 rounded-lg overflow-auto">
              {data?.content as string || 'Loading file content...'}
            </pre>
          </div>
        ) : (
          <div className="text-base-content/50">No file selected</div>
        )}
      </div>
    </div>
  )
}

function DiffContent({ data }: { data?: Record<string, unknown> }) {
  const filePath = data?.path as string | undefined
  const getGitDiff = useProjectStore((state) => state.getGitDiff)
  const [diff, setDiff] = useState<{ oldValue: string; newValue: string } | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [splitView, setSplitView] = useState(true)

  // Reactive theme state via MutationObserver
  const [isDarkTheme, setIsDarkTheme] = useState(() => {
    return document.documentElement.getAttribute('data-theme') === 'business'
  })

  // Watch for theme changes
  useEffect(() => {
    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (mutation.attributeName === 'data-theme') {
          const theme = document.documentElement.getAttribute('data-theme')
          setIsDarkTheme(theme === 'business')
        }
      }
    })

    observer.observe(document.documentElement, { attributes: true })
    return () => observer.disconnect()
  }, [])

  // Fetch diff with cancellation guard
  useEffect(() => {
    if (!filePath) return

    let cancelled = false
    // Schedule state updates outside synchronous effect execution
    queueMicrotask(() => {
      if (cancelled) return
      setLoading(true)
      setError(null)
    })

    getGitDiff(false)
      .then((fullDiff) => {
        if (cancelled) return
        const parsed = parseDiffForFile(fullDiff, filePath)
        setDiff(parsed)
      })
      .catch((err) => {
        if (cancelled) return
        setError(err instanceof Error ? err.message : 'Failed to load diff')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [filePath, getGitDiff])

  if (!filePath) {
    return (
      <div className="flex items-center justify-center h-full text-base-content/50">
        <p className="text-sm">No file selected</p>
      </div>
    )
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <span className="loading loading-spinner loading-md"></span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full text-error">
        <p className="text-sm">{error}</p>
      </div>
    )
  }

  return (
    <div className="h-full overflow-auto">
      <div className="sticky top-0 z-10 bg-base-100 border-b border-base-300 px-4 py-2 flex items-center justify-between">
        <div>
          <span className="text-sm font-medium">{filePath}</span>
          <span className="text-xs text-base-content/50 ml-2">
            ({data?.status as string || 'modified'})
          </span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setSplitView(true)}
            className={`btn btn-xs ${splitView ? 'btn-primary' : 'btn-ghost'}`}
            aria-pressed={splitView}
            aria-label="Split view"
          >
            Split
          </button>
          <button
            onClick={() => setSplitView(false)}
            className={`btn btn-xs ${!splitView ? 'btn-primary' : 'btn-ghost'}`}
            aria-pressed={!splitView}
            aria-label="Unified view"
          >
            Unified
          </button>
        </div>
      </div>
      {diff ? (
        <Suspense fallback={<div className="flex items-center justify-center h-32"><span className="loading loading-spinner loading-sm" /></div>}>
        <ReactDiffViewer
          oldValue={diff.oldValue}
          newValue={diff.newValue}
          splitView={splitView}
          compareMethod={DiffMethod.WORDS}
          useDarkTheme={isDarkTheme}
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
                gutterBackground: 'var(--fallback-b2,oklch(var(--b2)/1))',
                gutterBackgroundDark: 'var(--fallback-b3,oklch(var(--b3)/1))',
                codeFoldGutterBackground: 'var(--fallback-b2,oklch(var(--b2)/1))',
                codeFoldBackground: 'var(--fallback-b2,oklch(var(--b2)/1))',
              },
            },
          }}
        />
        </Suspense>
      ) : (
        <div className="flex items-center justify-center h-64 text-base-content/50">
          <p className="text-sm">No changes to display</p>
        </div>
      )}
    </div>
  )
}

// Parse unified diff to extract old/new content for a specific file
function parseDiffForFile(fullDiff: string, filePath: string): { oldValue: string; newValue: string } | null {
  if (!fullDiff) return null

  // Find the section for this file in the unified diff
  const lines = fullDiff.split('\n')
  let inTargetFile = false
  let oldLines: string[] = []
  let newLines: string[] = []

  for (const line of lines) {
    // Check for file header (diff --git a/path b/path)
    if (line.startsWith('diff --git')) {
      // Parse exact paths from "diff --git a/path b/path" header
      // Match pattern: diff --git a/<path> b/<path>
      const match = line.match(/^diff --git a\/(.+?) b\/(.+)$/)
      if (match) {
        const [, leftPath, rightPath] = match
        // Exact match instead of includes()
        inTargetFile = leftPath === filePath || rightPath === filePath
      } else {
        inTargetFile = false
      }
      if (inTargetFile) {
        oldLines = []
        newLines = []
      }
      continue
    }

    if (!inTargetFile) continue

    // Skip metadata lines
    if (line.startsWith('index ') || line.startsWith('--- ') || line.startsWith('+++ ') || line.startsWith('@@')) {
      continue
    }

    // Parse diff content
    if (line.startsWith('-')) {
      oldLines.push(line.slice(1))
    } else if (line.startsWith('+')) {
      newLines.push(line.slice(1))
    } else if (line.startsWith(' ')) {
      // Context line - appears in both
      oldLines.push(line.slice(1))
      newLines.push(line.slice(1))
    }
  }

  if (oldLines.length === 0 && newLines.length === 0) {
    return null
  }

  return {
    oldValue: oldLines.join('\n'),
    newValue: newLines.join('\n'),
  }
}

function SpecContent({ data }: { data?: Record<string, unknown> }) {
  const loadSpec = useProjectStore(s => s.loadSpec)
  const loadPlan = useProjectStore(s => s.loadPlan)
  const connected = useProjectStore(s => s.connected)
  const mode = (data?.mode as string) || 'spec'

  const [specs, setSpecs] = useState<Array<{ path: string; content: string }>>([])
  const [loading, setLoading] = useState(false)
  const [expandedPath, setExpandedPath] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!connected) return

    let cancelled = false

    async function fetchSpecs() {
      setLoading(true)
      setError(null)
      setSpecs([])

      const loader = mode === 'plan' ? loadPlan : loadSpec
      try {
        const result = await loader()
        if (!cancelled) {
          setSpecs(result)
          if (result.length === 1) {
            setExpandedPath(result[0].path)
          }
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    fetchSpecs()
    return () => { cancelled = true }
  }, [connected, mode, loadSpec, loadPlan])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <span className="loading loading-spinner loading-md"></span>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full text-error">
        <p className="text-sm">{error}</p>
      </div>
    )
  }

  if (specs.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-base-content/50">
        <div className="text-center">
          <svg className="w-10 h-10 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <p className="text-sm">No {mode === 'plan' ? 'plan' : 'specification'} available</p>
        </div>
      </div>
    )
  }

  return (
    <div className="h-full overflow-auto">
      <div className="sticky top-0 z-10 bg-base-100 border-b border-base-300 px-4 py-2">
        <span className="text-sm font-medium">
          {mode === 'plan' ? 'Plan' : 'Specification'} ({specs.length} file{specs.length !== 1 ? 's' : ''})
        </span>
      </div>
      <div className="p-4 space-y-3">
        {specs.map(spec => {
          const fileName = spec.path.split('/').pop() || spec.path
          const isExpanded = expandedPath === spec.path || specs.length === 1

          return (
            <div key={spec.path} className="rounded-lg bg-base-200 border border-base-300 overflow-hidden">
              <button
                className="w-full px-4 py-2 text-left hover:bg-base-300/50 transition-colors flex items-center justify-between"
                onClick={() => setExpandedPath(isExpanded ? null : spec.path)}
              >
                <span className="text-sm font-mono">{fileName}</span>
                <svg
                  className={`w-4 h-4 text-base-content/40 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
                  fill="none" viewBox="0 0 24 24" stroke="currentColor"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </button>
              {isExpanded && (
                <div className="border-t border-base-300 px-4 py-3">
                  <pre className="text-sm text-base-content/80 whitespace-pre-wrap leading-relaxed">{spec.content}</pre>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function AgentContent({ data }: { data?: Record<string, unknown> }) {
  const agentId = data?.agentId as string | undefined
  const agentName = data?.name as string | undefined

  return (
    <div className="h-full p-4 overflow-auto">
      <div className="space-y-4">
        {agentName ? (
          <>
            <div className="flex items-center gap-3">
              <span className="status-dot status-running" />
              <span className="font-medium">{agentName}</span>
              <span className="text-xs text-base-content/50">ID: {agentId}</span>
            </div>
            <div className="bg-neutral text-neutral-content p-4 rounded-lg font-mono text-sm">
              Agent output will appear here...
            </div>
          </>
        ) : (
          <div className="text-base-content/50">No agent selected</div>
        )}
      </div>
    </div>
  )
}

function ChatContent() {
  const setModalCommand = useLayoutStore(s => s.setModalCommand)
  return (
    <div className="h-full">
      <Suspense fallback={LazyFallback}>
        <ChatWidget embedded onModalCommand={setModalCommand} />
      </Suspense>
    </div>
  )
}

function ScreenshotsContent() {
  return (
    <div className="h-full">
      <ScreenshotsPanel />
    </div>
  )
}

function JobsContent() {
  return (
    <div className="h-full">
      <JobsPanel />
    </div>
  )
}

function FilesContent() {
  return (
    <div className="h-full">
      <FileBrowserWidget />
    </div>
  )
}

function BrowserContent() {
  return (
    <div className="h-full">
      <Suspense fallback={LazyFallback}>
        <BrowserPanel />
      </Suspense>
    </div>
  )
}

function TaskContent({ data }: { data?: Record<string, unknown> }) {
  return (
    <div className="h-full">
      <TaskPanel data={data} />
    </div>
  )
}

function ReviewContent({ data }: { data?: Record<string, unknown> }) {
  return (
    <div className="h-full">
      <Suspense fallback={LazyFallback}>
        <ReviewPanel data={data} />
      </Suspense>
    </div>
  )
}

function FileChangesContent({ data }: { data?: Record<string, unknown> }) {
  return (
    <div className="h-full">
      <FileChangesPanel data={data} />
    </div>
  )
}
