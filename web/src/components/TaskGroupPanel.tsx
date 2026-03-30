import { useState, useEffect, useCallback } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import type { TaskGroup } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

export function TaskGroupPanel({ onClose }: { onClose: () => void }) {
  const {
    taskGroups,
    loadTaskGroups,
    createTaskGroup,
    addTaskToGroup,
    submitTaskGroup,
    getTaskGroupStatus,
    removeTaskGroup,
    connected,
  } = useGlobalStore()
  const [newLabel, setNewLabel] = useState('')
  const [creating, setCreating] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  // Add task form state (per group)
  const [addingToGroup, setAddingToGroup] = useState<string | null>(null)
  const [addProjectDir, setAddProjectDir] = useState('')
  const [addTaskId, setAddTaskId] = useState('')
  const [submitting, setSubmitting] = useState<string | null>(null)
  const [statusDetail, setStatusDetail] = useState<TaskGroup | null>(null)

  useEffect(() => {
    if (connected) void loadTaskGroups()
  }, [connected, loadTaskGroups])

  const handleCreate = useCallback(async () => {
    if (!newLabel.trim()) return
    setCreating(true)
    await createTaskGroup(newLabel.trim())
    setNewLabel('')
    setCreating(false)
  }, [newLabel, createTaskGroup])

  const handleAddTask = useCallback(async (groupId: string) => {
    if (!addTaskId.trim() || !addProjectDir.trim()) return
    await addTaskToGroup(groupId, addTaskId.trim(), addProjectDir.trim())
    setAddTaskId('')
    setAddProjectDir('')
    setAddingToGroup(null)
  }, [addTaskId, addProjectDir, addTaskToGroup])

  const handleSubmit = useCallback(async (groupId: string) => {
    setSubmitting(groupId)
    await submitTaskGroup(groupId)
    setSubmitting(null)
  }, [submitTaskGroup])

  const handleRefreshStatus = useCallback(async (groupId: string) => {
    const result = await getTaskGroupStatus(groupId)
    setStatusDetail(result)
  }, [getTaskGroupStatus])

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
                  <p className="text-sm opacity-50">No tasks in this group yet.</p>
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

                {/* Add task form */}
                {addingToGroup === g.id ? (
                  <div className="mt-3 p-2 bg-base-300 rounded space-y-2">
                    <input
                      type="text"
                      className="input input-bordered input-xs w-full"
                      placeholder="Project directory (e.g. /path/to/project)"
                      value={addProjectDir}
                      onChange={(e) => setAddProjectDir(e.target.value)}
                    />
                    <input
                      type="text"
                      className="input input-bordered input-xs w-full"
                      placeholder="Task ID"
                      value={addTaskId}
                      onChange={(e) => setAddTaskId(e.target.value)}
                      onKeyDown={(e) => e.key === 'Enter' && handleAddTask(g.id)}
                    />
                    <div className="flex gap-2 justify-end">
                      <button className="btn btn-ghost btn-xs" onClick={() => setAddingToGroup(null)}>
                        Cancel
                      </button>
                      <button
                        className="btn btn-primary btn-xs"
                        onClick={() => handleAddTask(g.id)}
                        disabled={!addTaskId.trim() || !addProjectDir.trim()}
                      >
                        Add
                      </button>
                    </div>
                  </div>
                ) : (
                  <button
                    className="btn btn-ghost btn-xs mt-2"
                    onClick={() => { setAddingToGroup(g.id); setAddProjectDir(''); setAddTaskId('') }}
                  >
                    + Add Task
                  </button>
                )}

                {/* Status detail (after refresh) */}
                {statusDetail && statusDetail.id === g.id && (
                  <div className="mt-2 p-2 bg-base-300 rounded text-xs">
                    <div>Status: <strong>{statusDetail.status}</strong></div>
                    <div>Tasks: {statusDetail.tasks.length}</div>
                    <div>Updated: {statusDetail.updated_at}</div>
                  </div>
                )}

                <div className="mt-2 flex gap-2 justify-end">
                  <button
                    className="btn btn-ghost btn-xs"
                    onClick={() => handleRefreshStatus(g.id)}
                  >
                    Refresh
                  </button>
                  <button
                    className="btn btn-info btn-xs"
                    onClick={() => handleSubmit(g.id)}
                    disabled={submitting === g.id || g.tasks.length === 0}
                  >
                    {submitting === g.id ? 'Submitting...' : 'Submit Group'}
                  </button>
                  <button
                    className="btn btn-ghost btn-xs text-error"
                    onClick={() => handleRemove(g.id)}
                  >
                    Remove
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
