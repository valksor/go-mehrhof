import { useState } from 'react'
import { useProjectStore, QueuedTask } from '../stores/projectStore'
import { EmptyState } from './EmptyState'

export function TaskQueue() {
  const { taskQueue, queueTask, dequeueTask, reorderQueue, connected, loading } = useProjectStore()
  const [showAdd, setShowAdd] = useState(false)
  const [source, setSource] = useState('')
  const [title, setTitle] = useState('')

  const handleAdd = async () => {
    if (!source.trim()) return
    await queueTask(source.trim(), title.trim() || undefined)
    setSource('')
    setTitle('')
    setShowAdd(false)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && source.trim()) {
      void handleAdd()
    }
    if (e.key === 'Escape') {
      setShowAdd(false)
    }
  }

  const handleMoveUp = (task: QueuedTask) => {
    if (task.position > 1) {
      void reorderQueue(task.id, task.position - 1)
    }
  }

  const handleMoveDown = (task: QueuedTask) => {
    if (task.position < taskQueue.length) {
      void reorderQueue(task.id, task.position + 1)
    }
  }

  return (
    <section className="card bg-base-200">
      <div className="card-body">
        <div className="flex items-center justify-between mb-2">
          <h2 className="card-title text-base-content text-sm flex items-center gap-2">
            <svg aria-hidden="true" className="w-4 h-4 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 10h16M4 14h16M4 18h16" />
            </svg>
            Queue
            {taskQueue.length > 0 && (
              <span className="badge badge-sm badge-primary">{taskQueue.length}</span>
            )}
          </h2>
          <button
            onClick={() => setShowAdd(!showAdd)}
            disabled={!connected || loading}
            className="btn btn-ghost btn-xs"
            aria-label="Add task to queue"
          >
            <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
          </button>
        </div>

        {/* Add form */}
        {showAdd && (
          <div className="space-y-2 mb-3 p-2 bg-base-300 rounded-lg">
            <input
              type="text"
              value={source}
              onChange={e => setSource(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="github.com/owner/repo/issues/123"
              className="input input-bordered input-sm w-full font-mono text-xs"
              ref={(el) => el?.focus()}
            />
            <input
              type="text"
              value={title}
              onChange={e => setTitle(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Optional title"
              className="input input-bordered input-sm w-full text-xs"
            />
            <div className="flex justify-end gap-1">
              <button onClick={() => setShowAdd(false)} className="btn btn-ghost btn-xs">
                Cancel
              </button>
              <button
                onClick={handleAdd}
                disabled={!source.trim()}
                className="btn btn-primary btn-xs"
              >
                Add
              </button>
            </div>
          </div>
        )}

        {/* Queue list */}
        {taskQueue.length === 0 && !showAdd && (
          <EmptyState title="No tasks in queue" icon="📋" />
        )}

        {taskQueue.length > 0 && (
          <ul className="space-y-1">
            {taskQueue.map((task) => (
              <li key={task.id} className="flex items-center gap-2 p-1.5 bg-base-300 rounded text-xs group">
                <span className="badge badge-xs badge-ghost font-mono">{task.position}</span>
                <div className="flex-1 min-w-0">
                  <p className="truncate font-medium">
                    {task.title || task.source}
                  </p>
                  {task.title && (
                    <p className="truncate text-base-content/50 font-mono text-[10px]">{task.source}</p>
                  )}
                </div>
                <div className="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    onClick={() => handleMoveUp(task)}
                    disabled={task.position <= 1}
                    className="btn btn-ghost btn-xs px-1"
                    aria-label="Move up"
                  >
                    <svg aria-hidden="true" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
                    </svg>
                  </button>
                  <button
                    onClick={() => handleMoveDown(task)}
                    disabled={task.position >= taskQueue.length}
                    className="btn btn-ghost btn-xs px-1"
                    aria-label="Move down"
                  >
                    <svg aria-hidden="true" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>
                  <button
                    onClick={() => dequeueTask(task.id)}
                    className="btn btn-ghost btn-xs px-1 text-error"
                    aria-label="Remove from queue"
                  >
                    <svg aria-hidden="true" className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  )
}
