import { useState, useCallback, useEffect } from 'react'
import { useGlobalStore } from '../../stores/globalStore'

interface TestResult {
  ok: boolean
  detail: string
}

interface ProviderInfo {
  name: string
  label: string
  env_var: string
  help_url: string
  help_steps: string
  scopes: string
  token_prefix: string
  configured: boolean
}

interface LoginResult {
  saved: boolean
  valid: boolean
  detail: string
  env_path: string
  masked: string
}

export function ProviderTestButtons() {
  const { client } = useGlobalStore()
  const [providers, setProviders] = useState<ProviderInfo[]>([])
  const [testing, setTesting] = useState<Record<string, boolean>>({})
  const [results, setResults] = useState<Record<string, TestResult>>({})
  const [loginProvider, setLoginProvider] = useState<string | null>(null)
  const [tokenInput, setTokenInput] = useState('')
  const [saving, setSaving] = useState(false)
  const [loginResult, setLoginResult] = useState<LoginResult | null>(null)

  const loadProviders = useCallback(async () => {
    if (!client) return
    try {
      const result = await client.call<{ providers: ProviderInfo[] }>('providers.list', {})
      setProviders(result.providers || [])
    } catch {
      // Fall back to static list
      setProviders([
        { name: 'github', label: 'GitHub', env_var: 'GITHUB_TOKEN', help_url: '', help_steps: '', scopes: '', token_prefix: '', configured: false },
        { name: 'gitlab', label: 'GitLab', env_var: 'GITLAB_TOKEN', help_url: '', help_steps: '', scopes: '', token_prefix: '', configured: false },
        { name: 'linear', label: 'Linear', env_var: 'LINEAR_TOKEN', help_url: '', help_steps: '', scopes: '', token_prefix: '', configured: false },
        { name: 'wrike', label: 'Wrike', env_var: 'WRIKE_TOKEN', help_url: '', help_steps: '', scopes: '', token_prefix: '', configured: false },
      ])
    }
  }, [client])

  useEffect(() => {
    loadProviders()
  }, [loadProviders])

  const handleTest = async (provider: string) => {
    if (!client) return

    setTesting(prev => ({ ...prev, [provider]: true }))
    setResults(prev => {
      const next = { ...prev }
      delete next[provider]
      return next
    })

    try {
      const result = await client.call<TestResult>('providers.test', {
        provider,
        token: '__use_configured__',
      })
      setResults(prev => ({ ...prev, [provider]: result }))
    } catch (err) {
      setResults(prev => ({
        ...prev,
        [provider]: { ok: false, detail: err instanceof Error ? err.message : 'Test failed' },
      }))
    } finally {
      setTesting(prev => ({ ...prev, [provider]: false }))
    }
  }

  const handleLogin = async (providerName: string) => {
    if (!client || !tokenInput.trim()) return

    setSaving(true)
    setLoginResult(null)

    try {
      const result = await client.call<LoginResult>('provider.login', {
        provider: providerName,
        token: tokenInput.trim(),
      })
      setLoginResult(result)
      if (result.saved) {
        setTokenInput('')
        await loadProviders()
      }
    } catch (err) {
      setLoginResult({
        saved: false,
        valid: false,
        detail: err instanceof Error ? err.message : 'Login failed',
        env_path: '',
        masked: '',
      })
    } finally {
      setSaving(false)
    }
  }

  const activeProvider = providers.find(p => p.name === loginProvider)

  return (
    <div className="mt-6 p-4 bg-base-200 rounded-lg">
      <h3 className="font-medium text-sm mb-3">Provider Connections</h3>
      <div className="space-y-2">
        {providers.map(p => (
          <div key={p.name} className="flex items-center gap-3">
            <span className="text-sm w-16">{p.label}</span>
            {p.configured && (
              <span className="badge badge-xs badge-success">configured</span>
            )}
            <button
              onClick={() => handleTest(p.name)}
              disabled={testing[p.name] || !client}
              className="btn btn-xs btn-outline"
            >
              {testing[p.name] ? (
                <span className="loading loading-spinner loading-xs"></span>
              ) : (
                'Verify'
              )}
            </button>
            <button
              onClick={() => {
                setLoginProvider(loginProvider === p.name ? null : p.name)
                setTokenInput('')
                setLoginResult(null)
              }}
              className={`btn btn-xs ${loginProvider === p.name ? 'btn-active' : 'btn-ghost'}`}
            >
              {p.configured ? 'Update Token' : 'Login'}
            </button>
            {results[p.name] && (
              <span className={`text-xs ${results[p.name].ok ? 'text-success' : 'text-error'}`}>
                {results[p.name].detail}
              </span>
            )}
          </div>
        ))}
      </div>

      {/* Login form */}
      {activeProvider && (
        <div className="mt-4 p-3 bg-base-100 rounded-lg border border-base-300">
          <h4 className="font-medium text-sm mb-2">{activeProvider.label} Token Setup</h4>

          {activeProvider.help_url && (
            <div className="text-xs text-base-content/60 space-y-1 mb-3">
              <p>
                Get token:{' '}
                <a
                  href={activeProvider.help_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="link link-primary"
                >
                  {activeProvider.help_url}
                </a>
              </p>
              {activeProvider.help_steps && <p>Steps: {activeProvider.help_steps}</p>}
              {activeProvider.scopes && <p>Required scopes: {activeProvider.scopes}</p>}
              {activeProvider.token_prefix && <p>Token format: starts with <code>{activeProvider.token_prefix}</code></p>}
            </div>
          )}

          <div className="flex gap-2">
            <input
              type="password"
              value={tokenInput}
              onChange={e => setTokenInput(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter' && tokenInput.trim()) handleLogin(activeProvider.name)
              }}
              placeholder={`Paste your ${activeProvider.label} API token`}
              className="input input-bordered input-sm flex-1"
              autoComplete="off"
              aria-label={`${activeProvider.label} API token`}
            />
            <button
              onClick={() => handleLogin(activeProvider.name)}
              disabled={saving || !tokenInput.trim() || !client}
              className="btn btn-primary btn-sm"
            >
              {saving ? (
                <span className="loading loading-spinner loading-xs"></span>
              ) : (
                'Save'
              )}
            </button>
            <button
              onClick={() => {
                setLoginProvider(null)
                setTokenInput('')
                setLoginResult(null)
              }}
              className="btn btn-ghost btn-sm"
            >
              Cancel
            </button>
          </div>

          {loginResult && (
            <div className={`mt-2 text-xs ${loginResult.valid ? 'text-success' : 'text-warning'}`}>
              {loginResult.saved ? (
                <>
                  Token saved{loginResult.env_path ? ` to ${loginResult.env_path}` : ''}.
                  {' '}{loginResult.detail}
                  {loginResult.masked && <span className="text-base-content/50"> ({loginResult.masked})</span>}
                </>
              ) : (
                <span className="text-error">{loginResult.detail}</span>
              )}
            </div>
          )}
        </div>
      )}

      <p className="text-xs text-base-content/50 mt-2">
        Tokens are saved to the global .env file and used for provider API access.
      </p>
    </div>
  )
}
