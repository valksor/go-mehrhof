import { lazy, Suspense, useState, useMemo } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { useProjectStore } from '../stores/projectStore'
import { useLayoutStore } from '../stores/layoutStore'
import { useDebugStore } from '../stores/debugStore'
import { useDocsURL } from '../hooks/useDocsURL'
import { useKeyboardShortcuts } from '../hooks/useKeyboardShortcuts'
import { Widget, TaskIcon, FilesIcon, CheckpointsIcon } from './Widget'
import { ErrorBoundary } from './ErrorBoundary'
import { PanelLayout } from './PanelLayout'
import { TaskWidget } from './TaskWidget'
import { CheckpointsWidget } from './CheckpointsWidget'
import { RecapWidget } from './RecapWidget'
import { PhaseMetricsWidget } from './PhaseMetricsWidget'
import { ChecklistWidget } from './ChecklistWidget'
import { ReviewHistoryWidget } from './ReviewHistoryWidget'
import { FileChangesWidget } from './FileChangesWidget'
import { AgentPanel } from './AgentPanel'
import { TaskQueue } from './TaskQueue'
import { TaskHistory } from './TaskHistory'
import { ThemeToggle } from './ThemeToggle'
import { ViewModeToggle } from './ViewModeToggle'
import { StatusBadge } from './StatusIndicator'
import { WorkflowBar } from './WorkflowBar'

// Lazy-loaded modal components (only rendered when opened)
const Settings = lazy(() => import('./Settings').then(m => ({ default: m.Settings })))
const LogsPanel = lazy(() => import('./LogsPanel').then(m => ({ default: m.LogsPanel })))
const CIStatusPanel = lazy(() => import('./CIStatusPanel').then(m => ({ default: m.CIStatusPanel })))
const PolicyPanel = lazy(() => import('./PolicyPanel').then(m => ({ default: m.PolicyPanel })))
const HooksPanel = lazy(() => import('./HooksPanel').then(m => ({ default: m.HooksPanel })))
const CodegraphPanel = lazy(() => import('./CodegraphPanel').then(m => ({ default: m.CodegraphPanel })))

function ReviewIcon() {
  return (
    <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
    </svg>
  )
}

function HistoryIcon() {
  return (
    <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  )
}

function ChecklistIcon() {
  return (
    <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
    </svg>
  )
}

// State labels - defined at module level to avoid recreation on every render
const STATE_LABELS: Record<string, string> = {
  'none': 'No Task',
  'loaded': 'Ready',
  'planning': 'Planning...',
  'planned': 'Planned',
  'implementing': 'Implementing...',
  'implemented': 'Implemented',
  'reviewing': 'Reviewing...',
  'reviewed': 'Reviewed',
  'submitted': 'Submitted',
  'failed': 'Failed',
}

export function ProjectView() {
  const { selectedProject, selectProject, agentStatus } = useGlobalStore()
  const { task, state, fileChanges, reviews, output, taskQueue } = useProjectStore()
  const { widgetStates } = useLayoutStore()
  const [showSettings, setShowSettings] = useState(false)
  const [showLogs, setShowLogs] = useState(false)
  const [showCIStatus, setShowCIStatus] = useState(false)
  const [showPolicy, setShowPolicy] = useState(false)
  const [showHooks, setShowHooks] = useState(false)
  const [showCodegraph, setShowCodegraph] = useState(false)
  const docsData = useDocsURL()
  const debugEnabled = useDebugStore(s => s.enabled)
  useKeyboardShortcuts() // Register keyboard shortcuts (overlay rendered in App.tsx)

  // Memoize status type to avoid recalculation on every render
  // Must be before early return to satisfy React hooks rules
  const statusType = useMemo(() => {
    if (!task?.state || state === 'none') return 'idle'
    if (state === 'implemented' || state === 'submitted') return 'success'
    if (state === 'planning' || state === 'implementing' || state === 'reviewing') return 'running'
    if (state === 'failed') return 'error'
    return 'idle'
  }, [task?.state, state])

  // Memoize state label
  const stateLabel = useMemo(() => {
    return STATE_LABELS[state || 'none'] || state || 'No Task'
  }, [state])

  if (!selectedProject) return null

  const projectName = selectedProject.path.split('/').pop() || 'Project'

  // Header component
  const header = (
    <header className="header-enhanced sticky top-0 z-10">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2 sm:gap-0 px-3 sm:px-4 py-2 sm:py-3">
        <div className="flex items-center gap-2 sm:gap-3 w-full sm:w-auto">
          <button
            onClick={() => selectProject(null)}
            aria-label="Back to projects"
            className="flex items-center gap-1 sm:gap-1.5 text-xs sm:text-sm text-base-content/60 hover:text-base-content transition-colors"
          >
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            <span className="hidden sm:inline">Projects</span>
          </button>
          <div className="w-px h-4 sm:h-5 bg-base-300" />
          <div className="flex items-center gap-2 min-w-0 flex-1">
            <div aria-hidden="true" className="w-6 h-6 sm:w-7 sm:h-7 rounded-lg bg-primary flex items-center justify-center text-primary-content font-semibold text-xs sm:text-sm flex-shrink-0">
              {projectName[0].toUpperCase()}
            </div>
            <div className="min-w-0">
              <h1 className="font-medium text-xs sm:text-sm text-base-content truncate">{projectName}</h1>
              <p className="text-[10px] sm:text-xs text-base-content/50 font-mono truncate max-w-[120px] sm:max-w-[200px]">{selectedProject.path}</p>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2 sm:gap-3 w-full sm:w-auto justify-between sm:justify-end flex-wrap">
          <div className="flex items-center gap-1 sm:gap-2 flex-wrap">
            {/* Documentation link */}
            {docsData?.url && (
              <a
                href={docsData.url}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-ghost btn-xs sm:btn-sm btn-circle"
                aria-label="Documentation"
              >
                <svg aria-hidden="true" className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </a>
            )}
            <button
              onClick={() => setShowLogs(true)}
              className="btn btn-ghost btn-xs sm:btn-sm btn-circle relative"
              aria-label="View logs"
            >
              <svg aria-hidden="true" className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h7" />
              </svg>
              {output.length > 0 && (
                <span className="absolute -top-1 -right-1 w-2 h-2 bg-primary rounded-full" />
              )}
            </button>
            <button
              onClick={() => setShowCIStatus(true)}
              className="btn btn-ghost btn-xs sm:btn-sm btn-circle hidden sm:inline-flex"
              aria-label="CI Status"
            >
              <svg aria-hidden="true" className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
            </button>
            <button
              onClick={() => setShowPolicy(true)}
              className="btn btn-ghost btn-xs sm:btn-sm btn-circle hidden sm:inline-flex"
              aria-label="Policy Checks"
            >
              <svg aria-hidden="true" className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
            </button>
            <button
              onClick={() => setShowCodegraph(true)}
              className="btn btn-ghost btn-xs sm:btn-sm btn-circle hidden sm:inline-flex"
              aria-label="Code Graph"
              title="Code Graph"
            >
              <svg aria-hidden="true" className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
              </svg>
            </button>
            <button
              onClick={() => setShowHooks(true)}
              className="btn btn-ghost btn-xs sm:btn-sm btn-circle hidden sm:inline-flex"
              aria-label="Workflow Hooks"
            >
              <svg aria-hidden="true" className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
              </svg>
            </button>
            <button
              onClick={() => setShowSettings(true)}
              className="btn btn-ghost btn-xs sm:btn-sm btn-circle"
              aria-label="Settings"
            >
              <svg aria-hidden="true" className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
            </button>
            <ViewModeToggle />
            <ThemeToggle />
          </div>
          <StatusBadge status={statusType} label={stateLabel} />
        </div>
      </div>
    </header>
  )

  // Left sidebar content
  const leftContent = (
    <ErrorBoundary>
      <Widget
        id="task"
        title="Task"
        icon={<TaskIcon />}
        defaultCollapsed={widgetStates.task?.collapsed}
      >
        <TaskWidget embedded />
      </Widget>

      <Widget
        id="files"
        title="File Changes"
        icon={<FilesIcon />}
        defaultCollapsed={widgetStates.files?.collapsed}
        actions={
          fileChanges && fileChanges.length > 0 ? (
            <span className="text-xs text-base-content/50">{fileChanges.length}</span>
          ) : null
        }
      >
        <FileChangesWidget embedded />
      </Widget>

      {taskQueue.length > 0 && <TaskQueue />}
    </ErrorBoundary>
  )

  // Right panel content
  const rightContent = (
    <ErrorBoundary>
      <RecapWidget />
      <AgentPanel />

      <Widget
        id="checkpoints"
        title="Checkpoints"
        icon={<CheckpointsIcon />}
        defaultCollapsed={widgetStates.checkpoints?.collapsed}
      >
        <CheckpointsWidget embedded />
      </Widget>

      <Widget
        id="phase-metrics"
        title="Phase Metrics"
        icon={<CheckpointsIcon />}
        defaultCollapsed={true}
      >
        <PhaseMetricsWidget embedded />
      </Widget>

      {/* Review History widget — defaultCollapsed, not in layout store */}
      <Widget
        id="review-history"
        title="Review History"
        icon={<ReviewIcon />}
        defaultCollapsed={true}
        actions={
          reviews && reviews.length > 0 ? (
            <span className="text-xs text-base-content/50">{reviews.length}</span>
          ) : null
        }
      >
        <ReviewHistoryWidget embedded />
      </Widget>

      <Widget
        id="checklist"
        title="Review Checklist"
        icon={<ChecklistIcon />}
        defaultCollapsed={true}
      >
        <ChecklistWidget embedded />
      </Widget>

      <Widget
        id="history"
        title="Task History"
        icon={<HistoryIcon />}
        defaultCollapsed={true}
      >
        <TaskHistory />
      </Widget>
    </ErrorBoundary>
  )

  return (
    <div className="h-screen flex flex-col bg-base-100">
      {agentStatus?.simulation_mode && (
        <div role="alert" className="alert alert-warning rounded-none py-2 px-4 text-sm">
          <svg aria-hidden="true" className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <span>Simulation mode — no AI agent connected. Tasks will not be implemented.</span>
        </div>
      )}
      <WorkflowBar />
      <PanelLayout
        header={header}
        leftContent={leftContent}
        rightContent={rightContent}
      />
      <Suspense fallback={null}>
        {showSettings && (
          <Settings
            isOpen={showSettings}
            onClose={() => setShowSettings(false)}
            defaultScope="project"
          />
        )}
        {showLogs && (
          <LogsPanel
            isOpen={showLogs}
            onClose={() => setShowLogs(false)}
          />
        )}
        {showCIStatus && (
          <CIStatusPanel
            isOpen={showCIStatus}
            onClose={() => setShowCIStatus(false)}
          />
        )}
        {showPolicy && (
          <PolicyPanel
            isOpen={showPolicy}
            onClose={() => setShowPolicy(false)}
          />
        )}
        {showHooks && (
          <HooksPanel
            isOpen={showHooks}
            onClose={() => setShowHooks(false)}
          />
        )}
        {showCodegraph && (
          <CodegraphPanel
            isOpen={showCodegraph}
            onClose={() => setShowCodegraph(false)}
          />
        )}
      </Suspense>

      {/* Debug mode indicator */}
      {debugEnabled && (
        <div className="fixed bottom-2 right-2 z-50 badge badge-warning badge-sm gap-1 opacity-70">
          <svg aria-hidden="true" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
          </svg>
          DEBUG
        </div>
      )}
    </div>
  )
}
