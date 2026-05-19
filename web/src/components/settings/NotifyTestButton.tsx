import { useState } from 'react'
import { useGlobalStore } from '../../stores/globalStore'

interface NotifyTestResult {
  sent: number
  message: string
}

export function NotifyTestButton() {
  const { client } = useGlobalStore()
  const [testing, setTesting] = useState(false)
  const [result, setResult] = useState<NotifyTestResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleTest = async () => {
    if (!client) return

    setTesting(true)
    setError(null)
    setResult(null)

    try {
      const res = await client.call<NotifyTestResult>('notify.test')
      setResult(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Test failed')
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className="mt-6 pt-4 border-t border-base-300">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold text-base-content/70">Notification Webhooks</h3>
        <button onClick={handleTest} disabled={testing || !client} className="btn btn-sm btn-outline">
          {testing ? (
            <span className="loading loading-spinner loading-xs"></span>
          ) : (
            <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
              />
            </svg>
          )}
          Send Test
        </button>
      </div>

      {error && (
        <div className="alert alert-error py-2 text-sm" role="alert">
          <span>{error}</span>
        </div>
      )}

      {result && (
        <div className={`alert py-2 text-sm ${result.sent > 0 ? 'alert-success' : 'alert-warning'}`}>
          <span>
            {result.sent > 0
              ? `Test notification sent to ${result.sent} endpoint${result.sent !== 1 ? 's' : ''}`
              : 'No webhook endpoints configured'}
          </span>
          {result.message && <span className="text-xs opacity-80 block">{result.message}</span>}
        </div>
      )}

      <p className="text-xs text-base-content/50 mt-2">
        Sends a test notification to all configured webhook endpoints.
      </p>
    </div>
  )
}
