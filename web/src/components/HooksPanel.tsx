import { useState, useCallback, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface HookInfo {
  command: string
  description: string
  required: boolean
}

interface HooksPanelProps {
  isOpen: boolean
  onClose: () => void
}

export function HooksPanel({ isOpen, onClose }: HooksPanelProps) {
  const { client, connected } = useProjectStore()

  const [hooks, setHooks] = useState<Record<string, HookInfo[]>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadHooks = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<Record<string, HookInfo[]>>('hooks.list', {})
      setHooks(result || {})
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load hooks')
      setHooks({})
    } finally {
      setLoading(false)
    }
  }, [client, connected])

  useEffect(() => {
    if (isOpen && connected) {
      loadHooks()
    }
  }, [isOpen, connected, loadHooks])

  const eventNames = Object.keys(hooks)

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Workflow Hooks" size="2xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Toolbar */}
        <div className="flex items-center justify-end mb-4">
          <button
            onClick={loadHooks}
            disabled={loading || !connected}
            className="btn btn-ghost btn-sm"
            aria-label="Refresh hooks"
          >
            {loading ? (
              <span className="loading loading-spinner loading-xs"></span>
            ) : (
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            )}
            Refresh
          </button>
        </div>

        {/* Error */}
        {error && (
          <div className="alert alert-error py-2 mb-4">
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Content */}
        <div className="flex-1 overflow-y-auto">
          {loading && eventNames.length === 0 ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : eventNames.length === 0 ? (
            <div className="text-center py-12 text-base-content/50">
              <svg aria-hidden="true" className="w-10 h-10 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
              </svg>
              <p>No workflow hooks configured.</p>
            </div>
          ) : (
            <div className="space-y-4">
              {eventNames.map(event => (
                <div key={event}>
                  <h4 className="text-xs font-semibold uppercase tracking-wider text-base-content/60 mb-2">
                    {event}
                  </h4>
                  <div className="space-y-1">
                    {hooks[event].map((hook, i) => (
                      <div key={i} className="flex items-start gap-2 p-2 rounded-lg bg-base-200">
                        <span className={`badge badge-xs mt-0.5 ${hook.required ? 'badge-error' : 'badge-ghost'}`}>
                          {hook.required ? 'required' : 'optional'}
                        </span>
                        <div className="min-w-0">
                          <div className="text-sm font-medium truncate">
                            {hook.description || hook.command}
                          </div>
                          {hook.description && (
                            <code className="text-xs text-base-content/50 font-mono">{hook.command}</code>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
