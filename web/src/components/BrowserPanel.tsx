import { useState, useEffect, useCallback } from 'react'
import { useBrowserStore, type ConsoleMessage, type NetworkRequest } from '../stores/browserStore'

type ActionCategory = 'navigate' | 'interact' | 'form' | 'capture' | 'console' | 'network'

function ConsolePanel() {
  const { console: getConsole } = useBrowserStore()
  const [messages, setMessages] = useState<ConsoleMessage[]>([])
  const [loading, setLoading] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const result = await getConsole()
      setMessages(result.messages || [])
    } catch (err) {
      console.error('Failed to fetch console messages:', err)
    } finally {
      setLoading(false)
    }
  }, [getConsole])

  useEffect(() => { void refresh() }, [refresh])

  const typeColor: Record<string, string> = {
    error: 'text-error',
    warning: 'text-warning',
    log: 'text-base-content',
    info: 'text-info',
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-semibold">Console Messages</h3>
        <button onClick={refresh} disabled={loading} className="btn btn-ghost btn-xs">
          {loading ? <span className="loading loading-spinner loading-xs" /> : 'Refresh'}
        </button>
      </div>
      {messages.length === 0 ? (
        <div className="text-xs text-base-content/50">No console messages captured.</div>
      ) : (
        <div className="space-y-1 max-h-64 overflow-y-auto">
          {messages.map((msg, i) => (
            <div key={i} className={`text-xs font-mono ${typeColor[msg.type] || 'text-base-content/70'}`}>
              <span className="opacity-60">[{msg.type}]</span> {msg.text}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function NetworkPanel() {
  const { network: getNetwork } = useBrowserStore()
  const [requests, setRequests] = useState<NetworkRequest[]>([])
  const [loading, setLoading] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const result = await getNetwork()
      setRequests(result.requests || [])
    } catch (err) {
      console.error('Failed to fetch network requests:', err)
    } finally {
      setLoading(false)
    }
  }, [getNetwork])

  useEffect(() => { void refresh() }, [refresh])

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-semibold">Network Requests</h3>
        <button onClick={refresh} disabled={loading} className="btn btn-ghost btn-xs">
          {loading ? <span className="loading loading-spinner loading-xs" /> : 'Refresh'}
        </button>
      </div>
      {requests.length === 0 ? (
        <div className="text-xs text-base-content/50">No network requests captured.</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="table table-xs">
            <thead>
              <tr>
                <th>Method</th>
                <th>URL</th>
                <th>Status</th>
                <th>Type</th>
              </tr>
            </thead>
            <tbody>
              {requests.map((req, i) => (
                <tr key={i}>
                  <td className="font-mono text-xs">{req.method}</td>
                  <td className="font-mono text-xs max-w-xs truncate">{req.url}</td>
                  <td className={`text-xs ${req.status && req.status >= 400 ? 'text-error' : ''}`}>
                    {req.status || '—'}
                  </td>
                  <td className="text-xs opacity-60">{req.resource_type || ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

export function BrowserPanel() {
  const {
    status, loading, error, lastResult, currentUrl, currentTitle,
    checkStatus, install, navigate, back, forward, reload,
    click, type, fill, select, hover, focus, press, scroll, wait,
    dialog, upload, screenshot, snapshot, pdf, eval: evalJs,
    clearError, clearResult
  } = useBrowserStore()

  const [activeCategory, setActiveCategory] = useState<ActionCategory>('navigate')

  // Form states
  const [url, setUrl] = useState('')
  const [selector, setSelector] = useState('')
  const [value, setValue] = useState('')
  const [key, setKey] = useState('')
  const [scrollDir, setScrollDir] = useState<'up' | 'down' | 'left' | 'right'>('down')
  const [scrollAmount, setScrollAmount] = useState(500)
  const [jsCode, setJsCode] = useState('')
  const [snapshotResult, setSnapshotResult] = useState('')
  const [evalResult, setEvalResult] = useState('')

  useEffect(() => {
    void checkStatus()
  }, [checkStatus])

  const handleNavigate = async () => {
    if (!url) return
    await navigate(url)
    setUrl('')
  }

  const handleAction = async (action: () => Promise<unknown>) => {
    try {
      await action()
      setSelector('')
      setValue('')
    } catch {
      // Error is already set in store
    }
  }

  const handleSnapshot = async () => {
    try {
      const result = await snapshot()
      setSnapshotResult(result.snapshot)
    } catch {
      // Error handled
    }
  }

  const handleEval = async () => {
    if (!jsCode) return
    try {
      const result = await evalJs(jsCode)
      setEvalResult(result.error || result.result)
    } catch {
      // Error handled
    }
  }

  // Not installed view
  if (status && !status.installed) {
    return (
      <div className="h-full flex flex-col items-center justify-center p-6 text-center">
        <svg className="w-16 h-16 text-base-content/30 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
        </svg>
        <h3 className="text-lg font-semibold mb-2">Browser Runtime Not Installed</h3>
        <p className="text-sm text-base-content/60 mb-4">
          Install the browser runtime to enable automation features.
        </p>
        <button
          onClick={install}
          disabled={loading}
          className="btn btn-primary"
        >
          {loading ? (
            <>
              <span className="loading loading-spinner loading-sm"></span>
              Installing...
            </>
          ) : (
            <>
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
              Install Browser Runtime
            </>
          )}
        </button>
      </div>
    )
  }

  return (
    <div className="h-full flex flex-col overflow-hidden">
      {/* Header with current page info */}
      <div className="flex-shrink-0 border-b border-base-300 p-3">
        <div className="flex items-center gap-2 mb-2">
          {/* Navigation buttons */}
          <button onClick={back} disabled={loading} className="btn btn-ghost btn-xs btn-square" title="Back">
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
          </button>
          <button onClick={forward} disabled={loading} className="btn btn-ghost btn-xs btn-square" title="Forward">
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
          <button onClick={reload} disabled={loading} className="btn btn-ghost btn-xs btn-square" title="Reload">
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>

          {/* URL input */}
          <div className="flex-1 join">
            <input
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleNavigate()}
              placeholder="Enter URL..."
              className="input input-bordered input-sm join-item flex-1"
            />
            <button onClick={handleNavigate} disabled={loading || !url} className="btn btn-primary btn-sm join-item">
              Go
            </button>
          </div>
        </div>

        {/* Current page info */}
        {(currentUrl || currentTitle) && (
          <div className="text-xs text-base-content/60 truncate">
            {currentTitle && <span className="font-medium">{currentTitle}</span>}
            {currentTitle && currentUrl && <span className="mx-1">-</span>}
            {currentUrl && <span>{currentUrl}</span>}
          </div>
        )}
      </div>

      {/* Category tabs */}
      <div className="flex-shrink-0 tabs tabs-boxed bg-base-200 rounded-none p-1 gap-1">
        <button
          className={`tab tab-sm ${activeCategory === 'navigate' ? 'tab-active' : ''}`}
          onClick={() => setActiveCategory('navigate')}
        >
          Navigate
        </button>
        <button
          className={`tab tab-sm ${activeCategory === 'interact' ? 'tab-active' : ''}`}
          onClick={() => setActiveCategory('interact')}
        >
          Interact
        </button>
        <button
          className={`tab tab-sm ${activeCategory === 'form' ? 'tab-active' : ''}`}
          onClick={() => setActiveCategory('form')}
        >
          Forms
        </button>
        <button
          className={`tab tab-sm ${activeCategory === 'capture' ? 'tab-active' : ''}`}
          onClick={() => setActiveCategory('capture')}
        >
          Capture
        </button>
        <button
          className={`tab tab-sm ${activeCategory === 'console' ? 'tab-active' : ''}`}
          onClick={() => setActiveCategory('console')}
        >
          Console
        </button>
        <button
          className={`tab tab-sm ${activeCategory === 'network' ? 'tab-active' : ''}`}
          onClick={() => setActiveCategory('network')}
        >
          Network
        </button>
      </div>

      {/* Action panels */}
      <div className="flex-1 overflow-auto p-3 space-y-3">
        {/* Error display */}
        {error && (
          <div className="alert alert-error py-2">
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span className="text-sm">{error}</span>
            <button onClick={clearError} className="btn btn-ghost btn-xs">Dismiss</button>
          </div>
        )}

        {/* Success display */}
        {lastResult?.success && (
          <div className="alert alert-success py-2">
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
            <span className="text-sm">Action completed successfully</span>
            <button onClick={clearResult} className="btn btn-ghost btn-xs">Dismiss</button>
          </div>
        )}

        {/* Navigate panel */}
        {activeCategory === 'navigate' && (
          <div className="space-y-3">
            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Wait for Element</h4>
                <div className="join w-full">
                  <input
                    type="text"
                    value={selector}
                    onChange={(e) => setSelector(e.target.value)}
                    placeholder="CSS selector..."
                    className="input input-bordered input-sm join-item flex-1"
                  />
                  <button
                    onClick={() => handleAction(() => wait(selector))}
                    disabled={loading || !selector}
                    className="btn btn-sm join-item"
                  >
                    Wait
                  </button>
                </div>
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Scroll Page</h4>
                <div className="flex gap-2">
                  <select
                    value={scrollDir}
                    onChange={(e) => setScrollDir(e.target.value as typeof scrollDir)}
                    className="select select-bordered select-sm"
                  >
                    <option value="up">Up</option>
                    <option value="down">Down</option>
                    <option value="left">Left</option>
                    <option value="right">Right</option>
                  </select>
                  <input
                    type="number"
                    value={scrollAmount}
                    onChange={(e) => setScrollAmount(parseInt(e.target.value) || 500)}
                    className="input input-bordered input-sm w-24"
                    placeholder="Amount"
                  />
                  <button
                    onClick={() => handleAction(() => scroll(scrollDir, scrollAmount))}
                    disabled={loading}
                    className="btn btn-sm flex-1"
                  >
                    Scroll
                  </button>
                </div>
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Evaluate JavaScript</h4>
                <textarea
                  value={jsCode}
                  onChange={(e) => setJsCode(e.target.value)}
                  placeholder="document.title"
                  className="textarea textarea-bordered textarea-sm w-full h-20 font-mono text-xs"
                />
                <button
                  onClick={handleEval}
                  disabled={loading || !jsCode}
                  className="btn btn-sm w-full"
                >
                  Execute
                </button>
                {evalResult && (
                  <pre className="mt-2 p-2 bg-neutral text-neutral-content rounded text-xs overflow-auto max-h-32">
                    {evalResult}
                  </pre>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Interact panel */}
        {activeCategory === 'interact' && (
          <div className="space-y-3">
            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Click Element</h4>
                <div className="join w-full">
                  <input
                    type="text"
                    value={selector}
                    onChange={(e) => setSelector(e.target.value)}
                    placeholder="CSS selector..."
                    className="input input-bordered input-sm join-item flex-1"
                  />
                  <button
                    onClick={() => handleAction(() => click(selector))}
                    disabled={loading || !selector}
                    className="btn btn-sm join-item"
                  >
                    Click
                  </button>
                </div>
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Hover / Focus</h4>
                <div className="join w-full">
                  <input
                    type="text"
                    value={selector}
                    onChange={(e) => setSelector(e.target.value)}
                    placeholder="CSS selector..."
                    className="input input-bordered input-sm join-item flex-1"
                  />
                  <button
                    onClick={() => handleAction(() => hover(selector))}
                    disabled={loading || !selector}
                    className="btn btn-sm join-item"
                  >
                    Hover
                  </button>
                  <button
                    onClick={() => handleAction(() => focus(selector))}
                    disabled={loading || !selector}
                    className="btn btn-sm join-item"
                  >
                    Focus
                  </button>
                </div>
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Press Key</h4>
                <div className="join w-full">
                  <input
                    type="text"
                    value={key}
                    onChange={(e) => setKey(e.target.value)}
                    placeholder="Enter, Escape, Tab, Control+a..."
                    className="input input-bordered input-sm join-item flex-1"
                  />
                  <button
                    onClick={() => handleAction(() => press(key))}
                    disabled={loading || !key}
                    className="btn btn-sm join-item"
                  >
                    Press
                  </button>
                </div>
                <div className="flex flex-wrap gap-1 mt-2">
                  {['Enter', 'Escape', 'Tab', 'ArrowDown', 'ArrowUp'].map((k) => (
                    <button key={k} onClick={() => setKey(k)} className="btn btn-xs btn-ghost">
                      {k}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Handle Dialog</h4>
                <p className="text-xs text-base-content/60 mb-2">Accept or dismiss alert/confirm/prompt</p>
                <div className="flex gap-2">
                  <button
                    onClick={() => handleAction(() => dialog('accept'))}
                    disabled={loading}
                    className="btn btn-sm btn-success flex-1"
                  >
                    Accept
                  </button>
                  <button
                    onClick={() => handleAction(() => dialog('dismiss'))}
                    disabled={loading}
                    className="btn btn-sm btn-error flex-1"
                  >
                    Dismiss
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Form panel */}
        {activeCategory === 'form' && (
          <div className="space-y-3">
            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Type Text</h4>
                <input
                  type="text"
                  value={selector}
                  onChange={(e) => setSelector(e.target.value)}
                  placeholder="CSS selector..."
                  className="input input-bordered input-sm w-full mb-2"
                />
                <div className="join w-full">
                  <input
                    type="text"
                    value={value}
                    onChange={(e) => setValue(e.target.value)}
                    placeholder="Text to type..."
                    className="input input-bordered input-sm join-item flex-1"
                  />
                  <button
                    onClick={() => handleAction(() => type(selector, value))}
                    disabled={loading || !selector}
                    className="btn btn-sm join-item"
                  >
                    Type
                  </button>
                </div>
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Fill Input</h4>
                <p className="text-xs text-base-content/60 mb-2">Clears existing value and sets new one</p>
                <input
                  type="text"
                  value={selector}
                  onChange={(e) => setSelector(e.target.value)}
                  placeholder="CSS selector..."
                  className="input input-bordered input-sm w-full mb-2"
                />
                <div className="join w-full">
                  <input
                    type="text"
                    value={value}
                    onChange={(e) => setValue(e.target.value)}
                    placeholder="Value..."
                    className="input input-bordered input-sm join-item flex-1"
                  />
                  <button
                    onClick={() => handleAction(() => fill(selector, value))}
                    disabled={loading || !selector}
                    className="btn btn-sm join-item"
                  >
                    Fill
                  </button>
                </div>
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Select Option</h4>
                <input
                  type="text"
                  value={selector}
                  onChange={(e) => setSelector(e.target.value)}
                  placeholder="CSS selector for <select>..."
                  className="input input-bordered input-sm w-full mb-2"
                />
                <div className="join w-full">
                  <input
                    type="text"
                    value={value}
                    onChange={(e) => setValue(e.target.value)}
                    placeholder="Option value or label..."
                    className="input input-bordered input-sm join-item flex-1"
                  />
                  <button
                    onClick={() => handleAction(() => select(selector, [value]))}
                    disabled={loading || !selector || !value}
                    className="btn btn-sm join-item"
                  >
                    Select
                  </button>
                </div>
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Upload File</h4>
                <input
                  type="text"
                  value={selector}
                  onChange={(e) => setSelector(e.target.value)}
                  placeholder="CSS selector for file input..."
                  className="input input-bordered input-sm w-full mb-2"
                />
                <div className="join w-full">
                  <input
                    type="text"
                    value={value}
                    onChange={(e) => setValue(e.target.value)}
                    placeholder="Absolute file path..."
                    className="input input-bordered input-sm join-item flex-1"
                  />
                  <button
                    onClick={() => handleAction(() => upload(selector, [value]))}
                    disabled={loading || !selector || !value}
                    className="btn btn-sm join-item"
                  >
                    Upload
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Capture panel */}
        {activeCategory === 'capture' && (
          <div className="space-y-3">
            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Screenshot</h4>
                <div className="flex gap-2">
                  <button
                    onClick={() => handleAction(() => screenshot())}
                    disabled={loading}
                    className="btn btn-sm flex-1"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
                    </svg>
                    Capture
                  </button>
                  <button
                    onClick={() => handleAction(() => screenshot({ fullPage: true }))}
                    disabled={loading}
                    className="btn btn-sm flex-1"
                  >
                    Full Page
                  </button>
                </div>
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Accessibility Snapshot</h4>
                <p className="text-xs text-base-content/60 mb-2">Get page structure as accessibility tree</p>
                <button
                  onClick={handleSnapshot}
                  disabled={loading}
                  className="btn btn-sm w-full"
                >
                  Get Snapshot
                </button>
                {snapshotResult && (
                  <pre className="mt-2 p-2 bg-neutral text-neutral-content rounded text-xs overflow-auto max-h-48">
                    {snapshotResult}
                  </pre>
                )}
              </div>
            </div>

            <div className="card bg-base-200">
              <div className="card-body p-3">
                <h4 className="card-title text-sm">Generate PDF</h4>
                <div className="flex gap-2">
                  <button
                    onClick={() => handleAction(() => pdf())}
                    disabled={loading}
                    className="btn btn-sm flex-1"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
                    </svg>
                    A4 Portrait
                  </button>
                  <button
                    onClick={() => handleAction(() => pdf({ landscape: true }))}
                    disabled={loading}
                    className="btn btn-sm flex-1"
                  >
                    Landscape
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}

        {activeCategory === 'console' && (
          <ConsolePanel />
        )}

        {activeCategory === 'network' && (
          <NetworkPanel />
        )}
      </div>

      {/* Loading indicator */}
      {loading && (
        <div className="flex-shrink-0 flex items-center justify-center gap-2 py-2 bg-base-200 text-primary">
          <span className="loading loading-spinner loading-sm"></span>
          <span className="text-sm">Working...</span>
        </div>
      )}

      {/* Status bar */}
      <div className="flex-shrink-0 border-t border-base-300 p-2 text-xs text-base-content/60 flex items-center justify-between">
        <span>
          {status?.config?.browser || 'chromium'} |
          {status?.config?.headless ? ' headless' : ' headed'}
        </span>
        <span>{status?.version || 'Unknown version'}</span>
      </div>
    </div>
  )
}
