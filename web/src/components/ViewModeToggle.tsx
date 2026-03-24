import { useViewModeStore } from '../stores/viewModeStore'

export function ViewModeToggle() {
  const { mode, setMode } = useViewModeStore()

  return (
    <div className="join" data-testid="view-mode-toggle">
      <button
        className={`btn btn-sm join-item ${mode === 'simple' ? 'btn-active btn-primary' : ''}`}
        onClick={() => setMode('simple')}
        title="Simple mode — streamlined interface"
        aria-pressed={mode === 'simple'}
      >
        Simple
      </button>
      <button
        className={`btn btn-sm join-item ${mode === 'developer' ? 'btn-active btn-primary' : ''}`}
        onClick={() => setMode('developer')}
        title="Developer mode — full control"
        aria-pressed={mode === 'developer'}
      >
        Developer
      </button>
    </div>
  )
}
