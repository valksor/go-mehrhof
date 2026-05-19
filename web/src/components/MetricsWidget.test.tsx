import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MetricsWidget } from './MetricsWidget'

const mockLoadMetrics = vi.fn()
const mockLoadMetricsHistory = vi.fn()

let mockState: Record<string, unknown> = {
  metrics: null,
  metricsHistory: [],
  loadMetrics: mockLoadMetrics,
  loadMetricsHistory: mockLoadMetricsHistory,
}

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: (selector: (s: Record<string, unknown>) => unknown) => selector(mockState),
}))

const baseMetrics = {
  jobs_submitted: 15,
  jobs_completed: 10,
  jobs_failed: 3,
  jobs_in_progress: 2,
  rpc_requests: 120,
  p99_latency_ms: 42.678,
  agent_connects: 5,
  events_dropped: 0,
}

describe('MetricsWidget', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockState = {
      metrics: null,
      metricsHistory: [],
      loadMetrics: mockLoadMetrics,
      loadMetricsHistory: mockLoadMetricsHistory,
    }
  })

  it('shows "No metrics available" when metrics is null', () => {
    render(<MetricsWidget />)
    expect(screen.getByText('No metrics available')).toBeInTheDocument()
  })

  it('shows Refresh button when no metrics', () => {
    render(<MetricsWidget />)
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeInTheDocument()
  })

  it('calls loadMetrics on Refresh click (no metrics state)', () => {
    render(<MetricsWidget />)
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(mockLoadMetrics).toHaveBeenCalledOnce()
  })

  it('shows metric values when present', () => {
    mockState.metrics = { ...baseMetrics }
    render(<MetricsWidget />)
    expect(screen.getByText('15')).toBeInTheDocument()
    expect(screen.getByText('10')).toBeInTheDocument()
    expect(screen.getByText('120')).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
  })

  it('shows jobs submitted value', () => {
    mockState.metrics = { ...baseMetrics, jobs_submitted: 99 }
    render(<MetricsWidget />)
    expect(screen.getByText('99')).toBeInTheDocument()
    expect(screen.getByText('Jobs Submitted')).toBeInTheDocument()
  })

  it('shows jobs failed with error styling', () => {
    mockState.metrics = { ...baseMetrics, jobs_failed: 7 }
    render(<MetricsWidget />)
    const failedValue = screen.getByText('7')
    expect(failedValue).toBeInTheDocument()
    expect(failedValue).toHaveClass('text-error')
  })

  it('shows p99 latency formatted', () => {
    mockState.metrics = { ...baseMetrics, p99_latency_ms: 42.678 }
    render(<MetricsWidget />)
    expect(screen.getByText('42.7ms')).toBeInTheDocument()
  })

  it('shows events dropped as 0 when falsy', () => {
    mockState.metrics = { ...baseMetrics, events_dropped: 0 }
    render(<MetricsWidget />)
    const droppedLabel = screen.getByText('Events Dropped')
    const droppedValue = droppedLabel.nextElementSibling
    expect(droppedValue).toHaveTextContent('0')
  })

  it('toggles history view on History button click', () => {
    mockState.metrics = { ...baseMetrics }
    render(<MetricsWidget />)
    expect(screen.queryByText('No history data yet')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'History' }))
    expect(screen.getByText('No history data yet')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'History' }))
    expect(screen.queryByText('No history data yet')).not.toBeInTheDocument()
  })

  it('calls loadMetricsHistory on first history toggle', () => {
    mockState.metrics = { ...baseMetrics }
    render(<MetricsWidget />)
    fireEvent.click(screen.getByRole('button', { name: 'History' }))
    expect(mockLoadMetricsHistory).toHaveBeenCalledOnce()
  })

  it('shows "No history data yet" when history is empty', () => {
    mockState.metrics = { ...baseMetrics }
    mockState.metricsHistory = []
    render(<MetricsWidget />)
    fireEvent.click(screen.getByRole('button', { name: 'History' }))
    expect(screen.getByText('No history data yet')).toBeInTheDocument()
  })

  it('shows history data with sparklines', () => {
    const entries = [
      { ...baseMetrics, jobs_submitted: 5, jobs_completed: 3, jobs_failed: 1, rpc_requests: 50 },
      { ...baseMetrics, jobs_submitted: 10, jobs_completed: 7, jobs_failed: 2, rpc_requests: 80 },
      { ...baseMetrics, jobs_submitted: 15, jobs_completed: 10, jobs_failed: 3, rpc_requests: 120 },
    ]
    mockState.metrics = { ...baseMetrics }
    mockState.metricsHistory = entries
    render(<MetricsWidget />)
    fireEvent.click(screen.getByRole('button', { name: 'History' }))
    expect(screen.getByText('Jobs Submitted')).toBeInTheDocument()
    expect(screen.getByText('Jobs Completed')).toBeInTheDocument()
    expect(screen.getByText('Jobs Failed')).toBeInTheDocument()
    expect(screen.getByText('RPC Requests')).toBeInTheDocument()
    // Latest values shown
    expect(screen.getByText('15')).toBeInTheDocument()
    expect(screen.getByText('10')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('120')).toBeInTheDocument()
    // Sparkline SVGs rendered (one per series)
    const svgs = document.querySelectorAll('svg')
    expect(svgs.length).toBe(4)
    svgs.forEach((svg) => {
      expect(svg.querySelector('polyline')).toBeTruthy()
    })
  })

  it('shows per-agent performance table', () => {
    mockState.metrics = {
      ...baseMetrics,
      agents: {
        claude: { tokens: 5000, requests: 20, errors: 1, avg_latency_ms: 150 },
        codex: { tokens: 3000, requests: 15, errors: 0, avg_latency_ms: 200 },
      },
    }
    render(<MetricsWidget />)
    expect(screen.getByText('Agent Performance')).toBeInTheDocument()
    expect(screen.getByText('claude')).toBeInTheDocument()
    expect(screen.getByText('codex')).toBeInTheDocument()
    expect(screen.getByText('5,000')).toBeInTheDocument()
    expect(screen.getByText('3,000')).toBeInTheDocument()
    expect(screen.getByText('150ms')).toBeInTheDocument()
    expect(screen.getByText('200ms')).toBeInTheDocument()
  })

  it('sorts agents by tokens descending', () => {
    mockState.metrics = {
      ...baseMetrics,
      agents: {
        low: { tokens: 100, requests: 1, errors: 0, avg_latency_ms: 10 },
        high: { tokens: 9000, requests: 50, errors: 2, avg_latency_ms: 80 },
        mid: { tokens: 3000, requests: 20, errors: 0, avg_latency_ms: 120 },
      },
    }
    render(<MetricsWidget />)
    const rows = document.querySelectorAll('tbody tr')
    expect(rows.length).toBe(3)
    const names = Array.from(rows).map((r) => r.querySelector('td')?.textContent)
    expect(names).toEqual(['high', 'mid', 'low'])
  })

  it('does not show agent table when no agents', () => {
    mockState.metrics = { ...baseMetrics }
    render(<MetricsWidget />)
    expect(screen.queryByText('Agent Performance')).not.toBeInTheDocument()
  })

  it('handles null p99_latency_ms', () => {
    mockState.metrics = { ...baseMetrics, p99_latency_ms: null }
    render(<MetricsWidget />)
    expect(screen.getByText('0.0ms')).toBeInTheDocument()
  })
})
