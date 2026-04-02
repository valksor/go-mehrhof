import { render, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { TaskWidget } from './TaskWidget'

const mockStart = vi.fn()
const mockQuickStart = vi.fn()
const mockQueueTask = vi.fn()
const mockUndo = vi.fn()
const mockReset = vi.fn()
const mockRetry = vi.fn()
const mockAddTag = vi.fn()
const mockRemoveTag = vi.fn()

let mockProjectState: Record<string, unknown> = {}

vi.mock('../stores/projectStore', () => {
  const fn = (selector?: (s: Record<string, unknown>) => unknown) =>
    selector ? selector(mockProjectState) : mockProjectState
  fn.getState = () => mockProjectState
  fn.subscribe = () => () => {}
  return { useProjectStore: fn }
})

vi.mock('./FilePicker', () => ({
  FilePicker: ({ isOpen }: { isOpen: boolean }) =>
    isOpen ? <div data-testid="file-picker">File Picker</div> : null,
}))

function resetState(overrides: Record<string, unknown> = {}) {
  mockProjectState = {
    task: null,
    state: 'none',
    start: mockStart,
    quickStart: mockQuickStart,
    queueTask: mockQueueTask,
    loading: false,
    error: null,
    connected: true,
    connecting: false,
    undo: mockUndo,
    reset: mockReset,
    retry: mockRetry,
    phaseError: null,
    needsRecovery: null,
    ciFixStatus: null,
    tags: [],
    addTag: mockAddTag,
    removeTag: mockRemoveTag,
    client: { call: vi.fn() },
    ...overrides,
  }
}

describe('TaskWidget', () => {
  beforeEach(() => {
    resetState()
    vi.clearAllMocks()
  })

  // --- Load Task view (no active task) ---

  it('renders Load Task heading when no task', () => {
    const { getAllByText } = render(<TaskWidget />)
    // "Load Task" appears in heading and button
    expect(getAllByText('Load Task').length).toBeGreaterThanOrEqual(1)
  })

  it('shows Quick Task, From File, From URL tabs', () => {
    const { getByText } = render(<TaskWidget />)
    expect(getByText('Quick Task')).toBeInTheDocument()
    expect(getByText('From File')).toBeInTheDocument()
    expect(getByText('From URL')).toBeInTheDocument()
  })

  it('shows textarea in Quick Task tab', () => {
    const { getByPlaceholderText } = render(<TaskWidget />)
    expect(getByPlaceholderText(/Describe what you want to work on/)).toBeInTheDocument()
  })

  it('shows Load Task button', () => {
    const { getByRole } = render(<TaskWidget />)
    // The button is disabled by default (empty description)
    const btn = getByRole('button', { name: 'Load Task' })
    expect(btn).toBeInTheDocument()
  })

  it('shows Connected status when connected', () => {
    const { getByTestId } = render(<TaskWidget />)
    expect(getByTestId('task-connection-status').textContent).toBe('Connected')
  })

  it('shows Not connected status when disconnected', () => {
    resetState({ connected: false })
    const { getByTestId } = render(<TaskWidget />)
    expect(getByTestId('task-connection-status').textContent).toBe('Not connected')
  })

  it('shows Connecting status when connecting', () => {
    resetState({ connecting: true })
    const { getByTestId } = render(<TaskWidget />)
    expect(getByTestId('task-connection-status').textContent).toBe('Connecting to worktree...')
  })

  it('disables Load Task button when not connected', () => {
    resetState({ connected: false })
    const { getByRole } = render(<TaskWidget />)
    const btn = getByRole('button', { name: 'Load Task' })
    expect(btn).toBeDisabled()
  })

  it('disables Load Task button when description is empty', () => {
    const { getByRole } = render(<TaskWidget />)
    const btn = getByRole('button', { name: 'Load Task' })
    expect(btn).toBeDisabled()
  })

  it('calls start when Load Task is clicked with description', () => {
    const { getByPlaceholderText, getByRole } = render(<TaskWidget />)
    const textarea = getByPlaceholderText(/Describe what you want to work on/)
    fireEvent.change(textarea, { target: { value: 'Fix the bug' } })
    getByRole('button', { name: 'Load Task' }).click()
    expect(mockStart).toHaveBeenCalledWith('empty:Fix the bug')
  })

  it('switches to From URL tab', () => {
    const { getByRole, getByPlaceholderText } = render(<TaskWidget />)
    fireEvent.click(getByRole('tab', { name: 'From URL' }))
    expect(getByPlaceholderText('github.com/owner/repo/issues/123')).toBeInTheDocument()
  })

  it('switches to From File tab', () => {
    const { getByRole, getByText } = render(<TaskWidget />)
    fireEvent.click(getByRole('tab', { name: 'From File' }))
    expect(getByText('Browse')).toBeInTheDocument()
    expect(getByText('No file selected')).toBeInTheDocument()
  })

  it('shows Quick Fix toggle', () => {
    const { getByText } = render(<TaskWidget />)
    expect(getByText('Quick Fix')).toBeInTheDocument()
  })

  it('shows error message when error exists', () => {
    resetState({ error: 'Something failed' })
    const { getByText } = render(<TaskWidget />)
    expect(getByText('Something failed')).toBeInTheDocument()
  })

  it('detects GitHub provider in URL input', () => {
    const { getByRole, getByText, getByPlaceholderText } = render(<TaskWidget />)
    fireEvent.click(getByRole('tab', { name: 'From URL' }))
    const urlInput = getByPlaceholderText('github.com/owner/repo/issues/123')
    fireEvent.change(urlInput, { target: { value: 'https://github.com/owner/repo/issues/1' } })
    expect(getByText('GitHub detected')).toBeInTheDocument()
  })

  it('detects GitLab provider in URL input', () => {
    const { getByRole, getByText, getByPlaceholderText } = render(<TaskWidget />)
    fireEvent.click(getByRole('tab', { name: 'From URL' }))
    const urlInput = getByPlaceholderText('github.com/owner/repo/issues/123')
    fireEvent.change(urlInput, { target: { value: 'https://gitlab.com/group/project/-/issues/1' } })
    expect(getByText('GitLab detected')).toBeInTheDocument()
  })

  it('shows task title and source when task is active', () => {
    resetState({
      task: { title: 'Existing Task', source: 'empty:test', description: '' },
      state: 'implementing',
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText('Existing Task')).toBeInTheDocument()
    expect(getByText('empty:test')).toBeInTheDocument()
  })

  // --- Task loaded view ---

  it('shows Current Task heading when task exists', () => {
    resetState({
      task: { title: 'My Task', source: 'github:issue/1', description: 'Do the thing', branch: 'feat/task-1' },
      state: 'loaded',
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText('Current Task')).toBeInTheDocument()
    expect(getByText('My Task')).toBeInTheDocument()
    expect(getByText('github:issue/1')).toBeInTheDocument()
  })

  it('shows state badge', () => {
    resetState({
      task: { title: 'Task', source: 's', description: '' },
      state: 'implementing',
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText('implementing')).toBeInTheDocument()
  })

  it('shows description when available', () => {
    resetState({
      task: { title: 'T', source: 's', description: 'The description text' },
      state: 'loaded',
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText('The description text')).toBeInTheDocument()
  })

  it('shows branch info', () => {
    resetState({
      task: { title: 'T', source: 's', description: '', branch: 'feat/my-branch' },
      state: 'loaded',
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText('feat/my-branch')).toBeInTheDocument()
  })

  it('shows isolated badge for worktree tasks', () => {
    resetState({
      task: { title: 'T', source: 's', description: '', branch: 'b', worktreePath: '/tmp/wt' },
      state: 'loaded',
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText('isolated')).toBeInTheDocument()
  })

  it('shows tags and add tag button', () => {
    resetState({
      task: { title: 'T', source: 's', description: '' },
      state: 'loaded',
      tags: ['bug', 'urgent'],
    })
    const { getByText, getByRole } = render(<TaskWidget />)
    expect(getByText('bug')).toBeInTheDocument()
    expect(getByText('urgent')).toBeInTheDocument()
    expect(getByRole('button', { name: 'Add tag' })).toBeInTheDocument()
  })

  it('shows tag removal buttons', () => {
    resetState({
      task: { title: 'T', source: 's', description: '' },
      state: 'loaded',
      tags: ['mytag'],
    })
    const { getByRole } = render(<TaskWidget />)
    const removeBtn = getByRole('button', { name: 'Remove tag mytag' })
    removeBtn.click()
    expect(mockRemoveTag).toHaveBeenCalledWith('mytag')
  })

  it('shows recovery banner when needsRecovery is set', () => {
    resetState({
      task: { title: 'T', source: 's', description: '' },
      state: 'loaded',
      needsRecovery: 'implementing',
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText(/Task was interrupted during/)).toBeInTheDocument()
    expect(getByText('implementing')).toBeInTheDocument()
  })

  it('shows failed state recovery hints', () => {
    resetState({
      task: { title: 'T', source: 's', description: '' },
      state: 'failed',
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText(/Task encountered an error/)).toBeInTheDocument()
    expect(getByText('Undo')).toBeInTheDocument()
    expect(getByText('Reset')).toBeInTheDocument()
  })

  it('calls undo when Undo is clicked in failed state', () => {
    resetState({
      task: { title: 'T', source: 's', description: '' },
      state: 'failed',
    })
    const { getByText } = render(<TaskWidget />)
    getByText('Undo').click()
    expect(mockUndo).toHaveBeenCalled()
  })

  it('calls reset when Reset is clicked in failed state', () => {
    resetState({
      task: { title: 'T', source: 's', description: '' },
      state: 'failed',
    })
    const { getByText } = render(<TaskWidget />)
    getByText('Reset').click()
    expect(mockReset).toHaveBeenCalled()
  })

  it('shows CI fix status when active', () => {
    resetState({
      task: { title: 'T', source: 's', description: '' },
      state: 'loaded',
      ciFixStatus: { active: true, attempt: 2, maxAttempts: 3 },
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText(/CI Fix: attempt 2\/3/)).toBeInTheDocument()
  })

  it('shows CI fix passed badge', () => {
    resetState({
      task: { title: 'T', source: 's', description: '' },
      state: 'loaded',
      ciFixStatus: { active: false, result: 'success' },
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText('CI Fix: passed')).toBeInTheDocument()
  })

  it('shows CI fix failed badge', () => {
    resetState({
      task: { title: 'T', source: 's', description: '' },
      state: 'loaded',
      ciFixStatus: { active: false, result: 'failed' },
    })
    const { getByText } = render(<TaskWidget />)
    expect(getByText('CI Fix: failed')).toBeInTheDocument()
  })

  it('renders as plain div when embedded (no card wrapper)', () => {
    const { container } = render(<TaskWidget embedded />)
    // Should not have card wrapper
    expect(container.querySelector('.card')).not.toBeInTheDocument()
    // But should still show the tab content
    expect(container.querySelector('[role="tablist"]')).toBeInTheDocument()
  })

  it('shows context items when task has them', () => {
    resetState({
      task: { title: 'T', source: 's', description: '', contextItems: [{ type: 'file', ref: 'main.go', label: 'main.go' }] },
      state: 'loaded',
    })
    const { container } = render(<TaskWidget />)
    // The badge renders icon + text across child nodes — check container text
    expect(container.textContent).toContain('main.go')
  })
})
