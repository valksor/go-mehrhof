import { useState, useRef, useEffect } from 'react'
import { useLayoutStore, type Tab, type TabType } from '../stores/layoutStore'

interface TabBarProps {
  className?: string
}

const TAB_TYPE_OPTIONS: Array<{ type: TabType; label: string; icon: string }> = [
  { type: 'chat', label: 'Chat', icon: 'chat' },
  { type: 'spec', label: 'Spec', icon: 'document' },
  { type: 'screenshots', label: 'Screenshots', icon: 'camera' },
  { type: 'jobs', label: 'Jobs', icon: 'jobs' },
  { type: 'files', label: 'Files', icon: 'files' },
  { type: 'browser', label: 'Browser', icon: 'browser' },
]

export function TabBar({ className = '' }: TabBarProps) {
  const { tabs, activeTabId, setActiveTab, closeTab, openTab } = useLayoutStore()
  const [showAddMenu, setShowAddMenu] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const tabListRef = useRef<HTMLDivElement>(null)

  // Close menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowAddMenu(false)
      }
    }
    if (showAddMenu) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [showAddMenu])

  const handleTabClick = (tabId: string) => {
    setActiveTab(tabId)
  }

  const handleTabClose = (e: React.MouseEvent, tabId: string) => {
    e.stopPropagation()
    closeTab(tabId)
  }

  const handleAddTab = (type: TabType) => {
    const newId = `tab-${crypto.randomUUID()}`
    const labels: Record<TabType, string> = {
      chat: 'Chat',
      output: 'Output',
      file: 'File',
      diff: 'Diff',
      spec: 'Spec',
      agent: 'Agent',
      screenshots: 'Screenshots',
      jobs: 'Jobs',
      files: 'Files',
      browser: 'Browser',
      task: 'Task',
      review: 'Review',
      filechanges: 'File Changes',
      'submit-preview': 'PR Preview',
    }
    openTab({
      id: newId,
      type,
      title: labels[type],
      closeable: true,
    })
    setShowAddMenu(false)
  }

  // Arrow key navigation within the tablist
  const handleTabKeyDown = (e: React.KeyboardEvent, index: number) => {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    const tabElements = tabListRef.current?.querySelectorAll<HTMLElement>('[role="tab"]')
    if (!tabElements) return
    const next = e.key === 'ArrowRight'
      ? (index + 1) % tabElements.length
      : (index - 1 + tabElements.length) % tabElements.length
    const nextElement = tabElements[next]
    nextElement.focus()
    // Derive tab ID from DOM element to handle potential order mismatches
    const tabId = nextElement.dataset.tabId
    if (tabId) setActiveTab(tabId)
  }

  return (
    <div className={`tab-bar ${className}`}>
      {/* tablist wraps only the actual tabs, not the add button */}
      <div
        ref={tabListRef}
        role="tablist"
        aria-label="Open tabs"
        className="contents"
      >
        {tabs.map((tab, index) => (
          <div
            key={tab.id}
            role="tab"
            data-tab-id={tab.id}
            tabIndex={activeTabId === tab.id ? 0 : -1}
            aria-selected={activeTabId === tab.id}
            className={`tab ${activeTabId === tab.id ? 'tab-active' : ''}`}
            onClick={() => handleTabClick(tab.id)}
            onKeyDown={(e) => {
              // Handle Enter/Space to activate tab (standard button behavior)
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                handleTabClick(tab.id)
              }
              handleTabKeyDown(e, index)
            }}
          >
            <TabIcon type={tab.type} />
            <span className="truncate max-w-[120px]">{tab.title}</span>
            {tab.closeable !== false && (
              <button
                type="button"
                className="tab-close"
                onClick={(e) => handleTabClose(e, tab.id)}
                aria-label={`Close ${tab.title}`}
              >
                <svg aria-hidden="true" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            )}
          </div>
        ))}
      </div>

      {/* Add tab button with dropdown — outside tablist, it is not a tab */}
      <div className="relative" ref={menuRef}>
        <button
          type="button"
          aria-label="Add new tab"
          aria-expanded={showAddMenu}
          aria-haspopup="menu"
          className="flex items-center justify-center w-8 h-8 rounded border border-dashed border-base-content/30 hover:border-primary hover:bg-base-200 cursor-pointer"
          onClick={(e) => {
            e.stopPropagation()
            setShowAddMenu(!showAddMenu)
          }}
        >
          <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>

        {showAddMenu && (
          <div
            role="menu"
            aria-label="Tab types"
            className="absolute top-full left-0 mt-1 py-1 bg-base-200 rounded-lg shadow-lg border border-base-300 min-w-[120px] z-50"
          >
            {TAB_TYPE_OPTIONS.map((option) => (
              <button
                key={option.type}
                role="menuitem"
                className="w-full px-3 py-1.5 text-left text-sm hover:bg-base-300 flex items-center gap-2 transition-colors"
                onClick={() => handleAddTab(option.type)}
              >
                <TabIcon type={option.type} />
                {option.label}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function TabIcon({ type }: { type: Tab['type'] }) {
  switch (type) {
    case 'file':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
      )
    case 'diff':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
        </svg>
      )
    case 'spec':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
        </svg>
      )
    case 'agent':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
        </svg>
      )
    case 'chat':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
        </svg>
      )
    case 'jobs':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 10h16M4 14h16M4 18h16" />
        </svg>
      )
    case 'files':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
        </svg>
      )
    case 'browser':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
        </svg>
      )
    case 'screenshots':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      )
    case 'task':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
        </svg>
      )
    case 'review':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    case 'filechanges':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16m-7 6h7" />
        </svg>
      )
    case 'submit-preview':
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4M16 17H4m0 0l4 4m-4-4l4-4" />
        </svg>
      )
    case 'output':
    default:
      return (
        <svg aria-hidden="true" className="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
        </svg>
      )
  }
}
