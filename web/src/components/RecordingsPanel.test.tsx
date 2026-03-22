import { render, waitFor, fireEvent, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { RecordingsPanel } from './RecordingsPanel'

const mockCall = vi.fn()
const mockClient = { call: mockCall }

let mockConnected = true

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: Object.assign(
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

const sampleRecordings = [
  {
    path: '/tmp/recordings/rec-001.jsonl',
    job_id: 'job-abc123def4567x',
    agent: 'claude',
    model: 'opus',
    started_at: '2026-03-01T10:00:00Z',
    lines: 42,
  },
  {
    path: '/tmp/recordings/rec-002.jsonl',
    job_id: 'job-xyz789',
    agent: 'codex',
    started_at: '2026-03-02T14:30:00Z',
    lines: 100,
  },
]

const sampleViewResult = {
  header: {
    job_id: 'job-abc123def4567x',
    agent: 'claude',
    model: 'opus',
    work_dir: '/workspace/project',
    started_at: '2026-03-01T10:00:00Z',
  },
  records: [
    {
      timestamp: '2026-03-01T10:00:01Z',
      job_id: 'job-abc123def4567x',
      direction: 'in' as const,
      type: 'prompt',
      event: 'Hello agent',
    },
    {
      timestamp: '2026-03-01T10:00:02Z',
      job_id: 'job-abc123def4567x',
      direction: 'out' as const,
      type: 'response',
      event: { text: 'Hello user' },
    },
  ],
}

describe('RecordingsPanel', () => {
  beforeEach(() => {
    mockCall.mockReset()
    mockConnected = true
  })

  it('does not render when closed', () => {
    const { queryByRole } = render(
      <RecordingsPanel isOpen={false} onClose={vi.fn()} />,
    )
    expect(queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders modal with title when open', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [] })
    const { getByText, findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByText('Recordings')).toBeInTheDocument()
    await findByText('No recordings found')
  })

  it('shows loading spinner while fetching', () => {
    mockCall.mockReturnValue(new Promise(() => {}))
    const { container } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(container.querySelector('.loading-spinner')).toBeInTheDocument()
  })

  it('shows empty state when no recordings', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [] })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('No recordings found')).toBeInTheDocument()
  })

  it('shows helper text in empty state', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [] })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(
      await findByText('Recordings are created when agents run tasks'),
    ).toBeInTheDocument()
  })

  it('renders recording list with count', async () => {
    mockCall.mockResolvedValueOnce({ recordings: sampleRecordings })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('2 recordings')).toBeInTheDocument()
  })

  it('shows singular "recording" for one item', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [sampleRecordings[0]] })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('1 recording')).toBeInTheDocument()
  })

  it('shows truncated job ID for long IDs', async () => {
    mockCall.mockResolvedValueOnce({ recordings: sampleRecordings })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    // job-abc123def4567x is 18 chars (> 16), should be truncated to first 16 + "..."
    expect(await findByText('job-abc123def456...')).toBeInTheDocument()
  })

  it('shows full job ID for short IDs', async () => {
    mockCall.mockResolvedValueOnce({ recordings: sampleRecordings })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('job-xyz789')).toBeInTheDocument()
  })

  it('shows agent badge', async () => {
    mockCall.mockResolvedValueOnce({ recordings: sampleRecordings })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('claude')).toBeInTheDocument()
    expect(await findByText('codex')).toBeInTheDocument()
  })

  it('shows model badge when present', async () => {
    mockCall.mockResolvedValueOnce({ recordings: sampleRecordings })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('opus')).toBeInTheDocument()
  })

  it('shows line count', async () => {
    mockCall.mockResolvedValueOnce({ recordings: sampleRecordings })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('42 lines')).toBeInTheDocument()
    expect(await findByText('100 lines')).toBeInTheDocument()
  })

  it('shows filename extracted from path', async () => {
    mockCall.mockResolvedValueOnce({ recordings: sampleRecordings })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('rec-001.jsonl')).toBeInTheDocument()
  })

  it('has a filter input with placeholder', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [] })
    const { getByPlaceholderText, findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByPlaceholderText('Filter by job ID...')).toBeInTheDocument()
    await findByText('No recordings found')
  })

  it('has a Filter button', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [] })
    const { getByText, findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByText('Filter')).toBeInTheDocument()
    await findByText('No recordings found')
  })

  it('calls recordings.list on open', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [] })
    render(<RecordingsPanel isOpen={true} onClose={vi.fn()} />)
    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('recordings.list', {})
    })
  })

  it('passes job filter to RPC call', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [] })
    const { getByPlaceholderText, getByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    const input = getByPlaceholderText('Filter by job ID...')
    fireEvent.change(input, { target: { value: 'job-abc' } })

    mockCall.mockResolvedValueOnce({ recordings: [] })
    await act(async () => { getByText('Filter').click() })

    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('recordings.list', { job: 'job-abc' })
    })
  })

  it('triggers filter on Enter key in input', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [] })
    const { getByPlaceholderText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    const input = getByPlaceholderText('Filter by job ID...')
    fireEvent.change(input, { target: { value: 'job-xyz' } })

    mockCall.mockResolvedValueOnce({ recordings: [] })
    await act(async () => { fireEvent.keyDown(input, { key: 'Enter' }) })

    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('recordings.list', { job: 'job-xyz' })
    })
  })

  it('shows error when RPC call fails', async () => {
    mockCall.mockRejectedValueOnce(new Error('Server error'))
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Server error')).toBeInTheDocument()
  })

  it('shows generic error for non-Error rejects', async () => {
    mockCall.mockRejectedValueOnce('unknown')
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Failed to load recordings')).toBeInTheDocument()
  })

  it('expands recording detail on click', async () => {
    mockCall.mockResolvedValueOnce({ recordings: sampleRecordings })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )

    const recButton = await findByText('rec-001.jsonl')

    mockCall.mockResolvedValueOnce(sampleViewResult)
    await act(async () => { recButton.closest('button')!.click() })

    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('recordings.view', {
        file: '/tmp/recordings/rec-001.jsonl',
      })
    })
  })

  it('shows recording records after expanding', async () => {
    mockCall.mockResolvedValueOnce({ recordings: sampleRecordings })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )

    const recButton = await findByText('rec-001.jsonl')
    mockCall.mockResolvedValueOnce(sampleViewResult)
    await act(async () => { recButton.closest('button')!.click() })

    expect(await findByText('Hello agent')).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', async () => {
    mockCall.mockResolvedValueOnce({ recordings: [] })
    const onClose = vi.fn()
    const { getByLabelText, findByText } = render(
      <RecordingsPanel isOpen={true} onClose={onClose} />,
    )
    await findByText('No recordings found')
    getByLabelText('Close dialog').click()
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('handles null recordings in response gracefully', async () => {
    mockCall.mockResolvedValueOnce({ recordings: null })
    const { findByText } = render(
      <RecordingsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('No recordings found')).toBeInTheDocument()
  })
})
