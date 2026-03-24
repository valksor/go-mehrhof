import { useState, useRef, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import type { SpecEntry } from '../stores/projectStore'
import { useGlobalStore } from '../stores/globalStore'
import { ViewModeToggle } from './ViewModeToggle'
import { ThemeToggle } from './ThemeToggle'
import { SimpleTimeline } from './SimpleTimeline'
import { SimpleChatWidget } from './SimpleChatWidget'
import { SimpleReviewSummary } from './SimpleReviewSummary'

const STATE_LABELS: Record<string, string> = {
  none: 'Ready to start',
  loaded: 'Task loaded — ready to plan',
  planning: 'Planning...',
  planned: 'Plan ready — review and build',
  implementing: 'Writing code...',
  implemented: 'Code complete — review changes',
  simplifying: 'Cleaning up code...',
  optimizing: 'Improving code quality...',
  reviewing: 'AI is reviewing...',
  submitted: 'PR submitted!',
  failed: 'Something went wrong',
  waiting: 'Waiting for your input',
  paused: 'Paused',
}

const ACTIVE_STATES = new Set(['planning', 'implementing', 'simplifying', 'optimizing', 'reviewing'])

export function SimpleProjectView() {
  const {
    state,
    task,
    output,
    error,
    loading,
    start,
    plan,
    implement,
    review,
    submit,
    finish,
    retry,
    stop,
  } = useProjectStore()
  const { selectedProject, selectProject } = useGlobalStore()
  const [taskSource, setTaskSource] = useState('')
  const [showFilePicker, setShowFilePicker] = useState(false)
  const [projectFiles, setProjectFiles] = useState<Array<{ path: string }>>([])
  const [filesLoading, setFilesLoading] = useState(false)
  const chatRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const listFiles = useProjectStore((s) => s.listFiles)

  const loadSpec = useProjectStore((s) => s.loadSpec)
  const [specs, setSpecs] = useState<SpecEntry[]>([])
  const [showSpec, setShowSpec] = useState(false)
  const specLoadedForState = useRef('')

  // Auto-load specs when plan is ready
  useEffect(() => {
    if ((state === 'planned' || state === 'implemented') && specLoadedForState.current !== state) {
      specLoadedForState.current = state
      loadSpec().then(setSpecs)
    }
  }, [state, loadSpec])

  const projectName = selectedProject?.path?.split('/').pop() ?? 'Project'
  const isActive = ACTIVE_STATES.has(state)

  // Focus task input on mount when in none state
  useEffect(() => {
    if (state === 'none') {
      inputRef.current?.focus()
    }
  }, [state])

  // Focus chat when waiting for input
  useEffect(() => {
    if (state === 'waiting') {
      const chatInput = document.querySelector<HTMLElement>('[data-chat-input]')
      chatInput?.focus()
    }
  }, [state])

  const handleStart = async () => {
    const trimmed = taskSource.trim()
    if (!trimmed) return
    try {
      await start(trimmed)
      setTaskSource('')
    } catch (err) {
      console.error('Failed to start task:', err)
    }
  }

  const handleFinish = async () => {
    const result = await finish()
    if (result) {
      selectProject(null)
    }
  }

  const handleRequestChanges = () => {
    const chatInput = document.querySelector<HTMLElement>('[data-chat-input]')
    chatInput?.focus()
  }

  // Last ~20 lines of output during active states
  const outputTail = isActive ? output.slice(-20) : []

  return (
    <div className="min-h-screen bg-base-100">
      {/* Header */}
      <header className="sticky top-0 z-10 bg-base-100 border-b border-base-300 px-4 py-3">
        <div className="max-w-2xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button
              onClick={() => selectProject(null)}
              className="btn btn-ghost btn-sm btn-square"
              aria-label="Back to projects"
              title="Back to projects"
            >
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
              </svg>
            </button>
            <h1 className="font-semibold text-base-content truncate">{projectName}</h1>
          </div>
          <div className="flex items-center gap-2">
            <ViewModeToggle />
            <ThemeToggle />
          </div>
        </div>
      </header>

      {/* Content */}
      <div className="max-w-2xl mx-auto px-4 py-6 space-y-6">
        {/* Status banner */}
        <div className={`text-center py-3 px-4 rounded-xl ${
          state === 'failed' ? 'bg-error/10 text-error' :
          state === 'submitted' ? 'bg-success/10 text-success' :
          isActive ? 'bg-primary/10 text-primary' :
          'bg-base-200 text-base-content/70'
        }`}>
          <p className="font-medium">{STATE_LABELS[state] ?? state}</p>
          {task?.title && state !== 'none' && (
            <p className="text-sm opacity-70 mt-1 truncate">{task.title}</p>
          )}
        </div>

        {/* Timeline */}
        <section aria-label="Activity timeline">
          <SimpleTimeline />
        </section>

        {/* Streaming output during active states */}
        {outputTail.length > 0 && (
          <div className="bg-base-200 rounded-lg p-3 max-h-40 overflow-y-auto">
            <pre className="text-xs text-base-content/60 font-mono whitespace-pre-wrap">
              {outputTail.join('\n')}
            </pre>
          </div>
        )}

        {/* State-dependent content */}
        <section aria-label="Actions" className="space-y-4">
          {/* None: task input */}
          {state === 'none' && (
            <div className="space-y-3">
              <label className="text-sm text-base-content/70" htmlFor="task-source">
                Enter a GitHub issue URL, file path, or task description:
              </label>
              <textarea
                ref={inputRef}
                id="task-source"
                value={taskSource}
                onChange={(e) => setTaskSource(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) handleStart() }}
                placeholder={"Paste a GitHub issue URL, file path, or describe the task...\n\nExamples:\n  https://github.com/org/repo/issues/42\n  file:task.md\n  Add error handling to the login flow"}
                className="textarea textarea-bordered w-full min-h-32 text-base leading-relaxed"
                disabled={loading}
                rows={5}
              />
              {error && (
                <p className="text-sm text-error bg-error/10 rounded-lg px-4 py-2">{error}</p>
              )}
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-base-content/40">Ctrl+Enter to start</span>
                  <button
                    onClick={async () => {
                      if (showFilePicker) { setShowFilePicker(false); return }
                      setFilesLoading(true)
                      setShowFilePicker(true)
                      const files = await listFiles('', ['.md', '.txt', '.yaml', '.yml'], 3)
                      setProjectFiles(files)
                      setFilesLoading(false)
                    }}
                    className="btn btn-ghost btn-xs gap-1"
                    type="button"
                  >
                    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" /></svg>
                    {showFilePicker ? 'Hide' : 'Browse files'}
                  </button>
                </div>
                <button
                  onClick={handleStart}
                  disabled={!taskSource.trim() || loading}
                  className="btn btn-primary"
                >
                  {loading ? <span className="loading loading-spinner loading-sm"></span> : 'Start'}
                </button>
              </div>
              {showFilePicker && (
                <div className="border border-base-300 rounded-lg overflow-hidden">
                  {filesLoading ? (
                    <div className="flex items-center justify-center py-6">
                      <span className="loading loading-spinner loading-sm" />
                    </div>
                  ) : projectFiles.length === 0 ? (
                    <div className="text-sm text-base-content/50 py-4 text-center">No task files found (.md, .txt, .yaml)</div>
                  ) : (
                    <ul className="max-h-48 overflow-y-auto">
                      {projectFiles.map((f) => (
                        <li key={f.path}>
                          <button
                            onClick={() => { setTaskSource(`file:${f.path}`); setShowFilePicker(false); inputRef.current?.focus() }}
                            className="w-full text-left px-3 py-1.5 text-sm hover:bg-base-200 transition-colors flex items-center gap-2 cursor-pointer"
                          >
                            <svg className="w-3.5 h-3.5 text-base-content/40 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                            <span className="truncate">{f.path}</span>
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              )}
            </div>
          )}

          {/* Loaded: start planning */}
          {state === 'loaded' && (
            <div className="text-center">
              <button
                onClick={() => plan()}
                disabled={loading}
                className="btn btn-primary btn-lg"
              >
                {loading ? <span className="loading loading-spinner loading-sm"></span> : 'Make a Plan'}
              </button>
              <p className="text-xs text-base-content/50 mt-2">The AI will analyze your task and create a plan</p>
            </div>
          )}

          {/* Planned: show spec + build */}
          {state === 'planned' && (
            <div className="space-y-4">
              {specs.length > 0 && (
                <div className="border border-base-300 rounded-lg overflow-hidden">
                  <button
                    onClick={() => setShowSpec(!showSpec)}
                    className="w-full flex items-center justify-between px-4 py-2.5 bg-base-200 hover:bg-base-300 transition-colors text-sm font-medium"
                  >
                    <span>View Plan</span>
                    <svg className={`w-4 h-4 transition-transform ${showSpec ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>
                  {showSpec && (
                    <div className="px-4 py-3 max-h-80 overflow-y-auto">
                      {specs.map((spec) => (
                        <div key={spec.path}>
                          <div className="text-xs text-base-content/50 mb-1 font-mono">{spec.path}</div>
                          <pre className="text-sm whitespace-pre-wrap text-base-content/80">{spec.content}</pre>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
              <div className="text-center">
                <button
                  onClick={() => implement()}
                  disabled={loading}
                  className="btn btn-primary btn-lg"
                >
                  {loading ? <span className="loading loading-spinner loading-sm"></span> : 'Build'}
                </button>
              </div>
            </div>
          )}

          {/* Active states: stop button */}
          {isActive && (
            <div className="text-center">
              <button
                onClick={() => stop()}
                className="btn btn-ghost btn-sm"
              >
                Stop
              </button>
            </div>
          )}

          {/* Implemented: review summary */}
          {state === 'implemented' && (
            <SimpleReviewSummary
              loading={loading}
              onReview={async () => {
                await review({ approve: true })
                // Check if review succeeded before submitting
                if (useProjectStore.getState().error) return
                await submit()
              }}
              onRequestChanges={handleRequestChanges}
            />
          )}

          {/* Submitted: show PR link + finish */}
          {state === 'submitted' && (
            <div className="text-center space-y-3">
              {/* PR URL extracted from output stream — pr_url is not yet surfaced through the store */}
              {(() => {
                const prLine = [...output].reverse().find((l: string) => l.includes('http') && (l.includes('/pull/') || l.includes('/merge_requests/')))
                const prUrl = prLine?.match(/(https?:\/\/\S+)/)?.[1]
                return prUrl ? (
                  <a href={prUrl} target="_blank" rel="noopener noreferrer" className="btn btn-outline btn-sm gap-2">
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                    View Pull Request
                  </a>
                ) : (
                  <p className="text-sm text-base-content/50">PR was created. Check your repository for details.</p>
                )
              })()}
              <div>
                <button
                  onClick={handleFinish}
                  disabled={loading}
                  className="btn btn-primary btn-lg"
                >
                  {loading ? <span className="loading loading-spinner loading-sm"></span> : 'Finish & Return'}
                </button>
              </div>
            </div>
          )}

          {/* Failed: retry */}
          {state === 'failed' && (
            <div className="text-center space-y-2">
              {error && (
                <p className="text-sm text-error bg-error/10 rounded-lg px-4 py-2">{error}</p>
              )}
              <button
                onClick={() => retry()}
                disabled={loading}
                className="btn btn-warning"
              >
                {loading ? <span className="loading loading-spinner loading-sm"></span> : 'Try Again'}
              </button>
            </div>
          )}

          {/* Waiting: prompt to use chat */}
          {state === 'waiting' && (
            <div className="text-center text-sm text-base-content/60">
              The AI needs your input. Use the chat below to respond.
            </div>
          )}

          {/* Paused: resume */}
          {state === 'paused' && (
            <div className="text-center">
              <button
                onClick={() => retry()}
                disabled={loading}
                className="btn btn-primary"
              >
                Resume
              </button>
            </div>
          )}
        </section>

        {/* Chat */}
        <section ref={chatRef} aria-label="Chat" className="border-t border-base-300 pt-4">
          <SimpleChatWidget />
        </section>
      </div>
    </div>
  )
}
