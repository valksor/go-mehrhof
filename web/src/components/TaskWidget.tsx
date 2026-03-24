import { useState, useMemo, useRef, useCallback, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { FilePicker } from './FilePicker'
import type { FailureClass } from '../lib/events'

interface ContextRef {
  type: string
  ref: string
  label: string
}

interface FileEntry {
  rel_path: string
  is_dir: boolean
}

function contextIcon(type: string) {
  switch (type) {
    case 'file': return '📄'
    case 'symbol': return '🔗'
    case 'commit': return '📌'
    default: return '@'
  }
}

type DetectedProvider = { name: string; icon: string; example: string } | null

function detectProvider(url: string): DetectedProvider {
  const u = url.trim().toLowerCase()
  if (u.includes('github.com') || u.includes('github:')) {
    return { name: 'GitHub', icon: 'GH', example: 'github.com/owner/repo/issues/123' }
  }
  if (u.includes('gitlab.com') || u.includes('gitlab:')) {
    return { name: 'GitLab', icon: 'GL', example: 'gitlab.com/group/project/-/issues/456' }
  }
  if (u.includes('linear.app') || u.includes('linear:')) {
    return { name: 'Linear', icon: 'LN', example: 'linear.app/team/issue/ABC-123' }
  }
  if (u.includes('wrike.com') || u.includes('wrike:')) {
    return { name: 'Wrike', icon: 'WR', example: 'wrike.com/open.htm?id=12345' }
  }

  return null
}

interface TaskWidgetProps {
  embedded?: boolean
}

function stateBadgeClass(state: string, failureClass?: FailureClass): string {
  if (state === 'failed') {
    switch (failureClass) {
      case 'recoverable': return 'badge-warning'
      case 'degraded': return 'badge-warning'
      case 'skippable': return 'badge-info'
      case 'hard_stop':
      default: return 'badge-error'
    }
  }
  if (state === 'implemented') return 'badge-success'
  if (state === 'planned') return 'badge-primary'
  if (state === 'planning' || state === 'implementing') return 'badge-warning'
  if (state === 'submitted') return 'badge-secondary'
  return 'badge-ghost'
}

function errorBannerClass(failureClass?: FailureClass): string {
  switch (failureClass) {
    case 'recoverable': return 'text-warning bg-warning/10 border-warning/20'
    case 'degraded': return 'text-warning bg-warning/10 border-warning/20'
    case 'skippable': return 'text-info bg-info/10 border-info/20'
    case 'hard_stop':
    default: return 'text-error bg-error/10 border-error/20'
  }
}

export function TaskWidget({ embedded = false }: TaskWidgetProps) {
  const { task, state, start, quickStart, queueTask, loading, error, connected, connecting, undo, reset, retry, phaseError, needsRecovery, ciFixStatus, tags, addTag, removeTag } = useProjectStore()
  const [inputMode, setInputMode] = useState<'quick' | 'file' | 'url'>('quick')
  const [taskDescription, setTaskDescription] = useState('')
  const [urlInput, setUrlInput] = useState('')
  const [selectedFile, setSelectedFile] = useState('')
  const [showFilePicker, setShowFilePicker] = useState(false)
  const [quickMode, setQuickMode] = useState(false)
  const [showTagInput, setShowTagInput] = useState(false)
  const tagCancelledRef = useRef(false)
  const [newTag, setNewTag] = useState('')
  const detectedProvider = useMemo(() => detectProvider(urlInput), [urlInput])

  // Context items (@-mentions)
  // Prefix conventions: @ = file, @@ = symbol, @# = commit
  const [contextItems, setContextItems] = useState<ContextRef[]>([])
  const [atQuery, setAtQuery] = useState('')
  const [atMode, setAtMode] = useState<'file' | 'symbol' | 'commit' | null>(null)
  const [atResults, setAtResults] = useState<Array<{ label: string; value: string }>>([])
  const [atIndex, setAtIndex] = useState(0)
  const [showAtMenu, setShowAtMenu] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const searchContext = useCallback(async (mode: 'file' | 'symbol' | 'commit', query: string) => {
    const client = useProjectStore.getState().client
    if (!client || !connected || query.length < 1) {
      setAtResults([])
      return
    }
    try {
      if (mode === 'file') {
        const result = await client.call<{ entries: FileEntry[] }>('files.search', { query, max_results: 8 })
        setAtResults((result.entries || []).map(e => ({ label: `${e.is_dir ? '📁' : '📄'} ${e.rel_path}`, value: e.rel_path })))
      } else if (mode === 'symbol') {
        const result = await client.call<{ content: string; label: string }>('context.resolve', { type: 'symbol', ref: query })
        // Parse symbol results into individual lines
        const lines = result.content.split('\n').filter(Boolean).slice(0, 8)
        setAtResults(lines.map(line => {
          // Extract symbol name from format "kind Name (file:line, package pkg)"
          const match = line.replace(/^- /, '').match(/^\S+\s+(\S+)/)
          const symbolName = match ? match[1] : query
          return { label: `🔗 ${line.replace(/^- /, '')}`, value: symbolName }
        }))
      } else if (mode === 'commit') {
        const result = await client.call<{ commits: Array<{ sha: string; message: string }> }>('git.log', { count: 10 })
        const commits = result.commits || []
        setAtResults(commits.filter(c => c.sha.startsWith(query) || c.message.toLowerCase().includes(query.toLowerCase()))
          .slice(0, 8).map(c => ({ label: `📌 ${c.sha.slice(0, 8)} ${c.message}`, value: c.sha })))
      }
      setAtIndex(0)
    } catch {
      if (mode === 'commit') {
        // Fallback: try context.resolve directly for single commit
        try {
          const client = useProjectStore.getState().client
          if (client && query.length >= 4) {
            await client.call<{ content: string }>('context.resolve', { type: 'commit', ref: query })
            setAtResults([{ label: `📌 ${query}`, value: query }])
          }
        } catch { /* no results */ }
      }
      setAtResults([])
    }
  }, [connected])

  useEffect(() => {
    if (!showAtMenu || !atQuery || !atMode) return
    const timer = setTimeout(() => searchContext(atMode, atQuery), 150)
    return () => clearTimeout(timer)
  }, [atQuery, atMode, showAtMenu, searchContext])

  const handleDescriptionChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = e.target.value
    setTaskDescription(value)

    const cursorPos = e.target.selectionStart
    const textBeforeCursor = value.slice(0, cursorPos)

    // Detect @-mention mode: @@ = symbol, @# = commit, @ = file
    const symbolMatch = textBeforeCursor.match(/@@(\S*)$/)
    const commitMatch = textBeforeCursor.match(/@#(\S*)$/)
    const fileMatch = textBeforeCursor.match(/@(\S*)$/)

    if (symbolMatch) {
      setShowAtMenu(true)
      setAtMode('symbol')
      setAtQuery(symbolMatch[1])
    } else if (commitMatch) {
      setShowAtMenu(true)
      setAtMode('commit')
      setAtQuery(commitMatch[1])
    } else if (fileMatch && !fileMatch[1].startsWith('@') && !fileMatch[1].startsWith('#')) {
      setShowAtMenu(true)
      setAtMode('file')
      setAtQuery(fileMatch[1])
    } else {
      setShowAtMenu(false)
      setAtMode(null)
      setAtQuery('')
    }
  }

  const insertContextRef = (item: { label: string; value: string }) => {
    const cursorPos = textareaRef.current?.selectionStart || taskDescription.length
    const textBefore = taskDescription.slice(0, cursorPos)
    const textAfter = taskDescription.slice(cursorPos)
    // Remove the @-trigger text
    const triggerMatch = textBefore.match(/@@?\S*$|@#\S*$/)
    if (triggerMatch) {
      const triggerPos = cursorPos - triggerMatch[0].length
      setTaskDescription(taskDescription.slice(0, triggerPos) + textAfter)
    }
    const type = atMode || 'file'
    setContextItems(prev => [...prev, { type, ref: item.value, label: item.value }])
    setShowAtMenu(false)
    setAtMode(null)
    setAtQuery('')
    textareaRef.current?.focus()
  }

  const removeContextItem = (index: number) => {
    setContextItems(prev => prev.filter((_, i) => i !== index))
  }

  // When a task is active, queue instead of start
  const hasActiveTask = task && state !== 'none'

  const loadOrQueue = (source: string, title?: string) => {
    if (hasActiveTask) {
      queueTask(source, title)
    } else if (quickMode) {
      quickStart(source)
    } else {
      start(source)
    }
  }

  const handleQuickLoad = () => {
    if (taskDescription.trim()) {
      const source = `empty:${taskDescription.trim()}`
      if (contextItems.length > 0) {
        // Use start directly to pass context items
        start(source, quickMode, contextItems)
      } else {
        loadOrQueue(source, taskDescription.trim())
      }
      setTaskDescription('')
      setContextItems([])
    }
  }

  const handleUrlLoad = () => {
    if (urlInput.trim()) {
      loadOrQueue(urlInput.trim())
      setUrlInput('')
    }
  }

  const handleFileLoad = () => {
    if (selectedFile) {
      const source = `file:${selectedFile}`
      loadOrQueue(source, selectedFile.split('/').pop() || selectedFile)
      setSelectedFile('')
    }
  }

  const handleFileSelect = (path: string) => {
    setSelectedFile(path)
  }

  const handleQuickKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && e.ctrlKey && taskDescription.trim() && connected && !loading) {
      handleQuickLoad()
    }
  }

  const handleUrlKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && urlInput.trim() && connected && !loading) {
      handleUrlLoad()
    }
  }

  // Content when no task is loaded
  const loadTaskContent = (
    <>
      {/* Tab selector */}
      <div role="tablist" className="tabs tabs-boxed tabs-sm mb-3 bg-base-300 gap-1 p-1">
        <button
          role="tab"
          className={`tab rounded ${inputMode === 'quick' ? 'tab-active' : 'border border-base-content/10'}`}
          onClick={() => setInputMode('quick')}
        >
          Quick Task
        </button>
        <button
          role="tab"
          className={`tab rounded ${inputMode === 'file' ? 'tab-active' : 'border border-base-content/10'}`}
          onClick={() => setInputMode('file')}
        >
          From File
        </button>
        <button
          role="tab"
          className={`tab rounded ${inputMode === 'url' ? 'tab-active' : 'border border-base-content/10'}`}
          onClick={() => setInputMode('url')}
        >
          From URL
        </button>
      </div>

      {/* Quick Task tab */}
      {inputMode === 'quick' && (
        <div className="space-y-2">
          <div className="relative">
            <textarea
              ref={textareaRef}
              value={taskDescription}
              onChange={handleDescriptionChange}
              onKeyDown={(e) => {
                if (showAtMenu && atResults.length > 0) {
                  if (e.key === 'ArrowDown') { e.preventDefault(); setAtIndex(i => Math.min(i + 1, atResults.length - 1)) }
                  else if (e.key === 'ArrowUp') { e.preventDefault(); setAtIndex(i => Math.max(i - 1, 0)) }
                  else if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); insertContextRef(atResults[atIndex]) }
                  else if (e.key === 'Escape') { setShowAtMenu(false) }
                  return
                }
                handleQuickKeyDown(e)
              }}
              placeholder="Describe what you want to work on... (@ file, @@ symbol, @# commit)"
              className="textarea textarea-bordered w-full text-sm resize-none"
              rows={3}
              disabled={loading}
            />
            {showAtMenu && atResults.length > 0 && (
              <div className="absolute z-50 left-0 right-0 mt-1 bg-base-100 border border-base-300 rounded-lg shadow-lg max-h-48 overflow-y-auto">
                {atMode && (
                  <div className="px-3 py-1 text-xs text-base-content/40 border-b border-base-300">
                    {atMode === 'file' ? 'Files' : atMode === 'symbol' ? 'Symbols' : 'Commits'}
                  </div>
                )}
                {atResults.map((item, i) => (
                  <button
                    key={`${item.value}-${i}`}
                    className={`w-full text-left px-3 py-1.5 text-sm font-mono truncate hover:bg-base-200 ${i === atIndex ? 'bg-base-200' : ''}`}
                    onMouseDown={(e) => { e.preventDefault(); insertContextRef(item) }}
                  >
                    {item.label}
                  </button>
                ))}
              </div>
            )}
          </div>
          {contextItems.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {contextItems.map((ci, i) => (
                <span key={`${ci.type}-${ci.ref}`} className="badge badge-sm badge-outline gap-1">
                  {contextIcon(ci.type)} {ci.label || ci.ref}
                  <button className="cursor-pointer hover:text-error" onClick={() => removeContextItem(i)} aria-label={`Remove ${ci.ref}`}>&times;</button>
                </span>
              ))}
            </div>
          )}
          <div className="flex justify-between items-center">
            <span className="text-xs text-base-content/50">Ctrl+Enter to load · @ file · @@ symbol · @# commit</span>
            <button
              onClick={handleQuickLoad}
              disabled={loading || !taskDescription.trim() || !connected}
              className="btn btn-primary btn-sm"
            >
              {loading ? <span className="loading loading-spinner loading-xs" /> : hasActiveTask ? 'Queue Task' : quickMode ? 'Quick Fix' : 'Load Task'}
            </button>
          </div>
        </div>
      )}

      {/* From File tab */}
      {inputMode === 'file' && (
        <div className="space-y-2">
          <div className="flex gap-2 items-center">
            <button
              onClick={() => setShowFilePicker(true)}
              disabled={loading || !connected}
              className="btn btn-outline btn-sm gap-2"
            >
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
              </svg>
              Browse
            </button>
            {selectedFile ? (
              <code className="flex-1 text-sm bg-base-300 px-2 py-1 rounded truncate">{selectedFile}</code>
            ) : (
              <span className="flex-1 text-sm text-base-content/50">No file selected</span>
            )}
          </div>
          {selectedFile && (
            <div className="flex justify-end">
              <button
                onClick={handleFileLoad}
                disabled={loading || !connected}
                className="btn btn-primary btn-sm"
              >
                {loading ? <span className="loading loading-spinner loading-xs" /> : hasActiveTask ? 'Queue Task' : quickMode ? 'Quick Fix' : 'Load Task'}
              </button>
            </div>
          )}
        </div>
      )}

      {/* From URL tab */}
      {inputMode === 'url' && (
        <div className="space-y-2">
          <div className="relative">
            <input
              type="text"
              value={urlInput}
              onChange={e => setUrlInput(e.target.value)}
              onKeyDown={handleUrlKeyDown}
              placeholder="github.com/owner/repo/issues/123"
              className={`input input-bordered w-full font-mono text-sm input-sm ${detectedProvider ? 'pr-14' : ''}`}
              disabled={loading}
            />
            {detectedProvider && (
              <span className="absolute right-2 top-1/2 -translate-y-1/2 badge badge-sm badge-primary">
                {detectedProvider.icon}
              </span>
            )}
          </div>
          <div className="flex justify-between items-center">
            <span className="text-xs text-base-content/50">
              {detectedProvider
                ? `${detectedProvider.name} detected`
                : 'GitHub, GitLab, Linear, or Wrike URLs'}
            </span>
            <button
              onClick={handleUrlLoad}
              disabled={loading || !urlInput.trim() || !connected}
              className="btn btn-primary btn-sm"
            >
              {loading ? <span className="loading loading-spinner loading-xs" /> : hasActiveTask ? 'Queue Task' : quickMode ? 'Quick Fix' : 'Load Task'}
            </button>
          </div>
        </div>
      )}

      {/* Quick Fix toggle */}
      {!hasActiveTask && (
        <label className="flex items-center gap-2 cursor-pointer mt-3">
          <input
            type="checkbox"
            className="toggle toggle-sm toggle-primary"
            checked={quickMode}
            onChange={e => setQuickMode(e.target.checked)}
            disabled={loading}
          />
          <span className="text-sm text-base-content/70">Quick Fix</span>
          <span className="text-xs text-base-content/40">(skip planning, auto-submit)</span>
        </label>
      )}

      {/* Connection status */}
      <p className={`text-sm mt-3 ${
        connecting ? 'text-warning' :
        connected ? 'text-success' :
        'text-base-content/50'
      }`} data-testid="task-connection-status">
        {connecting ? 'Connecting to worktree...' :
         connected ? 'Connected' :
         'Not connected'}
      </p>

      {error && (
        <p className="text-sm text-error bg-error/10 px-3 py-2 rounded-lg border border-error/20 mt-2">{error}</p>
      )}

      {/* File Picker Modal */}
      <FilePicker
        isOpen={showFilePicker}
        onClose={() => setShowFilePicker(false)}
        onSelect={handleFileSelect}
      />
    </>
  )

  // Content when task is loaded
  const taskContent = task && (
    <>
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <h3 className="font-medium text-base-content truncate">{task.title}</h3>
          <p className="text-xs text-base-content/60 font-mono truncate">{task.source}</p>
        </div>
        <span className={`flex-shrink-0 badge badge-sm ${stateBadgeClass(state, phaseError?.class)}`}>
          {state}
        </span>
      </div>

      {/* Tags */}
      <div className="flex flex-wrap items-center gap-1 mt-2">
        {tags.map(tag => (
          <span key={tag} className="badge badge-sm badge-outline gap-1">
            {tag}
            <button
              className="cursor-pointer hover:text-error"
              onClick={() => removeTag(tag)}
              aria-label={`Remove tag ${tag}`}
            >
              &times;
            </button>
          </span>
        ))}
        {showTagInput ? (
          <input
            type="text"
            value={newTag}
            onChange={e => setNewTag(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && newTag.trim()) {
                addTag(newTag.trim())
                setNewTag('')
                setShowTagInput(false)
              }
              if (e.key === 'Escape') {
                tagCancelledRef.current = true
                setNewTag('')
                setShowTagInput(false)
              }
            }}
            onBlur={() => {
              if (newTag.trim() && !tagCancelledRef.current) {
                addTag(newTag.trim())
              }
              tagCancelledRef.current = false
              setNewTag('')
              setShowTagInput(false)
            }}
            placeholder="tag"
            className="input input-xs input-bordered w-20"
            ref={(el) => el?.focus()}
          />
        ) : (
          <button
            className="badge badge-sm badge-ghost cursor-pointer hover:badge-outline"
            onClick={() => setShowTagInput(true)}
            aria-label="Add tag"
          >
            +
          </button>
        )}
      </div>

      {task.contextItems && task.contextItems.length > 0 && (
        <div className="flex flex-wrap gap-1 mt-2">
          {task.contextItems.map(ci => (
            <span key={`${ci.type}-${ci.ref}`} className="badge badge-sm badge-info badge-outline gap-1">
              {contextIcon(ci.type)} {ci.label || ci.ref}
            </span>
          ))}
        </div>
      )}
      {task.description && (
        <p className="text-sm text-base-content/80 leading-relaxed mt-3 line-clamp-3">{task.description}</p>
      )}
      {task.branch && (
        <div className="flex items-center gap-2 text-xs mt-3">
          <svg aria-hidden="true" className="w-3 h-3 text-base-content/50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
          </svg>
          <span className="text-base-content/60">Branch:</span>
          <code className="font-mono text-primary bg-neutral px-1.5 py-0.5 rounded text-xs">{task.branch}</code>
          {task.worktreePath && (
            <span className="badge badge-xs badge-info gap-1" aria-label={`Isolated worktree: ${task.worktreePath}`}>
              <svg aria-hidden="true" className="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2" />
              </svg>
              isolated
            </span>
          )}
        </div>
      )}
      {error && (
        <p className={`text-sm px-3 py-2 rounded-lg border mt-3 ${errorBannerClass(phaseError?.class)}`}>{error}</p>
      )}

      {/* Recovery banner when task was interrupted */}
      {needsRecovery && (
        <div className="alert alert-warning text-sm mt-3">
          <svg aria-hidden="true" className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <span>Task was interrupted during <strong>{needsRecovery}</strong>.</span>
          <button className="btn btn-warning btn-xs" onClick={() => void retry()} disabled={loading}>
            Retry
          </button>
        </div>
      )}

      {/* CI fix loop status */}
      {ciFixStatus?.active && (
        <div className="mt-2">
          <span className="badge badge-info badge-sm gap-1">
            <span className="loading loading-spinner loading-xs" />
            CI Fix: attempt {ciFixStatus.attempt ?? '?'}/{ciFixStatus.maxAttempts ?? '?'}
          </span>
        </div>
      )}
      {ciFixStatus && !ciFixStatus.active && ciFixStatus.result === 'success' && (
        <div className="mt-2">
          <span className="badge badge-success badge-sm">CI Fix: passed</span>
        </div>
      )}
      {ciFixStatus && !ciFixStatus.active && ciFixStatus.result === 'failed' && (
        <div className="mt-2">
          <span className="badge badge-error badge-sm">CI Fix: failed</span>
        </div>
      )}

      {/* Recovery hints when task has failed */}
      {state === 'failed' && (
        <div className="mt-3 p-3 bg-warning/10 border border-warning/20 rounded-lg">
          <p className="text-sm text-warning-content mb-2">Task encountered an error. You can:</p>
          <div className="flex gap-2 flex-wrap">
            <button
              onClick={() => undo()}
              disabled={loading || !connected}
              className="btn btn-sm btn-outline"
            >
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
              </svg>
              Undo
            </button>
            <button
              onClick={() => reset()}
              disabled={loading || !connected}
              className="btn btn-sm btn-outline btn-warning"
            >
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              Reset
            </button>
          </div>
        </div>
      )}
    </>
  )

  const content = task ? taskContent : loadTaskContent

  if (embedded) {
    return <div>{content}</div>
  }

  return (
    <section className="card bg-base-200">
      <div className="card-body">
        <h2 className="card-title text-base-content flex items-center gap-2 mb-4">
          <svg aria-hidden="true" className="w-5 h-5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
          </svg>
          {task ? 'Current Task' : 'Load Task'}
        </h2>
        {content}
      </div>
    </section>
  )
}
