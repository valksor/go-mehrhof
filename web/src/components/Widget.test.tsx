import { render, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { Widget, TaskIcon, FilesIcon, OutputIcon, CheckpointsIcon, AgentIcon } from './Widget'

describe('Widget', () => {
  it('renders title', () => {
    const { getByText } = render(
      <Widget id="test" title="Test Widget">
        <p>Content</p>
      </Widget>,
    )
    expect(getByText('Test Widget')).toBeInTheDocument()
  })

  it('renders children', () => {
    const { getByText } = render(
      <Widget id="test" title="Test">
        <p>Hello World</p>
      </Widget>,
    )
    expect(getByText('Hello World')).toBeInTheDocument()
  })

  it('renders icon when provided', () => {
    const { getByText } = render(
      <Widget id="test" title="Test" icon={<span>🔧</span>}>
        <p>Content</p>
      </Widget>,
    )
    expect(getByText('🔧')).toBeInTheDocument()
  })

  it('toggles collapse on chevron click', () => {
    const { getByLabelText } = render(
      <Widget id="test" title="Test">
        <p>Content</p>
      </Widget>,
    )
    const chevron = getByLabelText('Collapse')
    fireEvent.click(chevron)
    expect(getByLabelText('Expand')).toBeInTheDocument()
  })

  it('starts collapsed when defaultCollapsed is true', () => {
    const { getByLabelText } = render(
      <Widget id="test" title="Test" defaultCollapsed>
        <p>Content</p>
      </Widget>,
    )
    expect(getByLabelText('Expand')).toBeInTheDocument()
  })

  it('renders close button when closeable', () => {
    const onClose = vi.fn()
    const { getByLabelText } = render(
      <Widget id="test" title="Test" closeable onClose={onClose}>
        <p>Content</p>
      </Widget>,
    )
    const closeBtn = getByLabelText('Close widget')
    expect(closeBtn).toBeInTheDocument()
    fireEvent.click(closeBtn)
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('does not render close button when not closeable', () => {
    const { queryByLabelText } = render(
      <Widget id="test" title="Test">
        <p>Content</p>
      </Widget>,
    )
    expect(queryByLabelText('Close widget')).not.toBeInTheDocument()
  })

  it('renders actions slot', () => {
    const { getByText } = render(
      <Widget id="test" title="Test" actions={<button>Action</button>}>
        <p>Content</p>
      </Widget>,
    )
    expect(getByText('Action')).toBeInTheDocument()
  })

  it('sets data-widget-id attribute', () => {
    const { container } = render(
      <Widget id="my-widget" title="Test">
        <p>Content</p>
      </Widget>,
    )
    expect(container.querySelector('[data-widget-id="my-widget"]')).toBeInTheDocument()
  })
})

describe('Icon components', () => {
  it('renders TaskIcon', () => {
    const { container } = render(<TaskIcon />)
    expect(container.querySelector('svg')).toBeInTheDocument()
  })

  it('renders FilesIcon', () => {
    const { container } = render(<FilesIcon />)
    expect(container.querySelector('svg')).toBeInTheDocument()
  })

  it('renders OutputIcon', () => {
    const { container } = render(<OutputIcon />)
    expect(container.querySelector('svg')).toBeInTheDocument()
  })

  it('renders CheckpointsIcon', () => {
    const { container } = render(<CheckpointsIcon />)
    expect(container.querySelector('svg')).toBeInTheDocument()
  })

  it('renders AgentIcon', () => {
    const { container } = render(<AgentIcon />)
    expect(container.querySelector('svg')).toBeInTheDocument()
  })
})
