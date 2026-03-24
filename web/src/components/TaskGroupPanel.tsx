import { useState, useEffect, useCallback } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import type { TaskGroup } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

export function TaskGroupPanel({ onClose }: { onClose: () => void }) {
  const { taskGroups, loadTaskGroups, createTaskGroup, removeTaskGroup, connected } = useGlobalStore()
  const [newLabel, setNewLabel] = useState('')
  const [creating, setCreating] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  useEffect(() => {
    if (connected) loadTaskGroups()
  }, [connected, loadTaskGroups])

  const handleCreate = useCallback(async () => {
    if (!newLabel.trim()) return
    setCreating(true)
    await createTaskGroup(newLabel.trim())
    setNewLabel('')
    setCreating(false)
  }, [newLabel, createTaskGroup])

  const handleRemove = useCallback(async (id: string) => {
    await removeTaskGroup(id)
  }, [removeTaskGroup])

  const statusBadge = (status: string) => {
    const colors: Record<string, string> = {
      active: 'badge-warning',
      ready: 'badge-success',
      submitted: 'badge-info',
      completed: 'badge-ghost',
    }
    return <span className={`badge badge-sm ${colors[status] || 'badge-ghost'}`}>{status}</span>
  }

  return (
    <AccessibleModal isOpen onClose={onClose} title="Task Groups" size="2xl">
      <p className="text-sm opacity-70 mb-4">
        Link tasks across repositories for synchronized lifecycle operations.
      </p>

      {/* Create form */}
      <div className="flex gap-2 mb-4">
        <input
          type="text"
          className="input input-bordered input-sm flex-1"
          placeholder="Group label (e.g. API + Client)"
          value={newLabel}
          onChange={(e) => setNewLabel(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
        />
        <button
          className="btn btn-primary btn-sm"
          onClick={handleCreate}
          disabled={creating || !newLabel.trim()}
        >
          {creating ? 'Creating...' : 'Create'}
        </button>
      </div>

      {/* Groups list */}
      {taskGroups.length === 0 ? (
        <p className="text-center opacity-50 py-8">No task groups yet.</p>
      ) : (
        <div className="space-y-2">
          {taskGroups.map((g: TaskGroup) => (
            <div key={g.id} className="collapse collapse-arrow bg-base-200">
              <input
                type="checkbox"
                checked={expandedId === g.id}
                onChange={() => setExpandedId(expandedId === g.id ? null : g.id)}
              />
              <div className="collapse-title text-sm font-medium flex items-center gap-2">
                <span>{g.label}</span>
                {statusBadge(g.status)}
                <span className="opacity-50 text-xs ml-auto mr-8">
                  {g.tasks.length} task{g.tasks.length !== 1 ? 's' : ''}
                </span>
              </div>
              <div className="collapse-content">
                <div className="text-xs opacity-60 mb-2">ID: {g.id}</div>
                {g.tasks.length === 0 ? (
                  <p className="text-sm opacity-50">No tasks in group. Use CLI to add tasks.</p>
                ) : (
                  <table className="table table-xs">
                    <thead>
                      <tr>
                        <th>Task</th>
                        <th>State</th>
                        <th>Project</th>
                      </tr>
                    </thead>
                    <tbody>
                      {g.tasks.map((t) => (
                        <tr key={t.task_id}>
                          <td className="font-mono text-xs">{t.task_id}</td>
                          <td>{t.state}</td>
                          <td className="text-xs opacity-70 truncate max-w-48">{t.project_dir}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
                <div className="mt-2 flex justify-end">
                  <button
                    className="btn btn-ghost btn-xs text-error"
                    onClick={() => handleRemove(g.id)}
                  >
                    Remove Group
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </AccessibleModal>
  )
}
