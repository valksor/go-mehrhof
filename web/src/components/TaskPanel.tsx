import type { ReactNode } from 'react'
import { useProjectStore } from '../stores/projectStore'

interface TaskPanelProps {
  data?: Record<string, unknown>
}

export function TaskPanel({ data }: TaskPanelProps) {
  const task = useProjectStore((state) => state.task)
  const riskScore = useProjectStore((state) => state.riskScore)
  const taskData = data?.task as typeof task

  // Use data from props if available, otherwise from store
  const displayTask = taskData || task

  if (!displayTask) {
    return (
      <div className="flex items-center justify-center h-full text-base-content/50">
        <div className="text-center">
          <svg className="w-12 h-12 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
          </svg>
          <p className="text-sm">No task loaded</p>
        </div>
      </div>
    )
  }

  // Parse source to get provider type
  const sourceType = displayTask.source?.split(':')[0] || 'unknown'

  return (
    <div className="h-full overflow-auto p-4">
      <div className="space-y-4">
        {/* Header */}
        <div className="flex items-start gap-3">
          <SourceBadge type={sourceType} />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="text-lg font-semibold truncate">{displayTask.title}</h2>
              {riskScore && riskScore.level === 'high' && (
                <span className="badge badge-error badge-sm gap-1 shrink-0">
                  <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
                  </svg>
                  High Risk
                </span>
              )}
            </div>
            <p className="text-xs text-base-content/60 font-mono">{displayTask.id}</p>
          </div>
        </div>

        {/* Description */}
        {displayTask.description && (
          <div className="bg-base-200 rounded-lg p-4">
            <h3 className="text-sm font-medium mb-2 text-base-content/70">Description</h3>
            <div className="prose prose-sm max-w-none text-base-content/80 whitespace-pre-wrap">
              {displayTask.description}
            </div>
          </div>
        )}

        {/* Metadata Grid */}
        <div className="grid grid-cols-2 gap-3">
          <MetadataItem label="Source" value={displayTask.source ?? '—'} mono />
          <MetadataItem label="State" value={displayTask.state ?? '—'} />
          {displayTask.branch && (
            <MetadataItem label="Branch" value={displayTask.branch} mono />
          )}
          {displayTask.worktreePath && (
            <MetadataItem
              label="Worktree"
              value={displayTask.worktreePath.split('/').slice(-2).join('/')}
              mono
            />
          )}
        </div>
      </div>
    </div>
  )
}

function SourceBadge({ type }: { type: string }) {
  const config: Record<string, { color: string; icon: ReactNode }> = {
    github: {
      color: 'badge-neutral',
      icon: (
        <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
          <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
        </svg>
      ),
    },
    gitlab: {
      color: 'badge-warning',
      icon: (
        <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
          <path d="M22.65 14.39L12 22.13 1.35 14.39a.84.84 0 0 1-.3-.94l1.22-3.78 2.44-7.51A.42.42 0 0 1 4.82 2a.43.43 0 0 1 .58 0 .42.42 0 0 1 .11.18l2.44 7.49h8.1l2.44-7.51A.42.42 0 0 1 18.6 2a.43.43 0 0 1 .58 0 .42.42 0 0 1 .11.18l2.44 7.51L23 13.45a.84.84 0 0 1-.35.94z" />
        </svg>
      ),
    },
    wrike: {
      color: 'badge-success',
      icon: (
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
        </svg>
      ),
    },
    file: {
      color: 'badge-info',
      icon: (
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
      ),
    },
    unknown: {
      color: 'badge-ghost',
      icon: (
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      ),
    },
  }

  const { color, icon } = config[type] || config.unknown

  return (
    <div className={`badge ${color} gap-1 font-medium`}>
      {icon}
      <span className="capitalize">{type}</span>
    </div>
  )
}

function MetadataItem({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="bg-base-200/50 rounded-lg p-3">
      <div className="text-xs text-base-content/50 mb-1">{label}</div>
      <div className={`text-sm truncate ${mono ? 'font-mono' : ''}`}>{value}</div>
    </div>
  )
}
