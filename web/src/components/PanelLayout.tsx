import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useLayoutStore } from '../stores/layoutStore'
import { useProjectStore } from '../stores/projectStore'
import { TabBar } from './TabBar'
import { TabPanel } from './TabPanel'

interface PanelLayoutProps {
  leftContent: ReactNode
  rightContent: ReactNode
  header?: ReactNode
}

export function PanelLayout({ leftContent, rightContent, header }: PanelLayoutProps) {
  const { panelSizes, setPanelSize, bottomPanelVisible, toggleBottomPanel, openTab, setActiveTab } = useLayoutStore()
  const [leftWidth, setLeftWidth] = useState(panelSizes.left)
  const [rightWidth, setRightWidth] = useState(panelSizes.right)
  const [isResizingLeft, setIsResizingLeft] = useState(false)
  const [isResizingRight, setIsResizingRight] = useState(false)
  const [mobilePanel, setMobilePanel] = useState<'main' | 'left' | 'right'>('main')
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  // Handle mouse move for resizing
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!containerRef.current) return
      const containerRect = containerRef.current.getBoundingClientRect()
      const containerWidth = containerRect.width

      if (isResizingLeft) {
        const newWidth = ((e.clientX - containerRect.left) / containerWidth) * 100
        const clamped = Math.max(15, Math.min(35, newWidth))
        setLeftWidth(clamped)
      }

      if (isResizingRight) {
        const newWidth = ((containerRect.right - e.clientX) / containerWidth) * 100
        const clamped = Math.max(15, Math.min(35, newWidth))
        setRightWidth(clamped)
      }
    }

    const handleMouseUp = () => {
      if (isResizingLeft) {
        setPanelSize('left', leftWidth)
      }
      if (isResizingRight) {
        setPanelSize('right', rightWidth)
      }
      setIsResizingLeft(false)
      setIsResizingRight(false)
    }

    if (isResizingLeft || isResizingRight) {
      document.addEventListener('mousemove', handleMouseMove)
      document.addEventListener('mouseup', handleMouseUp)
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
    }

    return () => {
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
  }, [isResizingLeft, isResizingRight, leftWidth, rightWidth, setPanelSize])

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      {header && (
        <div className="flex-shrink-0">
          {header}
        </div>
      )}

      {/* Main content area */}
      <div className="flex-1 min-h-0 flex flex-col">
        {/* Desktop: Three-column layout, Mobile: Single panel with navigation */}
        <div ref={containerRef} className="flex-1 min-h-0 flex">
          {/* Left sidebar - hidden on mobile unless active */}
          <aside
            aria-label="Left sidebar"
            className={`flex-shrink-0 overflow-hidden bg-base-200 ${
              mobilePanel === 'left' ? 'flex md:flex' : 'hidden md:flex'
            } flex-col`}
            style={{ width: mobilePanel === 'left' ? '100%' : undefined }}
          >
            <div
              className="h-full overflow-y-auto p-3 space-y-3 w-full md:w-auto"
              style={{ width: mobilePanel !== 'left' ? `${leftWidth}vw` : undefined }}
            >
              {leftContent}
            </div>
          </aside>

          {/* Left resize handle - hidden on mobile */}
          <div
            role="slider"
            aria-label="Resize left sidebar"
            aria-orientation="horizontal"
            aria-valuenow={leftWidth}
            aria-valuemin={15}
            aria-valuemax={35}
            tabIndex={0}
            className={`hidden md:block w-1 flex-shrink-0 cursor-col-resize transition-colors duration-150 focus:outline-none ${
              isResizingLeft ? 'bg-primary/60' : 'bg-transparent hover:bg-primary/30 focus-visible:bg-primary/50'
            }`}
            onMouseDown={() => setIsResizingLeft(true)}
            onKeyDown={(e) => {
              if (e.key === 'ArrowLeft') {
                e.preventDefault()
                const newWidth = Math.max(15, leftWidth - 2)
                setLeftWidth(newWidth)
                setPanelSize('left', newWidth)
              }
              if (e.key === 'ArrowRight') {
                e.preventDefault()
                const newWidth = Math.min(35, leftWidth + 2)
                setLeftWidth(newWidth)
                setPanelSize('left', newWidth)
              }
            }}
          />

          {/* Main content area with tabs - hidden on mobile unless active */}
          <div className={`flex-1 min-w-0 flex-col bg-base-100 ${
            mobilePanel === 'main' ? 'flex' : 'hidden md:flex'
          }`}>
            <TabBar />
            <div className="flex-1 min-h-0 overflow-hidden">
              <TabPanel />
            </div>
          </div>

          {/* Right resize handle - hidden on mobile */}
          <div
            role="slider"
            aria-label="Resize right sidebar"
            aria-orientation="horizontal"
            aria-valuenow={rightWidth}
            aria-valuemin={15}
            aria-valuemax={35}
            tabIndex={0}
            className={`hidden md:block w-1 flex-shrink-0 cursor-col-resize transition-colors duration-150 focus:outline-none ${
              isResizingRight ? 'bg-primary/60' : 'bg-transparent hover:bg-primary/30 focus-visible:bg-primary/50'
            }`}
            onMouseDown={() => setIsResizingRight(true)}
            onKeyDown={(e) => {
              if (e.key === 'ArrowLeft') {
                e.preventDefault()
                const newWidth = Math.min(35, rightWidth + 2)
                setRightWidth(newWidth)
                setPanelSize('right', newWidth)
              }
              if (e.key === 'ArrowRight') {
                e.preventDefault()
                const newWidth = Math.max(15, rightWidth - 2)
                setRightWidth(newWidth)
                setPanelSize('right', newWidth)
              }
            }}
          />

          {/* Right sidebar - hidden on mobile unless active */}
          <aside
            aria-label="Right sidebar"
            className={`flex-shrink-0 overflow-hidden bg-base-200 ${
              mobilePanel === 'right' ? 'flex md:flex' : 'hidden md:flex'
            } flex-col`}
            style={{ width: mobilePanel === 'right' ? '100%' : undefined }}
          >
            <div
              className="h-full overflow-y-auto p-3 space-y-3 w-full md:w-auto"
              style={{ width: mobilePanel !== 'right' ? `${rightWidth}vw` : undefined }}
            >
              {rightContent}
            </div>
          </aside>
        </div>

        {/* Bottom panel (collapsible) - hidden on mobile */}
        {bottomPanelVisible && (
          <div className="hidden md:block flex-shrink-0 border-t border-base-300" style={{ height: '200px' }}>
            <div className="relative">
              <button
                onClick={toggleBottomPanel}
                className="absolute left-1/2 -translate-x-1/2 -top-3 px-2 py-0.5 rounded bg-base-300 text-xs text-base-content/60 hover:text-base-content hover:bg-base-200 transition-colors z-10"
                aria-label="Collapse output panel"
              >
                <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                </svg>
              </button>
            </div>
            <BottomPanelContent />
          </div>
        )}
      </div>

      {/* Show bottom panel toggle when hidden - hidden on mobile */}
      {!bottomPanelVisible && (
        <button
          onClick={toggleBottomPanel}
          className="hidden md:flex items-center justify-center gap-2 py-1 text-xs text-base-content/50 hover:text-base-content hover:bg-base-200 transition-colors border-t border-base-300"
        >
          <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
          </svg>
          Show Output Panel
        </button>
      )}

      {/* Mobile navigation bar */}
      <div className="md:hidden flex-shrink-0 border-t border-base-300 bg-base-200">
        {/* Mobile "More" menu overlay */}
        {mobileMenuOpen && (
          <div className="border-b border-base-300 bg-base-100 p-2 grid grid-cols-3 gap-1">
            {[
              { label: 'Output', panel: 'main' as const, tab: 'output' },
              { label: 'Files', panel: 'main' as const, tab: 'files' },
              { label: 'Screenshots', panel: 'main' as const, tab: 'screenshots' },
              { label: 'Browser', panel: 'main' as const, tab: 'browser' },
              { label: 'Jobs', panel: 'main' as const, tab: 'jobs' },
              { label: 'Reviews', panel: 'main' as const, tab: 'review' },
            ].map(item => (
              <button
                key={item.label}
                onClick={() => {
                  setMobilePanel(item.panel)
                  openTab({
                    id: `${item.tab}-mobile`,
                    type: item.tab as import('../stores/layoutStore').TabType,
                    title: item.label,
                    closeable: true,
                  })
                  setActiveTab(`${item.tab}-mobile`)
                  setMobileMenuOpen(false)
                }}
                className="btn btn-ghost btn-sm min-h-[44px] text-xs"
              >
                {item.label}
              </button>
            ))}
          </div>
        )}
        <div className="flex" role="tablist">
          <button
            onClick={() => { setMobilePanel('left'); setMobileMenuOpen(false) }}
            role="tab"
            aria-selected={mobilePanel === 'left'}
            className={`flex-1 flex flex-col items-center gap-1 py-2 min-h-[44px] text-xs transition-colors ${
              mobilePanel === 'left' ? 'text-primary bg-primary/10' : 'text-base-content/60'
            }`}
          >
            <svg aria-hidden="true" className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
            Task
          </button>
          <button
            onClick={() => { setMobilePanel('main'); setMobileMenuOpen(false) }}
            role="tab"
            aria-selected={mobilePanel === 'main' && !mobileMenuOpen}
            className={`flex-1 flex flex-col items-center gap-1 py-2 min-h-[44px] text-xs transition-colors ${
              mobilePanel === 'main' && !mobileMenuOpen ? 'text-primary bg-primary/10' : 'text-base-content/60'
            }`}
          >
            <svg aria-hidden="true" className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            Chat
          </button>
          <button
            onClick={() => { setMobilePanel('right'); setMobileMenuOpen(false) }}
            role="tab"
            aria-selected={mobilePanel === 'right'}
            className={`flex-1 flex flex-col items-center gap-1 py-2 min-h-[44px] text-xs transition-colors ${
              mobilePanel === 'right' ? 'text-primary bg-primary/10' : 'text-base-content/60'
            }`}
          >
            <svg aria-hidden="true" className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
            Actions
          </button>
          <button
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
            aria-haspopup="menu"
            aria-expanded={mobileMenuOpen}
            aria-label="More panels"
            className={`flex-1 flex flex-col items-center gap-1 py-2 min-h-[44px] text-xs transition-colors ${
              mobileMenuOpen ? 'text-primary bg-primary/10' : 'text-base-content/60'
            }`}
          >
            <svg aria-hidden="true" className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h.01M12 12h.01M19 12h.01M6 12a1 1 0 11-2 0 1 1 0 012 0zm7 0a1 1 0 11-2 0 1 1 0 012 0zm7 0a1 1 0 11-2 0 1 1 0 012 0z" />
            </svg>
            More
          </button>
        </div>
      </div>
    </div>
  )
}

// Bottom panel content - shows output/terminal
function BottomPanelContent() {
  const { output, clearOutput } = useProjectStore()
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [output])

  return (
    <div className="h-full flex flex-col bg-base-200">
      <div className="flex items-center justify-between px-3 py-2 border-b border-base-300">
        <span className="text-sm font-medium text-base-content/70 flex items-center gap-2">
          <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
          Output
        </span>
        <button
          onClick={clearOutput}
          className="btn btn-ghost btn-xs"
        >
          Clear
        </button>
      </div>
      <div className="flex-1 overflow-auto p-2">
        <div role="log" aria-label="Task output" className="bg-neutral rounded-lg p-3 h-full overflow-auto font-mono text-sm text-neutral-content">
          {output.length === 0 ? (
            <div className="text-neutral-content/50 text-center py-4">
              No output yet
            </div>
          ) : (
            <div className="space-y-0.5">
              {output.map((line, i) => (
                <div
                  key={i}
                  className={`leading-relaxed ${
                    line.startsWith('ERROR') || line.startsWith('error') ? 'text-error' :
                    line.startsWith('WARN') || line.startsWith('warn') ? 'text-warning' :
                    line.startsWith('✓') || line.startsWith('success') ? 'text-success' :
                    ''
                  }`}
                >
                  {line}
                </div>
              ))}
              <div ref={endRef} />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
