import { useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import type { TaskListSummary } from '../types/socket'

const STATE_BADGE: Record<string, string> = {
  none: 'badge-ghost',
  loaded: 'badge-info',
  planning: 'badge-warning',
  planned: 'badge-primary',
  implementing: 'badge-warning',
  implemented: 'badge-success',
  reviewing: 'badge-warning',
  submitted: 'badge-secondary',
  failed: 'badge-error',
  paused: 'badge-ghost',
  waiting: 'badge-ghost',
  optimizing: 'badge-warning',
  simplifying: 'badge-warning',
}

function stateIsActive(state: string): boolean {
  return state !== 'none' && state !== 'submitted'
}

interface ActiveTasksWidgetProps {
  onSelectProject?: (path: string) => void
}

export function ActiveTasksWidget({ onSelectProject }: ActiveTasksWidgetProps) {
  const { activeTasks, loadActiveTasks, connected, projects, selectProject } = useGlobalStore()

  // Active tasks are updated via WebSocket push events (task_state_changed).
  // Initial load on connect; subsequent updates are event-driven via globalStore subscription.
  useEffect(() => {
    if (!connected) return
    void loadActiveTasks()
  }, [connected, loadActiveTasks])

  const active = activeTasks.filter(t => stateIsActive(t.state))

  if (active.length === 0) return null

  const handleClick = (task: TaskListSummary) => {
    // Select project by path (strict matching to avoid cross-project task display)
    const project = projects.find(p => p.path === task.path)
    if (project) {
      selectProject(project)
    } else if (onSelectProject) {
      onSelectProject(task.path)
    }
  }

  return (
    <section className="card bg-base-200 max-w-2xl mx-auto mt-4">
      <div className="card-body">
        <h2 className="card-title text-base-content flex items-center gap-2">
          <svg className="w-5 h-5 text-warning" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
          Active Tasks
          <span className="badge badge-sm badge-warning">{active.length}</span>
        </h2>

        <div className="space-y-2 mt-2">
          {active.map(task => (
            <button
              key={task.id}
              onClick={() => handleClick(task)}
              className="w-full text-left p-3 rounded-lg bg-base-100 hover:bg-base-300 border border-transparent hover:border-primary/30 transition-all duration-150 group"
            >
              <div className="flex items-center justify-between gap-2">
                <div className="flex-1 min-w-0">
                  {task.task_title ? (
                    <p className="font-medium text-sm text-base-content truncate">{task.task_title}</p>
                  ) : (
                    <p className="font-mono text-xs text-base-content/60 truncate">{task.path.split('/').pop()}</p>
                  )}
                  <div className="flex items-center gap-2 mt-0.5">
                    {task.source && (
                      <span className="text-xs text-base-content/50 font-mono truncate">{task.source}</span>
                    )}
                    {!task.source && (
                      <span className="text-xs text-base-content/40 font-mono truncate">{task.path}</span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-1 flex-shrink-0">
                  {task.pending_prompt_id && (
                    <span className="badge badge-xs badge-accent" title="Waiting for input">prompt</span>
                  )}
                  {task.last_failure_class === 'hard_stop' && (
                    <span className="badge badge-xs badge-error" title={task.last_error || 'Blocked'}>blocked</span>
                  )}
                  {task.queue_count != null && task.queue_count > 0 && (
                    <span className="badge badge-xs badge-outline">+{task.queue_count}</span>
                  )}
                  <span className={`badge badge-sm ${STATE_BADGE[task.state] || 'badge-ghost'}`}>
                    {task.state}
                  </span>
                </div>
              </div>
            </button>
          ))}
        </div>
      </div>
    </section>
  )
}
