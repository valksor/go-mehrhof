import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { storeName } from '../meta'

type Theme = 'light' | 'dark'

const THEME_MAP = {
  light: 'corporate',
  dark: 'business',
} as const

interface ThemeState {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggle: () => void
}

let systemThemeListenerAdded = false

const applyTheme = (theme: Theme) => {
  document.documentElement.setAttribute('data-theme', THEME_MAP[theme])
}

// Detect system preference for first-time visitors
const getSystemTheme = (): Theme => {
  if (typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches) {
    return 'dark'
  }
  return 'light'
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: getSystemTheme(),
      setTheme: (theme: Theme) => {
        set({ theme })
        applyTheme(theme)
      },
      toggle: () => {
        const newTheme = get().theme === 'light' ? 'dark' : 'light'
        set({ theme: newTheme })
        applyTheme(newTheme)
      },
    }),
    {
      name: storeName('theme'),
      onRehydrateStorage: () => (state) => {
        if (state) {
          applyTheme(state.theme)
        }

        // Listen for system preference changes and auto-switch (guard against duplicate listeners)
        if (typeof window !== 'undefined' && window.matchMedia && !systemThemeListenerAdded) {
          systemThemeListenerAdded = true
          const mq = window.matchMedia('(prefers-color-scheme: dark)')
          mq.addEventListener('change', (e) => {
            const newTheme: Theme = e.matches ? 'dark' : 'light'
            useThemeStore.getState().setTheme(newTheme)
          })
        }
      },
    },
  ),
)
