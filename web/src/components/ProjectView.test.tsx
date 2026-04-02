import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ProjectView } from './ProjectView'

// Mock all stores
const mockSelectProject = vi.fn()

let mockGlobalState: Record<string, unknown> = {}
let mockProjectState: Record<string, unknown> = {}
let mockLayoutState: Record<string, unknown> = {}
let mockDebugEnabled = false

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

vi.mock('../stores/layoutStore', () => ({
  useLayoutStore: Object.assign(
    (selector?: (s: Record<string, unknown>) => unknown) => {
      if (selector) return selector(mockLayoutState)
      return mockLayoutState
    },
    {
      getState: () => mockLayoutState,
    },
  ),
}))

vi.mock('../stores/debugStore', () => ({
  useDebugStore: (selector: (s: { enabled: boolean }) => unknown) => selector({ enabled: mockDebugEnabled }),
}))

vi.mock('../hooks/useDocsURL', () => ({
  useDocsURL: () => null,
}))

vi.mock('../hooks/useKeyboardShortcuts', () => ({
  useKeyboardShortcuts: () => {},
}))

// Mock all child components to avoid deep rendering
vi.mock('./PanelLayout', () => ({
  PanelLayout: ({ header, leftContent, rightContent }: { header?: React.ReactNode; leftContent: React.ReactNode; rightContent: React.ReactNode }) => (
    <div data-testid="panel-layout">
      {header && <div data-testid="panel-header">{header}</div>}
      <div data-testid="panel-left">{leftContent}</div>
      <div data-testid="panel-right">{rightContent}</div>
    </div>
  ),
}))

vi.mock('./Widget', () => ({
  Widget: ({ title, children }: { title: string; children: React.ReactNode }) => <div data-testid={`widget-${title}`}>{children}</div>,
  TaskIcon: () => <span>TaskIcon</span>,
  FilesIcon: () => <span>FilesIcon</span>,
  CheckpointsIcon: () => <span>CheckpointsIcon</span>,
}))

vi.mock('./ErrorBoundary', () => ({
  ErrorBoundary: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

vi.mock('./TaskWidget', () => ({ TaskWidget: () => <div data-testid="task-widget">TaskWidget</div> }))
vi.mock('./SuggestionBanner', () => ({ SuggestionBanner: () => null }))
vi.mock('./CheckpointsWidget', () => ({ CheckpointsWidget: () => null }))
vi.mock('./RecapWidget', () => ({ RecapWidget: () => null }))
vi.mock('./PhaseMetricsWidget', () => ({ PhaseMetricsWidget: () => null }))
vi.mock('./ChecklistWidget', () => ({ ChecklistWidget: () => null }))
vi.mock('./ReviewHistoryWidget', () => ({ ReviewHistoryWidget: () => null }))
vi.mock('./FileChangesWidget', () => ({ FileChangesWidget: () => null }))
vi.mock('./AgentPanel', () => ({ AgentPanel: () => null }))
vi.mock('./TaskQueue', () => ({ TaskQueue: () => null }))
vi.mock('./TaskHistory', () => ({ TaskHistory: () => null }))
vi.mock('./ThemeToggle', () => ({ ThemeToggle: () => null }))
vi.mock('./ViewModeToggle', () => ({ ViewModeToggle: () => null }))
vi.mock('./StatusIndicator', () => ({ StatusBadge: ({ label }: { label: string }) => <span data-testid="status-badge">{label}</span> }))
vi.mock('./WorkflowBar', () => ({ WorkflowBar: () => <div data-testid="workflow-bar" /> }))

describe('ProjectView', () => {
  beforeEach(() => {
    mockSelectProject.mockReset()
    mockDebugEnabled = false
    mockGlobalState = {
      selectedProject: { path: '/home/user/my-project' },
      selectProject: mockSelectProject,
      agentStatus: null,
    }
    mockProjectState = {
      task: null,
      state: 'none',
      fileChanges: [],
      reviews: [],
      output: [],
      taskQueue: [],
    }
    mockLayoutState = {
      widgetStates: {
        task: { collapsed: false, visible: true },
        files: { collapsed: false, visible: true },
        checkpoints: { collapsed: false, visible: true },
      },
    }
  })

  it('returns null when no project is selected', () => {
    mockGlobalState.selectedProject = null
    const { container } = render(<ProjectView />)
    expect(container.innerHTML).toBe('')
  })

  it('renders project name in header', () => {
    const { getByText } = render(<ProjectView />)
    expect(getByText('my-project')).toBeInTheDocument()
  })

  it('renders the back to projects button', () => {
    const { getByLabelText } = render(<ProjectView />)
    expect(getByLabelText('Back to projects')).toBeInTheDocument()
  })

  it('calls selectProject(null) when back button is clicked', () => {
    const { getByLabelText } = render(<ProjectView />)
    getByLabelText('Back to projects').click()
    expect(mockSelectProject).toHaveBeenCalledWith(null)
  })

  it('renders the PanelLayout', () => {
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('panel-layout')).toBeInTheDocument()
  })

  it('renders the WorkflowBar', () => {
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('workflow-bar')).toBeInTheDocument()
  })

  it('shows status badge with state label', () => {
    mockProjectState.state = 'implementing'
    mockProjectState.task = { state: 'implementing' }
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('Implementing...')
  })

  it('shows simulation mode alert when active', () => {
    mockGlobalState.agentStatus = { simulation_mode: true }
    const { getByText } = render(<ProjectView />)
    expect(getByText(/Simulation mode/)).toBeInTheDocument()
  })

  it('does not show simulation mode alert when inactive', () => {
    mockGlobalState.agentStatus = { simulation_mode: false }
    const { queryByText } = render(<ProjectView />)
    expect(queryByText(/Simulation mode/)).not.toBeInTheDocument()
  })

  it('shows DEBUG badge when debug mode is enabled', () => {
    mockDebugEnabled = true
    const { getByText } = render(<ProjectView />)
    expect(getByText('DEBUG')).toBeInTheDocument()
  })

  it('does not show DEBUG badge when debug mode is disabled', () => {
    mockDebugEnabled = false
    const { queryByText } = render(<ProjectView />)
    expect(queryByText('DEBUG')).not.toBeInTheDocument()
  })

  it('renders Settings button', () => {
    const { getByLabelText } = render(<ProjectView />)
    expect(getByLabelText('Settings')).toBeInTheDocument()
  })

  it('renders View logs button', () => {
    const { getByLabelText } = render(<ProjectView />)
    expect(getByLabelText('View logs')).toBeInTheDocument()
  })

  it('shows project path', () => {
    const { getByText } = render(<ProjectView />)
    expect(getByText('/home/user/my-project')).toBeInTheDocument()
  })
})
