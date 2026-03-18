import { useEffect, useRef, useState } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useGlobalStore } from '../stores/globalStore'
import { useLayoutStore } from '../stores/layoutStore'

export interface Shortcut {
  keys: string
  description: string
  section: string
}

export const SHORTCUTS: Shortcut[] = [
  // Navigation
  { keys: '?', description: 'Toggle shortcuts help', section: 'Navigation' },
  { keys: 'Ctrl+/', description: 'Toggle shortcuts help', section: 'Navigation' },
  { keys: '/', description: 'Focus chat input', section: 'Navigation' },
  { keys: 'Escape', description: 'Close help / deselect project', section: 'Navigation' },
  { keys: 'j', description: 'Next project', section: 'Navigation' },
  { keys: 'k', description: 'Previous project', section: 'Navigation' },
  { keys: 'g p', description: 'Go to projects (GlobalView)', section: 'Navigation' },
  { keys: 'g a', description: 'Abort/stop current operation', section: 'Navigation' },

  // Tab switching
  { keys: '1-5', description: 'Switch to tab by index', section: 'Tabs' },

  // Workflow actions
  { keys: 'g l', description: 'Plan (start planning)', section: 'Workflow' },
  { keys: 'g i', description: 'Implement', section: 'Workflow' },
  { keys: 'g r', description: 'Review', section: 'Workflow' },
  { keys: 'g s', description: 'Submit', section: 'Workflow' },
  { keys: 'g x', description: 'Stop current operation', section: 'Workflow' },
  { keys: 'Ctrl+z', description: 'Undo', section: 'Workflow' },
  { keys: 'Ctrl+Shift+z', description: 'Redo', section: 'Workflow' },
]

function isInputFocused(): boolean {
  const el = document.activeElement
  if (!el) return false
  const tag = el.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA') return true
  if ((el as HTMLElement).isContentEditable) return true
  return false
}

export function useKeyboardShortcuts() {
  const [showHelp, setShowHelp] = useState(false)
  const showHelpRef = useRef(false)
  const chordKeyRef = useRef<string | null>(null)
  const chordTimeRef = useRef<number>(0)

  // Keep ref in sync with state so the keydown handler always sees current value
  useEffect(() => {
    showHelpRef.current = showHelp
  }, [showHelp])

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      const ctrlOrMeta = e.ctrlKey || e.metaKey
      const inInput = isInputFocused()

      // Escape — close help overlay (works everywhere), or deselect project
      if (e.key === 'Escape') {
        if (showHelpRef.current) {
          setShowHelp(false)
          return
        }
        if (!inInput) {
          const { selectProject } = useGlobalStore.getState()
          selectProject(null)
        }
        return
      }

      // Ctrl+/ or Cmd+/ — toggle help (works even in inputs)
      if (ctrlOrMeta && e.key === '/') {
        e.preventDefault()
        setShowHelp((prev) => !prev)
        return
      }

      // Ctrl+z / Cmd+z — undo (works even in inputs for workflow undo)
      if (ctrlOrMeta && !e.shiftKey && e.key === 'z') {
        // Only intercept if we have an active project (otherwise let browser handle)
        const { selectedProject } = useGlobalStore.getState()
        if (selectedProject) {
          e.preventDefault()
          const { undo } = useProjectStore.getState()
          undo()
        }
        return
      }

      // Ctrl+Shift+z / Cmd+Shift+z — redo (works even in inputs)
      if (ctrlOrMeta && e.shiftKey && (e.key === 'z' || e.key === 'Z')) {
        const { selectedProject } = useGlobalStore.getState()
        if (selectedProject) {
          e.preventDefault()
          const { redo } = useProjectStore.getState()
          redo()
        }
        return
      }

      // Everything below is skipped when in input fields
      if (inInput) return

      // ? — toggle help overlay
      if (e.key === '?' && !ctrlOrMeta) {
        e.preventDefault()
        setShowHelp((prev) => !prev)
        return
      }

      // / — focus chat input
      if (e.key === '/' && !ctrlOrMeta && !e.shiftKey && !e.altKey) {
        e.preventDefault()
        const chatInput = document.querySelector<HTMLElement>('[data-chat-input]')
        if (chatInput) chatInput.focus()
        return
      }

      // j/k — navigate projects (only in GlobalView, i.e. no project selected)
      if (e.key === 'j' && !ctrlOrMeta && !e.shiftKey && !e.altKey) {
        const { selectedProject } = useGlobalStore.getState()
        if (!selectedProject || !selectedProject.path) {
          // In GlobalView — navigate project list
          e.preventDefault()
          useGlobalStore.getState().selectNextProject()
          return
        }
      }

      if (e.key === 'k' && !ctrlOrMeta && !e.shiftKey && !e.altKey) {
        const { selectedProject } = useGlobalStore.getState()
        if (!selectedProject || !selectedProject.path) {
          e.preventDefault()
          useGlobalStore.getState().selectPrevProject()
          return
        }
      }

      // Chord: g p — go to projects
      const now = Date.now()
      if (e.key === 'g' && !ctrlOrMeta && !e.shiftKey && !e.altKey) {
        chordKeyRef.current = 'g'
        chordTimeRef.current = now
        return
      }

      if (
        e.key === 'p' &&
        !ctrlOrMeta &&
        !e.shiftKey &&
        !e.altKey &&
        chordKeyRef.current === 'g' &&
        now - chordTimeRef.current < 500
      ) {
        e.preventDefault()
        chordKeyRef.current = null
        const { selectProject } = useGlobalStore.getState()
        selectProject(null)
        return
      }

      // Chord: g a — abort/stop current operation
      if (
        e.key === 'a' &&
        !ctrlOrMeta &&
        !e.shiftKey &&
        !e.altKey &&
        chordKeyRef.current === 'g' &&
        now - chordTimeRef.current < 500
      ) {
        e.preventDefault()
        chordKeyRef.current = null
        const { abort } = useProjectStore.getState()
        abort()
        return
      }

      // Chord: g l — plan
      if (
        e.key === 'l' &&
        !ctrlOrMeta &&
        !e.shiftKey &&
        !e.altKey &&
        chordKeyRef.current === 'g' &&
        now - chordTimeRef.current < 500
      ) {
        e.preventDefault()
        chordKeyRef.current = null
        const { plan } = useProjectStore.getState()
        plan(false)
        return
      }

      // Chord: g i — implement
      if (
        e.key === 'i' &&
        !ctrlOrMeta &&
        !e.shiftKey &&
        !e.altKey &&
        chordKeyRef.current === 'g' &&
        now - chordTimeRef.current < 500
      ) {
        e.preventDefault()
        chordKeyRef.current = null
        const { implement } = useProjectStore.getState()
        implement(false)
        return
      }

      // Chord: g r — review
      if (
        e.key === 'r' &&
        !ctrlOrMeta &&
        !e.shiftKey &&
        !e.altKey &&
        chordKeyRef.current === 'g' &&
        now - chordTimeRef.current < 500
      ) {
        e.preventDefault()
        chordKeyRef.current = null
        const { review } = useProjectStore.getState()
        review({ approve: true })
        return
      }

      // Chord: g s — submit (opens modal)
      if (
        e.key === 's' &&
        !ctrlOrMeta &&
        !e.shiftKey &&
        !e.altKey &&
        chordKeyRef.current === 'g' &&
        now - chordTimeRef.current < 500
      ) {
        e.preventDefault()
        chordKeyRef.current = null
        const { setModalCommand } = useLayoutStore.getState()
        setModalCommand('submit')
        return
      }

      // Chord: g x — stop
      if (
        e.key === 'x' &&
        !ctrlOrMeta &&
        !e.shiftKey &&
        !e.altKey &&
        chordKeyRef.current === 'g' &&
        now - chordTimeRef.current < 500
      ) {
        e.preventDefault()
        chordKeyRef.current = null
        const { stop } = useProjectStore.getState()
        stop()
        return
      }

      // Reset chord if a non-matching key is pressed
      if (chordKeyRef.current && e.key !== 'g') {
        chordKeyRef.current = null
      }

      // 1-5 — switch to tab by index
      const num = parseInt(e.key, 10)
      if (num >= 1 && num <= 5 && !ctrlOrMeta && !e.shiftKey && !e.altKey) {
        e.preventDefault()
        const { tabs, setActiveTab } = useLayoutStore.getState()
        const targetTab = tabs[num - 1]
        if (targetTab) {
          setActiveTab(targetTab.id)
        }
        return
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  return { showHelp, setShowHelp }
}
