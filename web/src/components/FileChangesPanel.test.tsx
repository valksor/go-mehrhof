import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { FileChangesPanel } from './FileChangesPanel'

const mockGetGitDiffAgainst = vi.fn()
const mockOpenTab = vi.fn()

let mockProjectState: Record<string, unknown> = {}
let mockLayoutState: Record<string, unknown> = {}

vi.mock('../stores/projectStore', () => ({
  useProjectStore: (selector: (s: Record<string, unknown>) => unknown) => selector(mockProjectState),
}))

vi.mock('../stores/layoutStore', () => ({
  useLayoutStore: (selector: (s: Record<string, unknown>) => unknown) => selector(mockLayoutState),
}))

const makeChange = (path: string, status: 'added' | 'modified' | 'deleted' | 'renamed') => ({ path, status })

describe('FileChangesPanel', () => {
  beforeEach(() => {
    mockGetGitDiffAgainst.mockReset()
    mockOpenTab.mockReset()
    mockProjectState = {
      fileChanges: [],
      getGitDiffAgainst: mockGetGitDiffAgainst,
    }
    mockLayoutState = {
      openTab: mockOpenTab,
    }
  })

  it('renders empty state when no file changes', () => {
    const { getByText } = render(<FileChangesPanel />)
    expect(getByText('No file changes')).toBeInTheDocument()
  })

  it('shows file count header', () => {
    mockProjectState.fileChanges = [makeChange('src/main.ts', 'modified')]
    const { getByText } = render(<FileChangesPanel />)
    expect(getByText('1 Files Changed')).toBeInTheDocument()
  })

  it('shows multiple file count', () => {
    mockProjectState.fileChanges = [
      makeChange('src/a.ts', 'added'),
      makeChange('src/b.ts', 'modified'),
      makeChange('src/c.ts', 'deleted'),
    ]
    const { getByText } = render(<FileChangesPanel />)
    expect(getByText('3 Files Changed')).toBeInTheDocument()
  })

  it('groups files by status with badges', () => {
    mockProjectState.fileChanges = [
      makeChange('src/new.ts', 'added'),
      makeChange('src/edit.ts', 'modified'),
      makeChange('src/old.ts', 'deleted'),
      makeChange('src/move.ts', 'renamed'),
    ]
    const { getByText } = render(<FileChangesPanel />)
    expect(getByText('+1')).toBeInTheDocument()
    expect(getByText('~1')).toBeInTheDocument()
    expect(getByText('-1')).toBeInTheDocument()
    expect(getByText('1 renamed')).toBeInTheDocument()
  })

  it('renders section headers for file groups', () => {
    mockProjectState.fileChanges = [
      makeChange('src/new.ts', 'added'),
      makeChange('src/edit.ts', 'modified'),
    ]
    const { getByText } = render(<FileChangesPanel />)
    expect(getByText('Added')).toBeInTheDocument()
    expect(getByText('Modified')).toBeInTheDocument()
  })

  it('renders file name and directory separately', () => {
    mockProjectState.fileChanges = [makeChange('src/components/App.tsx', 'modified')]
    const { getByText } = render(<FileChangesPanel />)
    expect(getByText('App.tsx')).toBeInTheDocument()
    expect(getByText('src/components/')).toBeInTheDocument()
  })

  it('calls openTab when a file is clicked', () => {
    mockProjectState.fileChanges = [makeChange('src/main.ts', 'modified')]
    const { getByText } = render(<FileChangesPanel />)
    getByText('main.ts').closest('button')?.click()
    expect(mockOpenTab).toHaveBeenCalledWith(expect.objectContaining({
      id: 'diff-src/main.ts',
      type: 'diff',
      title: 'main.ts',
    }))
  })

  it('has a Diff stat button', () => {
    mockProjectState.fileChanges = [makeChange('src/main.ts', 'modified')]
    const { getByLabelText } = render(<FileChangesPanel />)
    expect(getByLabelText('Compare against base commit')).toBeInTheDocument()
  })

  it('uses fileChanges from data prop when provided', () => {
    const dataChanges = [makeChange('from/prop.ts', 'added')]
    const { getByText } = render(<FileChangesPanel data={{ fileChanges: dataChanges }} />)
    expect(getByText('1 Files Changed')).toBeInTheDocument()
    expect(getByText('prop.ts')).toBeInTheDocument()
  })

  it('does not show Deleted section when no deleted files', () => {
    mockProjectState.fileChanges = [makeChange('src/a.ts', 'added')]
    const { queryByText } = render(<FileChangesPanel />)
    expect(queryByText('Deleted')).not.toBeInTheDocument()
  })
})
