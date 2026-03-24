import { useState, useCallback, useEffect } from 'react'
import { useShallow } from 'zustand/react/shallow'
import { useProjectStore } from '../stores/projectStore'
import { useChatStore } from '../stores/chatStore'
import { useLayoutStore } from '../stores/layoutStore'
import { ConfirmModal } from './ui/ConfirmModal'
import type { FailureClass } from '../lib/events'

const EXPLAIN_PROMPT = 'Explain what you did in the last action, why you made those choices, and any assumptions or constraints you encountered.'

interface WorkflowStep {
  id: string
  label: string
  /** Task states that map to this step being current */
  states: string[]
  /** State required to trigger this step */
  triggerState?: string
  /** Action to run when clicked */
  action?: () => Promise<void>
}

const ACTIVE_STATES = new Set(['planning', 'implementing', 'simplifying', 'optimizing', 'reviewing'])

const HINTS: Record<string, string> = {
  none: 'No active task. Load one with the task panel.',
  loaded: 'Task loaded — click Plan to start.',
  planning: 'Planning in progress...',
  planned: 'Plan ready — click Implement.',
  implementing: 'Implementation in progress...',
  implemented: 'Code complete — Review, Simplify, or Optimize.',
  simplifying: 'Simplification in progress...',
  optimizing: 'Optimization in progress...',
  reviewing: 'Review in progress...',
  submitted: 'PR submitted. Check CI status, merge when ready, then click Finish to clean up.',
  failed: 'Last action failed. Check errors or Undo.',
  waiting: 'Waiting for input.',
  paused: 'Task paused.',
}

function failureBadgeClass(isFailed: boolean, failureClass?: FailureClass): string {
  if (!isFailed) return 'badge-warning'
  switch (failureClass) {
    case 'recoverable': return 'badge-warning'
    case 'degraded': return 'badge-warning'
    case 'skippable': return 'badge-info'
    case 'hard_stop':
    default: return 'badge-error'
  }
}

function failureBadgeLabel(isFailed: boolean, failureClass?: FailureClass): string {
  if (!isFailed) return 'Paused'
  switch (failureClass) {
    case 'recoverable': return 'Retrying...'
    case 'degraded': return 'Degraded'
    case 'skippable': return 'Skipped'
    case 'hard_stop':
    default: return 'Failed'
  }
}

export function WorkflowBar() {
  const {
    state, plan, implement, review, submit, finish,
    stop, undo, redo, abandon, approveTransition, retry,
    loading, checkpoints, redoStack, approveRemote, mergeRemote, refresh, error,
    phaseError, dryRunMode, toggleDryRun, skipPhases, autoFixStatus,
  } = useProjectStore(useShallow(s => ({
    state: s.state,
    plan: s.plan,
    implement: s.implement,
    review: s.review,
    submit: s.submit,
    finish: s.finish,
    stop: s.stop,
    undo: s.undo,
    redo: s.redo,
    abandon: s.abandon,
    approveTransition: s.approveTransition,
    retry: s.retry,
    loading: s.loading,
    checkpoints: s.checkpoints,
    redoStack: s.redoStack,
    approveRemote: s.approveRemote,
    mergeRemote: s.mergeRemote,
    refresh: s.refresh,
    error: s.error,
    phaseError: s.phaseError,
    dryRunMode: s.dryRunMode,
    toggleDryRun: s.toggleDryRun,
    skipPhases: s.skipPhases,
    autoFixStatus: s.autoFixStatus,
  })))

  const [showSubmitModal, setShowSubmitModal] = useState(false)
  const [submitDeleteBranch, setSubmitDeleteBranch] = useState(false)
  const [showFinishModal, setShowFinishModal] = useState(false)
  const [finishDeleteRemote, setFinishDeleteRemote] = useState(false)
  const [showAbandonModal, setShowAbandonModal] = useState(false)
  const [abandonKeepBranch, setAbandonKeepBranch] = useState(false)

  // React to modal commands from chat slash commands
  useEffect(() => {
    return useLayoutStore.subscribe((state) => {
      if (!state.modalCommand) return
      if (state.modalCommand === 'submit') setShowSubmitModal(true)
      else if (state.modalCommand === 'finish') setShowFinishModal(true)
      else if (state.modalCommand === 'abandon') setShowAbandonModal(true)
      state.setModalCommand(null)
    })
  }, [])

  const steps: WorkflowStep[] = [
    { id: 'load', label: 'Load', states: ['loaded'] },
    { id: 'plan', label: 'Plan', states: ['planning', 'planned'], triggerState: 'loaded', action: () => plan() },
    { id: 'implement', label: 'Implement', states: ['implementing', 'implemented'], triggerState: 'planned', action: () => implement() },
    { id: 'review', label: 'Review', states: ['reviewing'], triggerState: 'implemented', action: () => review({ approve: true }) },
    { id: 'submit', label: 'Submit', states: ['submitted'], triggerState: 'reviewing' },
    { id: 'finish', label: 'Finish', states: [] },
  ]

  const stepIndex = steps.findIndex(step => step.states.includes(state))
  const isFailed = state === 'failed'
  const isPaused = state === 'paused' || state === 'waiting'
  const isActive = ACTIVE_STATES.has(state)
  const canUndo = checkpoints.length > 0
  const canRedo = redoStack.length > 0
  const isSubmitted = state === 'submitted'

  const handleStepClick = useCallback(async (step: WorkflowStep) => {
    if (step.id === 'submit') {
      setShowSubmitModal(true)
      return
    }
    if (step.action) {
      await step.action()
    }
  }, [])

  const handleSubmit = async () => {
    setShowSubmitModal(false)
    const override = useProjectStore.getState().prBodyOverride
    await submit({ delete_branch: submitDeleteBranch, ...(override ? { body: override } : {}) })
    setSubmitDeleteBranch(false)
    useProjectStore.getState().setPrBodyOverride(null)
  }

  const handleFinish = async () => {
    setShowFinishModal(false)
    await finish({ delete_remote: finishDeleteRemote })
    setFinishDeleteRemote(false)
  }

  const handleAbandon = async () => {
    setShowAbandonModal(false)
    await abandon(abandonKeepBranch)
    setAbandonKeepBranch(false)
  }

  if (state === 'none') return null

  const hint = HINTS[state] || ''

  return (
    <>
      <div className="flex items-center gap-1 px-3 py-1.5 bg-base-200/50 border-b border-base-300">
        {/* Dry-run toggle */}
        <label className="flex items-center gap-1 mr-2 cursor-pointer">
          <input
            type="checkbox"
            className={`checkbox checkbox-xs ${dryRunMode ? 'checkbox-warning' : ''}`}
            checked={dryRunMode}
            onChange={toggleDryRun}
          />
          <span className={`text-xs ${dryRunMode ? 'text-warning font-semibold' : 'opacity-60'}`}>
            Dry Run
          </span>
        </label>

        {/* Status indicator for failed/paused */}
        {(isFailed || isPaused) && (
          <span className={`badge badge-xs mr-1 ${failureBadgeClass(isFailed, phaseError?.class)}`}>
            {failureBadgeLabel(isFailed, phaseError?.class)}
          </span>
        )}

        {/* Retry button — re-run the failed phase */}
        {isFailed && (
          <button
            onClick={() => retry()}
            disabled={loading}
            className="btn btn-xs btn-error gap-1"
            aria-label="Retry failed phase"
          >
            <svg aria-hidden="true" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Retry
          </button>
        )}

        {/* Step indicators */}
        <div className="flex items-center gap-0.5 overflow-x-auto">
          {steps.map((step, index) => {
            const isCompleted = stepIndex > index
            const isCurrent = stepIndex === index
            const isClickable = !loading && step.triggerState === state && (step.action || step.id === 'submit')

            let dotClass = 'w-2 h-2 rounded-full flex-shrink-0 transition-colors'
            let labelClass = 'text-[10px] sm:text-xs transition-colors whitespace-nowrap'

            if (isCompleted) {
              dotClass += ' bg-success'
              labelClass += ' text-success'
            } else if (isCurrent && isActive) {
              dotClass += ' bg-primary animate-pulse'
              labelClass += ' text-primary font-medium'
            } else if (isCurrent) {
              dotClass += ' bg-primary'
              labelClass += ' text-primary font-medium'
            } else if (isClickable) {
              dotClass += ' bg-primary/60'
              labelClass += ' text-primary/80'
            } else {
              dotClass += ' bg-base-300'
              labelClass += ' text-base-content/30'
            }

            const isSkipped = skipPhases.includes(step.id)

            const showAutoFix = isCurrent && autoFixStatus?.active

            const stepContent = (
              <div className="flex items-center gap-1">
                <div className={isSkipped ? 'w-2 h-2 rounded-full flex-shrink-0 bg-base-300 opacity-30' : dotClass} />
                <span className={isSkipped ? 'text-[10px] sm:text-xs text-base-content/20 line-through' : labelClass}>{step.label}</span>
                {showAutoFix && (
                  <span className="badge badge-xs badge-warning animate-pulse" title={`Auto-fix attempt ${autoFixStatus.attempt ?? '?'}/${autoFixStatus.maxAttempts ?? '?'}`}>
                    fix {autoFixStatus.attempt}/{autoFixStatus.maxAttempts}
                  </span>
                )}
              </div>
            )

            return (
              <div key={step.id} className="contents">
                {isClickable ? (
                  <button
                    onClick={() => handleStepClick(step)}
                    disabled={loading}
                    className="flex items-center gap-1 hover:bg-base-300 rounded px-1 -mx-1 transition-colors cursor-pointer"
                    aria-label={`${step.label} — click to start`}
                  >
                    {stepContent}
                  </button>
                ) : (
                  stepContent
                )}
                {index < steps.length - 1 && (
                  <div className={`flex-1 h-px min-w-2 max-w-6 transition-colors ${
                    isCompleted ? 'bg-success/50' : 'bg-base-300'
                  }`} />
                )}
              </div>
            )
          })}
        </div>

        {/* Approval gate button — shown when a transition requires explicit approval */}
        {error && error.includes('approval required') && (
          <button
            onClick={() => {
              // Extract event name from error message (e.g., "Run: kvelmo approve submit")
              const match = error.match(/kvelmo approve (\w+)/)
              const event = match ? match[1] : 'submit'
              approveTransition(event)
            }}
            disabled={loading}
            className="btn btn-xs btn-warning ml-2 gap-1"
            aria-label="Approve transition"
          >
            <svg aria-hidden="true" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
            Approve
          </button>
        )}

        {/* Stop button — gracefully halts the current operation */}
        {isActive && (
          <button
            onClick={() => stop()}
            disabled={loading}
            className="btn btn-ghost btn-xs text-warning hover:text-warning ml-1 gap-1"
            aria-label="Stop current operation (preserves state)"
          >
            <svg aria-hidden="true" className="w-3 h-3" fill="currentColor" viewBox="0 0 24 24">
              <rect x="6" y="6" width="12" height="12" rx="1" />
            </svg>
            <span className="hidden sm:inline">Stop</span>
          </button>
        )}

        {/* Hint text */}
        <span className="text-xs text-base-content/50 ml-2 truncate hidden sm:inline">
          {hint}
        </span>

        {/* Spacer */}
        <div className="flex-1" />

        {/* Post-submit actions */}
        {isSubmitted && (
          <div className="flex items-center gap-1 mr-2">
            <button
              onClick={() => refresh()}
              disabled={loading}
              className="btn btn-ghost btn-xs"
              aria-label="Check PR status"
            >
              <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </button>
            <button
              onClick={() => approveRemote()}
              disabled={loading}
              className="btn btn-ghost btn-xs"
              aria-label="Approve PR"
            >
              <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            <button
              onClick={() => mergeRemote('rebase')}
              disabled={loading}
              className="btn btn-ghost btn-xs"
              aria-label="Merge PR"
            >
              <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
              </svg>
            </button>
            <button
              onClick={() => setShowFinishModal(true)}
              disabled={loading}
              className="btn btn-ghost btn-xs text-success"
              aria-label="Finish and clean up"
            >
              <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
            </button>
          </div>
        )}

        {/* Undo/Redo */}
        {(canUndo || canRedo) && (
          <div className="flex items-center gap-0.5">
            <button
              onClick={() => undo()}
              disabled={!canUndo || loading}
              className="btn btn-ghost btn-xs btn-square"
              aria-label={`Undo (${checkpoints.length} checkpoints)`}
            >
              <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
              </svg>
            </button>
            <button
              onClick={() => redo()}
              disabled={!canRedo || loading}
              className="btn btn-ghost btn-xs btn-square"
              aria-label={`Redo (${redoStack.length} in stack)`}
            >
              <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 10h-10a8 8 0 00-8 8v2m18-10l-6 6m6-6l-6-6" />
              </svg>
            </button>
          </div>
        )}

        {/* Explain — ask agent what it did last */}
        <button
          onClick={() => useChatStore.getState().sendMessage(EXPLAIN_PROMPT)}
          disabled={loading}
          className="btn btn-ghost btn-xs btn-square"
          aria-label="Explain last action"
          title="Explain last action"
        >
          <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01" />
          </svg>
        </button>

        {/* Abandon (danger zone — small icon) */}
        <button
          onClick={() => setShowAbandonModal(true)}
          disabled={loading}
          className="btn btn-ghost btn-xs btn-square text-error/60 hover:text-error ml-1"
          aria-label="Abandon task"
        >
          <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        {/* Skip phases indicator */}
        {skipPhases.length > 0 && (
          <span className="text-[10px] text-base-content/40 ml-1" title={`Skipping: ${skipPhases.join(', ')}`}>
            skip: {skipPhases.join(', ')}
          </span>
        )}

        {/* Loading spinner */}
        {loading && (
          <span className="loading loading-spinner loading-xs text-primary ml-1" />
        )}
      </div>

      {/* Modals */}
      <ConfirmModal
        isOpen={showSubmitModal}
        onClose={() => { setShowSubmitModal(false); setSubmitDeleteBranch(false) }}
        onConfirm={handleSubmit}
        title="Submit Pull Request"
        description="This will create a pull request for the current task."
        confirmLabel="Submit PR"
        confirmClass="btn btn-primary"
      >
        <label className="flex items-center gap-3 cursor-pointer mb-2">
          <input
            type="checkbox"
            className="checkbox checkbox-sm"
            checked={submitDeleteBranch}
            onChange={e => setSubmitDeleteBranch(e.target.checked)}
          />
          <span className="text-sm">Delete branch after submitting</span>
        </label>
        <button
          onClick={() => { setShowSubmitModal(false); submit({ dry_run: true }) }}
          className="btn btn-sm btn-ghost btn-block mt-1"
        >
          Preview PR (dry-run)
        </button>
      </ConfirmModal>

      <ConfirmModal
        isOpen={showFinishModal}
        onClose={() => { setShowFinishModal(false); setFinishDeleteRemote(false) }}
        onConfirm={handleFinish}
        title="Finish Task"
        description="This will switch to the base branch, pull latest changes, and delete the feature branch."
        confirmLabel="Finish"
        confirmClass="btn btn-success"
      >
        <label className="flex items-center gap-3 cursor-pointer mb-2">
          <input
            type="checkbox"
            className="checkbox checkbox-sm"
            checked={finishDeleteRemote}
            onChange={e => setFinishDeleteRemote(e.target.checked)}
          />
          <span className="text-sm">Also delete remote branch</span>
        </label>
      </ConfirmModal>

      <ConfirmModal
        isOpen={showAbandonModal}
        onClose={() => { setShowAbandonModal(false); setAbandonKeepBranch(false) }}
        onConfirm={handleAbandon}
        title="Abandon Task?"
        description="This will discard the current task and reset the worktree. This action cannot be undone."
        confirmLabel="Abandon"
        confirmClass="btn btn-error"
      >
        <label className="flex items-center gap-3 cursor-pointer mb-2">
          <input
            type="checkbox"
            className="checkbox checkbox-sm"
            checked={abandonKeepBranch}
            onChange={e => setAbandonKeepBranch(e.target.checked)}
          />
          <span className="text-sm">Keep branch after abandoning</span>
        </label>
      </ConfirmModal>
    </>
  )
}
