import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { StatsWidget } from './StatsWidget'

const mockLoadMetrics = vi.fn()
const mockLoadActiveTasks = vi.fn()
const mockLoadCacheStats = vi.fn()
const mockClearCache = vi.fn()

let mockState = {
  connected: true,
  metrics: null as Record<string, number> | null,
  activeTasks: [] as Array<{ state?: string }>,
  workers: [] as Array<{ id: string }>,
  workerStats: null as Record<string, number> | null,
  loadMetrics: mockLoadMetrics,
  loadActiveTasks: mockLoadActiveTasks,
}

let mockProjectState = {
  connected: false,
  cacheStats: null as { enabled: boolean; entries: number; hits: number; misses: number; hit_rate: number; tokens_saved: number } | null,
  loadCacheStats: mockLoadCacheStats,
  clearCache: mockClearCache,
}

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: (selector: (s: typeof mockState) => unknown) => selector(mockState),
}))

vi.mock('../stores/projectStore', () => ({
  useProjectStore: (selector: (s: typeof mockProjectState) => unknown) => selector(mockProjectState),
}))

describe('StatsWidget', () => {
  beforeEach(() => {
    mockLoadMetrics.mockClear()
    mockLoadActiveTasks.mockClear()
    mockLoadCacheStats.mockClear()
    mockClearCache.mockClear()
    mockState = {
      connected: true,
      metrics: null,
      activeTasks: [],
      workers: [],
      workerStats: null,
      loadMetrics: mockLoadMetrics,
      loadActiveTasks: mockLoadActiveTasks,
    }
    mockProjectState = {
      connected: false,
      cacheStats: null,
      loadCacheStats: mockLoadCacheStats,
      clearCache: mockClearCache,
    }
  })

  it('renders the Stats heading', () => {
    const { getByText } = render(<StatsWidget />)
    expect(getByText('Stats')).toBeInTheDocument()
  })

  it('renders a Refresh button', () => {
    const { getByText } = render(<StatsWidget />)
    expect(getByText('Refresh')).toBeInTheDocument()
  })

  it('shows "--" for success rate when no metrics', () => {
    const { getAllByText } = render(<StatsWidget />)
    const dashes = getAllByText('--')
    expect(dashes.length).toBeGreaterThanOrEqual(1)
  })

  it('shows "--" for avg latency when no metrics', () => {
    const { getAllByText } = render(<StatsWidget />)
    const dashes = getAllByText('--')
    // Two dashes: success rate + avg latency
    expect(dashes).toHaveLength(2)
  })

  it('shows 0 for active tasks when none exist', () => {
    const { getByText } = render(<StatsWidget />)
    expect(getByText('0')).toBeInTheDocument()
  })

  it('computes success rate from metrics', () => {
    mockState.metrics = {
      jobs_completed: 9,
      jobs_failed: 1,
      avg_latency_ms: 12.345,
    }
    const { getByText } = render(<StatsWidget />)
    expect(getByText('90%')).toBeInTheDocument()
  })

  it('displays avg latency from metrics', () => {
    mockState.metrics = {
      jobs_completed: 0,
      jobs_failed: 0,
      avg_latency_ms: 42.678,
    }
    const { getByText } = render(<StatsWidget />)
    expect(getByText('42.7ms')).toBeInTheDocument()
  })

  it('shows worker stats from workerStats', () => {
    mockState.workerStats = {
      total_workers: 5,
      working_workers: 2,
      available_workers: 3,
    }
    const { getByText } = render(<StatsWidget />)
    expect(getByText('2 active / 3 idle')).toBeInTheDocument()
  })

  it('falls back to workers array length when no workerStats', () => {
    mockState.workers = [{ id: 'w1' }, { id: 'w2' }]
    const { getByText } = render(<StatsWidget />)
    expect(getByText('0 active / 2 idle')).toBeInTheDocument()
  })

  it('counts active tasks by state', () => {
    mockState.activeTasks = [
      { state: 'implementing' },
      { state: 'implementing' },
      { state: 'planning' },
    ]
    const { getByText } = render(<StatsWidget />)
    expect(getByText('2 implementing')).toBeInTheDocument()
    expect(getByText('1 planning')).toBeInTheDocument()
  })

  it('shows "Tasks by State" section when active tasks exist', () => {
    mockState.activeTasks = [{ state: 'loaded' }]
    const { getByText } = render(<StatsWidget />)
    expect(getByText('Tasks by State')).toBeInTheDocument()
  })

  it('does not show "Tasks by State" section when no active tasks', () => {
    const { queryByText } = render(<StatsWidget />)
    expect(queryByText('Tasks by State')).not.toBeInTheDocument()
  })

  it('ignores tasks with state "none"', () => {
    mockState.activeTasks = [{ state: 'none' }, { state: 'loaded' }]
    const { getByText, queryByText } = render(<StatsWidget />)
    expect(getByText('1 loaded')).toBeInTheDocument()
    expect(queryByText('none')).not.toBeInTheDocument()
  })

  it('ignores tasks with no state', () => {
    mockState.activeTasks = [{}]
    const { queryByText } = render(<StatsWidget />)
    expect(queryByText('Tasks by State')).not.toBeInTheDocument()
  })

  it('calls loadMetrics and loadActiveTasks on Refresh click', () => {
    const { getByText } = render(<StatsWidget />)
    getByText('Refresh').click()
    expect(mockLoadMetrics).toHaveBeenCalled()
    expect(mockLoadActiveTasks).toHaveBeenCalled()
  })

  it('calls loadMetrics and loadActiveTasks on mount when connected', () => {
    render(<StatsWidget />)
    expect(mockLoadMetrics).toHaveBeenCalled()
    expect(mockLoadActiveTasks).toHaveBeenCalled()
  })

  it('does not call load functions on mount when disconnected', () => {
    mockState.connected = false
    render(<StatsWidget />)
    expect(mockLoadMetrics).not.toHaveBeenCalled()
    expect(mockLoadActiveTasks).not.toHaveBeenCalled()
  })

  it('applies badge-error class to failed tasks', () => {
    mockState.activeTasks = [{ state: 'failed' }]
    const { getByText } = render(<StatsWidget />)
    expect(getByText('1 failed').className).toContain('badge-error')
  })

  it('applies badge-success class to implemented tasks', () => {
    mockState.activeTasks = [{ state: 'implemented' }]
    const { getByText } = render(<StatsWidget />)
    expect(getByText('1 implemented').className).toContain('badge-success')
  })

  it('applies badge-warning class to in-progress tasks', () => {
    mockState.activeTasks = [{ state: 'implementing' }]
    const { getByText } = render(<StatsWidget />)
    expect(getByText('1 implementing').className).toContain('badge-warning')
  })

  it('shows cache stats when enabled', () => {
    mockProjectState.connected = true
    mockProjectState.cacheStats = {
      enabled: true,
      entries: 42,
      hits: 10,
      misses: 5,
      hit_rate: 0.6667,
      tokens_saved: 1234,
    }
    const { getByText } = render(<StatsWidget />)
    expect(getByText('Response Cache')).toBeInTheDocument()
    expect(getByText('42')).toBeInTheDocument()
    expect(getByText('10 / 5')).toBeInTheDocument()
    expect(getByText('66.7%')).toBeInTheDocument()
    expect(getByText('1,234')).toBeInTheDocument()
  })

  it('does not show cache stats when disabled', () => {
    mockProjectState.cacheStats = { enabled: false, entries: 0, hits: 0, misses: 0, hit_rate: 0, tokens_saved: 0 }
    const { queryByText } = render(<StatsWidget />)
    expect(queryByText('Response Cache')).not.toBeInTheDocument()
  })

  it('shows Clear button when cache has entries', () => {
    mockProjectState.connected = true
    mockProjectState.cacheStats = {
      enabled: true,
      entries: 5,
      hits: 1,
      misses: 1,
      hit_rate: 0.5,
      tokens_saved: 100,
    }
    const { getByText } = render(<StatsWidget />)
    expect(getByText('Clear')).toBeInTheDocument()
  })
})
