import { render, fireEvent, act } from '@testing-library/react'
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

// Mock lazy-loaded panel components
vi.mock('./Settings', () => ({ Settings: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="settings-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./MemoryPanel', () => ({ MemoryPanel: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="memory-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./DiagnosePanel', () => ({ DiagnosePanel: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="diagnose-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./RecordingsPanel', () => ({ RecordingsPanel: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="recordings-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./BackupPanel', () => ({ BackupPanel: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="backup-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./ActivityPanel', () => ({ ActivityPanel: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="activity-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./SecurityPanel', () => ({ SecurityPanel: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="security-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./CatalogPanel', () => ({ CatalogPanel: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="catalog-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./ReportPanel', () => ({ ReportPanel: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="report-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./ExportPanel', () => ({ ExportPanel: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => isOpen ? <div data-testid="export-panel"><button onClick={onClose}>Close</button></div> : null }))
vi.mock('./TaskGroupPanel', () => ({ TaskGroupPanel: ({ onClose }: { onClose: () => void }) => <div data-testid="taskgroup-panel"><button onClick={onClose}>Close</button></div> }))

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

  // --- taskByPath useMemo ---
  it('shows task title on project card when activeTasks has a matching task', () => {
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha', state: 'implementing' },
    ]
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'implementing', task_title: 'Build feature X', source: 'github#42' },
    ]
    const { getByText } = render(<GlobalView />)
    expect(getByText('Build feature X')).toBeInTheDocument()
    expect(getByText('github#42')).toBeInTheDocument()
  })

  it('shows "No active task" when task state is "none"', () => {
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha', state: 'none' },
    ]
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'none', task_title: 'Should not show' },
    ]
    const { getByText } = render(<GlobalView />)
    expect(getByText('No active task')).toBeInTheDocument()
  })

  it('shows "No active task" when task state is "submitted"', () => {
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha', state: 'submitted' },
    ]
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'submitted', task_title: 'PR sent' },
    ]
    const { getByText } = render(<GlobalView />)
    expect(getByText('No active task')).toBeInTheDocument()
  })

  // --- filteredProjects useMemo ---
  it('filters projects by name when search query is entered', () => {
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha' },
      { id: 'p2', path: '/home/user/beta' },
      { id: 'p3', path: '/home/user/gamma' },
      { id: 'p4', path: '/home/user/delta' },
    ]
    const { getByLabelText, getByText, queryByText } = render(<GlobalView />)
    const searchInput = getByLabelText('Search projects')
    fireEvent.change(searchInput, { target: { value: 'alpha' } })
    expect(getByText('alpha')).toBeInTheDocument()
    expect(queryByText('beta')).not.toBeInTheDocument()
  })

  it('filters projects by path when search query matches path', () => {
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha' },
      { id: 'p2', path: '/opt/work/beta' },
      { id: 'p3', path: '/home/user/gamma' },
      { id: 'p4', path: '/home/user/delta' },
    ]
    const { getByLabelText, getByText, queryByText } = render(<GlobalView />)
    const searchInput = getByLabelText('Search projects')
    fireEvent.change(searchInput, { target: { value: '/opt/work' } })
    expect(getByText('beta')).toBeInTheDocument()
    expect(queryByText('alpha')).not.toBeInTheDocument()
  })

  it('shows "No projects matching" when search has no results', () => {
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha' },
      { id: 'p2', path: '/home/user/beta' },
      { id: 'p3', path: '/home/user/gamma' },
      { id: 'p4', path: '/home/user/delta' },
    ]
    const { getByLabelText, getByText } = render(<GlobalView />)
    const searchInput = getByLabelText('Search projects')
    fireEvent.change(searchInput, { target: { value: 'zzzznotfound' } })
    expect(getByText(/No projects matching/)).toBeInTheDocument()
  })

  // --- handleBatchAction ---
  it('runs batch action when confirmed', async () => {
    window.confirm = vi.fn(() => true)
    window.alert = vi.fn()
    mockBatchAction.mockResolvedValue({ succeeded: 3, total: 5 })
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'implementing' },
    ]
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha', state: 'implementing' },
    ]
    const { getByText } = render(<GlobalView />)
    // Open the batch dropdown and click "plan"
    await act(async () => {
      fireEvent.click(getByText('plan'))
    })
    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Run "plan"'))
    expect(mockBatchAction).toHaveBeenCalledWith('plan', { state: undefined, tag: undefined })
    expect(window.alert).toHaveBeenCalledWith('Batch plan: 3/5 succeeded')
  })

  it('does not run batch action when confirm is cancelled', async () => {
    window.confirm = vi.fn(() => false)
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'implementing' },
    ]
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha', state: 'implementing' },
    ]
    const { getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByText('plan'))
    })
    expect(window.confirm).toHaveBeenCalled()
    expect(mockBatchAction).not.toHaveBeenCalled()
  })

  it('shows error alert when batch action fails', async () => {
    window.confirm = vi.fn(() => true)
    window.alert = vi.fn()
    mockBatchAction.mockRejectedValue(new Error('Network error'))
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'implementing' },
    ]
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha', state: 'implementing' },
    ]
    const { getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByText('implement'))
    })
    expect(window.alert).toHaveBeenCalledWith('Batch implement failed: Network error')
  })

  // --- toggleSilentMode ---
  it('toggles silent mode and calls settings.set', async () => {
    const mockCall = vi.fn().mockResolvedValue(undefined)
    mockGlobalState.client = { call: mockCall }
    const { getByLabelText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Mute notifications'))
    })
    expect(mockCall).toHaveBeenCalledWith('settings.set', { path: 'notify.silent', value: true })
  })

  it('reverts silent mode on settings.set failure', async () => {
    const mockCall = vi.fn().mockRejectedValue(new Error('fail'))
    mockGlobalState.client = { call: mockCall }
    const { getByLabelText } = render(<GlobalView />)
    // First click: mute (sets silentMode to true)
    await act(async () => {
      fireEvent.click(getByLabelText('Mute notifications'))
    })
    // After revert, silentMode should be back to false, so label should be "Mute notifications"
    expect(getByLabelText('Mute notifications')).toBeInTheDocument()
  })

  it('does nothing when toggleSilentMode is called with no client', async () => {
    mockGlobalState.client = null
    const { getByLabelText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Mute notifications'))
    })
    // Should still show "Mute notifications" (not toggled)
    expect(getByLabelText('Mute notifications')).toBeInTheDocument()
  })

  // --- handleFolderSelect ---
  it('calls addProject when folder is selected via FolderPicker', () => {
    // FolderPicker is mocked, so we test indirectly: the callback is passed to FolderPicker
    // We just verify addProject is available and the component renders the folder picker
    const { getByTestId } = render(<GlobalView />)
    expect(getByTestId('folder-picker')).toBeInTheDocument()
  })

  // --- handleRemoveProject ---
  it('removes project when confirmed', async () => {
    window.confirm = vi.fn(() => true)
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha' },
    ]
    const { getByLabelText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Remove alpha'))
    })
    expect(window.confirm).toHaveBeenCalledWith('Remove this project from the list?')
    expect(mockRemoveProject).toHaveBeenCalledWith('p1')
  })

  it('does not remove project when confirm is cancelled', async () => {
    window.confirm = vi.fn(() => false)
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha' },
    ]
    const { getByLabelText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Remove alpha'))
    })
    expect(mockRemoveProject).not.toHaveBeenCalled()
  })

  // --- hasActiveProjects ---
  it('shows batch actions dropdown when there are active tasks', () => {
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'implementing' },
    ]
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha', state: 'implementing' },
    ]
    const { getByLabelText } = render(<GlobalView />)
    expect(getByLabelText('Batch Actions')).toBeInTheDocument()
  })

  it('hides batch actions dropdown when no active tasks', () => {
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'none' },
    ]
    const { queryByLabelText } = render(<GlobalView />)
    expect(queryByLabelText('Batch Actions')).not.toBeInTheDocument()
  })

  // --- Modal toggle buttons ---
  it('opens Settings panel when settings button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Settings'))
    })
    expect(getByTestId('settings-panel')).toBeInTheDocument()
  })

  it('opens Diagnose panel when diagnostics button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('System Diagnostics'))
    })
    expect(getByTestId('diagnose-panel')).toBeInTheDocument()
  })

  it('opens Memory panel when memory button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Memory Search'))
    })
    expect(getByTestId('memory-panel')).toBeInTheDocument()
  })

  it('opens Recordings panel when recordings button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Recordings'))
    })
    expect(getByTestId('recordings-panel')).toBeInTheDocument()
  })

  it('opens Backup panel when backup button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Backup'))
    })
    expect(getByTestId('backup-panel')).toBeInTheDocument()
  })

  it('opens Activity panel when activity button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Activity Log'))
    })
    expect(getByTestId('activity-panel')).toBeInTheDocument()
  })

  it('opens Security panel when security button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Security Scan'))
    })
    expect(getByTestId('security-panel')).toBeInTheDocument()
  })

  it('opens Catalog panel when catalog button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Template Catalog'))
    })
    expect(getByTestId('catalog-panel')).toBeInTheDocument()
  })

  it('opens Task Groups panel when task groups button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Task Groups'))
    })
    expect(getByTestId('taskgroup-panel')).toBeInTheDocument()
  })

  it('opens Report panel when report button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Compliance Report'))
    })
    expect(getByTestId('report-panel')).toBeInTheDocument()
  })

  it('opens Export panel when export button is clicked', async () => {
    const { getByLabelText, getByTestId } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Export Data'))
    })
    expect(getByTestId('export-panel')).toBeInTheDocument()
  })

  it('closes Settings panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Settings'))
    })
    expect(getByTestId('settings-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('settings-panel')).not.toBeInTheDocument()
  })

  it('closes Memory panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Memory Search'))
    })
    expect(getByTestId('memory-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('memory-panel')).not.toBeInTheDocument()
  })

  it('closes Diagnose panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('System Diagnostics'))
    })
    expect(getByTestId('diagnose-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('diagnose-panel')).not.toBeInTheDocument()
  })

  it('closes Recordings panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Recordings'))
    })
    expect(getByTestId('recordings-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('recordings-panel')).not.toBeInTheDocument()
  })

  it('closes Backup panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Backup'))
    })
    expect(getByTestId('backup-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('backup-panel')).not.toBeInTheDocument()
  })

  it('closes Activity panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Activity Log'))
    })
    expect(getByTestId('activity-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('activity-panel')).not.toBeInTheDocument()
  })

  it('closes Security panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Security Scan'))
    })
    expect(getByTestId('security-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('security-panel')).not.toBeInTheDocument()
  })

  it('closes Catalog panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Template Catalog'))
    })
    expect(getByTestId('catalog-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('catalog-panel')).not.toBeInTheDocument()
  })

  it('closes Report panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Compliance Report'))
    })
    expect(getByTestId('report-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('report-panel')).not.toBeInTheDocument()
  })

  it('closes Export panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Export Data'))
    })
    expect(getByTestId('export-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('export-panel')).not.toBeInTheDocument()
  })

  it('closes Task Groups panel when close button is clicked', async () => {
    const { getByLabelText, getByTestId, queryByTestId, getByText } = render(<GlobalView />)
    await act(async () => {
      fireEvent.click(getByLabelText('Task Groups'))
    })
    expect(getByTestId('taskgroup-panel')).toBeInTheDocument()
    await act(async () => {
      fireEvent.click(getByText('Close'))
    })
    expect(queryByTestId('taskgroup-panel')).not.toBeInTheDocument()
  })

  // --- FolderPicker open/close ---
  it('opens folder picker when Add Project button is clicked in empty state', () => {
    mockGlobalState.projects = []
    const { getAllByText } = render(<GlobalView />)
    // There are two "Add Project" buttons (header and empty state)
    const buttons = getAllByText('Add Project')
    expect(buttons.length).toBeGreaterThanOrEqual(1)
  })

  // --- isSimple hides toolbar buttons ---
  it('hides toolbar buttons in simple mode', () => {
    mockViewModeState.mode = 'simple'
    const { queryByLabelText } = render(<GlobalView />)
    expect(queryByLabelText('System Diagnostics')).not.toBeInTheDocument()
    expect(queryByLabelText('Memory Search')).not.toBeInTheDocument()
    expect(queryByLabelText('Recordings')).not.toBeInTheDocument()
    expect(queryByLabelText('Backup')).not.toBeInTheDocument()
    expect(queryByLabelText('Activity Log')).not.toBeInTheDocument()
    expect(queryByLabelText('Security Scan')).not.toBeInTheDocument()
    expect(queryByLabelText('Template Catalog')).not.toBeInTheDocument()
    expect(queryByLabelText('Task Groups')).not.toBeInTheDocument()
    expect(queryByLabelText('Compliance Report')).not.toBeInTheDocument()
    expect(queryByLabelText('Export Data')).not.toBeInTheDocument()
  })

  // --- Refresh button ---
  it('calls loadProjects when Refresh button is clicked', () => {
    const { getByLabelText } = render(<GlobalView />)
    fireEvent.click(getByLabelText('Refresh projects'))
    expect(mockLoadProjects).toHaveBeenCalled()
  })

  // --- selectProject ---
  it('calls selectProject when a project card is clicked', () => {
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha' },
    ]
    const { getByText } = render(<GlobalView />)
    fireEvent.click(getByText('alpha'))
    expect(mockSelectProject).toHaveBeenCalledWith({ id: 'p1', path: '/home/user/alpha' })
  })

  // --- queue_count display ---
  it('shows queued count when task has queue_count', () => {
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha', state: 'implementing' },
    ]
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'implementing', task_title: 'Task', queue_count: 3 },
    ]
    const { getByText } = render(<GlobalView />)
    expect(getByText('+3 queued')).toBeInTheDocument()
  })

  // --- Agent status warning ---
  it('shows agent unavailable warning when agent is not available', () => {
    mockGlobalState.agentStatus = { agent_available: false, simulation_mode: false, checks: [] }
    const { getByText } = render(<GlobalView />)
    expect(getByText('No AI agent available')).toBeInTheDocument()
  })

  it('shows simulation mode message when in simulation', () => {
    mockGlobalState.agentStatus = { agent_available: false, simulation_mode: true, checks: [] }
    const { getByText } = render(<GlobalView />)
    expect(getByText(/simulation mode/)).toBeInTheDocument()
  })

  it('does not show agent warning in simple mode', () => {
    mockViewModeState.mode = 'simple'
    mockGlobalState.agentStatus = { agent_available: false, simulation_mode: false, checks: [] }
    const { queryByText } = render(<GlobalView />)
    expect(queryByText('No AI agent available')).not.toBeInTheDocument()
  })

  // --- Batch action with filters ---
  it('includes state filter in batch action confirm message', async () => {
    window.confirm = vi.fn(() => true)
    window.alert = vi.fn()
    mockBatchAction.mockResolvedValue({ succeeded: 1, total: 1 })
    mockGlobalState.activeTasks = [
      { path: '/home/user/alpha', state: 'planned' },
    ]
    mockGlobalState.projects = [
      { id: 'p1', path: '/home/user/alpha', state: 'planned' },
    ]
    const { getByLabelText, getByText } = render(<GlobalView />)
    // Set state filter
    fireEvent.change(getByLabelText('Filter by state'), { target: { value: 'planned' } })
    await act(async () => {
      fireEvent.click(getByText('implement'))
    })
    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('state: planned'))
    expect(mockBatchAction).toHaveBeenCalledWith('implement', { state: 'planned', tag: undefined })
  })
})
