import { useEffect, useRef } from 'react'
import { useAnnouncer } from './ui/useAnnouncer'
import { useGlobalStore } from '../stores/globalStore'
import { useProjectStore } from '../stores/projectStore'

/** Extract project name from path, handling Windows/Unix paths and edge cases */
function getProjectName(path: string): string {
  if (!path) return 'project'
  // Normalize separators (Windows uses backslashes)
  const normalized = path.replace(/\\/g, '/')
  const segments = normalized.split('/').filter(Boolean)
  return segments.pop() || path || 'project'
}

export function StateAnnouncer() {
  const { announce } = useAnnouncer()
  const connected = useGlobalStore((s) => s.connected)
  const connecting = useGlobalStore((s) => s.connecting)
  const selectedProject = useGlobalStore((s) => s.selectedProject)
  const taskState = useProjectStore((s) => s.state)

  // Initialize refs to current values so we don't announce on first render
  const prev = useRef({ connected, connecting, projectId: selectedProject?.id, taskState })

  useEffect(() => {
    const p = prev.current

    if (connecting && !p.connecting) {
      announce('Connecting to server')
    }
    if (connected && !p.connected) {
      announce('Connected to server')
    }
    if (!connected && p.connected) {
      announce('Disconnected from server', 'assertive')
    }
    if (selectedProject?.id !== p.projectId) {
      if (selectedProject) {
        announce(`Opened project ${getProjectName(selectedProject.path)}`)
      } else {
        announce('Project closed')
      }
    }
    if (taskState !== p.taskState) {
      const labels: Record<string, string> = {
        none: 'Idle',
        loaded: 'Task loaded',
        planning: 'Planning in progress',
        planned: 'Planning complete',
        implementing: 'Implementation in progress',
        implemented: 'Implementation complete',
        optimizing: 'Optimizing',
        simplifying: 'Simplifying',
        reviewing: 'Review in progress',
        submitted: 'Task submitted',
        waiting: 'Waiting',
        paused: 'Paused',
        failed: 'Task failed',
      }
      const label = labels[taskState]
      if (label) {
        announce(label, taskState === 'failed' ? 'assertive' : 'polite')
      }
    }

    prev.current = { connected, connecting, projectId: selectedProject?.id, taskState }
  }, [connected, connecting, selectedProject, taskState, announce])

  return null
}
