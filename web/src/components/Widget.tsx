import { useState, useRef, useEffect, useId, type ReactNode } from 'react'

interface WidgetProps {
  id: string
  title: string
  icon?: ReactNode
  defaultCollapsed?: boolean
  closeable?: boolean
  onClose?: () => void
  actions?: ReactNode
  children: ReactNode
  className?: string
  noPadding?: boolean
}

export function Widget({
  id,
  title,
  icon,
  defaultCollapsed = false,
  closeable = false,
  onClose,
  actions,
  children,
  className = '',
  noPadding = false,
}: WidgetProps) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed)
  const contentRef = useRef<HTMLDivElement>(null)
  const [contentHeight, setContentHeight] = useState<number | 'auto'>('auto')
  const contentId = useId()

  // Measure content height for smooth animation
  useEffect(() => {
    if (contentRef.current) {
      setContentHeight(contentRef.current.scrollHeight)
    }
  }, [children])

  const toggleCollapsed = () => {
    setCollapsed(!collapsed)
  }

  return (
    <section
      className={`widget ${collapsed ? 'widget-collapsed' : ''} ${className}`}
      data-widget-id={id}
    >
      {/* Header */}
      <div className="widget-header">
        <button
          onClick={toggleCollapsed}
          className="widget-chevron"
          aria-expanded={!collapsed}
          aria-controls={contentId}
          aria-label={collapsed ? 'Expand' : 'Collapse'}
        >
          <svg
            aria-hidden="true"
            className={`w-4 h-4 transition-transform duration-200 ${collapsed ? '' : 'rotate-90'}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M9 5l7 7-7 7"
            />
          </svg>
        </button>

        <div className="widget-title">
          {icon && <span className="widget-icon">{icon}</span>}
          <span>{title}</span>
        </div>

        <div className="widget-actions">
          {actions}
          {closeable && (
            <button
              onClick={onClose}
              className="widget-close"
              aria-label="Close widget"
            >
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          )}
        </div>
      </div>

      {/* Content with animated collapse */}
      <div
        id={contentId}
        className="widget-content-wrapper"
        style={{
          height: collapsed ? 0 : contentHeight,
          overflow: 'hidden',
          transition: 'height 0.2s ease-out',
        }}
      >
        <div
          ref={contentRef}
          className={`widget-body ${noPadding ? '' : 'widget-body-padded'}`}
        >
          {children}
        </div>
      </div>
    </section>
  )
}

// Icon components for common use
export function TaskIcon() {
  return (
    <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
    </svg>
  )
}

export function FilesIcon() {
  return (
    <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
    </svg>
  )
}

export function OutputIcon() {
  return (
    <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
    </svg>
  )
}

export function CheckpointsIcon() {
  return (
    <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  )
}

export function AgentIcon() {
  return (
    <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
    </svg>
  )
}
