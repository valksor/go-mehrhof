import { useEffect, useRef } from 'react'
import { useProjectStore } from '../stores/projectStore'

const ACTIVE_STATES = new Set(['planning', 'implementing', 'simplifying', 'optimizing', 'reviewing'])

const ACTIVE_STATE_LABELS: Record<string, string> = {
  planning: 'Writing plan...',
  implementing: 'Writing code...',
  simplifying: 'Cleaning up...',
  optimizing: 'Improving code...',
  reviewing: 'Reviewing changes...',
}

function checkpointLabel(message: string): string | null {
  const lower = message.toLowerCase()
  if (lower.includes('safety')) return null // Skip internal checkpoints
  if (lower.includes('plan')) return 'Plan complete'
  if (lower.includes('implement')) return 'Code complete'
  if (lower.includes('simplify')) return 'Code cleaned up'
  if (lower.includes('optimize')) return 'Code optimized'
  if (lower.includes('review')) return 'Review complete'
  if (lower.includes('submit')) return 'PR submitted'
  return message
}

function formatTime(timestamp: string): string {
  try {
    return new Date(timestamp).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
  } catch {
    return ''
  }
}

export function SimpleTimeline() {
  const checkpoints = useProjectStore((s) => s.checkpoints)
  const state = useProjectStore((s) => s.state)
  const bottomRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom when new entries appear
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [checkpoints.length, state])

  const entries = checkpoints
    .map((cp) => {
      const label = checkpointLabel(cp.message)
      if (!label) return null
      return { key: cp.sha, time: formatTime(cp.timestamp), label }
    })
    .filter(Boolean) as Array<{ key: string; time: string; label: string }>

  const isActive = ACTIVE_STATES.has(state)

  if (entries.length === 0 && !isActive) {
    return (
      <div className="text-center py-6 text-base-content/40 text-sm">
        No activity yet
      </div>
    )
  }

  return (
    <div className="max-h-48 overflow-y-auto space-y-1 px-1">
      {entries.map((entry) => (
        <div key={entry.key} className="flex items-center gap-3 text-sm py-1">
          <span className="text-base-content/40 text-xs w-16 text-right flex-shrink-0 font-mono">
            {entry.time}
          </span>
          <span className="text-success flex-shrink-0" aria-hidden="true">
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
          </span>
          <span className="text-base-content/80">{entry.label}</span>
        </div>
      ))}

      {/* Active state indicator */}
      {isActive && (
        <div className="flex items-center gap-3 text-sm py-1">
          <span className="text-base-content/40 text-xs w-16 text-right flex-shrink-0 font-mono">
            now
          </span>
          <span className="loading loading-spinner loading-xs text-primary flex-shrink-0" aria-hidden="true"></span>
          <span className="text-base-content/80">{ACTIVE_STATE_LABELS[state] ?? 'Working...'}</span>
        </div>
      )}

      <div ref={bottomRef} />
    </div>
  )
}
