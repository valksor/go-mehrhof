import { useEffect } from 'react'
import { useViewModeStore, type ViewMode } from '../stores/viewModeStore'

export function ModePickerModal() {
  const { setMode, setIsFirstVisit } = useViewModeStore()

  const pickMode = (mode: ViewMode) => {
    setMode(mode)
    setIsFirstVisit(false)
  }

  // Escape defaults to simple mode so users are never stuck.
  // Store setters are stable references, so the empty dep array is safe.
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        useViewModeStore.getState().setMode('simple')
        useViewModeStore.getState().setIsFirstVisit(false)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" role="dialog" aria-modal="true" aria-labelledby="mode-picker-title">
      <div className="bg-base-200 rounded-2xl shadow-2xl p-8 max-w-lg w-full">
        <h2 id="mode-picker-title" className="text-2xl font-bold text-center mb-2">Welcome to kvelmo</h2>
        <p className="text-base-content/60 text-center mb-8">How would you like to use kvelmo?</p>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {/* Simple mode card */}
          <button
            onClick={() => pickMode('simple')}
            className="card bg-base-100 hover:bg-primary/10 border-2 border-transparent hover:border-primary transition-all cursor-pointer p-6 text-left"
          >
            <div className="text-3xl mb-3" aria-hidden="true">
              <svg className="w-8 h-8 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z" />
              </svg>
            </div>
            <h3 className="font-semibold text-lg mb-1">Simple</h3>
            <p className="text-sm text-base-content/60">
              Load tasks and let the AI handle it. Streamlined step-by-step workflow.
              Advanced tools available in Developer mode.
            </p>
          </button>

          {/* Developer mode card */}
          <button
            onClick={() => pickMode('developer')}
            className="card bg-base-100 hover:bg-primary/10 border-2 border-transparent hover:border-primary transition-all cursor-pointer p-6 text-left"
          >
            <div className="text-3xl mb-3" aria-hidden="true">
              <svg className="w-8 h-8 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
            </div>
            <h3 className="font-semibold text-lg mb-1">Developer</h3>
            <p className="text-sm text-base-content/60">
              Full control over the workflow with tabs, panels, and advanced tools.
            </p>
          </button>
        </div>

        <p className="text-xs text-base-content/40 text-center mt-4">
          Press Escape for Simple mode. You can switch anytime with Ctrl+Shift+V.
        </p>
      </div>
    </div>
  )
}
