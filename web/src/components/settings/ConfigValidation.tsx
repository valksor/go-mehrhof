import { useState } from 'react'
import { useGlobalStore } from '../../stores/globalStore'

interface ValidateCheck {
  name: string
  status: 'ok' | 'error' | 'warning'
  detail?: string
  fix?: string
}

interface ValidateResult {
  valid: boolean
  checks: ValidateCheck[]
}

export function ConfigValidation() {
  const { client } = useGlobalStore()
  const [result, setResult] = useState<ValidateResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleValidate = async () => {
    if (!client) return

    setLoading(true)
    setError(null)

    try {
      const res = await client.call<ValidateResult>('config.validate', {})
      setResult(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Validation failed')
      setResult(null)
    } finally {
      setLoading(false)
    }
  }

  const statusIcon = (status: string) => {
    switch (status) {
      case 'ok':
        return <span className="text-success">&#10003;</span>
      case 'error':
        return <span className="text-error">&#10007;</span>
      case 'warning':
        return <span className="text-warning">&#9888;</span>
      default:
        return null
    }
  }

  return (
    <div className="mt-6 pt-4 border-t border-base-300">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-semibold text-base-content/70">Configuration Validation</h3>
        <button onClick={handleValidate} disabled={loading || !client} className="btn btn-sm btn-outline btn-primary">
          {loading ? (
            <span className="loading loading-spinner loading-xs"></span>
          ) : (
            <svg aria-hidden="true" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          )}
          Validate
        </button>
      </div>

      {error && (
        <div className="alert alert-error py-2 text-sm mb-3" role="alert">
          <span>{error}</span>
        </div>
      )}

      {result && (
        <div className="space-y-1.5">
          {result.checks.map((check, i) => (
            <div key={i} className="flex items-start gap-2 text-sm">
              <span className="flex-shrink-0 w-4 text-center">{statusIcon(check.status)}</span>
              <div className="flex-1 min-w-0">
                <span className="font-medium">{check.name}</span>
                {check.detail && <span className="text-base-content/60 ml-1.5">({check.detail})</span>}
                {check.status === 'error' && check.fix && <p className="text-xs text-error/80 mt-0.5">{check.fix}</p>}
              </div>
            </div>
          ))}
          <div className={`text-sm font-medium mt-2 ${result.valid ? 'text-success' : 'text-error'}`}>
            {result.valid ? 'Configuration is valid' : 'Configuration has errors'}
          </div>
        </div>
      )}
    </div>
  )
}
