import { render, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MetricsPanel } from './MetricsPanel'

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

const fullMetrics = {
  jobs_submitted: 20,
  jobs_completed: 9,
  jobs_failed: 1,
  jobs_in_progress: 3,
  rpc_requests: 500,
  rpc_errors: 25,
  avg_latency_ms: 12.3,
  p99_latency_ms: 98.7,
  agent_connects: 4,
  agent_disconnects: 1,
  events_dropped: 2,
  permissions_approved: 30,
  permissions_denied: 5,
  tokens_consumed: 12345,
  agents: {
    claude: { tokens: 8000, requests: 50, errors: 2, avg_latency_ms: 15 },
    codex: { tokens: 4000, requests: 30, errors: 0, avg_latency_ms: 10 },
  },
}

describe('MetricsPanel', () => {
  beforeEach(() => {
    mockCall.mockReset()
    mockConnected = true
  })

  it('does not render when closed', () => {
    const { queryByRole } = render(
      <MetricsPanel isOpen={false} onClose={vi.fn()} />,
    )
    expect(queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders modal with title when open', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    const { getByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByText('Metrics')).toBeInTheDocument()
    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('metrics', {})
    })
  })

  it('shows loading spinner while fetching', () => {
    mockCall.mockReturnValue(new Promise(() => {}))
    const { container } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(container.querySelector('.loading-spinner')).toBeInTheDocument()
  })

  it('shows empty state when no metrics', async () => {
    mockCall.mockResolvedValueOnce(null)
    const { findByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('No metrics available')).toBeInTheDocument()
  })

  it('shows job stats', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    const { findByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('20')).toBeInTheDocument()
    expect(await findByText('9')).toBeInTheDocument()
    expect(await findByText('1')).toBeInTheDocument()
    expect(await findByText('3')).toBeInTheDocument()
  })

  it('shows success rate', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    const { findByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('90.0%')).toBeInTheDocument()
  })

  it('shows dash for success rate with zero jobs', async () => {
    const zeroMetrics = {
      ...fullMetrics,
      jobs_completed: 0,
      jobs_failed: 0,
    }
    mockCall.mockResolvedValueOnce(zeroMetrics)
    const { findByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('—')).toBeInTheDocument()
  })

  it('shows RPC stats', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    const { findByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('500')).toBeInTheDocument()
    expect(await findByText('25')).toBeInTheDocument()
    expect(await findByText('12.3ms')).toBeInTheDocument()
    expect(await findByText('98.7ms')).toBeInTheDocument()
  })

  it('shows error rate', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    const { findByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('5.0% error rate')).toBeInTheDocument()
  })

  it('shows agent stats', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    const { findByText, findAllByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('12,345')).toBeInTheDocument()
    expect(await findByText('35')).toBeInTheDocument()
    expect(await findByText('30 approved, 5 denied')).toBeInTheDocument()
    // events_dropped = 2, which also appears in the per-agent errors column
    const twos = await findAllByText('2')
    expect(twos.length).toBeGreaterThanOrEqual(1)
  })

  it('shows per-agent breakdown table', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    const { findByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Per-Agent Breakdown')).toBeInTheDocument()
    expect(await findByText('claude')).toBeInTheDocument()
    expect(await findByText('codex')).toBeInTheDocument()
    expect(await findByText('8,000')).toBeInTheDocument()
    expect(await findByText('4,000')).toBeInTheDocument()
    expect(await findByText('15ms')).toBeInTheDocument()
    expect(await findByText('10ms')).toBeInTheDocument()
  })

  it('does not show per-agent table when no agents', async () => {
    const noAgents = { ...fullMetrics }
    delete (noAgents as Record<string, unknown>).agents
    mockCall.mockResolvedValueOnce(noAgents)
    const { findByText, queryByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    await findByText('Jobs')
    expect(queryByText('Per-Agent Breakdown')).not.toBeInTheDocument()
  })

  it('shows error on RPC failure', async () => {
    mockCall.mockRejectedValueOnce(new Error('Connection lost'))
    const { findByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Connection lost')).toBeInTheDocument()
  })

  it('shows generic error for non-Error', async () => {
    mockCall.mockRejectedValueOnce('something went wrong')
    const { findByText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Failed to load metrics')).toBeInTheDocument()
  })

  it('refresh button triggers reload', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    const { getByLabelText } = render(
      <MetricsPanel isOpen={true} onClose={vi.fn()} />,
    )

    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledTimes(1)
    })

    mockCall.mockResolvedValueOnce(fullMetrics)
    await act(async () => {
      getByLabelText('Refresh metrics').click()
    })

    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledTimes(2)
    })
  })

  it('auto-loads on open when connected', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    render(<MetricsPanel isOpen={true} onClose={vi.fn()} />)
    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('metrics', {})
    })
  })

  it('calls onClose when close button clicked', async () => {
    mockCall.mockResolvedValueOnce(fullMetrics)
    const onClose = vi.fn()
    const { getByLabelText, findByText } = render(
      <MetricsPanel isOpen={true} onClose={onClose} />,
    )
    await findByText('Jobs')
    getByLabelText('Close dialog').click()
    expect(onClose).toHaveBeenCalledOnce()
  })
})
