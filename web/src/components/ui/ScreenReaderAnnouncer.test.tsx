import { render, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ScreenReaderAnnouncer } from './ScreenReaderAnnouncer'
import { useAnnouncer } from './useAnnouncer'

function TestPolite() {
  const { announce } = useAnnouncer()
  return <button onClick={() => announce('polite message')}>Go</button>
}

describe('ScreenReaderAnnouncer', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('renders polite live region', () => {
    const { getByRole } = render(
      <ScreenReaderAnnouncer>
        <div />
      </ScreenReaderAnnouncer>,
    )
    expect(getByRole('status')).toBeInTheDocument()
  })

  it('renders assertive live region', () => {
    const { getByRole } = render(
      <ScreenReaderAnnouncer>
        <div />
      </ScreenReaderAnnouncer>,
    )
    expect(getByRole('alert')).toBeInTheDocument()
  })

  it('announces a polite message after setTimeout(fn, 0)', async () => {
    const { getByRole, getByText } = render(
      <ScreenReaderAnnouncer>
        <TestPolite />
      </ScreenReaderAnnouncer>,
    )
    await act(async () => {
      getByText('Go').click()
      vi.advanceTimersByTime(10) // fires the 0ms timer but not the 1000ms clear timer
    })
    expect(getByRole('status').textContent).toBe('polite message')
  })

  it('throws when useAnnouncer used outside provider', () => {
    expect(() => render(<TestPolite />)).toThrow('useAnnouncer must be used within ScreenReaderAnnouncer')
  })
})
