import { useState } from 'react'
import { useGlobalStore } from '../../stores/globalStore'

interface Drift {
  path: string
  expected: unknown
  actual: unknown
  description: string
}

interface ConfigCheckResult {
  drifts: Drift[]
  count: number
}

export function ConfigCheckButton() {
  const { client } = useGlobalStore()
  const [result, setResult] = useState<ConfigCheckResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleCheck = async () => {
    if (!client) return

    setLoading(true)
    setError(null)

    try {
      const res = await client.call<ConfigCheckResult>('config.check', {})
      setResult(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Config check failed')
      setResult(null)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="mt-6 pt-4 border-t border-base-300">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold text-base-content/70">Config Drift Detection</h3>
        <button onClick={handleCheck} disabled={loading || !client} className="btn btn-sm btn-outline">
          {loading ? (
            <span className="loading loading-spinner loading-xs"></span>
          ) : (
            <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
              />
            </svg>
          )}
          Check Config
        </button>
      </div>

      {error && (
        <div className="alert alert-error py-2 text-sm mb-3" role="alert">
          <span>{error}</span>
        </div>
      )}

      {result && (
        <div>
          {result.count === 0 ? (
            <div className="alert alert-success py-2 text-sm">
              <span>No drift detected between global and project settings.</span>
            </div>
          ) : (
            <div className="space-y-2">
              <div className="alert alert-warning py-2 text-sm">
                <span>
                  {result.count} difference{result.count !== 1 ? 's' : ''} found between global and project settings.
                </span>
              </div>
              <div className="space-y-1.5 max-h-48 overflow-y-auto">
                {result.drifts.map((drift, i) => (
                  <div key={i} className="text-xs bg-base-200 rounded p-2">
                    <div className="font-mono font-medium text-warning">{drift.path}</div>
                    <div className="text-base-content/60 mt-0.5">{drift.description}</div>
                    <div className="flex gap-4 mt-1">
                      <span className="text-base-content/50">
                        Expected: <span className="font-mono">{JSON.stringify(drift.expected ?? null)}</span>
                      </span>
                      <span className="text-base-content/50">
                        Actual: <span className="font-mono">{JSON.stringify(drift.actual ?? null)}</span>
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      <p className="text-xs text-base-content/50 mt-2">
        Compares global and project settings to detect configuration drift.
      </p>
    </div>
  )
}
