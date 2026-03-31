import { render, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SuggestionBanner } from './SuggestionBanner'

const mockCall = vi.fn()
const mockClient = { call: mockCall }

let mockState: Record<string, unknown> = {
  connected: true,
  client: mockClient,
  state: 'loaded',
}

vi.mock('../stores/projectStore', () => ({
  useProjectStore: (selector: (s: Record<string, unknown>) => unknown) => selector(mockState),
}))

const skipSuggestion = {
  phase: 'simplify',
  confidence: 0.85,
  reason: 'No complex code detected',
  sample_size: 12,
}

const agentSuggestion = {
  phase: 'implement',
  agent: 'codex',
  confidence: 0.9,
  reason: 'Best performance on TypeScript',
  sample_size: 8,
}

describe('SuggestionBanner', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockState = {
      connected: true,
      client: mockClient,
      state: 'loaded',
    }
  })

  it('renders nothing when no suggestions', async () => {
    mockCall.mockResolvedValue({ skip: null, agent: null })
    const { container } = render(<SuggestionBanner />)
    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('suggestions.list', {})
    })
    expect(container.innerHTML).toBe('')
  })

  it('renders nothing when not connected', () => {
    mockState.connected = false
    const { container } = render(<SuggestionBanner />)
    expect(container.innerHTML).toBe('')
    expect(mockCall).not.toHaveBeenCalled()
  })

  it('renders nothing when state is not loaded or planned', () => {
    mockState.state = 'implementing'
    const { container } = render(<SuggestionBanner />)
    expect(container.innerHTML).toBe('')
    expect(mockCall).not.toHaveBeenCalled()
  })

  it('fetches suggestions when loaded', async () => {
    mockState.state = 'loaded'
    mockCall.mockResolvedValue({ skip: null, agent: null })
    await act(async () => {
      render(<SuggestionBanner />)
    })
    expect(mockCall).toHaveBeenCalledWith('suggestions.list', {})
  })

  it('fetches suggestions when planned', async () => {
    mockState.state = 'planned'
    mockCall.mockResolvedValue({ skip: null, agent: null })
    await act(async () => {
      render(<SuggestionBanner />)
    })
    expect(mockCall).toHaveBeenCalledWith('suggestions.list', {})
  })

  it('renders skip suggestions', async () => {
    mockCall.mockResolvedValue({ skip: [skipSuggestion], agent: null })
    const { getByText } = render(<SuggestionBanner />)
    await waitFor(() => {
      expect(getByText('simplify')).toBeInTheDocument()
    })
  })

  it('renders agent suggestions', async () => {
    mockCall.mockResolvedValue({ skip: null, agent: [agentSuggestion] })
    const { getByText } = render(<SuggestionBanner />)
    await waitFor(() => {
      expect(getByText('codex')).toBeInTheDocument()
    })
  })

  it('dismiss button hides skip suggestion', async () => {
    mockCall.mockResolvedValue({ skip: [skipSuggestion], agent: null })
    const { getByText, queryByText } = render(<SuggestionBanner />)
    await waitFor(() => {
      expect(getByText('simplify')).toBeInTheDocument()
    })
    await act(async () => {
      getByText('Dismiss').click()
    })
    expect(queryByText('simplify')).not.toBeInTheDocument()
  })

  it('dismiss button hides agent suggestion', async () => {
    mockCall.mockResolvedValue({ skip: null, agent: [agentSuggestion] })
    const { getByText, queryByText } = render(<SuggestionBanner />)
    await waitFor(() => {
      expect(getByText('codex')).toBeInTheDocument()
    })
    await act(async () => {
      getByText('Dismiss').click()
    })
    expect(queryByText('codex')).not.toBeInTheDocument()
  })

  it('handles RPC error gracefully', async () => {
    mockCall.mockRejectedValue(new Error('RPC failed'))
    const { container } = render(<SuggestionBanner />)
    await waitFor(() => {
      expect(mockCall).toHaveBeenCalled()
    })
    expect(container.innerHTML).toBe('')
  })

  it('shows reason text for suggestions', async () => {
    mockCall.mockResolvedValue({ skip: [skipSuggestion], agent: [agentSuggestion] })
    const { getByText } = render(<SuggestionBanner />)
    await waitFor(() => {
      expect(getByText('(No complex code detected)')).toBeInTheDocument()
      expect(getByText('(Best performance on TypeScript)')).toBeInTheDocument()
    })
  })

  it('renders both skip and agent suggestions together', async () => {
    mockCall.mockResolvedValue({ skip: [skipSuggestion], agent: [agentSuggestion] })
    const { getByText } = render(<SuggestionBanner />)
    await waitFor(() => {
      expect(getByText('simplify')).toBeInTheDocument()
      expect(getByText('codex')).toBeInTheDocument()
    })
  })
})
