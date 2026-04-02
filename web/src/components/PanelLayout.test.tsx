import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { PanelLayout } from './PanelLayout'

const mockToggleBottomPanel = vi.fn()
const mockOpenTab = vi.fn()
const mockSetActiveTab = vi.fn()
const mockSetPanelSize = vi.fn()
const mockClearOutput = vi.fn()

let mockLayoutState: Record<string, unknown> = {}
let mockProjectState: Record<string, unknown> = {}

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

// Mock child components to avoid deep rendering
vi.mock('./TabBar', () => ({
  TabBar: () => <div data-testid="tab-bar">TabBar</div>,
}))

vi.mock('./TabPanel', () => ({
  TabPanel: () => <div data-testid="tab-panel">TabPanel</div>,
}))

describe('PanelLayout', () => {
  beforeEach(() => {
    mockToggleBottomPanel.mockReset()
    mockOpenTab.mockReset()
    mockSetActiveTab.mockReset()
    mockSetPanelSize.mockReset()
    mockClearOutput.mockReset()
    mockLayoutState = {
      panelSizes: { left: 20, right: 20 },
      setPanelSize: mockSetPanelSize,
      bottomPanelVisible: false,
      toggleBottomPanel: mockToggleBottomPanel,
      openTab: mockOpenTab,
      setActiveTab: mockSetActiveTab,
    }
    mockProjectState = {
      output: [],
      clearOutput: mockClearOutput,
    }
  })

  it('renders left and right content', () => {
    const { getByText } = render(
      <PanelLayout
        leftContent={<div>Left Panel</div>}
        rightContent={<div>Right Panel</div>}
      />,
    )
    expect(getByText('Left Panel')).toBeInTheDocument()
    expect(getByText('Right Panel')).toBeInTheDocument()
  })

  it('renders header when provided', () => {
    const { getByText } = render(
      <PanelLayout
        header={<div>Header Content</div>}
        leftContent={<div>Left</div>}
        rightContent={<div>Right</div>}
      />,
    )
    expect(getByText('Header Content')).toBeInTheDocument()
  })

  it('does not render header when not provided', () => {
    const { queryByText } = render(
      <PanelLayout
        leftContent={<div>Left</div>}
        rightContent={<div>Right</div>}
      />,
    )
    expect(queryByText('Header Content')).not.toBeInTheDocument()
  })

  it('renders TabBar and TabPanel', () => {
    const { getByTestId } = render(
      <PanelLayout
        leftContent={<div>Left</div>}
        rightContent={<div>Right</div>}
      />,
    )
    expect(getByTestId('tab-bar')).toBeInTheDocument()
    expect(getByTestId('tab-panel')).toBeInTheDocument()
  })

  it('renders resize handles with slider role', () => {
    const { getByLabelText } = render(
      <PanelLayout
        leftContent={<div>Left</div>}
        rightContent={<div>Right</div>}
      />,
    )
    expect(getByLabelText('Resize left sidebar')).toBeInTheDocument()
    expect(getByLabelText('Resize right sidebar')).toBeInTheDocument()
  })

  it('shows "Show Output Panel" button when bottom panel hidden', () => {
    mockLayoutState.bottomPanelVisible = false
    const { getByText } = render(
      <PanelLayout
        leftContent={<div>Left</div>}
        rightContent={<div>Right</div>}
      />,
    )
    expect(getByText('Show Output Panel')).toBeInTheDocument()
  })

  it('shows collapse button when bottom panel is visible', () => {
    mockLayoutState.bottomPanelVisible = true
    const { getByLabelText } = render(
      <PanelLayout
        leftContent={<div>Left</div>}
        rightContent={<div>Right</div>}
      />,
    )
    expect(getByLabelText('Collapse output panel')).toBeInTheDocument()
  })

  it('renders mobile navigation bar with tab buttons', () => {
    const { getByText } = render(
      <PanelLayout
        leftContent={<div>Left</div>}
        rightContent={<div>Right</div>}
      />,
    )
    expect(getByText('Task')).toBeInTheDocument()
    expect(getByText('Chat')).toBeInTheDocument()
    expect(getByText('Actions')).toBeInTheDocument()
    expect(getByText('More')).toBeInTheDocument()
  })

  it('renders left and right sidebar with aria labels', () => {
    const { getByLabelText } = render(
      <PanelLayout
        leftContent={<div>Left</div>}
        rightContent={<div>Right</div>}
      />,
    )
    expect(getByLabelText('Left sidebar')).toBeInTheDocument()
    expect(getByLabelText('Right sidebar')).toBeInTheDocument()
  })
})
