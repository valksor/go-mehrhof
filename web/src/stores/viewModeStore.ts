import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { storeName } from '../meta'

export type ViewMode = 'simple' | 'developer'

interface ViewModeState {
  mode: ViewMode
  isFirstVisit: boolean
  setMode: (mode: ViewMode) => void
  toggle: () => void
  setIsFirstVisit: (value: boolean) => void
}

let hydrated = false

export function isViewModeHydrated(): boolean {
  return hydrated
}

export const useViewModeStore = create<ViewModeState>()(
  persist(
    (set, get) => ({
      mode: 'simple',
      isFirstVisit: true,
      setMode: (mode: ViewMode) => {
        set({ mode })
      },
      toggle: () => {
        const newMode = get().mode === 'simple' ? 'developer' : 'simple'
        set({ mode: newMode })
      },
      setIsFirstVisit: (value: boolean) => {
        set({ isFirstVisit: value })
      },
    }),
    {
      name: storeName('viewMode'),
      version: 1,
      migrate: (persistedState: unknown) => {
        const state = persistedState as Record<string, unknown>
        // Migration guard: if mode was persisted from v0 (before isFirstVisit existed),
        // coerce isFirstVisit to false so existing users don't see the modal
        if (state && state.isFirstVisit === undefined) {
          state.isFirstVisit = false
        }
        return state as unknown as ViewModeState
      },
      onRehydrateStorage: () => () => {
        hydrated = true
      },
    }
  )
)
