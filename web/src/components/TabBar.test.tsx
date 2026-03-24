import { render, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { TabBar } from './TabBar'

const mockSetActiveTab = vi.fn()
const mockCloseTab = vi.fn()
const mockOpenTab = vi.fn()

vi.mock('../stores/layoutStore', () => ({
  useLayoutStore: () => ({
    tabs: [
      { id: 'tab-1', type: 'chat', title: 'Chat', closeable: true },
      { id: 'tab-2', type: 'spec', title: 'Spec', closeable: false },
    ],
    activeTabId: 'tab-1',
    setActiveTab: mockSetActiveTab,
    closeTab: mockCloseTab,
    openTab: mockOpenTab,
  }),
}))

beforeEach(() => {
  vi.clearAllMocks()
})

describe('TabBar accessibility', () => {
  it('has role="tablist" on the tab container', () => {
    const { getByRole } = render(<TabBar />)
    expect(getByRole('tablist')).toBeInTheDocument()
  })

  it('tab buttons have role="tab"', () => {
    const { getAllByRole } = render(<TabBar />)
    expect(getAllByRole('tab')).toHaveLength(2)
  })

  it('active tab has aria-selected="true"', () => {
    const { getAllByRole } = render(<TabBar />)
    expect(getAllByRole('tab')[0]).toHaveAttribute('aria-selected', 'true')
  })

  it('inactive tab has aria-selected="false"', () => {
    const { getAllByRole } = render(<TabBar />)
    expect(getAllByRole('tab')[1]).toHaveAttribute('aria-selected', 'false')
  })

  it('add-tab button has aria-label', () => {
    const { getByRole } = render(<TabBar />)
    expect(getByRole('button', { name: /add new tab/i })).toBeInTheDocument()
  })

  it('add-tab dropdown has aria-expanded', () => {
    const { getByRole } = render(<TabBar />)
    const btn = getByRole('button', { name: /add new tab/i })
    expect(btn).toHaveAttribute('aria-expanded', 'false')
  })

  it('tab SVG icons are aria-hidden', () => {
    const { getAllByRole } = render(<TabBar />)
    const tabs = getAllByRole('tab')
    tabs.forEach((tab: HTMLElement) => {
      const svgs = tab.querySelectorAll('svg')
      svgs.forEach((svg: SVGSVGElement) => {
        expect(svg).toHaveAttribute('aria-hidden', 'true')
      })
    })
  })
})

describe('TabBar interactions', () => {
  it('clicking a tab calls setActiveTab', () => {
    const { getAllByRole } = render(<TabBar />)
    const tabs = getAllByRole('tab')
    fireEvent.click(tabs[1])
    expect(mockSetActiveTab).toHaveBeenCalledWith('tab-2')
  })

  it('clicking close button calls closeTab', () => {
    const { getAllByRole } = render(<TabBar />)
    const tabs = getAllByRole('tab')
    // First tab is closeable — find the close button inside it
    const closeButton = tabs[0].querySelector('button')
    expect(closeButton).toBeInTheDocument()
    fireEvent.click(closeButton!)
    expect(mockCloseTab).toHaveBeenCalledWith('tab-1')
  })

  it('non-closeable tab has no close button', () => {
    const { getAllByRole } = render(<TabBar />)
    const tabs = getAllByRole('tab')
    // tab-2 has closeable: false — should have no nested button
    const buttons = tabs[1].querySelectorAll('button')
    expect(buttons.length).toBe(0)
  })

  it('clicking add-tab button toggles dropdown', () => {
    const { getByRole } = render(<TabBar />)
    const addBtn = getByRole('button', { name: /add new tab/i })
    fireEvent.click(addBtn)
    expect(addBtn).toHaveAttribute('aria-expanded', 'true')
  })

  it('selecting from add-tab dropdown calls openTab', () => {
    const { getByRole, getAllByRole } = render(<TabBar />)
    const addBtn = getByRole('button', { name: /add new tab/i })
    fireEvent.click(addBtn)
    // Click a menu item — use role="menuitem" to avoid matching tab labels
    const menuItems = getAllByRole('menuitem')
    expect(menuItems.length).toBeGreaterThan(0)
    fireEvent.click(menuItems[0]) // First option (Chat)
    expect(mockOpenTab).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'chat',
        title: 'Chat',
        closeable: true,
      })
    )
  })

  it('arrow keys navigate between tabs', () => {
    const { getAllByRole } = render(<TabBar />)
    const tabs = getAllByRole('tab')
    tabs[0].focus()
    fireEvent.keyDown(tabs[0], { key: 'ArrowRight' })
    // The second tab should now be focused
    expect(document.activeElement).toBe(tabs[1])
  })
})
