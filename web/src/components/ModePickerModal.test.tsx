import { render, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ModePickerModal } from './ModePickerModal'

const mockSetMode = vi.fn()
const mockSetIsFirstVisit = vi.fn()

vi.mock('../stores/viewModeStore', () => {
  const store = {
    setMode: (...args: unknown[]) => mockSetMode(...args),
    setIsFirstVisit: (...args: unknown[]) => mockSetIsFirstVisit(...args),
  }
  return {
    useViewModeStore: Object.assign(
      () => store,
      { getState: () => store },
    ),
  }
})

describe('ModePickerModal', () => {
  beforeEach(() => {
    mockSetMode.mockClear()
    mockSetIsFirstVisit.mockClear()
  })

  it('renders welcome heading', () => {
    const { getByText } = render(<ModePickerModal />)
    expect(getByText('Welcome to kvelmo')).toBeInTheDocument()
  })

  it('renders Simple and Developer mode buttons', () => {
    const { getByText } = render(<ModePickerModal />)
    expect(getByText('Simple')).toBeInTheDocument()
    expect(getByText('Developer')).toBeInTheDocument()
  })

  it('sets simple mode when Simple clicked', () => {
    const { getByText } = render(<ModePickerModal />)
    // Click the card with "Simple" heading
    const simpleCard = getByText('Simple').closest('button')!
    fireEvent.click(simpleCard)
    expect(mockSetMode).toHaveBeenCalledWith('simple')
    expect(mockSetIsFirstVisit).toHaveBeenCalledWith(false)
  })

  it('sets developer mode when Developer clicked', () => {
    const { getByText } = render(<ModePickerModal />)
    const devCard = getByText('Developer').closest('button')!
    fireEvent.click(devCard)
    expect(mockSetMode).toHaveBeenCalledWith('developer')
    expect(mockSetIsFirstVisit).toHaveBeenCalledWith(false)
  })

  it('has accessible dialog role', () => {
    const { container } = render(<ModePickerModal />)
    expect(container.querySelector('[role="dialog"]')).toBeInTheDocument()
    expect(container.querySelector('[aria-modal="true"]')).toBeInTheDocument()
  })

  it('shows keyboard shortcut hint', () => {
    const { getByText } = render(<ModePickerModal />)
    expect(getByText(/Ctrl\+Shift\+V/)).toBeInTheDocument()
  })
})
