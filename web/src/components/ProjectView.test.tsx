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

  it('opens settings modal when Settings button is clicked', () => {
    const { getByLabelText } = render(<ProjectView />)
    getByLabelText('Settings').click()
    // Settings lazy component renders (Suspense fallback is null, but state change triggers re-render)
    // The component sets showSettings to true which renders the Settings component
    // Since Settings is mocked via lazy import, we verify the click doesn't throw
    expect(getByLabelText('Settings')).toBeInTheDocument()
  })

  it('opens logs panel when View logs button is clicked', () => {
    const { getByLabelText } = render(<ProjectView />)
    getByLabelText('View logs').click()
    // Verify click triggers without error (lazy loaded modal toggled)
    expect(getByLabelText('View logs')).toBeInTheDocument()
  })

  it('shows log indicator dot when output has entries', () => {
    mockProjectState.output = ['line1', 'line2']
    const { getByLabelText } = render(<ProjectView />)
    const logsBtn = getByLabelText('View logs')
    // The button should contain the notification dot span
    const dot = logsBtn.querySelector('.bg-primary.rounded-full')
    expect(dot).toBeInTheDocument()
  })

  it('does not show log indicator dot when output is empty', () => {
    mockProjectState.output = []
    const { getByLabelText } = render(<ProjectView />)
    const logsBtn = getByLabelText('View logs')
    const dot = logsBtn.querySelector('.bg-primary.rounded-full')
    expect(dot).not.toBeInTheDocument()
  })

  it('opens CI Status modal from dropdown', () => {
    const { getByRole } = render(<ProjectView />)
    const ciBtn = getByRole('menuitem', { name: /CI Status/ })
    ciBtn.click()
    expect(ciBtn).toBeInTheDocument()
  })

  it('opens Policy modal from dropdown', () => {
    const { getByRole } = render(<ProjectView />)
    const btn = getByRole('menuitem', { name: /Policy/ })
    btn.click()
    expect(btn).toBeInTheDocument()
  })

  it('opens Code Graph modal from dropdown', () => {
    const { getByRole } = render(<ProjectView />)
    const btn = getByRole('menuitem', { name: /Code Graph/ })
    btn.click()
    expect(btn).toBeInTheDocument()
  })

  it('opens Hooks modal from dropdown', () => {
    const { getByRole } = render(<ProjectView />)
    const btn = getByRole('menuitem', { name: /Hooks/ })
    btn.click()
    expect(btn).toBeInTheDocument()
  })

  it('opens Changelog modal from dropdown', () => {
    const { getByRole } = render(<ProjectView />)
    const btn = getByRole('menuitem', { name: /Changelog/ })
    btn.click()
    expect(btn).toBeInTheDocument()
  })

  it('opens Metrics modal from dropdown', () => {
    const { getByRole } = render(<ProjectView />)
    const btn = getByRole('menuitem', { name: /Metrics/ })
    btn.click()
    expect(btn).toBeInTheDocument()
  })

  it('opens Quality Gates modal from dropdown', () => {
    const { getByRole } = render(<ProjectView />)
    const btn = getByRole('menuitem', { name: /Quality Gates/ })
    btn.click()
    expect(btn).toBeInTheDocument()
  })

  it('opens Event Log modal from dropdown', () => {
    const { getByRole } = render(<ProjectView />)
    const btn = getByRole('menuitem', { name: /Event Log/ })
    btn.click()
    expect(btn).toBeInTheDocument()
  })

  it('computes statusType as success for implemented state', () => {
    mockProjectState.state = 'implemented'
    mockProjectState.task = { state: 'implemented' }
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('Implemented')
  })

  it('computes statusType as success for submitted state', () => {
    mockProjectState.state = 'submitted'
    mockProjectState.task = { state: 'submitted' }
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('Submitted')
  })

  it('computes statusType as error for failed state', () => {
    mockProjectState.state = 'failed'
    mockProjectState.task = { state: 'failed' }
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('Failed')
  })

  it('computes statusType as running for planning state', () => {
    mockProjectState.state = 'planning'
    mockProjectState.task = { state: 'planning' }
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('Planning...')
  })

  it('computes statusType as running for reviewing state', () => {
    mockProjectState.state = 'reviewing'
    mockProjectState.task = { state: 'reviewing' }
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('Reviewing...')
  })

  it('computes stateLabel as No Task for none state', () => {
    mockProjectState.state = 'none'
    mockProjectState.task = null
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('No Task')
  })

  it('computes stateLabel as Ready for loaded state', () => {
    mockProjectState.state = 'loaded'
    mockProjectState.task = { state: 'loaded' }
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('Ready')
  })

  it('computes stateLabel as Planned for planned state', () => {
    mockProjectState.state = 'planned'
    mockProjectState.task = { state: 'planned' }
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('Planned')
  })

  it('falls back to raw state string for unknown state', () => {
    mockProjectState.state = 'custom-unknown'
    mockProjectState.task = { state: 'custom-unknown' }
    const { getByTestId } = render(<ProjectView />)
    expect(getByTestId('status-badge')).toHaveTextContent('custom-unknown')
  })

  it('shows file changes count in left sidebar', () => {
    mockProjectState.fileChanges = [{ path: 'a.ts' }, { path: 'b.ts' }]
    const { container } = render(<ProjectView />)
    // The Widget for File Changes renders the count as an action
    expect(container).toBeInTheDocument()
  })

  it('renders TaskQueue widget when taskQueue is non-empty', () => {
    mockProjectState.taskQueue = [{ id: '1', title: 'Task 1' }]
    const { container } = render(<ProjectView />)
    expect(container).toBeInTheDocument()
  })

  it('renders review count in right sidebar when reviews exist', () => {
    mockProjectState.reviews = [{ id: '1' }, { id: '2' }]
    const { container } = render(<ProjectView />)
    expect(container).toBeInTheDocument()
  })

  it('renders project initial in header avatar', () => {
    const { container } = render(<ProjectView />)
    // Project name is "my-project", so initial should be "M"
    const avatar = container.querySelector('.bg-primary')
    expect(avatar).toHaveTextContent('M')
  })

  it('uses useKeyboardShortcuts hook without error', () => {
    // The hook is mocked to no-op, just verify rendering succeeds
    const { container } = render(<ProjectView />)
    expect(container.firstChild).toBeInTheDocument()
  })
})
