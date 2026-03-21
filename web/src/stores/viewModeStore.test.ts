import { describe, it, expect, beforeEach } from 'vitest'
import { useViewModeStore, isViewModeHydrated } from './viewModeStore'

describe('viewModeStore', () => {
  beforeEach(() => {
    // Reset store to defaults
    useViewModeStore.setState({
      mode: 'developer',
      isFirstVisit: true,
    })
  })

  it('defaults to developer mode', () => {
    expect(useViewModeStore.getState().mode).toBe('developer')
  })

  it('defaults to isFirstVisit true', () => {
    expect(useViewModeStore.getState().isFirstVisit).toBe(true)
  })

  it('setMode changes mode', () => {
    useViewModeStore.getState().setMode('simple')
    expect(useViewModeStore.getState().mode).toBe('simple')

    useViewModeStore.getState().setMode('developer')
    expect(useViewModeStore.getState().mode).toBe('developer')
  })

  it('toggle switches between modes', () => {
    expect(useViewModeStore.getState().mode).toBe('developer')

    useViewModeStore.getState().toggle()
    expect(useViewModeStore.getState().mode).toBe('simple')

    useViewModeStore.getState().toggle()
    expect(useViewModeStore.getState().mode).toBe('developer')
  })

  it('setIsFirstVisit updates the flag', () => {
    expect(useViewModeStore.getState().isFirstVisit).toBe(true)

    useViewModeStore.getState().setIsFirstVisit(false)
    expect(useViewModeStore.getState().isFirstVisit).toBe(false)
  })

  it('isViewModeHydrated returns true after store creation', () => {
    // In test environment with happy-dom, localStorage is mocked and available,
    // so zustand persist rehydrates synchronously, triggering onRehydrateStorage.
    expect(isViewModeHydrated()).toBe(true)
  })
})
