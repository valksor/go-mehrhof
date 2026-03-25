import { useEffect, useState } from 'react'
import { useProjectStore } from '../stores/projectStore'

interface SkipSuggestion {
  phase: string
  confidence: number
  reason: string
  sample_size: number
}

interface AgentSuggestion {
  phase: string
  agent: string
  confidence: number
  reason: string
  sample_size: number
}

interface Suggestions {
  skip: SkipSuggestion[] | null
  agent: AgentSuggestion[] | null
}

export function SuggestionBanner() {
  const connected = useProjectStore(s => s.connected)
  const client = useProjectStore(s => s.client)
  const state = useProjectStore(s => s.state)
  const [suggestions, setSuggestions] = useState<Suggestions | null>(null)
  const [dismissed, setDismissed] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (!client || !connected) return
    if (state !== 'loaded' && state !== 'planned') return

    let cancelled = false
    client.call<Suggestions>('suggestions.list', {}).then(result => {
      if (!cancelled) setSuggestions(result)
    }).catch(() => {
      // Ignore errors (feature is optional)
    })

    return () => { cancelled = true }
  }, [state, client, connected])

  if (!suggestions) return null

  const skipItems = (suggestions.skip || []).filter(s => !dismissed.has(`skip:${s.phase}`))
  const agentItems = (suggestions.agent || []).filter(s => !dismissed.has(`agent:${s.agent}`))

  if (skipItems.length === 0 && agentItems.length === 0) return null

  return (
    <div className="space-y-1.5 mb-2">
      {skipItems.map(s => (
        <div key={`skip:${s.phase}`} className="alert alert-info py-1.5 px-3 text-xs">
          <div className="flex-1">
            <span className="font-medium">Suggestion:</span> Skip <span className="font-mono">{s.phase}</span> phase
            <span className="opacity-60 ml-1">({s.reason})</span>
          </div>
          <div className="flex gap-1">
            <button
              className="btn btn-xs btn-ghost"
              onClick={() => setDismissed(prev => new Set(prev).add(`skip:${s.phase}`))}
            >
              Dismiss
            </button>
          </div>
        </div>
      ))}
      {agentItems.map(s => (
        <div key={`agent:${s.agent}`} className="alert alert-info py-1.5 px-3 text-xs">
          <div className="flex-1">
            <span className="font-medium">Suggestion:</span> Use <span className="font-mono">{s.agent}</span> as default agent
            <span className="opacity-60 ml-1">({s.reason})</span>
          </div>
          <div className="flex gap-1">
            <button
              className="btn btn-xs btn-ghost"
              onClick={() => setDismissed(prev => new Set(prev).add(`agent:${s.agent}`))}
            >
              Dismiss
            </button>
          </div>
        </div>
      ))}
    </div>
  )
}
