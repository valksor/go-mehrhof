import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'

// Mock localStorage for Zustand persist middleware
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key]
    }),
    clear: vi.fn(() => {
      store = {}
    }),
    get length() {
      return Object.keys(store).length
    },
    key: vi.fn((index: number) => Object.keys(store)[index] ?? null),
  }
})()

vi.stubGlobal('localStorage', localStorageMock)
vi.stubGlobal('sessionStorage', localStorageMock)

beforeEach(() => {
  localStorageMock.clear()
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

// happy-dom cannot compute CSS tabbability — FocusTrap throws when it can't find
// focusable elements. Render children passthrough in tests.
vi.mock('focus-trap-react', () => ({
  FocusTrap: ({ children }: { children: unknown }) => children,
}))
