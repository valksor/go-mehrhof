import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useThemeStore } from './themeStore'

describe('themeStore', () => {
  const mockSetAttribute = vi.fn()

  beforeEach(() => {
    // Reset store state between tests (don't use replace=true which removes actions)
    useThemeStore.setState({ theme: 'light' })
    // Mock document.documentElement.setAttribute
    vi.spyOn(document.documentElement, 'setAttribute').mockImplementation(mockSetAttribute)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('initial state', () => {
    it('defaults to light theme', () => {
      expect(useThemeStore.getState().theme).toBe('light')
    })
  })

  describe('setTheme', () => {
    it('updates theme state to dark', () => {
      useThemeStore.getState().setTheme('dark')
      expect(useThemeStore.getState().theme).toBe('dark')
    })

    it('updates theme state to light', () => {
      useThemeStore.setState({ theme: 'dark' })
      useThemeStore.getState().setTheme('light')
      expect(useThemeStore.getState().theme).toBe('light')
    })

    it('applies theme to document element', () => {
      useThemeStore.getState().setTheme('dark')
      expect(mockSetAttribute).toHaveBeenCalledWith('data-theme', 'business')
    })

    it('applies light theme to document element', () => {
      useThemeStore.getState().setTheme('light')
      expect(mockSetAttribute).toHaveBeenCalledWith('data-theme', 'corporate')
    })
  })

  describe('toggle', () => {
    it('switches from light to dark', () => {
      useThemeStore.setState({ theme: 'light' })
      useThemeStore.getState().toggle()
      expect(useThemeStore.getState().theme).toBe('dark')
    })

    it('switches from dark to light', () => {
      useThemeStore.setState({ theme: 'dark' })
      useThemeStore.getState().toggle()
      expect(useThemeStore.getState().theme).toBe('light')
    })

    it('applies toggled theme to document', () => {
      useThemeStore.setState({ theme: 'light' })
      useThemeStore.getState().toggle()
      expect(mockSetAttribute).toHaveBeenCalledWith('data-theme', 'business')
    })

    it('can toggle multiple times', () => {
      useThemeStore.setState({ theme: 'light' })

      useThemeStore.getState().toggle()
      expect(useThemeStore.getState().theme).toBe('dark')

      useThemeStore.getState().toggle()
      expect(useThemeStore.getState().theme).toBe('light')

      useThemeStore.getState().toggle()
      expect(useThemeStore.getState().theme).toBe('dark')
    })
  })

  describe('persistence', () => {
    it('uses kvelmo-theme as storage key', () => {
      // Trigger a state change to persist
      useThemeStore.getState().setTheme('dark')

      // Check localStorage was called with correct key
      // eslint-disable-next-line @typescript-eslint/unbound-method -- localStorage is mocked in vitest
      expect(localStorage.setItem).toHaveBeenCalledWith('kvelmo-theme', expect.any(String))
    })

    it('persists theme state', () => {
      useThemeStore.getState().setTheme('dark')

      const stored = localStorage.getItem('kvelmo-theme')
      expect(stored).toBeTruthy()
      const parsed = JSON.parse(stored!)
      expect(parsed.state.theme).toBe('dark')
    })
  })
})
