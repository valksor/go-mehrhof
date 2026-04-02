import { render } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { StatusIndicator, StatusBadge, type StatusType } from './StatusIndicator'

describe('StatusIndicator', () => {
  it('renders with default label for idle status', () => {
    const { getByText } = render(<StatusIndicator status="idle" />)
    expect(getByText('Idle')).toBeInTheDocument()
  })

  it('renders with default label for running status', () => {
    const { getByText } = render(<StatusIndicator status="running" />)
    expect(getByText('Running...')).toBeInTheDocument()
  })

  it('renders with default label for success status', () => {
    const { getByText } = render(<StatusIndicator status="success" />)
    expect(getByText('Completed')).toBeInTheDocument()
  })

  it('renders with default label for warning status', () => {
    const { getByText } = render(<StatusIndicator status="warning" />)
    expect(getByText('Warning')).toBeInTheDocument()
  })

  it('renders with default label for error status', () => {
    const { getByText } = render(<StatusIndicator status="error" />)
    expect(getByText('Failed')).toBeInTheDocument()
  })

  it('renders custom label when provided', () => {
    const { getByText } = render(<StatusIndicator status="running" label="Building..." />)
    expect(getByText('Building...')).toBeInTheDocument()
  })

  it('hides label when showLabel is false', () => {
    const { queryByText } = render(<StatusIndicator status="idle" showLabel={false} />)
    expect(queryByText('Idle')).not.toBeInTheDocument()
  })

  it('applies correct size class', () => {
    const { container } = render(<StatusIndicator status="idle" size="lg" />)
    expect(container.querySelector('.w-2\\.5')).toBeInTheDocument()
  })

  it('applies status-specific CSS class to dot', () => {
    const { container } = render(<StatusIndicator status="error" />)
    expect(container.querySelector('.status-error')).toBeInTheDocument()
  })
})

describe('StatusBadge', () => {
  it('renders with default label', () => {
    const { getByText } = render(<StatusBadge status="success" />)
    expect(getByText('Completed')).toBeInTheDocument()
  })

  it('renders custom label', () => {
    const { getByText } = render(<StatusBadge status="running" label="In Progress" />)
    expect(getByText('In Progress')).toBeInTheDocument()
  })

  it('applies badge-error class for error status', () => {
    const { container } = render(<StatusBadge status="error" />)
    expect(container.querySelector('.badge-error')).toBeInTheDocument()
  })

  it('applies badge-success class for success status', () => {
    const { container } = render(<StatusBadge status="success" />)
    expect(container.querySelector('.badge-success')).toBeInTheDocument()
  })

  const statuses: StatusType[] = ['idle', 'running', 'success', 'warning', 'error']
  for (const status of statuses) {
    it(`renders status dot for ${status}`, () => {
      const { container } = render(<StatusBadge status={status} />)
      expect(container.querySelector(`.status-${status}`)).toBeInTheDocument()
    })
  }
})
