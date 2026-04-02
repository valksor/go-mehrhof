import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { GlobalView } from './GlobalView'

const mockConnect = vi.fn()
const mockLoadProjects = vi.fn()
const mockAddProject = vi.fn()
const mockRemoveProject = vi.fn()
const mockSelectProject = vi.fn()
const mockBatchAction = vi.fn()

let mockGlobalState: Record<string, unknown> = {}
let mockViewModeState: Record<string, unknown> = {}

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

vi.mock('../stores/viewModeStore', () => ({
  useViewModeStore: Object.assign(
    (selector?: (s: Record<string, unknown>) => unknown) => {
      if (selector) return selector(mockViewModeState)
      return mockViewModeState
    },
    {
      getState: () => mockViewModeState,
    },
  ),
}))

vi.mock('../hooks/useDocsURL', () => ({
  useDocsURL: () => null,
}))

vi.mock('../meta', () => ({
  name: 'kvelmo',
}))

// Mock all child components to avoid deep rendering
vi.mock('./FolderPicker', () => ({
  FolderPicker: () => <div data-testid="folder-picker" />,
}))
vi.mock('./ViewModeToggle', () => ({ ViewModeToggle: () => null }))
vi.mock('./ThemeToggle', () => ({ ThemeToggle: () => null }))
vi.mock('./ActiveTasksWidget', () => ({ ActiveTasksWidget: () => <div data-testid="active-tasks" /> }))
vi.mock('./MetricsWidget', () => ({ MetricsWidget: () => null }))
vi.mock('./StatsWidget', () => ({ StatsWidget: () => null }))
vi.mock('./WorkersWidget', () => ({ WorkersWidget: () => null }))
vi.mock('./Onboarding', () => ({ Onboarding: () => null }))

describe('GlobalView', () => {
  beforeEach(() => {
    mockConnect.mockReset()
    mockLoadProjects.mockReset()
    mockAddProject.mockReset()
    mockRemoveProject.mockReset()
    mockSelectProject.mockReset()
    mockBatchAction.mockReset()

    mockGlobalState = {
      projects: [],
      loading: false,
      error: null,
      connected: true,
      connecting: false,
      reconnectAttempt: 0,
      selectedProject: null,
      agentStatus: null,
      activeTasks: [],
      connect: mockConnect,
      loadProjects: mockLoadProjects,
      addProject: mockAddProject,
      removeProject: mockRemoveProject,
      selectProject: mockSelectProject,
      batchAction: mockBatchAction,
    }
    mockViewModeState = {
      mode: 'full',
    }
  })

  it('renders the app name in the header', () => {
    const { getByText } = render(<GlobalView />)
    expect(getByText('kvelmo')).toBeInTheDocument()
  })

  it('shows "No projects yet" when project list is empty', () => {
    const { getByText } = render(<GlobalView />)
    expect(getByText(/No projects yet/)).toBeInTheDocument()
  })

  it('shows Reconnect button when disconnected with no reconnect attempts', () => {
    mockGlobalState.connected = false
    mockGlobalState.connecting = false
    mockGlobalState.reconnectAttempt = 0
    const { getByLabelText } = render(<GlobalView />)
    expect(getByLabelText('Reconnect')).toBeInTheDocument()
  })

  it('calls connect when Reconnect button is clicked', () => {
    mockGlobalState.connected = false
    mockGlobalState.connecting = false
    mockGlobalState.reconnectAttempt = 0
    const { getByLabelText } = render(<GlobalView />)
    getByLabelText('Reconnect').click()
    expect(mockConnect).toHaveBeenCalled()
  })

  it('shows reconnecting indicator during auto-reconnect', () => {
    mockGlobalState.connected = false
    mockGlobalState.connecting = false
    mockGlobalState.reconnectAttempt = 3
    const { getByText } = render(<GlobalView />)
    expect(getByText(/Reconnecting \(#3\)/)).toBeInTheDocument()
  })

  it('shows connecting indicator', () => {
    mockGlobalState.connecting = true
    mockGlobalState.connected = false
    const { getByText } = render(<GlobalView />)
    expect(getByText('Connecting...')).toBeInTheDocument()
  })

  it('shows error message when error exists', () => {
    mockGlobalState.error = 'Socket connection failed'
    const { getByText } = render(<GlobalView />)
    expect(getByText('Socket connection failed')).toBeInTheDocument()
  })

  it('renders project cards when projects exist', () => {
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/project-alpha' },
      { id: 'p2', path: '/home/user/project-beta' },
    ]
    const { getByText } = render(<GlobalView />)
    expect(getByText('project-alpha')).toBeInTheDocument()
    expect(getByText('project-beta')).toBeInTheDocument()
  })

  it('renders Add Project button', () => {
    const { getAllByText } = render(<GlobalView />)
    expect(getAllByText('Add Project').length).toBeGreaterThanOrEqual(1)
  })

  it('does not show Reconnect button when connected', () => {
    mockGlobalState.connected = true
    const { queryByLabelText } = render(<GlobalView />)
    expect(queryByLabelText('Reconnect')).not.toBeInTheDocument()
  })

  it('does not show error when there is none', () => {
    mockGlobalState.error = null
    const { queryByRole } = render(<GlobalView />)
    expect(queryByRole('alert')).not.toBeInTheDocument()
  })
})
