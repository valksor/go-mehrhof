import type React from 'react'
import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ErrorBoundary } from './ErrorBoundary'

// A component that throws on render
function ThrowingChild({ message }: { message: string }): React.ReactNode {
  throw new Error(message)
}

// Suppress console.error from ErrorBoundary's componentDidCatch during tests
beforeEach(() => {
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

describe('ErrorBoundary', () => {
  it('renders children when no error', () => {
    const { getByText } = render(
      <ErrorBoundary>
        <p>All good</p>
      </ErrorBoundary>,
    )
    expect(getByText('All good')).toBeInTheDocument()
  })

  it('shows default error UI when child throws', () => {
    const { getByText } = render(
      <ErrorBoundary>
        <ThrowingChild message="kaboom" />
      </ErrorBoundary>,
    )
    expect(getByText('Something went wrong')).toBeInTheDocument()
    expect(getByText('kaboom')).toBeInTheDocument()
  })

  it('shows fallback prop when provided', () => {
    const { getByText, queryByText } = render(
      <ErrorBoundary fallback={<p>Custom fallback</p>}>
        <ThrowingChild message="kaboom" />
      </ErrorBoundary>,
    )
    expect(getByText('Custom fallback')).toBeInTheDocument()
    expect(queryByText('Something went wrong')).not.toBeInTheDocument()
  })

  it('calls onError callback when child throws', () => {
    const onError = vi.fn()
    render(
      <ErrorBoundary onError={onError}>
        <ThrowingChild message="test error" />
      </ErrorBoundary>,
    )
    expect(onError).toHaveBeenCalledOnce()
    expect(onError.mock.calls[0][0]).toBeInstanceOf(Error)
    expect(onError.mock.calls[0][0].message).toBe('test error')
  })

  it('shows Try again and Reload Page buttons', () => {
    const { getByText } = render(
      <ErrorBoundary>
        <ThrowingChild message="err" />
      </ErrorBoundary>,
    )
    expect(getByText('Try again')).toBeInTheDocument()
    expect(getByText('Reload Page')).toBeInTheDocument()
  })

  it('renders Try again button in error state', () => {
    const { getByText } = render(
      <ErrorBoundary>
        <ThrowingChild message="err" />
      </ErrorBoundary>,
    )
    // Verify the Try again button exists and is a clickable button
    const btn = getByText('Try again')
    expect(btn).toBeInTheDocument()
    expect(btn.tagName).toBe('BUTTON')
  })

  it('calls window.location.reload when Reload Page is clicked', () => {
    const reloadMock = vi.fn()
    const originalLocation = window.location
    Object.defineProperty(window, 'location', {
      value: { reload: reloadMock },
      writable: true,
      configurable: true,
    })

    const { getByText } = render(
      <ErrorBoundary>
        <ThrowingChild message="err" />
      </ErrorBoundary>,
    )
    getByText('Reload Page').click()
    expect(reloadMock).toHaveBeenCalledOnce()

    // Restore original location
    Object.defineProperty(window, 'location', {
      value: originalLocation,
      writable: true,
      configurable: true,
    })
  })

  it('shows generic message when error has no message', () => {
    function ThrowNull(): React.ReactNode {
      throw new Error('')
    }
    const { getByText } = render(
      <ErrorBoundary>
        <ThrowNull />
      </ErrorBoundary>,
    )
    expect(getByText('An unexpected error occurred')).toBeInTheDocument()
  })
})
