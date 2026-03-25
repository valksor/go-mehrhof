import { useState, useEffect, useCallback } from 'react'
import { useProjectStore } from '../stores/projectStore'
import type { ForkInfo } from '../types/conductor'

interface ForkComparison {
  forks: Array<{
    id: string
    label: string
    state: string
    files_added: number
    files_modified: number
    lines_added: number
    lines_removed: number
    diff_stats: { files: number; added: number; removed: number }
  }>
}

export function ForkComparePanel() {
  const { forks, listForks, createFork, compareForks, selectFork, connected } = useProjectStore()
  const [comparison, setComparison] = useState<ForkComparison | null>(null)
  const [newLabel, setNewLabel] = useState('')
  const [creating, setCreating] = useState(false)
  const [selecting, setSelecting] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (connected) listForks()
  }, [connected, listForks])

  const handleCreate = useCallback(async () => {
    if (!newLabel.trim()) return
    setCreating(true)
    await createFork(newLabel.trim())
    setNewLabel('')
    setCreating(false)
  }, [newLabel, createFork])

  const handleCompare = useCallback(async () => {
    setLoading(true)
    const result = await compareForks()
    if (result) {
      setComparison(result as unknown as ForkComparison)
    }
    setLoading(false)
  }, [compareForks])

  const handleSelect = useCallback(async (forkId: string) => {
    setSelecting(forkId)
    await selectFork(forkId)
    setSelecting(null)
  }, [selectFork])

  return (
    <div className="p-4 space-y-4">
      <h3 className="text-lg font-semibold">Conversation Forks</h3>
      <p className="text-sm opacity-70">
        Create parallel implementation approaches and compare them side by side.
      </p>

      {/* Create fork */}
      <div className="flex gap-2">
        <input
          type="text"
          className="input input-bordered input-sm flex-1"
          placeholder="Fork label (e.g. approach-A)"
          value={newLabel}
          onChange={(e) => setNewLabel(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
        />
        <button
          className="btn btn-primary btn-sm"
          onClick={handleCreate}
          disabled={creating || !newLabel.trim()}
        >
          {creating ? 'Creating...' : 'Create Fork'}
        </button>
      </div>

      {/* Forks list */}
      {forks.length === 0 ? (
        <p className="text-center opacity-50 py-4">No forks yet.</p>
      ) : (
        <>
          <div className="overflow-x-auto">
            <table className="table table-sm">
              <thead>
                <tr>
                  <th>Label</th>
                  <th>State</th>
                  <th>Branch</th>
                  <th>Base</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {forks.map((f: ForkInfo) => (
                  <tr key={f.id}>
                    <td className="font-medium">{f.label}</td>
                    <td>
                      <span className="badge badge-sm badge-ghost">{f.state}</span>
                    </td>
                    <td className="font-mono text-xs">{f.branch}</td>
                    <td className="font-mono text-xs">{f.checkpoint_sha?.slice(0, 8)}</td>
                    <td>
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => handleSelect(f.id)}
                        disabled={selecting === f.id}
                      >
                        {selecting === f.id ? 'Selecting...' : 'Select'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <button
            className="btn btn-outline btn-sm"
            onClick={handleCompare}
            disabled={loading || forks.length < 2}
          >
            {loading ? 'Comparing...' : 'Compare Forks'}
          </button>

          {/* Comparison results */}
          {comparison && comparison.forks && comparison.forks.length > 0 && (
            <div className="mt-2">
              <h4 className="text-sm font-semibold mb-2">Comparison</h4>
              <div className="overflow-x-auto">
                <table className="table table-xs">
                  <thead>
                    <tr>
                      <th>Fork</th>
                      <th>Files</th>
                      <th>Lines Added</th>
                      <th>Lines Removed</th>
                    </tr>
                  </thead>
                  <tbody>
                    {comparison.forks.map((f) => (
                      <tr key={f.id}>
                        <td>{f.label}</td>
                        <td>{f.diff_stats?.files ?? 0}</td>
                        <td className="text-success">+{f.lines_added}</td>
                        <td className="text-error">-{f.lines_removed}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
