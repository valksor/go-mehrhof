import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SimpleProjectView } from './SimpleProjectView'

const mockStart = vi.fn()
const mockPlan = vi.fn()
const mockImplement = vi.fn()
const mockReview = vi.fn()
const mockSubmit = vi.fn()
const mockFinish = vi.fn()
const mockRetry = vi.fn()
const mockStop = vi.fn()
const mockSelectProject = vi.fn()
const mockLoadSpec = vi.fn()
const mockListFiles = vi.fn()

let mockProjectState: Record<string, unknown> = {}
let mockGlobalState: Record<string, unknown> = {}

vi.mock('../stores/projectStore', () => ({
  useProjectStore: Object.assign(
    (selector?: (s: Record<string, unknown>) => unknown) => {
      if (selector) return selector(mockProjectState)
      return mockProjectState
    },
    {
      getState: () => mockProjectState,
    },
  ),
}))

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: Object.assign(
    (selector?: (s: Record<string, unknown>) => unknown) => {
      if (selector) return selector(mockGlobalState)
      return mockGlobalState
    },
    {
      getState: () => mockGlobalState,
    },
  ),
}))

// Mock child components
vi.mock('./ViewModeToggle', () => ({ ViewModeToggle: () => null }))
vi.mock('./ThemeToggle', () => ({ ThemeToggle: () => null }))
vi.mock('./SimpleTimeline', () => ({ SimpleTimeline: () => <div data-testid="simple-timeline" /> }))
vi.mock('./SimpleChatWidget', () => ({ SimpleChatWidget: () => <div data-testid="simple-chat" /> }))
vi.mock('./SimpleReviewSummary', () => ({
  SimpleReviewSummary: ({ onReview, onRequestChanges }: { loading: boolean; onReview: () => void; onRequestChanges: () => void }) => (
    <div data-testid="review-summary">
      <button onClick={onReview}>Review</button>
      <button onClick={onRequestChanges}>Request Changes</button>
    </div>
  ),
}))

describe('SimpleProjectView', () => {
  beforeEach(() => {
    mockStart.mockReset()
    mockPlan.mockReset()
    mockImplement.mockReset()
    mockReview.mockReset()
    mockSubmit.mockReset()
    mockFinish.mockReset()
    mockRetry.mockReset()
    mockStop.mockReset()
    mockSelectProject.mockReset()
    mockLoadSpec.mockResolvedValue([])
    mockListFiles.mockResolvedValue([])

    mockProjectState = {
      state: 'none',
      task: null,
      output: [],
      error: null,
      loading: false,
      start: mockStart,
      plan: mockPlan,
      implement: mockImplement,
      review: mockReview,
      submit: mockSubmit,
      finish: mockFinish,
      retry: mockRetry,
      stop: mockStop,
      loadSpec: mockLoadSpec,
      listFiles: mockListFiles,
    }
    mockGlobalState = {
      selectedProject: { path: '/home/user/test-project' },
      selectProject: mockSelectProject,
    }
  })

  it('renders project name in header', () => {
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('test-project')).toBeInTheDocument()
  })

  it('renders back button', () => {
    const { getByLabelText } = render(<SimpleProjectView />)
    expect(getByLabelText('Back to projects')).toBeInTheDocument()
  })

  it('calls selectProject(null) when back button is clicked', () => {
    const { getByLabelText } = render(<SimpleProjectView />)
    getByLabelText('Back to projects').click()
    expect(mockSelectProject).toHaveBeenCalledWith(null)
  })

  it('shows status banner with state label', () => {
    mockProjectState.state = 'none'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Ready to start')).toBeInTheDocument()
  })

  it('shows task input in none state', () => {
    mockProjectState.state = 'none'
    const { getByLabelText } = render(<SimpleProjectView />)
    expect(getByLabelText(/Enter a GitHub issue URL/)).toBeInTheDocument()
  })

  it('shows Start button in none state', () => {
    mockProjectState.state = 'none'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Start')).toBeInTheDocument()
  })

  it('disables Start button when input is empty', () => {
    mockProjectState.state = 'none'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Start')).toBeDisabled()
  })

  it('shows Make a Plan button in loaded state', () => {
    mockProjectState.state = 'loaded'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Make a Plan')).toBeInTheDocument()
  })

  it('shows Build button in planned state', () => {
    mockProjectState.state = 'planned'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Build')).toBeInTheDocument()
  })

  it('shows Stop button during active states', () => {
    mockProjectState.state = 'implementing'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Stop')).toBeInTheDocument()
  })

  it('shows review summary in implemented state', () => {
    mockProjectState.state = 'implemented'
    const { getByTestId } = render(<SimpleProjectView />)
    expect(getByTestId('review-summary')).toBeInTheDocument()
  })

  it('shows Finish & Return button in submitted state', () => {
    mockProjectState.state = 'submitted'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Finish & Return')).toBeInTheDocument()
  })

  it('shows Try Again button in failed state', () => {
    mockProjectState.state = 'failed'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Try Again')).toBeInTheDocument()
  })

  it('shows error message in failed state', () => {
    mockProjectState.state = 'failed'
    mockProjectState.error = 'Something broke'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Something broke')).toBeInTheDocument()
  })

  it('shows waiting state message', () => {
    mockProjectState.state = 'waiting'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText(/The AI needs your input/)).toBeInTheDocument()
  })

  it('shows Resume button in paused state', () => {
    mockProjectState.state = 'paused'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Resume')).toBeInTheDocument()
  })

  it('renders the timeline', () => {
    const { getByTestId } = render(<SimpleProjectView />)
    expect(getByTestId('simple-timeline')).toBeInTheDocument()
  })

  it('renders the chat widget', () => {
    const { getByTestId } = render(<SimpleProjectView />)
    expect(getByTestId('simple-chat')).toBeInTheDocument()
  })

  it('shows task title when available', () => {
    mockProjectState.state = 'loaded'
    mockProjectState.task = { title: 'Fix the bug' }
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Fix the bug')).toBeInTheDocument()
  })

  it('shows streaming output during active states', () => {
    mockProjectState.state = 'implementing'
    mockProjectState.output = ['Building...', 'Step 1 done']
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText(/Building/)).toBeInTheDocument()
  })

  it('shows Browse files button in none state', () => {
    mockProjectState.state = 'none'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Browse files')).toBeInTheDocument()
  })

  it('shows Planning status label', () => {
    mockProjectState.state = 'planning'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Planning...')).toBeInTheDocument()
  })
})
