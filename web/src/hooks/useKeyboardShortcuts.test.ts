import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useKeyboardShortcuts, SHORTCUTS } from './useKeyboardShortcuts'

// Mock all stores
const mockSelectProject = vi.fn()
const mockSelectNextProject = vi.fn()
const mockSelectPrevProject = vi.fn()
const mockUndo = vi.fn()
const mockRedo = vi.fn()
const mockPlan = vi.fn()
const mockImplement = vi.fn()
const mockReview = vi.fn()
const mockStop = vi.fn()
const mockAbort = vi.fn()
const mockSetActiveTab = vi.fn()
const mockSetModalCommand = vi.fn()
const mockToggleViewMode = vi.fn()

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: {
    getState: () => ({
      selectedProject: { id: 'proj-1', path: '/test' },
      selectProject: mockSelectProject,
      selectNextProject: mockSelectNextProject,
      selectPrevProject: mockSelectPrevProject,
    }),
  },
}))

vi.mock('../stores/projectStore', () => ({
  useProjectStore: {
    getState: () => ({
      undo: mockUndo,
      redo: mockRedo,
      plan: mockPlan,
      implement: mockImplement,
      review: mockReview,
      stop: mockStop,
      abort: mockAbort,
    }),
  },
}))

vi.mock('../stores/layoutStore', () => ({
  useLayoutStore: {
    getState: () => ({
      tabs: [
        { id: 'chat', type: 'chat', title: 'Chat' },
        { id: 'files', type: 'files', title: 'Files' },
        { id: 'diff', type: 'diff', title: 'Diff' },
      ],
      setActiveTab: mockSetActiveTab,
      setModalCommand: mockSetModalCommand,
    }),
  },
}))

vi.mock('../stores/viewModeStore', () => ({
  useViewModeStore: {
    getState: () => ({
      mode: 'developer',
      toggle: mockToggleViewMode,
    }),
  },
}))

function fireKey(key: string, opts: Partial<KeyboardEventInit> = {}) {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, ...opts })
  window.dispatchEvent(event)
}

describe('useKeyboardShortcuts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns showHelp as false initially', () => {
    const { result } = renderHook(() => useKeyboardShortcuts())
    expect(result.current.showHelp).toBe(false)
  })

  it('? toggles help overlay', () => {
    const { result } = renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('?'))
    expect(result.current.showHelp).toBe(true)
    act(() => fireKey('?'))
    expect(result.current.showHelp).toBe(false)
  })

  it('Escape closes help overlay', () => {
    const { result } = renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('?'))
    expect(result.current.showHelp).toBe(true)
    act(() => fireKey('Escape'))
    expect(result.current.showHelp).toBe(false)
  })

  it('Ctrl+/ toggles help', () => {
    const { result } = renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('/', { ctrlKey: true }))
    expect(result.current.showHelp).toBe(true)
    act(() => fireKey('/', { ctrlKey: true }))
    expect(result.current.showHelp).toBe(false)
  })

  it('Ctrl+z triggers undo', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('z', { ctrlKey: true }))
    expect(mockUndo).toHaveBeenCalled()
  })

  it('Ctrl+Shift+z triggers redo', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('z', { ctrlKey: true, shiftKey: true }))
    expect(mockRedo).toHaveBeenCalled()
  })

  it('Ctrl+Shift+V toggles view mode', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('v', { ctrlKey: true, shiftKey: true }))
    expect(mockToggleViewMode).toHaveBeenCalled()
  })

  it('number keys 1-3 switch tabs in dev mode', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('1'))
    expect(mockSetActiveTab).toHaveBeenCalledWith('chat')
    act(() => fireKey('2'))
    expect(mockSetActiveTab).toHaveBeenCalledWith('files')
    act(() => fireKey('3'))
    expect(mockSetActiveTab).toHaveBeenCalledWith('diff')
  })

  it('cleans up event listener on unmount', () => {
    const spy = vi.spyOn(window, 'removeEventListener')
    const { unmount } = renderHook(() => useKeyboardShortcuts())
    unmount()
    expect(spy).toHaveBeenCalledWith('keydown', expect.any(Function))
    spy.mockRestore()
  })
})

describe('useKeyboardShortcuts chords', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('g p chord deselects project', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('g'))
    act(() => {
      vi.advanceTimersByTime(100)
    })
    act(() => fireKey('p'))
    expect(mockSelectProject).toHaveBeenCalledWith(null)
  })

  it('g a chord triggers abort', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('g'))
    act(() => {
      vi.advanceTimersByTime(100)
    })
    act(() => fireKey('a'))
    expect(mockAbort).toHaveBeenCalled()
  })

  it('g l chord triggers plan in dev mode', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('g'))
    act(() => {
      vi.advanceTimersByTime(100)
    })
    act(() => fireKey('l'))
    expect(mockPlan).toHaveBeenCalled()
  })

  it('g i chord triggers implement in dev mode', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('g'))
    act(() => {
      vi.advanceTimersByTime(100)
    })
    act(() => fireKey('i'))
    expect(mockImplement).toHaveBeenCalled()
  })

  it('g s chord opens submit modal in dev mode', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('g'))
    act(() => {
      vi.advanceTimersByTime(100)
    })
    act(() => fireKey('s'))
    expect(mockSetModalCommand).toHaveBeenCalledWith('submit')
  })

  it('g x chord triggers stop in dev mode', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('g'))
    act(() => {
      vi.advanceTimersByTime(100)
    })
    act(() => fireKey('x'))
    expect(mockStop).toHaveBeenCalled()
  })

  it('chord expires after 500ms', () => {
    renderHook(() => useKeyboardShortcuts())
    act(() => fireKey('g'))
    act(() => {
      vi.advanceTimersByTime(600)
    })
    act(() => fireKey('p'))
    expect(mockSelectProject).not.toHaveBeenCalled()
  })
})

describe('SHORTCUTS constant', () => {
  it('contains expected shortcut entries', () => {
    expect(SHORTCUTS.length).toBeGreaterThan(5)
    expect(SHORTCUTS.some((s) => s.keys === '?')).toBe(true)
    expect(SHORTCUTS.some((s) => s.keys === 'Escape')).toBe(true)
    expect(SHORTCUTS.some((s) => s.keys === 'Ctrl+z')).toBe(true)
  })

  it('has sections for each shortcut', () => {
    for (const s of SHORTCUTS) {
      expect(s.section).toBeTruthy()
    }
  })
})
