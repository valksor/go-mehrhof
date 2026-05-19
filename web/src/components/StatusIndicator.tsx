export type StatusType = 'idle' | 'running' | 'success' | 'warning' | 'error'

interface StatusIndicatorProps {
  status: StatusType
  label?: string
  showLabel?: boolean
  size?: 'sm' | 'md' | 'lg'
}

export function StatusIndicator({ status, label, showLabel = true, size = 'md' }: StatusIndicatorProps) {
  const sizeClasses = {
    sm: 'w-1.5 h-1.5',
    md: 'w-2 h-2',
    lg: 'w-2.5 h-2.5',
  }

  const statusLabel = label || getDefaultLabel(status)

  return (
    <span className="inline-flex items-center gap-2">
      <span className={`status-dot status-${status} ${sizeClasses[size]}`} />
      {showLabel && <span className="text-sm text-base-content/70">{statusLabel}</span>}
    </span>
  )
}

function getDefaultLabel(status: StatusType): string {
  switch (status) {
    case 'idle':
      return 'Idle'
    case 'running':
      return 'Running...'
    case 'success':
      return 'Completed'
    case 'warning':
      return 'Warning'
    case 'error':
      return 'Failed'
    default:
      return status
  }
}

// Badge variant with background
interface StatusBadgeProps {
  status: StatusType
  label?: string
}

export function StatusBadge({ status, label }: StatusBadgeProps) {
  const badgeClasses = {
    idle: 'badge-ghost',
    running: 'badge-warning',
    success: 'badge-success',
    warning: 'badge-warning',
    error: 'badge-error',
  }

  return (
    <span className={`badge ${badgeClasses[status]} gap-1.5`}>
      <span className={`status-dot status-${status} w-1.5 h-1.5`} />
      {label || getDefaultLabel(status)}
    </span>
  )
}
