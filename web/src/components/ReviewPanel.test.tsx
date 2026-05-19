import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ReviewPanel } from './ReviewPanel'

const mockLoadReview = vi.fn()
const mockApproveNode = vi.fn()
const mockRejectNode = vi.fn()
const mockEvaluateRisk = vi.fn()

let mockState: Record<string, unknown> = {}

vi.mock('../stores/projectStore', () => ({
  useProjectStore: Object.assign(
    (selector?: (s: Record<string, unknown>) => unknown) => {
      if (selector) return selector(mockState)
      return mockState
    },
    {
      getState: () => mockState,
    },
  ),
}))

const makeReview = (
  overrides: Partial<{ number: number; timestamp: string; approved: boolean; message: string }> = {},
) => ({
  number: 1,
  timestamp: '2026-04-01T12:00:00Z',
  approved: true,
  message: 'Looks good',
  ...overrides,
})

describe('ReviewPanel', () => {
  beforeEach(() => {
    mockLoadReview.mockReset()
    mockApproveNode.mockReset()
    mockRejectNode.mockReset()
    mockEvaluateRisk.mockReset()
    mockLoadReview.mockResolvedValue({
      number: 1,
      timestamp: '2026-04-01T12:00:00Z',
      approved: true,
      message: 'ok',
      content: '',
      findings: [],
    })
    mockState = {
      reviews: [],
      loadReview: mockLoadReview,
      pendingNodeApprovals: [],
      approveNode: mockApproveNode,
      rejectNode: mockRejectNode,
      riskScore: null,
      evaluateRisk: mockEvaluateRisk,
      client: null,
      connected: false,
    }
  })

  it('renders empty state when no reviews', () => {
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('No reviews yet')).toBeInTheDocument()
  })

  it('renders review header with number when reviews exist', () => {
    mockState.reviews = [makeReview()]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('Review #1')).toBeInTheDocument()
  })

  it('shows Approved badge when review is approved', () => {
    mockState.reviews = [makeReview({ approved: true })]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('Approved')).toBeInTheDocument()
  })

  it('shows Changes Requested badge when review is not approved', () => {
    mockState.reviews = [makeReview({ approved: false })]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('Changes Requested')).toBeInTheDocument()
  })

  it('shows summary message', () => {
    mockState.reviews = [makeReview({ message: 'All tests pass' })]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('All tests pass')).toBeInTheDocument()
  })

  it('shows "Risk not evaluated" when riskScore is null', () => {
    mockState.reviews = [makeReview()]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('Risk not evaluated')).toBeInTheDocument()
  })

  it('shows Evaluate button when risk not evaluated', () => {
    mockState.reviews = [makeReview()]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('Evaluate')).toBeInTheDocument()
  })

  it('shows risk score when available', () => {
    mockState.reviews = [makeReview()]
    mockState.riskScore = { score: 0.35, factors: {}, level: 'low' }
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('Risk Score')).toBeInTheDocument()
    expect(getByText('0.35 (low)')).toBeInTheDocument()
  })

  it('shows risk factors when available', () => {
    mockState.reviews = [makeReview()]
    mockState.riskScore = { score: 0.5, factors: { code_complexity: 0.3 }, level: 'medium' }
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('code complexity: 0.30')).toBeInTheDocument()
  })

  it('shows pending node approvals', () => {
    mockState.reviews = [makeReview()]
    mockState.pendingNodeApprovals = [{ nodeId: 'node-1', message: 'Needs review' }]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('Pending Approvals (1)')).toBeInTheDocument()
    expect(getByText('node-1')).toBeInTheDocument()
    expect(getByText('Needs review')).toBeInTheDocument()
  })

  it('renders Approve and Reject buttons for pending nodes', () => {
    mockState.reviews = [makeReview()]
    mockState.pendingNodeApprovals = [{ nodeId: 'node-1' }]
    const { getByLabelText } = render(<ReviewPanel />)
    expect(getByLabelText('Approve node node-1')).toBeInTheDocument()
    expect(getByLabelText('Reject node node-1')).toBeInTheDocument()
  })

  it('calls approveNode when Approve is clicked', () => {
    mockState.reviews = [makeReview()]
    mockState.pendingNodeApprovals = [{ nodeId: 'node-1' }]
    const { getByLabelText } = render(<ReviewPanel />)
    getByLabelText('Approve node node-1').click()
    expect(mockApproveNode).toHaveBeenCalledWith('node-1')
  })

  it('calls rejectNode when Reject is clicked', () => {
    mockState.reviews = [makeReview()]
    mockState.pendingNodeApprovals = [{ nodeId: 'node-1' }]
    const { getByLabelText } = render(<ReviewPanel />)
    getByLabelText('Reject node node-1').click()
    expect(mockRejectNode).toHaveBeenCalledWith('node-1')
  })

  it('shows History section when multiple reviews exist', () => {
    mockState.reviews = [makeReview({ number: 1 }), makeReview({ number: 2 })]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('History')).toBeInTheDocument()
  })

  it('does not show History section with single review', () => {
    mockState.reviews = [makeReview()]
    const { queryByText } = render(<ReviewPanel />)
    expect(queryByText('History')).not.toBeInTheDocument()
  })

  it('uses reviews from data prop when provided', () => {
    const dataReviews = [makeReview({ number: 99, message: 'From props' })]
    const { getByText } = render(<ReviewPanel data={{ reviews: dataReviews }} />)
    expect(getByText('Review #99')).toBeInTheDocument()
    expect(getByText('From props')).toBeInTheDocument()
  })

  it('shows "No issues found" when review has no findings and no error', () => {
    mockState.reviews = [makeReview()]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('No issues found')).toBeInTheDocument()
  })

  it('calls loadReview when a history item is clicked', async () => {
    mockState.reviews = [
      makeReview({ number: 1, message: 'First review' }),
      makeReview({ number: 2, message: 'Second review' }),
    ]
    mockLoadReview.mockResolvedValue({
      number: 1,
      timestamp: '2026-04-01T12:00:00Z',
      approved: true,
      message: 'ok',
      content: '',
      findings: [],
    })
    const { getByText } = render(<ReviewPanel />)
    // Wait for initial auto-load to settle
    await vi.waitFor(() => expect(mockLoadReview).toHaveBeenCalled())
    mockLoadReview.mockClear()
    // Click on the history item for review #1
    getByText('Review #1').click()
    expect(mockLoadReview).toHaveBeenCalledWith(1)
  })

  it('shows findings from loaded review detail', async () => {
    mockLoadReview.mockResolvedValue({
      number: 1,
      timestamp: '2026-04-01T12:00:00Z',
      approved: false,
      message: 'Issues found',
      content: '',
      findings: ['[HIGH] SQL injection risk', 'Missing validation'],
    })
    mockState.reviews = [makeReview({ number: 1, approved: false, message: 'Issues found' })]
    const { getByText } = render(<ReviewPanel />)
    await vi.waitFor(() => {
      expect(getByText('Findings (2)')).toBeInTheDocument()
    })
    expect(getByText('SQL injection risk')).toBeInTheDocument()
  })

  it('displays findings grouped by persona when tagged', async () => {
    mockLoadReview.mockResolvedValue({
      number: 1,
      timestamp: '2026-04-01T12:00:00Z',
      approved: false,
      message: 'Review',
      content: '',
      findings: ['[security] XSS vulnerability', '[performance] Slow query', 'General issue'],
    })
    mockState.reviews = [makeReview({ number: 1, approved: false })]
    const { getByText } = render(<ReviewPanel />)
    await vi.waitFor(() => {
      expect(getByText('Security (1)')).toBeInTheDocument()
    })
    expect(getByText('Performance (1)')).toBeInTheDocument()
    expect(getByText('General (1)')).toBeInTheDocument()
  })

  it('shows error state when loadReview fails', async () => {
    mockLoadReview.mockRejectedValue(new Error('Network error'))
    mockState.reviews = [makeReview({ number: 1 })]
    const { getByText } = render(<ReviewPanel />)
    await vi.waitFor(() => {
      expect(getByText('Network error')).toBeInTheDocument()
    })
  })

  it('shows risk score with medium level', () => {
    mockState.reviews = [makeReview()]
    mockState.riskScore = { score: 0.65, factors: { complexity: 0.4 }, level: 'medium' }
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('0.65 (medium)')).toBeInTheDocument()
    expect(getByText('complexity: 0.40')).toBeInTheDocument()
  })

  it('shows risk score with high level', () => {
    mockState.reviews = [makeReview()]
    mockState.riskScore = { score: 0.9, factors: {}, level: 'high' }
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('0.90 (high)')).toBeInTheDocument()
  })

  it('calls evaluateRisk when Evaluate button is clicked', () => {
    mockState.reviews = [makeReview()]
    mockEvaluateRisk.mockResolvedValue(undefined)
    const { getByText } = render(<ReviewPanel />)
    getByText('Evaluate').click()
    expect(mockEvaluateRisk).toHaveBeenCalled()
  })

  it('calls evaluateRisk when Re-evaluate button is clicked', () => {
    mockState.reviews = [makeReview()]
    mockState.riskScore = { score: 0.5, factors: {}, level: 'medium' }
    mockEvaluateRisk.mockResolvedValue(undefined)
    const { getByText } = render(<ReviewPanel />)
    getByText('Re-evaluate').click()
    expect(mockEvaluateRisk).toHaveBeenCalled()
  })

  it('shows Details button when risk score exists', () => {
    mockState.reviews = [makeReview()]
    mockState.riskScore = { score: 0.3, factors: {}, level: 'low' }
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('Details')).toBeInTheDocument()
  })

  it('shows finding with severity badge and classification', async () => {
    mockLoadReview.mockResolvedValue({
      number: 1,
      timestamp: '2026-04-01T12:00:00Z',
      approved: false,
      message: 'Review',
      content: '',
      findings: ['[CRITICAL] Memory leak detected {genuine}'],
    })
    mockState.reviews = [makeReview({ number: 1, approved: false })]
    const { getByText } = render(<ReviewPanel />)
    await vi.waitFor(() => {
      expect(getByText('critical')).toBeInTheDocument()
    })
    expect(getByText('genuine')).toBeInTheDocument()
    expect(getByText('Memory leak detected')).toBeInTheDocument()
  })

  it('renders multiple pending node approvals', () => {
    mockState.reviews = [makeReview()]
    mockState.pendingNodeApprovals = [
      { nodeId: 'node-a', message: 'Check A' },
      { nodeId: 'node-b', message: 'Check B' },
    ]
    const { getByText } = render(<ReviewPanel />)
    expect(getByText('Pending Approvals (2)')).toBeInTheDocument()
    expect(getByText('node-a')).toBeInTheDocument()
    expect(getByText('node-b')).toBeInTheDocument()
  })

  it('does not show pending approvals section when empty', () => {
    mockState.reviews = [makeReview()]
    mockState.pendingNodeApprovals = []
    const { queryByText } = render(<ReviewPanel />)
    expect(queryByText(/Pending Approvals/)).not.toBeInTheDocument()
  })
})
