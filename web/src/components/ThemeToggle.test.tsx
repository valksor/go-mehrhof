import { render, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ThemeToggle } from './ThemeToggle'

const mockToggle = vi.fn()
let mockTheme = 'dark'

vi.mock('../stores/themeStore', () => ({
  useThemeStore: () => ({ theme: mockTheme, toggle: mockToggle }),
}))

describe('ThemeToggle', () => {
  beforeEach(() => {
    mockToggle.mockClear()
    mockTheme = 'dark'
  })

  it('renders a button', () => {
    const { container } = render(<ThemeToggle />)
    expect(container.querySelector('button')).toBeInTheDocument()
  })

  it('shows "Switch to light" title in dark mode', () => {
    mockTheme = 'dark'
    const { getByTitle } = render(<ThemeToggle />)
    expect(getByTitle('Switch to light')).toBeInTheDocument()
  })

  it('shows "Switch to dark" title in light mode', () => {
    mockTheme = 'light'
    const { getByTitle } = render(<ThemeToggle />)
    expect(getByTitle('Switch to dark')).toBeInTheDocument()
  })

  it('calls toggle on click', () => {
    const { container } = render(<ThemeToggle />)
    fireEvent.click(container.querySelector('button')!)
    expect(mockToggle).toHaveBeenCalledOnce()
  })
})
