import { render, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { WorkflowBar } from './WorkflowBar'

const mockPlan = vi.fn()
const mockImplement = vi.fn()
const mockReview = vi.fn()
const mockSubmit = vi.fn()
const mockFinish = vi.fn()
const mockStop = vi.fn()
const mockUndo = vi.fn()
const mockRedo = vi.fn()
const mockAbandon = vi.fn()
const mockApproveTransition = vi.fn()
const mockRetry = vi.fn()
const mockApproveRemote = vi.fn()
const mockMergeRemote = vi.fn()
const mockRefresh = vi.fn()
const mockToggleDryRun = vi.fn()

let mockProjectState: Record<string, unknown> = {}

vi.mock('zustand/react/shallow', () => ({
  useShallow: (fn: unknown) => fn,
}))

vi.mock('../stores/projectStore', () => ({
  useProjectStore: Object.assign(
    (selector: (s: Record<string, unknown>) => unknown) => selector(mockProjectState),
    {
      getState: () => mockProjectState,
      subscribe: () => () => {},
    },
  ),
}))

vi.mock('../stores/chatStore', () => ({
  useChatStore: Object.assign(
    () => ({}),
    { getState: () => ({ sendMessage: vi.fn() }) },
  ),
}))

vi.mock('../stores/layoutStore', () => ({
  useLayoutStore: Object.assign(
    () => ({}),
    { subscribe: () => () => {} },
  ),
}))

vi.mock('./ui/ConfirmModal', () => ({
  ConfirmModal: ({ isOpen, title, children }: { isOpen: boolean; title: string; children?: React.ReactNode }) =>
    isOpen ? <div data-testid="confirm-modal">{title}{children}</div> : null,
}))

function resetProjectState(overrides: Record<string, unknown> = {}) {
  mockProjectState = {
    state: 'none',
    plan: mockPlan,
    implement: mockImplement,
    review: mockReview,
    submit: mockSubmit,
    finish: mockFinish,
    stop: mockStop,
    undo: mockUndo,
    redo: mockRedo,
    abandon: mockAbandon,
    approveTransition: mockApproveTransition,
    retry: mockRetry,
    loading: false,
    checkpoints: [],
    redoStack: [],
    approveRemote: mockApproveRemote,
    mergeRemote: mockMergeRemote,
    refresh: mockRefresh,
    error: null,
    phaseError: null,
    dryRunMode: false,
    toggleDryRun: mockToggleDryRun,
    skipPhases: [],
    autoFixStatus: null,
    phaseProgress: null,
    setPrBodyOverride: vi.fn(),
    prBodyOverride: null,
    ...overrides,
  }
}

describe('WorkflowBar', () => {
  beforeEach(() => {
    resetProjectState()
    vi.clearAllMocks()
  })

  it('renders nothing when state is none', () => {
    const { container } = render(<WorkflowBar />)
    expect(container.innerHTML).toBe('')
  })

  it('renders the workflow bar when state is loaded', () => {
    resetProjectState({ state: 'loaded' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Load')).toBeInTheDocument()
    expect(getByText('Plan')).toBeInTheDocument()
    expect(getByText('Implement')).toBeInTheDocument()
  })

  it('shows all step labels', () => {
    resetProjectState({ state: 'loaded' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Load')).toBeInTheDocument()
    expect(getByText('Plan')).toBeInTheDocument()
    expect(getByText('Implement')).toBeInTheDocument()
    expect(getByText('Review')).toBeInTheDocument()
    expect(getByText('Submit')).toBeInTheDocument()
    expect(getByText('Finish')).toBeInTheDocument()
  })

  it('shows hint text for loaded state', () => {
    resetProjectState({ state: 'loaded' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Task loaded — click Plan to start.')).toBeInTheDocument()
  })

  it('shows hint text for planned state', () => {
    resetProjectState({ state: 'planned' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Plan ready — click Implement.')).toBeInTheDocument()
  })

  it('shows Dry Run checkbox', () => {
    resetProjectState({ state: 'loaded' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Dry Run')).toBeInTheDocument()
  })

  it('toggles dry run mode', () => {
    resetProjectState({ state: 'loaded' })
    const { getByText } = render(<WorkflowBar />)
    getByText('Dry Run').click()
    expect(mockToggleDryRun).toHaveBeenCalled()
  })

  it('shows Failed badge when state is failed', () => {
    resetProjectState({ state: 'failed' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Failed')).toBeInTheDocument()
  })

  it('shows Retry button when state is failed', () => {
    resetProjectState({ state: 'failed' })
    const { getByRole } = render(<WorkflowBar />)
    const retryBtn = getByRole('button', { name: 'Retry failed phase' })
    expect(retryBtn).toBeInTheDocument()
    retryBtn.click()
    expect(mockRetry).toHaveBeenCalled()
  })

  it('shows Paused badge when state is paused', () => {
    resetProjectState({ state: 'paused' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Paused')).toBeInTheDocument()
  })

  it('shows Stop button during active states', () => {
    resetProjectState({ state: 'implementing' })
    const { getByRole } = render(<WorkflowBar />)
    const stopBtn = getByRole('button', { name: /Stop current operation/i })
    expect(stopBtn).toBeInTheDocument()
    stopBtn.click()
    expect(mockStop).toHaveBeenCalled()
  })

  it('does not show Stop button for non-active states', () => {
    resetProjectState({ state: 'loaded' })
    const { queryByRole } = render(<WorkflowBar />)
    expect(queryByRole('button', { name: /Stop current operation/i })).not.toBeInTheDocument()
  })

  it('shows Undo/Redo buttons when checkpoints exist', () => {
    resetProjectState({ state: 'loaded', checkpoints: ['cp1'], redoStack: ['cp2'] })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: /Undo/ })).toBeInTheDocument()
    expect(getByRole('button', { name: /Redo/ })).toBeInTheDocument()
  })

  it('does not show Undo/Redo when no checkpoints', () => {
    resetProjectState({ state: 'loaded', checkpoints: [], redoStack: [] })
    const { queryByRole } = render(<WorkflowBar />)
    expect(queryByRole('button', { name: /Undo/ })).not.toBeInTheDocument()
    expect(queryByRole('button', { name: /Redo/ })).not.toBeInTheDocument()
  })

  it('shows Approve button when error includes approval required', () => {
    resetProjectState({ state: 'loaded', error: 'Run: kvelmo approve submit — approval required' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Approve transition' })).toBeInTheDocument()
  })

  it('shows post-submit actions when state is submitted', () => {
    resetProjectState({ state: 'submitted' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Check PR status' })).toBeInTheDocument()
    expect(getByRole('button', { name: 'Approve PR' })).toBeInTheDocument()
    expect(getByRole('button', { name: 'Merge PR' })).toBeInTheDocument()
    expect(getByRole('button', { name: 'Finish and clean up' })).toBeInTheDocument()
  })

  it('shows Explain last action button', () => {
    resetProjectState({ state: 'loaded' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Explain last action' })).toBeInTheDocument()
  })

  it('shows Abandon task button', () => {
    resetProjectState({ state: 'loaded' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Abandon task' })).toBeInTheDocument()
  })

  it('opens abandon modal on Abandon click', () => {
    resetProjectState({ state: 'loaded' })
    const { getByRole, getByTestId } = render(<WorkflowBar />)
    act(() => {
      getByRole('button', { name: 'Abandon task' }).click()
    })
    expect(getByTestId('confirm-modal')).toBeInTheDocument()
  })

  it('shows skip phases indicator when phases are skipped', () => {
    resetProjectState({ state: 'loaded', skipPhases: ['simplify', 'optimize'] })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('skip: simplify, optimize')).toBeInTheDocument()
  })

  it('shows loading spinner when loading', () => {
    resetProjectState({ state: 'loaded', loading: true })
    const { container } = render(<WorkflowBar />)
    expect(container.querySelector('.loading-spinner')).toBeInTheDocument()
  })

  it('shows progress bar during active phase with progress', () => {
    resetProjectState({
      state: 'implementing',
      phaseProgress: { percent: 42, calibrated: true, eta: 120 },
    })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText(/42%/)).toBeInTheDocument()
    expect(getByText(/~2m/)).toBeInTheDocument()
  })

  it('shows failure badge variants based on failureClass', () => {
    resetProjectState({ state: 'failed', phaseError: { class: 'recoverable' } })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Retrying...')).toBeInTheDocument()
  })

  it('shows Degraded badge for degraded failure class', () => {
    resetProjectState({ state: 'failed', phaseError: { class: 'degraded' } })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Degraded')).toBeInTheDocument()
  })

  it('shows Skipped badge for skippable failure class', () => {
    resetProjectState({ state: 'failed', phaseError: { class: 'skippable' } })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Skipped')).toBeInTheDocument()
  })

  it('makes Plan step clickable when state is loaded', () => {
    resetProjectState({ state: 'loaded' })
    const { getByRole } = render(<WorkflowBar />)
    const planBtn = getByRole('button', { name: /Plan — click to start/ })
    expect(planBtn).toBeInTheDocument()
  })

  it('makes Implement step clickable when state is planned', () => {
    resetProjectState({ state: 'planned' })
    const { getByRole } = render(<WorkflowBar />)
    const implBtn = getByRole('button', { name: /Implement — click to start/ })
    expect(implBtn).toBeInTheDocument()
  })
})
