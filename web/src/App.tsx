import { useEffect } from 'react'
import { useGlobalStore } from './stores/globalStore'
import { useProjectStore } from './stores/projectStore'
import { useThemeStore } from './stores/themeStore'
import { useViewModeStore, isViewModeHydrated } from './stores/viewModeStore'
import { useLeakWatchdog } from './hooks/useLeakWatchdog'
import { useKeyboardShortcuts, SHORTCUTS } from './hooks/useKeyboardShortcuts'
import { ErrorBoundary } from './components/ErrorBoundary'
import { GlobalView } from './components/GlobalView'
import { ProjectView } from './components/ProjectView'
import { SimpleProjectView } from './components/SimpleProjectView'
import { ModePickerModal } from './components/ModePickerModal'
import { StateAnnouncer } from './components/StateAnnouncer'
import { checkForUpdates } from './lib/updater'

// Demo mode for testing UI without backend (dev builds only)
const DEMO_MODE = import.meta.env.DEV && new URLSearchParams(window.location.search).has('demo')

// URL param override for view mode — applied after rehydration so it wins over localStorage.
const URL_VIEW_MODE = new URLSearchParams(window.location.search).get('mode')
if (URL_VIEW_MODE === 'simple' || URL_VIEW_MODE === 'developer') {
  const applyUrlMode = () => {
    useViewModeStore.getState().setMode(URL_VIEW_MODE)
    useViewModeStore.getState().setIsFirstVisit(false)
  }
  if (isViewModeHydrated()) {
    applyUrlMode()
  } else {
    // Subscribe and apply once hydration fires
    const unsub = useViewModeStore.subscribe(() => {
      if (isViewModeHydrated()) {
        applyUrlMode()
        unsub()
      }
    })
  }
}

export default function App() {
  const { selectedProject, selectProject, connect, connected, connecting, projects } = useGlobalStore()
  const { connect: connectWorktree, disconnect: disconnectWorktree } = useProjectStore()
  const { theme, setTheme } = useThemeStore()
  const { mode, isFirstVisit } = useViewModeStore()

  const { showHelp, setShowHelp } = useKeyboardShortcuts()

  useLeakWatchdog((growthMB) => {
    console.error(`LeakWatchdog: heap grew +${growthMB.toFixed(0)}MB without GC recovery — reloading`)
    window.location.reload()
  })

  useEffect(() => {
    if (!DEMO_MODE) {
      connect()
      checkForUpdates()
    }
  }, [connect])

  // Restore selected project from sessionStorage after projects load
  useEffect(() => {
    if (DEMO_MODE || !connected || selectedProject) return

    const savedProjectId = sessionStorage.getItem('kvelmo-selectedProjectId')
    if (savedProjectId && projects.length > 0) {
      const project = projects.find(p => p.id === savedProjectId)
      if (project) {
        selectProject(project)
      } else {
        // Project no longer exists, clear sessionStorage
        sessionStorage.removeItem('kvelmo-selectedProjectId')
      }
    }
  }, [connected, projects, selectedProject, selectProject])

  // Initialize theme on mount (intentionally run once with initial theme value)
  useEffect(() => {
    setTheme(theme)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Connect to worktree socket when a project is selected
  useEffect(() => {
    if (DEMO_MODE) return
    if (selectedProject?.path) {
      connectWorktree(selectedProject.path)
    }
    return () => {
      disconnectWorktree()
    }
  }, [selectedProject?.path, connectWorktree, disconnectWorktree])

  // Demo mode: set mock project on mount
  useEffect(() => {
    if (!DEMO_MODE || selectedProject) return
    selectProject({
      id: 'demo-project',
      path: '/Users/demo/workspace/my-project',
      state: 'idle'
    })
  }, [selectedProject, selectProject])

  // Demo mode: show ProjectView with mock data
  if (DEMO_MODE) {
    return (
      <ErrorBoundary>
        <main id="main-content" tabIndex={-1} className="min-h-screen bg-base-100 transition-colors">
          {!isViewModeHydrated() ? null : (
            isFirstVisit ? <ModePickerModal /> :
            selectedProject
              ? (mode === 'simple' ? <SimpleProjectView /> : <ProjectView />)
              : (
                <div className="h-screen flex items-center justify-center">
                  <span className="loading loading-spinner loading-lg"></span>
                </div>
              )
          )}
        </main>
      </ErrorBoundary>
    )
  }

  if (connecting) {
    return (
      <ErrorBoundary>
        <main id="main-content" tabIndex={-1} className="min-h-screen bg-base-100 flex items-center justify-center">
          <div className="text-center">
            <span className="loading loading-spinner loading-lg text-primary"></span>
            <p className="mt-4 text-base-content/60">Connecting to server...</p>
          </div>
        </main>
      </ErrorBoundary>
    )
  }

  if (!connected) {
    return (
      <ErrorBoundary>
        <main id="main-content" tabIndex={-1} className="min-h-screen bg-base-100 flex items-center justify-center">
          <div className="text-center">
            <div className="text-error text-6xl mb-4" aria-hidden="true">!</div>
            <p className="text-base-content mb-4" role="alert">Failed to connect to server</p>
            <button onClick={() => connect()} className="btn btn-primary">
              Retry
            </button>
          </div>
        </main>
      </ErrorBoundary>
    )
  }

  return (
    <ErrorBoundary>
      <main id="main-content" tabIndex={-1} className="min-h-screen bg-base-100 transition-colors">
        <StateAnnouncer />
        {!isViewModeHydrated() ? null : (
          isFirstVisit ? <ModePickerModal /> :
          selectedProject
            ? (mode === 'simple' ? <SimpleProjectView /> : <ProjectView />)
            : <GlobalView />
        )}
      </main>
      {showHelp && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
          onClick={() => setShowHelp(false)}
          onKeyDown={(e) => { if (e.key === 'Escape') setShowHelp(false) }}
          role="button"
          tabIndex={0}
          aria-label="Close keyboard shortcuts dialog"
        >
          <div
            className="bg-base-200 rounded-lg shadow-xl p-6 max-w-md w-full mx-4"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
            role="presentation"
          >
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold">Keyboard Shortcuts</h2>
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => setShowHelp(false)}
                aria-label="Close"
              >
                Esc
              </button>
            </div>
            {Object.entries(
              SHORTCUTS.filter(s => !s.devOnly || mode === 'developer').reduce<Record<string, typeof SHORTCUTS>>((acc, s) => {
                ;(acc[s.section] ??= []).push(s)
                return acc
              }, {})
            ).map(([section, items]) => (
              <div key={section} className="mb-3">
                <h3 className="text-xs font-semibold uppercase text-base-content/50 mb-1">
                  {section}
                </h3>
                {items.map((s) => (
                  <div key={s.keys} className="flex justify-between py-1">
                    <span className="text-sm text-base-content/80">{s.description}</span>
                    <kbd className="kbd kbd-sm">{s.keys}</kbd>
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>
      )}
    </ErrorBoundary>
  )
}
