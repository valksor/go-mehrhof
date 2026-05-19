import { useState, useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { useDocsURL } from '../hooks/useDocsURL'
import { name } from '../meta'
import { AccessibleModal } from './ui/AccessibleModal'
import type { SettingsResponse } from '../types/settings'

interface OnboardingProps {
  onAddProject: () => void
}

export function Onboarding({ onAddProject }: OnboardingProps) {
  const client = useGlobalStore((state) => state.client)
  const [dismissed, setDismissed] = useState<boolean | null>(null) // null = loading
  const docsData = useDocsURL()

  // Fetch onboarding state from backend settings on mount
  useEffect(() => {
    if (!client) return

    client
      .call<SettingsResponse>('settings.get', {})
      .then((result) => {
        // Safe nested access - effective is Record<string, unknown>
        const ui = result.effective?.ui as Record<string, unknown> | undefined
        const isDismissed = ui?.onboarding_dismissed === true
        setDismissed(isDismissed)
      })
      .catch(() => {
        // On error, show onboarding (fail open)
        setDismissed(false)
      })
  }, [client])

  const handleDismiss = async () => {
    setDismissed(true)

    // Persist to backend settings
    if (client) {
      try {
        await client.call('settings.set', {
          scope: 'global',
          values: { 'ui.onboarding_dismissed': true },
        })
      } catch {
        // Ignore - onboarding won't show again this session anyway
      }
    }
  }

  const handleAddProject = () => {
    void handleDismiss()
    onAddProject()
  }

  // Don't render while loading or if dismissed
  if (dismissed === null || dismissed) return null

  return (
    <AccessibleModal isOpen={true} onClose={handleDismiss} title={`Welcome to ${name}`} size="lg">
      {/* Header */}
      <div className="text-center mb-6">
        <div className="w-16 h-16 rounded-2xl bg-primary flex items-center justify-center mx-auto mb-4 shadow-lg">
          <svg className="w-8 h-8 text-primary-content" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
        <p className="text-base-content/70 mt-2">AI-powered task orchestration for development</p>
      </div>

      {/* Steps */}
      <div className="space-y-4 mb-6">
        <div className="flex gap-3 items-start">
          <div className="w-8 h-8 rounded-full bg-primary/20 flex items-center justify-center flex-shrink-0">
            <span className="text-primary font-bold">1</span>
          </div>
          <div>
            <p className="font-medium text-base-content">Add a project</p>
            <p className="text-sm text-base-content/60">Select a folder with your code</p>
          </div>
        </div>

        <div className="flex gap-3 items-start">
          <div className="w-8 h-8 rounded-full bg-primary/20 flex items-center justify-center flex-shrink-0">
            <span className="text-primary font-bold">2</span>
          </div>
          <div>
            <p className="font-medium text-base-content">Load a task</p>
            <p className="text-sm text-base-content/60">From GitHub issue, file, or describe it</p>
          </div>
        </div>

        <div className="flex gap-3 items-start">
          <div className="w-8 h-8 rounded-full bg-primary/20 flex items-center justify-center flex-shrink-0">
            <span className="text-primary font-bold">3</span>
          </div>
          <div>
            <p className="font-medium text-base-content">Let AI work</p>
            <p className="text-sm text-base-content/60">Plan, implement, review, and submit</p>
          </div>
        </div>
      </div>

      {/* Actions */}
      <div className="flex flex-col sm:flex-row gap-3">
        <button onClick={handleAddProject} className="btn btn-primary flex-1">
          <svg aria-hidden="true" className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          Add Project
        </button>
        {docsData?.url && (
          <a
            href={docsData.url}
            target="_blank"
            rel="noopener noreferrer"
            onClick={handleDismiss}
            className="btn btn-outline flex-1"
          >
            <svg aria-hidden="true" className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
              />
            </svg>
            Read Docs
          </a>
        )}
      </div>

      {/* Skip link */}
      <button
        onClick={handleDismiss}
        className="w-full text-center text-sm text-base-content/50 hover:text-base-content mt-4 transition-colors"
      >
        Skip for now
      </button>
    </AccessibleModal>
  )
}
