import { useState, useCallback, useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface AccessPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface AccessToken {
  id: string
  role: string
  label: string
  created_at: string
  expires_at?: string
}

export function AccessPanel({ isOpen, onClose }: AccessPanelProps) {
  const { client, connected } = useGlobalStore()

  const [tokens, setTokens] = useState<AccessToken[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [newRole, setNewRole] = useState('operator')
  const [newLabel, setNewLabel] = useState('')
  const [createdToken, setCreatedToken] = useState<string | null>(null)

  const loadTokens = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<{ tokens: AccessToken[] }>('access.token.list', {})
      setTokens(result.tokens || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load tokens')
      setTokens([])
    } finally {
      setLoading(false)
    }
  }, [client, connected])

  useEffect(() => {
    if (isOpen && connected) {
      loadTokens()
    }
  }, [isOpen, connected, loadTokens])

  const handleCreate = async () => {
    if (!client) return

    try {
      const result = await client.call<{ token: string }>('access.token.create', {
        role: newRole,
        label: newLabel,
      })
      setCreatedToken(result.token)
      setNewLabel('')
      await loadTokens()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create token')
    }
  }

  const handleRevoke = async (id: string) => {
    if (!client) return
    if (!window.confirm('Revoke this token? This cannot be undone.')) return

    try {
      await client.call('access.token.revoke', { id })
      await loadTokens()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke token')
    }
  }

  const formatDate = (ts: string) => {
    try {
      return new Date(ts).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      })
    } catch {
      return ts
    }
  }

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Access Tokens" size="2xl">
      <div className="max-h-[70vh] flex flex-col gap-4">
        {/* Create token form */}
        <div className="flex flex-wrap items-end gap-2 p-3 bg-base-200 rounded-lg">
          <div className="flex-1 min-w-[120px]">
            <label htmlFor="token-label" className="label label-text text-xs">Label</label>
            <input
              id="token-label"
              type="text"
              value={newLabel}
              onChange={e => setNewLabel(e.target.value)}
              placeholder="My token"
              className="input input-bordered input-sm w-full"
            />
          </div>
          <div>
            <label htmlFor="token-role" className="label label-text text-xs">Role</label>
            <select
              id="token-role"
              value={newRole}
              onChange={e => setNewRole(e.target.value)}
              className="select select-bordered select-sm"
            >
              <option value="operator">Operator</option>
              <option value="viewer">Viewer</option>
            </select>
          </div>
          <button
            onClick={handleCreate}
            disabled={!connected}
            className="btn btn-primary btn-sm"
          >
            Create Token
          </button>
        </div>

        {/* Created token display */}
        {createdToken && (
          <div className="alert alert-success py-2">
            <div className="flex-1">
              <p className="text-sm font-medium">Token created — copy it now:</p>
              <code className="text-xs break-all select-all">{createdToken}</code>
              <p className="text-xs opacity-70 mt-1">This token cannot be retrieved later.</p>
            </div>
            <button onClick={() => setCreatedToken(null)} className="btn btn-ghost btn-xs">
              Dismiss
            </button>
          </div>
        )}

        {error && (
          <div className="alert alert-error py-2">
            <span className="text-sm">{error}</span>
          </div>
        )}

        {/* Token list */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <span className="loading loading-spinner loading-lg text-primary"></span>
            </div>
          ) : tokens.length === 0 ? (
            <div className="text-center py-12 text-base-content/50">
              <svg aria-hidden="true" className="w-10 h-10 mx-auto mb-3 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
              </svg>
              <p>No access tokens configured</p>
            </div>
          ) : (
            <table className="table table-sm table-zebra w-full">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Label</th>
                  <th>Role</th>
                  <th>Created</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {tokens.map(token => (
                  <tr key={token.id}>
                    <td className="font-mono text-xs">{token.id.substring(0, 12)}</td>
                    <td className="text-sm">{token.label || '-'}</td>
                    <td>
                      <span className={`badge badge-sm ${token.role === 'operator' ? 'badge-primary' : 'badge-ghost'}`}>
                        {token.role}
                      </span>
                    </td>
                    <td className="text-xs">{formatDate(token.created_at)}</td>
                    <td>
                      <button
                        onClick={() => handleRevoke(token.id)}
                        className="btn btn-ghost btn-xs text-error"
                        aria-label={`Revoke token ${token.id}`}
                      >
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </AccessibleModal>
  )
}
