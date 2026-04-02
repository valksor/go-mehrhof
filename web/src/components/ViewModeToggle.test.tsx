import { render, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ViewModeToggle } from './ViewModeToggle'

const mockSetMode = vi.fn()
let mockMode = 'developer'

vi.mock('../stores/viewModeStore', () => ({
  useViewModeStore: () => ({ mode: mockMode, setMode: mockSetMode }),
}))

describe('ViewModeToggle', () => {
  beforeEach(() => {
    mockSetMode.mockClear()
    mockMode = 'developer'
  })

  it('renders Simple and Developer buttons', () => {
    const { getByText } = render(<ViewModeToggle />)
    expect(getByText('Simple')).toBeInTheDocument()
    expect(getByText('Developer')).toBeInTheDocument()
  })

  it('marks Developer as active when in developer mode', () => {
    mockMode = 'developer'
    const { getByText } = render(<ViewModeToggle />)
    expect(getByText('Developer').getAttribute('aria-pressed')).toBe('true')
    expect(getByText('Simple').getAttribute('aria-pressed')).toBe('false')
  })

  it('marks Simple as active when in simple mode', () => {
    mockMode = 'simple'
    const { getByText } = render(<ViewModeToggle />)
    expect(getByText('Simple').getAttribute('aria-pressed')).toBe('true')
    expect(getByText('Developer').getAttribute('aria-pressed')).toBe('false')
  })

  it('calls setMode with "simple" when Simple button clicked', () => {
    const { getByText } = render(<ViewModeToggle />)
    fireEvent.click(getByText('Simple'))
    expect(mockSetMode).toHaveBeenCalledWith('simple')
  })

  it('calls setMode with "developer" when Developer button clicked', () => {
    mockMode = 'simple'
    const { getByText } = render(<ViewModeToggle />)
    fireEvent.click(getByText('Developer'))
    expect(mockSetMode).toHaveBeenCalledWith('developer')
  })

  it('has data-testid attribute', () => {
    const { getByTestId } = render(<ViewModeToggle />)
    expect(getByTestId('view-mode-toggle')).toBeInTheDocument()
  })
})
