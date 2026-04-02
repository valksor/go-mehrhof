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

  // --- Phase action button clicks ---

  it('calls plan() when Plan step is clicked in loaded state', () => {
    resetProjectState({ state: 'loaded' })
    const { getByRole } = render(<WorkflowBar />)
    const planBtn = getByRole('button', { name: /Plan — click to start/ })
    planBtn.click()
    expect(mockPlan).toHaveBeenCalled()
  })

  it('calls implement() when Implement step is clicked in planned state', () => {
    resetProjectState({ state: 'planned' })
    const { getByRole } = render(<WorkflowBar />)
    const implBtn = getByRole('button', { name: /Implement — click to start/ })
    implBtn.click()
    expect(mockImplement).toHaveBeenCalled()
  })

  it('calls review() when Review step is clicked in implemented state', () => {
    resetProjectState({ state: 'implemented' })
    const { getByRole } = render(<WorkflowBar />)
    const reviewBtn = getByRole('button', { name: /Review — click to start/ })
    reviewBtn.click()
    expect(mockReview).toHaveBeenCalledWith({ approve: true })
  })

  it('opens submit modal when Submit step is clicked in reviewing state', () => {
    resetProjectState({ state: 'reviewing' })
    const { getByRole, getByTestId } = render(<WorkflowBar />)
    const submitBtn = getByRole('button', { name: /Submit — click to start/ })
    act(() => {
      submitBtn.click()
    })
    expect(getByTestId('confirm-modal')).toBeInTheDocument()
  })

  // --- Undo/Redo interactions ---

  it('calls undo() when Undo button is clicked', () => {
    resetProjectState({ state: 'loaded', checkpoints: ['cp1'] })
    const { getByRole } = render(<WorkflowBar />)
    getByRole('button', { name: /Undo/ }).click()
    expect(mockUndo).toHaveBeenCalled()
  })

  it('calls redo() when Redo button is clicked', () => {
    resetProjectState({ state: 'loaded', checkpoints: ['cp1'], redoStack: ['cp2'] })
    const { getByRole } = render(<WorkflowBar />)
    getByRole('button', { name: /Redo/ }).click()
    expect(mockRedo).toHaveBeenCalled()
  })

  it('disables Undo when no checkpoints', () => {
    resetProjectState({ state: 'loaded', checkpoints: [], redoStack: ['cp1'] })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: /Undo/ })).toBeDisabled()
  })

  it('disables Redo when redoStack is empty', () => {
    resetProjectState({ state: 'loaded', checkpoints: ['cp1'], redoStack: [] })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: /Redo/ })).toBeDisabled()
  })

  it('shows checkpoint count in Undo aria-label', () => {
    resetProjectState({ state: 'loaded', checkpoints: ['a', 'b', 'c'] })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Undo (3 checkpoints)' })).toBeInTheDocument()
  })

  it('shows redo stack count in Redo aria-label', () => {
    resetProjectState({ state: 'loaded', checkpoints: ['a'], redoStack: ['x', 'y'] })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Redo (2 in stack)' })).toBeInTheDocument()
  })

  // --- Stop button during various active states ---

  it('shows Stop button during planning', () => {
    resetProjectState({ state: 'planning' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: /Stop current operation/ })).toBeInTheDocument()
  })

  it('shows Stop button during simplifying', () => {
    resetProjectState({ state: 'simplifying' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: /Stop current operation/ })).toBeInTheDocument()
  })

  it('shows Stop button during optimizing', () => {
    resetProjectState({ state: 'optimizing' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: /Stop current operation/ })).toBeInTheDocument()
  })

  it('shows Stop button during reviewing', () => {
    resetProjectState({ state: 'reviewing' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: /Stop current operation/ })).toBeInTheDocument()
  })

  it('calls stop() when Stop button is clicked', () => {
    resetProjectState({ state: 'planning' })
    const { getByRole } = render(<WorkflowBar />)
    getByRole('button', { name: /Stop current operation/ }).click()
    expect(mockStop).toHaveBeenCalled()
  })

  // --- Retry button ---

  it('disables Retry button when loading', () => {
    resetProjectState({ state: 'failed', loading: true })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Retry failed phase' })).toBeDisabled()
  })

  // --- Dry run toggle state ---

  it('shows checkbox as checked when dryRunMode is true', () => {
    resetProjectState({ state: 'loaded', dryRunMode: true })
    const { container } = render(<WorkflowBar />)
    const checkbox = container.querySelector('input[type="checkbox"]') as HTMLInputElement
    expect(checkbox.checked).toBe(true)
  })

  it('applies warning styling when dryRunMode is true', () => {
    resetProjectState({ state: 'loaded', dryRunMode: true })
    const { getByText } = render(<WorkflowBar />)
    const label = getByText('Dry Run')
    expect(label.className).toContain('text-warning')
  })

  // --- Approve transition ---

  it('calls approveTransition when Approve button is clicked', () => {
    resetProjectState({ state: 'loaded', error: 'Run: kvelmo approve submit — approval required' })
    const { getByRole } = render(<WorkflowBar />)
    getByRole('button', { name: 'Approve transition' }).click()
    expect(mockApproveTransition).toHaveBeenCalledWith('submit')
  })

  it('extracts event name from error for approveTransition', () => {
    resetProjectState({ state: 'loaded', error: 'Run: kvelmo approve implement — approval required' })
    const { getByRole } = render(<WorkflowBar />)
    getByRole('button', { name: 'Approve transition' }).click()
    expect(mockApproveTransition).toHaveBeenCalledWith('implement')
  })

  it('does not show Approve button when error does not contain approval required', () => {
    resetProjectState({ state: 'loaded', error: 'Some other error' })
    const { queryByRole } = render(<WorkflowBar />)
    expect(queryByRole('button', { name: 'Approve transition' })).not.toBeInTheDocument()
  })

  // --- Post-submit remote operations ---

  it('calls refresh() when Check PR status is clicked', () => {
    resetProjectState({ state: 'submitted' })
    const { getByRole } = render(<WorkflowBar />)
    getByRole('button', { name: 'Check PR status' }).click()
    expect(mockRefresh).toHaveBeenCalled()
  })

  it('calls approveRemote() when Approve PR is clicked', () => {
    resetProjectState({ state: 'submitted' })
    const { getByRole } = render(<WorkflowBar />)
    getByRole('button', { name: 'Approve PR' }).click()
    expect(mockApproveRemote).toHaveBeenCalled()
  })

  it('calls mergeRemote() when Merge PR is clicked', () => {
    resetProjectState({ state: 'submitted' })
    const { getByRole } = render(<WorkflowBar />)
    getByRole('button', { name: 'Merge PR' }).click()
    expect(mockMergeRemote).toHaveBeenCalledWith('rebase')
  })

  it('opens finish modal when Finish and clean up is clicked in submitted state', () => {
    resetProjectState({ state: 'submitted' })
    const { getByRole, getByTestId } = render(<WorkflowBar />)
    act(() => {
      getByRole('button', { name: 'Finish and clean up' }).click()
    })
    expect(getByTestId('confirm-modal')).toBeInTheDocument()
  })

  // --- Confirm modal interactions ---

  it('opens submit modal and shows Submit Pull Request title', () => {
    resetProjectState({ state: 'reviewing' })
    const { getByRole, getByTestId } = render(<WorkflowBar />)
    act(() => {
      getByRole('button', { name: /Submit — click to start/ }).click()
    })
    const modal = getByTestId('confirm-modal')
    expect(modal.textContent).toContain('Submit Pull Request')
  })

  it('opens abandon modal and shows Abandon Task title', () => {
    resetProjectState({ state: 'loaded' })
    const { getByRole, getByTestId } = render(<WorkflowBar />)
    act(() => {
      getByRole('button', { name: 'Abandon task' }).click()
    })
    const modal = getByTestId('confirm-modal')
    expect(modal.textContent).toContain('Abandon Task?')
  })

  it('opens finish modal from Finish button in submitted state', () => {
    resetProjectState({ state: 'submitted' })
    const { getByRole, getByTestId } = render(<WorkflowBar />)
    act(() => {
      getByRole('button', { name: 'Finish and clean up' }).click()
    })
    const modal = getByTestId('confirm-modal')
    expect(modal.textContent).toContain('Finish Task')
  })

  // --- Disabling buttons during loading ---

  it('disables step buttons when loading', () => {
    resetProjectState({ state: 'loaded', loading: true })
    const { queryByRole } = render(<WorkflowBar />)
    // Plan step should not be clickable when loading
    expect(queryByRole('button', { name: /Plan — click to start/ })).not.toBeInTheDocument()
  })

  it('disables Stop button when loading', () => {
    resetProjectState({ state: 'implementing', loading: true })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: /Stop current operation/ })).toBeDisabled()
  })

  it('disables Approve transition button when loading', () => {
    resetProjectState({ state: 'loaded', loading: true, error: 'approval required' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Approve transition' })).toBeDisabled()
  })

  it('disables post-submit buttons when loading', () => {
    resetProjectState({ state: 'submitted', loading: true })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Check PR status' })).toBeDisabled()
    expect(getByRole('button', { name: 'Approve PR' })).toBeDisabled()
    expect(getByRole('button', { name: 'Merge PR' })).toBeDisabled()
    expect(getByRole('button', { name: 'Finish and clean up' })).toBeDisabled()
  })

  // --- Explain button ---

  it('shows Explain last action button in all non-none states', () => {
    resetProjectState({ state: 'implementing' })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Explain last action' })).toBeInTheDocument()
  })

  it('disables Explain button when loading', () => {
    resetProjectState({ state: 'loaded', loading: true })
    const { getByRole } = render(<WorkflowBar />)
    expect(getByRole('button', { name: 'Explain last action' })).toBeDisabled()
  })

  // --- Phase progress ---

  it('does not show progress bar when not in active state', () => {
    resetProjectState({
      state: 'loaded',
      phaseProgress: { percent: 50, calibrated: true, eta: 60 },
    })
    const { queryByText } = render(<WorkflowBar />)
    expect(queryByText(/50%/)).not.toBeInTheDocument()
  })

  it('shows progress without ETA when not calibrated', () => {
    resetProjectState({
      state: 'implementing',
      phaseProgress: { percent: 30, calibrated: false, eta: 60 },
    })
    const { getByText, queryByText } = render(<WorkflowBar />)
    expect(getByText(/30%/)).toBeInTheDocument()
    expect(queryByText(/~1m/)).not.toBeInTheDocument()
  })

  it('shows progress with ETA in hours format', () => {
    resetProjectState({
      state: 'implementing',
      phaseProgress: { percent: 10, calibrated: true, eta: 3720 },
    })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText(/10%/)).toBeInTheDocument()
    expect(getByText(/~1h2m/)).toBeInTheDocument()
  })

  it('shows progress with ETA in seconds format', () => {
    resetProjectState({
      state: 'implementing',
      phaseProgress: { percent: 95, calibrated: true, eta: 45 },
    })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText(/95%/)).toBeInTheDocument()
    expect(getByText(/~45s/)).toBeInTheDocument()
  })

  it('does not show ETA when eta is negative', () => {
    resetProjectState({
      state: 'implementing',
      phaseProgress: { percent: 99, calibrated: true, eta: -1 },
    })
    const { getByText, queryByText } = render(<WorkflowBar />)
    expect(getByText(/99%/)).toBeInTheDocument()
    expect(queryByText(/~/)).not.toBeInTheDocument()
  })

  // --- Auto-fix status ---

  it('shows auto-fix badge on current step when active', () => {
    resetProjectState({
      state: 'implementing',
      autoFixStatus: { active: true, attempt: 2, maxAttempts: 5 },
    })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('fix 2/5')).toBeInTheDocument()
  })

  it('does not show auto-fix badge when not active', () => {
    resetProjectState({
      state: 'implementing',
      autoFixStatus: { active: false, attempt: 1, maxAttempts: 3 },
    })
    const { queryByText } = render(<WorkflowBar />)
    expect(queryByText(/fix \d+\/\d+/)).not.toBeInTheDocument()
  })

  // --- Hint text for various states ---

  it('shows hint for planning state', () => {
    resetProjectState({ state: 'planning' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Planning in progress...')).toBeInTheDocument()
  })

  it('shows hint for implementing state', () => {
    resetProjectState({ state: 'implementing' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Implementation in progress...')).toBeInTheDocument()
  })

  it('shows hint for implemented state', () => {
    resetProjectState({ state: 'implemented' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Code complete — Review, Simplify, or Optimize.')).toBeInTheDocument()
  })

  it('shows hint for submitted state', () => {
    resetProjectState({ state: 'submitted' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText(/PR submitted/)).toBeInTheDocument()
  })

  it('shows hint for failed state', () => {
    resetProjectState({ state: 'failed' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Last action failed. Check errors or Undo.')).toBeInTheDocument()
  })

  it('shows hint for waiting state', () => {
    resetProjectState({ state: 'waiting' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Waiting for input.')).toBeInTheDocument()
  })

  // --- Paused badge variants ---

  it('shows Paused badge for waiting state', () => {
    resetProjectState({ state: 'waiting' })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('Paused')).toBeInTheDocument()
  })

  // --- Skip phases visual indicator ---

  it('shows skip indicator with single phase', () => {
    resetProjectState({ state: 'loaded', skipPhases: ['optimize'] })
    const { getByText } = render(<WorkflowBar />)
    expect(getByText('skip: optimize')).toBeInTheDocument()
  })
})
