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

const makeReview = (overrides: Partial<{ number: number; timestamp: string; approved: boolean; message: string }> = {}) => ({
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
    mockLoadReview.mockResolvedValue({ number: 1, timestamp: '2026-04-01T12:00:00Z', approved: true, message: 'ok', content: '', findings: [] })
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
    mockState.pendingNodeApprovals = [
      { nodeId: 'node-1', message: 'Needs review' },
    ]
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
    mockState.reviews = [
      makeReview({ number: 1 }),
      makeReview({ number: 2 }),
    ]
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
})
