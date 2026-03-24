import { useState, useCallback, useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'

interface OnboardingGuideProps {
  onDismiss: () => void
}

interface Step {
  title: string
  description: string
  icon: string
  key: string // Maps to Go onboarding.Step
}

const STEPS: Step[] = [
  {
    title: 'Install Check',
    description: 'Ensure kvelmo is installed and the global socket is running.',
    icon: 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
    key: 'install_check',
  },
  {
    title: 'First Project',
    description: 'Register a project to create a worktree and connect to the socket.',
    icon: 'M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z',
    key: 'first_project',
  },
  {
    title: 'First Task',
    description: 'Load a task from GitHub, Linear, or a local file using kvelmo start.',
    icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2',
    key: 'first_task',
  },
  {
    title: 'First Plan',
    description: 'Run kvelmo plan to have the agent write a specification for your task.',
    icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z',
    key: 'first_plan',
  },
  {
    title: 'First Implement',
    description: 'Run kvelmo implement to have the agent write code based on the plan.',
    icon: 'M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4',
    key: 'first_implement',
  },
  {
    title: 'First Submit',
    description: 'Run kvelmo submit to create a pull request from the completed work.',
    icon: 'M9 5l7 7-7 7',
    key: 'first_submit',
  },
]

export function OnboardingGuide({ onDismiss }: OnboardingGuideProps) {
  const client = useGlobalStore(state => state.client)
  const [stepStatus, setStepStatus] = useState<Record<string, boolean>>({})

  // Load onboarding status from server
  useEffect(() => {
    if (!client) return

    client.call<{ steps: Record<string, boolean> }>('onboarding.status', {})
      .then(result => {
        setStepStatus(result.steps || {})
      })
      .catch(() => {
        // Ignore — steps will show as incomplete
      })
  }, [client])

  const completeStep = useCallback((step: Step) => {
    if (stepStatus[step.key]) return // Already completed

    setStepStatus(prev => ({ ...prev, [step.key]: true }))
    if (client) {
      client.call('onboarding.complete', { step: step.key }).catch(() => {
        setStepStatus(prev => ({ ...prev, [step.key]: false }))
      })
    }
  }, [client, stepStatus])

  const completedCount = STEPS.filter(s => stepStatus[s.key]).length
  const totalSteps = STEPS.length
  const allComplete = completedCount === totalSteps

  return (
    <div className="card bg-base-200 border border-base-300">
      <div className="card-body p-5">
        {/* Header */}
        <div className="flex items-start justify-between gap-4">
          <div>
            <h3 className="text-lg font-bold text-base-content">Getting Started with kvelmo</h3>
            <p className="text-sm text-base-content/60 mt-1">
              Follow these steps to set up your first AI-assisted development workflow.
            </p>
          </div>
          <button
            onClick={onDismiss}
            className="btn btn-ghost btn-sm btn-circle flex-shrink-0"
            aria-label="Dismiss onboarding guide"
          >
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Progress */}
        <div className="flex items-center gap-3 mt-3 mb-4">
          <progress
            className="progress progress-primary flex-1"
            value={completedCount}
            max={totalSteps}
          />
          <span className="text-xs text-base-content/50 whitespace-nowrap">
            {completedCount}/{totalSteps} complete
          </span>
        </div>

        {/* Steps */}
        <div className="space-y-2">
          {STEPS.map((step) => {
            const isCompleted = stepStatus[step.key] === true
            return (
              <button
                key={step.key}
                onClick={() => completeStep(step)}
                className={`flex items-start gap-3 w-full text-left rounded-lg p-3 transition-colors ${
                  isCompleted
                    ? 'bg-base-100/50 opacity-60'
                    : 'bg-base-100 hover:bg-base-100/80'
                }`}
                aria-label={`${isCompleted ? 'Completed' : 'Mark complete'}: ${step.title}`}
              >
                {/* Checkbox */}
                <div className={`flex-shrink-0 w-5 h-5 rounded-full border-2 flex items-center justify-center mt-0.5 ${
                  isCompleted
                    ? 'border-success bg-success'
                    : 'border-base-content/30'
                }`}>
                  {isCompleted && (
                    <svg aria-hidden="true" className="w-3 h-3 text-success-content" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                  )}
                </div>

                {/* Icon */}
                <svg aria-hidden="true" className="w-5 h-5 flex-shrink-0 text-primary/60 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d={step.icon} />
                </svg>

                {/* Content */}
                <div className="min-w-0">
                  <div className={`text-sm font-medium ${isCompleted ? 'line-through text-base-content/50' : 'text-base-content'}`}>
                    {step.title}
                  </div>
                  <div className="text-xs text-base-content/50 mt-0.5">
                    {step.description}
                  </div>
                </div>
              </button>
            )
          })}
        </div>

        {/* Footer */}
        {allComplete && (
          <div className="mt-4 text-center">
            <p className="text-sm text-success font-medium mb-2">All steps completed!</p>
            <button onClick={onDismiss} className="btn btn-primary btn-sm">
              Dismiss Guide
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
