import { render, act, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ChangelogPanel } from './ChangelogPanel'

const mockCall = vi.fn()
const mockClient = { call: mockCall }

let mockConnected = true

vi.mock('../stores/projectStore', () => ({
  useProjectStore: Object.assign(
    (selector?: (s: Record<string, unknown>) => unknown) => {
      const state = { connected: mockConnected, client: mockClient }
      if (selector) return selector(state)
      return state
    },
    {
      getState: () => ({ client: mockClient }),
    },
  ),
}))

describe('ChangelogPanel', () => {
  beforeEach(() => {
    mockCall.mockReset()
    mockConnected = true
  })

  it('does not render when closed', () => {
    const { queryByRole } = render(
      <ChangelogPanel isOpen={false} onClose={vi.fn()} />,
    )
    expect(queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders modal with title when open', () => {
    const { getByText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByText('Changelog')).toBeInTheDocument()
  })

  it('shows placeholder text initially', () => {
    const { getByText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(
      getByText('Enter source and target refs, then click Generate.'),
    ).toBeInTheDocument()
  })

  it('generate button is disabled without inputs', () => {
    const { getByText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByText('Generate').closest('button')).toBeDisabled()
  })

  it('generate button is disabled when disconnected', () => {
    mockConnected = false
    const { getByText, getByLabelText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )

    fireEvent.change(getByLabelText('Source ref'), {
      target: { value: 'v1.0.0' },
    })
    fireEvent.change(getByLabelText('Target ref'), {
      target: { value: 'v2.0.0' },
    })

    expect(getByText('Generate').closest('button')).toBeDisabled()
  })

  it('generates changelog on button click', async () => {
    mockCall.mockResolvedValueOnce({ markdown: '', entries: [] })
    const { getByText, getByLabelText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )

    fireEvent.change(getByLabelText('Source ref'), {
      target: { value: 'v1.0.0' },
    })
    fireEvent.change(getByLabelText('Target ref'), {
      target: { value: 'v2.0.0' },
    })

    await act(async () => {
      getByText('Generate').closest('button')!.click()
    })

    expect(mockCall).toHaveBeenCalledWith('changelog.generate', {
      source: 'v1.0.0',
      target: 'v2.0.0',
      full: false,
    })
  })

  it('shows content after generation', async () => {
    mockCall.mockResolvedValueOnce({
      markdown: 'changelog content',
      entries: [],
    })
    const { getByText, getByLabelText, findByText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )

    fireEvent.change(getByLabelText('Source ref'), {
      target: { value: 'v1.0.0' },
    })
    fireEvent.change(getByLabelText('Target ref'), {
      target: { value: 'v2.0.0' },
    })

    await act(async () => {
      getByText('Generate').closest('button')!.click()
    })

    expect(await findByText('changelog content')).toBeInTheDocument()
  })

  it('shows loading spinner during generation', () => {
    mockCall.mockReturnValue(new Promise(() => {}))
    const { getByText, getByLabelText, container } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )

    fireEvent.change(getByLabelText('Source ref'), {
      target: { value: 'v1.0.0' },
    })
    fireEvent.change(getByLabelText('Target ref'), {
      target: { value: 'v2.0.0' },
    })

    act(() => {
      getByText('Generate').closest('button')!.click()
    })

    expect(container.querySelector('.loading-spinner')).toBeInTheDocument()
  })

  it('shows error on generation failure', async () => {
    mockCall.mockRejectedValueOnce(new Error('Git ref not found'))
    const { getByText, getByLabelText, findByText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )

    fireEvent.change(getByLabelText('Source ref'), {
      target: { value: 'v1.0.0' },
    })
    fireEvent.change(getByLabelText('Target ref'), {
      target: { value: 'v2.0.0' },
    })

    await act(async () => {
      getByText('Generate').closest('button')!.click()
    })

    expect(await findByText('Git ref not found')).toBeInTheDocument()
  })

  it('shows generic error for non-Error failure', async () => {
    mockCall.mockRejectedValueOnce('something went wrong')
    const { getByText, getByLabelText, findByText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )

    fireEvent.change(getByLabelText('Source ref'), {
      target: { value: 'v1.0.0' },
    })
    fireEvent.change(getByLabelText('Target ref'), {
      target: { value: 'v2.0.0' },
    })

    await act(async () => {
      getByText('Generate').closest('button')!.click()
    })

    expect(
      await findByText('Failed to generate changelog'),
    ).toBeInTheDocument()
  })

  it('shows copy button when content exists', async () => {
    mockCall.mockResolvedValueOnce({
      markdown: 'some changelog',
      entries: [],
    })
    const { getByText, getByLabelText, findByLabelText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )

    fireEvent.change(getByLabelText('Source ref'), {
      target: { value: 'v1.0.0' },
    })
    fireEvent.change(getByLabelText('Target ref'), {
      target: { value: 'v2.0.0' },
    })

    await act(async () => {
      getByText('Generate').closest('button')!.click()
    })

    expect(await findByLabelText('Copy to clipboard')).toBeInTheDocument()
  })

  it('full descriptions checkbox toggles', async () => {
    mockCall.mockResolvedValueOnce({ markdown: '', entries: [] })
    const { getByText, getByLabelText } = render(
      <ChangelogPanel isOpen={true} onClose={vi.fn()} />,
    )

    fireEvent.change(getByLabelText('Source ref'), {
      target: { value: 'v1.0.0' },
    })
    fireEvent.change(getByLabelText('Target ref'), {
      target: { value: 'v2.0.0' },
    })

    // Check the "Full descriptions" checkbox
    const checkbox = getByText('Full descriptions')
      .closest('label')!
      .querySelector('input[type="checkbox"]')!
    fireEvent.click(checkbox)

    await act(async () => {
      getByText('Generate').closest('button')!.click()
    })

    expect(mockCall).toHaveBeenCalledWith('changelog.generate', {
      source: 'v1.0.0',
      target: 'v2.0.0',
      full: true,
    })
  })

  it('calls onClose when close button clicked', () => {
    const onClose = vi.fn()
    const { getByLabelText } = render(
      <ChangelogPanel isOpen={true} onClose={onClose} />,
    )
    getByLabelText('Close dialog').click()
    expect(onClose).toHaveBeenCalledOnce()
  })
})
