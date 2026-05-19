import { render } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { EmptyState } from './EmptyState'

describe('EmptyState', () => {
  it('renders the title', () => {
    const { getByText } = render(<EmptyState title="No items" />)
    expect(getByText('No items')).toBeInTheDocument()
  })

  it('renders description when provided', () => {
    const { getByText } = render(<EmptyState title="No items" description="Try adding some" />)
    expect(getByText('Try adding some')).toBeInTheDocument()
  })

  it('does not render description when omitted', () => {
    const { queryByText } = render(<EmptyState title="No items" />)
    // Only the title should be present
    expect(queryByText('Try adding some')).not.toBeInTheDocument()
  })

  it('renders icon when provided', () => {
    const { getByText } = render(<EmptyState title="Empty" icon="📭" />)
    expect(getByText('📭')).toBeInTheDocument()
  })

  it('does not render icon when omitted', () => {
    const { container } = render(<EmptyState title="Empty" />)
    // No span with text-3xl class should be present
    expect(container.querySelector('.text-3xl')).not.toBeInTheDocument()
  })
})
