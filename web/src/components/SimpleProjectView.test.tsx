import { render, fireEvent } from '@testing-library/react'
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
  SimpleReviewSummary: ({
    onReview,
    onRequestChanges,
  }: {
    loading: boolean
    onReview: () => void
    onRequestChanges: () => void
  }) => (
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

  it('calls plan when Make a Plan is clicked', () => {
    mockProjectState.state = 'loaded'
    const { getByText } = render(<SimpleProjectView />)
    getByText('Make a Plan').click()
    expect(mockPlan).toHaveBeenCalled()
  })

  it('calls implement when Build is clicked', () => {
    mockProjectState.state = 'planned'
    const { getByText } = render(<SimpleProjectView />)
    getByText('Build').click()
    expect(mockImplement).toHaveBeenCalled()
  })

  it('calls stop when Stop is clicked', () => {
    mockProjectState.state = 'implementing'
    const { getByText } = render(<SimpleProjectView />)
    getByText('Stop').click()
    expect(mockStop).toHaveBeenCalled()
  })

  it('calls retry when Try Again is clicked', () => {
    mockProjectState.state = 'failed'
    const { getByText } = render(<SimpleProjectView />)
    getByText('Try Again').click()
    expect(mockRetry).toHaveBeenCalled()
  })

  it('calls retry when Resume is clicked in paused state', () => {
    mockProjectState.state = 'paused'
    const { getByText } = render(<SimpleProjectView />)
    getByText('Resume').click()
    expect(mockRetry).toHaveBeenCalled()
  })

  it('calls start when Start is clicked with input', async () => {
    mockProjectState.state = 'none'
    mockStart.mockResolvedValue(undefined)
    const { getByText, getByLabelText } = render(<SimpleProjectView />)
    const input = getByLabelText(/Enter a GitHub issue URL/)
    fireEvent.change(input, { target: { value: 'Fix the login bug' } })
    getByText('Start').click()
    expect(mockStart).toHaveBeenCalledWith('Fix the login bug')
  })

  it('calls finish and navigates back when Finish & Return is clicked', async () => {
    mockProjectState.state = 'submitted'
    mockFinish.mockResolvedValue(true)
    const { getByText } = render(<SimpleProjectView />)
    getByText('Finish & Return').click()
    await vi.waitFor(() => {
      expect(mockFinish).toHaveBeenCalled()
      expect(mockSelectProject).toHaveBeenCalledWith(null)
    })
  })

  it('does not navigate back if finish returns falsy', async () => {
    mockProjectState.state = 'submitted'
    mockFinish.mockResolvedValue(null)
    const { getByText } = render(<SimpleProjectView />)
    getByText('Finish & Return').click()
    await vi.waitFor(() => {
      expect(mockFinish).toHaveBeenCalled()
    })
    expect(mockSelectProject).not.toHaveBeenCalled()
  })

  it('shows error message in none state when error exists', () => {
    mockProjectState.state = 'none'
    mockProjectState.error = 'Invalid task source'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Invalid task source')).toBeInTheDocument()
  })

  it('calls start on Ctrl+Enter in task input', () => {
    mockProjectState.state = 'none'
    mockStart.mockResolvedValue(undefined)
    const { getByLabelText } = render(<SimpleProjectView />)
    const input = getByLabelText(/Enter a GitHub issue URL/)
    fireEvent.change(input, { target: { value: 'task description' } })
    fireEvent.keyDown(input, { key: 'Enter', ctrlKey: true })
    expect(mockStart).toHaveBeenCalledWith('task description')
  })

  it('calls start on Meta+Enter in task input', () => {
    mockProjectState.state = 'none'
    mockStart.mockResolvedValue(undefined)
    const { getByLabelText } = render(<SimpleProjectView />)
    const input = getByLabelText(/Enter a GitHub issue URL/)
    fireEvent.change(input, { target: { value: 'task description' } })
    fireEvent.keyDown(input, { key: 'Enter', metaKey: true })
    expect(mockStart).toHaveBeenCalledWith('task description')
  })

  it('shows Browse files button and toggles file picker', async () => {
    mockProjectState.state = 'none'
    mockListFiles.mockResolvedValue([{ path: 'README.md' }, { path: 'task.md' }])
    const { getByText } = render(<SimpleProjectView />)
    getByText('Browse files').click()
    await vi.waitFor(() => {
      expect(mockListFiles).toHaveBeenCalled()
    })
  })

  it('hides file picker when Browse files clicked again', async () => {
    mockProjectState.state = 'none'
    mockListFiles.mockResolvedValue([{ path: 'README.md' }])
    const { getByText } = render(<SimpleProjectView />)
    // Open
    getByText('Browse files').click()
    await vi.waitFor(() => {
      expect(getByText('README.md')).toBeInTheDocument()
    })
    // Close
    getByText('Hide').click()
    await vi.waitFor(() => {
      expect(() => getByText('README.md')).toThrow()
    })
  })

  it('sets task source when file is selected from picker', async () => {
    mockProjectState.state = 'none'
    mockListFiles.mockResolvedValue([{ path: 'task.md' }])
    const { getByText, getByLabelText } = render(<SimpleProjectView />)
    getByText('Browse files').click()
    await vi.waitFor(() => {
      expect(getByText('task.md')).toBeInTheDocument()
    })
    getByText('task.md').click()
    await vi.waitFor(() => {
      const input = getByLabelText(/Enter a GitHub issue URL/) as HTMLTextAreaElement
      expect(input.value).toBe('file:task.md')
    })
  })

  it('shows PR link when output contains pull request URL', () => {
    mockProjectState.state = 'submitted'
    mockProjectState.output = ['Creating PR...', 'PR created: https://github.com/org/repo/pull/123']
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('View Pull Request')).toBeInTheDocument()
  })

  it('shows fallback text when no PR URL in output', () => {
    mockProjectState.state = 'submitted'
    mockProjectState.output = ['Done']
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText(/PR was created/)).toBeInTheDocument()
  })

  it('shows all status labels for each state', () => {
    const stateLabels: Record<string, string> = {
      loaded: 'Task loaded',
      implementing: 'Writing code...',
      implemented: 'Code complete',
      simplifying: 'Cleaning up code...',
      optimizing: 'Improving code quality...',
      reviewing: 'AI is reviewing...',
      submitted: 'PR submitted!',
      failed: 'Something went wrong',
      waiting: 'Waiting for your input',
      paused: 'Paused',
    }
    for (const [state, label] of Object.entries(stateLabels)) {
      mockProjectState.state = state
      const { getByText, unmount } = render(<SimpleProjectView />)
      expect(getByText(new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))).toBeInTheDocument()
      unmount()
    }
  })

  it('shows Stop button during simplifying state', () => {
    mockProjectState.state = 'simplifying'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Stop')).toBeInTheDocument()
  })

  it('shows Stop button during optimizing state', () => {
    mockProjectState.state = 'optimizing'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Stop')).toBeInTheDocument()
  })

  it('shows Stop button during reviewing state', () => {
    mockProjectState.state = 'reviewing'
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Stop')).toBeInTheDocument()
  })

  it('uses "Project" as fallback name when no path', () => {
    mockGlobalState.selectedProject = null
    const { getByText } = render(<SimpleProjectView />)
    expect(getByText('Project')).toBeInTheDocument()
  })

  it('disables Start button when loading', () => {
    mockProjectState.state = 'none'
    mockProjectState.loading = true
    const { container } = render(<SimpleProjectView />)
    // When loading, the button shows a spinner instead of "Start" text
    // Find the primary button in the task input area
    const submitBtn = container.querySelector('button.btn-primary') as HTMLButtonElement
    expect(submitBtn).toBeDisabled()
  })
})
