import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useLayoutStore, type Tab } from './layoutStore'
import { useProjectStore } from './projectStore'
import { useViewModeStore } from './viewModeStore'

const projectStoreBaseState = {
  task: null,
  state: 'none',
  fileChanges: [],
  reviews: [],
}

function setProjectWorkflowState(overrides: Record<string, unknown>) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- test helper: partial state overrides bypass strict store types
  useProjectStore.setState({ ...projectStoreBaseState, ...overrides } as any)
}

describe('layoutStore', () => {
  beforeEach(() => {
    // Reset to default state
    useLayoutStore.getState().resetLayout()
    useProjectStore.setState(projectStoreBaseState as any) // eslint-disable-line @typescript-eslint/no-explicit-any
    useViewModeStore.setState({ mode: 'developer', isFirstVisit: false })
    vi.clearAllMocks()
  })

  describe('initial state', () => {
    it('has default panels configuration', () => {
      const { panels } = useLayoutStore.getState()
      expect(panels.left).toEqual(['task', 'files'])
      expect(panels.right).toEqual(['checkpoints'])
      expect(panels.bottom).toEqual(['output'])
      expect(panels.main).toEqual([])
    })

    it('has all widgets visible and not collapsed by default', () => {
      const { widgetStates } = useLayoutStore.getState()
      expect(widgetStates.task).toEqual({ collapsed: false, visible: true })
      expect(widgetStates.files).toEqual({ collapsed: false, visible: true })
      expect(widgetStates.output).toEqual({ collapsed: false, visible: true })
      expect(widgetStates.checkpoints).toEqual({ collapsed: false, visible: true })
      expect(widgetStates.chat).toEqual({ collapsed: false, visible: true })
      expect(widgetStates.agents).toEqual({ collapsed: false, visible: true })
    })

    it('has default chat tab', () => {
      const { tabs, activeTabId } = useLayoutStore.getState()
      expect(tabs).toHaveLength(1)
      expect(tabs[0]).toEqual({
        id: 'chat-default',
        type: 'chat',
        title: 'Chat',
        closeable: false,
      })
      expect(activeTabId).toBe('chat-default')
    })

    it('has default panel sizes', () => {
      const { panelSizes } = useLayoutStore.getState()
      expect(panelSizes).toEqual({ left: 25, right: 25, bottom: 30 })
    })

    it('has bottom panel visible', () => {
      expect(useLayoutStore.getState().bottomPanelVisible).toBe(true)
    })
  })

  describe('widget actions', () => {
    describe('toggleWidgetCollapsed', () => {
      it('toggles widget collapsed state to true', () => {
        useLayoutStore.getState().toggleWidgetCollapsed('task')
        expect(useLayoutStore.getState().widgetStates.task.collapsed).toBe(true)
      })

      it('toggles widget collapsed state back to false', () => {
        useLayoutStore.getState().toggleWidgetCollapsed('task')
        useLayoutStore.getState().toggleWidgetCollapsed('task')
        expect(useLayoutStore.getState().widgetStates.task.collapsed).toBe(false)
      })

      it('preserves visible state when toggling', () => {
        useLayoutStore.getState().toggleWidgetCollapsed('task')
        expect(useLayoutStore.getState().widgetStates.task.visible).toBe(true)
      })

      it('does not affect other widgets', () => {
        useLayoutStore.getState().toggleWidgetCollapsed('task')
        expect(useLayoutStore.getState().widgetStates.files.collapsed).toBe(false)
      })
    })

    describe('setWidgetVisible', () => {
      it('sets widget visibility to false', () => {
        useLayoutStore.getState().setWidgetVisible('task', false)
        expect(useLayoutStore.getState().widgetStates.task.visible).toBe(false)
      })

      it('sets widget visibility to true', () => {
        useLayoutStore.getState().setWidgetVisible('task', false)
        useLayoutStore.getState().setWidgetVisible('task', true)
        expect(useLayoutStore.getState().widgetStates.task.visible).toBe(true)
      })

      it('preserves collapsed state', () => {
        useLayoutStore.getState().toggleWidgetCollapsed('task')
        useLayoutStore.getState().setWidgetVisible('task', false)
        expect(useLayoutStore.getState().widgetStates.task.collapsed).toBe(true)
      })
    })

    describe('moveWidget', () => {
      it('moves widget from one panel to another', () => {
        useLayoutStore.getState().moveWidget('task', 'right')
        const { panels } = useLayoutStore.getState()
        expect(panels.left).not.toContain('task')
        expect(panels.right).toContain('task')
      })

      it('removes widget from source panel', () => {
        useLayoutStore.getState().moveWidget('task', 'right')
        expect(useLayoutStore.getState().panels.left).toEqual(['files'])
      })

      it('appends widget to end by default', () => {
        useLayoutStore.getState().moveWidget('task', 'right')
        const { panels } = useLayoutStore.getState()
        expect(panels.right).toEqual(['checkpoints', 'task'])
      })

      it('inserts widget at specified index', () => {
        useLayoutStore.getState().moveWidget('task', 'right', 1)
        const { panels } = useLayoutStore.getState()
        expect(panels.right).toEqual(['checkpoints', 'task'])
      })

      it('inserts at beginning when index is 0', () => {
        useLayoutStore.getState().moveWidget('task', 'right', 0)
        const { panels } = useLayoutStore.getState()
        expect(panels.right).toEqual(['task', 'checkpoints'])
      })

      it('handles moving to empty panel', () => {
        useLayoutStore.getState().moveWidget('task', 'main')
        expect(useLayoutStore.getState().panels.main).toEqual(['task'])
      })
    })

    describe('reorderWidgets', () => {
      it('reorders widgets in a panel', () => {
        useLayoutStore.getState().reorderWidgets('left', ['files', 'task'])
        expect(useLayoutStore.getState().panels.left).toEqual(['files', 'task'])
      })

      it('does not affect other panels', () => {
        useLayoutStore.getState().reorderWidgets('left', ['files', 'task'])
        expect(useLayoutStore.getState().panels.right).toEqual(['checkpoints'])
      })
    })
  })

  describe('tab actions', () => {
    describe('openTab', () => {
      it('adds new tab', () => {
        const newTab: Tab = {
          id: 'test-tab',
          type: 'file',
          title: 'Test File',
          closeable: true,
        }
        useLayoutStore.getState().openTab(newTab)
        const { tabs } = useLayoutStore.getState()
        expect(tabs).toHaveLength(2)
        expect(tabs[1]).toEqual(newTab)
      })

      it('sets new tab as active', () => {
        useLayoutStore.getState().openTab({
          id: 'test-tab',
          type: 'file',
          title: 'Test',
        })
        expect(useLayoutStore.getState().activeTabId).toBe('test-tab')
      })

      it('does not duplicate existing tab', () => {
        const tab: Tab = { id: 'test-tab', type: 'file', title: 'Test' }
        useLayoutStore.getState().openTab(tab)
        useLayoutStore.getState().openTab(tab)
        expect(useLayoutStore.getState().tabs).toHaveLength(2) // default + test
      })

      it('activates existing tab when reopening', () => {
        useLayoutStore.getState().openTab({ id: 'tab1', type: 'file', title: 'Tab 1' })
        useLayoutStore.getState().openTab({ id: 'tab2', type: 'file', title: 'Tab 2' })
        expect(useLayoutStore.getState().activeTabId).toBe('tab2')

        useLayoutStore.getState().openTab({ id: 'tab1', type: 'file', title: 'Tab 1' })
        expect(useLayoutStore.getState().activeTabId).toBe('tab1')
      })
    })

    describe('closeTab', () => {
      beforeEach(() => {
        useLayoutStore.getState().openTab({ id: 'tab1', type: 'file', title: 'Tab 1', closeable: true })
        useLayoutStore.getState().openTab({ id: 'tab2', type: 'file', title: 'Tab 2', closeable: true })
      })

      it('removes tab from list', () => {
        useLayoutStore.getState().closeTab('tab1')
        const tabIds = useLayoutStore.getState().tabs.map((t) => t.id)
        expect(tabIds).not.toContain('tab1')
      })

      it('does not close non-closeable tabs', () => {
        useLayoutStore.getState().closeTab('chat-default')
        const tabIds = useLayoutStore.getState().tabs.map((t) => t.id)
        expect(tabIds).toContain('chat-default')
      })

      it('activates previous tab when closing active tab', () => {
        useLayoutStore.getState().setActiveTab('tab2')
        useLayoutStore.getState().closeTab('tab2')
        expect(useLayoutStore.getState().activeTabId).toBe('tab1')
      })

      it('activates next tab when closing first active tab', () => {
        useLayoutStore.getState().setActiveTab('chat-default')
        useLayoutStore.getState().openTab({ id: 'closeable', type: 'file', title: 'X', closeable: true })
        // tabs are now: chat-default, tab1, tab2, closeable
        // active is 'closeable'
        useLayoutStore.getState().closeTab('closeable')
        // Should activate tab2 (the one before)
        expect(useLayoutStore.getState().activeTabId).toBe('tab2')
      })

      it('sets activeTabId to null when all closeable tabs are closed', () => {
        // Close all closeable tabs
        useLayoutStore.getState().closeTab('tab1')
        useLayoutStore.getState().closeTab('tab2')
        // Only non-closeable chat-default remains
        expect(useLayoutStore.getState().tabs).toHaveLength(1)
        expect(useLayoutStore.getState().activeTabId).toBe('chat-default')
      })
    })

    describe('setActiveTab', () => {
      it('sets active tab', () => {
        useLayoutStore.getState().openTab({ id: 'tab1', type: 'file', title: 'Tab 1' })
        useLayoutStore.getState().setActiveTab('chat-default')
        expect(useLayoutStore.getState().activeTabId).toBe('chat-default')
      })
    })

    describe('reorderTabs', () => {
      it('reorders tabs', () => {
        useLayoutStore.getState().openTab({ id: 'tab1', type: 'file', title: 'Tab 1' })
        useLayoutStore.getState().openTab({ id: 'tab2', type: 'file', title: 'Tab 2' })

        const reordered: Tab[] = [
          { id: 'tab2', type: 'file', title: 'Tab 2' },
          { id: 'tab1', type: 'file', title: 'Tab 1' },
          { id: 'chat-default', type: 'chat', title: 'Chat', closeable: false },
        ]
        useLayoutStore.getState().reorderTabs(reordered)

        expect(useLayoutStore.getState().tabs.map((t) => t.id)).toEqual(['tab2', 'tab1', 'chat-default'])
      })
    })
  })

  describe('panel actions', () => {
    describe('setPanelSize', () => {
      it('sets left panel size', () => {
        useLayoutStore.getState().setPanelSize('left', 30)
        expect(useLayoutStore.getState().panelSizes.left).toBe(30)
      })

      it('sets right panel size', () => {
        useLayoutStore.getState().setPanelSize('right', 35)
        expect(useLayoutStore.getState().panelSizes.right).toBe(35)
      })

      it('sets bottom panel size', () => {
        useLayoutStore.getState().setPanelSize('bottom', 40)
        expect(useLayoutStore.getState().panelSizes.bottom).toBe(40)
      })

      it('clamps size to minimum of 10', () => {
        useLayoutStore.getState().setPanelSize('left', 5)
        expect(useLayoutStore.getState().panelSizes.left).toBe(10)
      })

      it('clamps size to maximum of 50', () => {
        useLayoutStore.getState().setPanelSize('left', 60)
        expect(useLayoutStore.getState().panelSizes.left).toBe(50)
      })

      it('preserves other panel sizes', () => {
        useLayoutStore.getState().setPanelSize('left', 30)
        expect(useLayoutStore.getState().panelSizes.right).toBe(25)
        expect(useLayoutStore.getState().panelSizes.bottom).toBe(30)
      })
    })

    describe('toggleBottomPanel', () => {
      it('toggles bottom panel to hidden', () => {
        useLayoutStore.getState().toggleBottomPanel()
        expect(useLayoutStore.getState().bottomPanelVisible).toBe(false)
      })

      it('toggles bottom panel back to visible', () => {
        useLayoutStore.getState().toggleBottomPanel()
        useLayoutStore.getState().toggleBottomPanel()
        expect(useLayoutStore.getState().bottomPanelVisible).toBe(true)
      })
    })
  })

  describe('resetLayout', () => {
    it('resets panels to default', () => {
      useLayoutStore.getState().moveWidget('task', 'right')
      useLayoutStore.getState().resetLayout()
      expect(useLayoutStore.getState().panels.left).toEqual(['task', 'files'])
    })

    it('resets widget states to default', () => {
      useLayoutStore.getState().toggleWidgetCollapsed('task')
      useLayoutStore.getState().setWidgetVisible('files', false)
      useLayoutStore.getState().resetLayout()
      expect(useLayoutStore.getState().widgetStates.task.collapsed).toBe(false)
      expect(useLayoutStore.getState().widgetStates.files.visible).toBe(true)
    })

    it('resets tabs to default', () => {
      useLayoutStore.getState().openTab({ id: 'test', type: 'file', title: 'Test' })
      useLayoutStore.getState().resetLayout()
      expect(useLayoutStore.getState().tabs).toHaveLength(1)
      expect(useLayoutStore.getState().tabs[0].id).toBe('chat-default')
    })

    it('resets active tab to default', () => {
      useLayoutStore.getState().openTab({ id: 'test', type: 'file', title: 'Test' })
      useLayoutStore.getState().resetLayout()
      expect(useLayoutStore.getState().activeTabId).toBe('chat-default')
    })

    it('resets panel sizes to default', () => {
      useLayoutStore.getState().setPanelSize('left', 40)
      useLayoutStore.getState().resetLayout()
      expect(useLayoutStore.getState().panelSizes.left).toBe(25)
    })

    it('resets bottom panel visibility to true', () => {
      useLayoutStore.getState().toggleBottomPanel()
      useLayoutStore.getState().resetLayout()
      expect(useLayoutStore.getState().bottomPanelVisible).toBe(true)
    })
  })

  describe('persistence', () => {
    it('uses kvelmo-layout as storage key', () => {
      useLayoutStore.getState().toggleWidgetCollapsed('task')
      // eslint-disable-next-line @typescript-eslint/unbound-method -- localStorage is mocked in vitest
      expect(localStorage.setItem).toHaveBeenCalledWith(
        'kvelmo-layout',
        expect.any(String)
      )
    })
  })

  describe('modalCommand', () => {
    it('starts with null modalCommand', () => {
      expect(useLayoutStore.getState().modalCommand).toBeNull()
    })

    it('setModalCommand sets the command', () => {
      useLayoutStore.getState().setModalCommand('submit')
      expect(useLayoutStore.getState().modalCommand).toBe('submit')
    })

    it('setModalCommand can clear with null', () => {
      useLayoutStore.getState().setModalCommand('finish')
      useLayoutStore.getState().setModalCommand(null)
      expect(useLayoutStore.getState().modalCommand).toBeNull()
    })

    it('setModalCommand supports all modal types', () => {
      for (const modal of ['submit', 'finish', 'abandon', 'delete'] as const) {
        useLayoutStore.getState().setModalCommand(modal)
        expect(useLayoutStore.getState().modalCommand).toBe(modal)
      }
    })
  })

  describe('edge cases', () => {
    it('moveWidget to same panel preserves widget', () => {
      useLayoutStore.getState().moveWidget('task', 'left')
      // task is removed and re-added, so order may shift but content is the same
      expect(useLayoutStore.getState().panels.left).toContain('task')
    })

    it('multiple tabs can be opened and navigated', () => {
      useLayoutStore.getState().openTab({ id: 'a', type: 'file', title: 'A', closeable: true })
      useLayoutStore.getState().openTab({ id: 'b', type: 'diff', title: 'B', closeable: true })
      useLayoutStore.getState().openTab({ id: 'c', type: 'agent', title: 'C', closeable: true })
      expect(useLayoutStore.getState().tabs).toHaveLength(4) // default + 3
      expect(useLayoutStore.getState().activeTabId).toBe('c')

      useLayoutStore.getState().setActiveTab('a')
      expect(useLayoutStore.getState().activeTabId).toBe('a')

      useLayoutStore.getState().closeTab('a')
      // Should activate the tab before 'a' or the one after
      expect(useLayoutStore.getState().activeTabId).not.toBe('a')
      expect(useLayoutStore.getState().tabs).toHaveLength(3)
    })
  })

  describe('reactive workflow tabs', () => {
    it('opens a task tab when the project enters the loaded state', () => {
      setProjectWorkflowState({ state: 'none' })

      setProjectWorkflowState({
        state: 'loaded',
        task: { id: 'task-1', title: 'Ship it', state: 'loaded', source: 'file' },
      })

      expect(useLayoutStore.getState().tabs).toContainEqual(
        expect.objectContaining({ id: 'task-view', type: 'task', title: 'Ship it' })
      )
      expect(useLayoutStore.getState().activeTabId).toBe('task-view')
    })

    it('opens diff tabs and focuses the first file when implementation touches a few files', () => {
      setProjectWorkflowState({ state: 'none' })

      setProjectWorkflowState({
        state: 'implemented',
        fileChanges: [
          { path: 'pkg/one.go', status: 'modified' },
          { path: 'pkg/two.go', status: 'added' },
        ],
      })

      const { tabs, activeTabId } = useLayoutStore.getState()
      expect(tabs).toContainEqual(expect.objectContaining({ id: 'diff-pkg/one.go', type: 'diff', title: 'one.go' }))
      expect(tabs).toContainEqual(expect.objectContaining({ id: 'diff-pkg/two.go', type: 'diff', title: 'two.go' }))
      expect(activeTabId).toBe('diff-pkg/one.go')
    })

    it('opens a file changes summary tab when implementation touches many files', () => {
      setProjectWorkflowState({ state: 'none' })

      setProjectWorkflowState({
        state: 'implemented',
        fileChanges: [
          { path: 'a.ts', status: 'modified' },
          { path: 'b.ts', status: 'modified' },
          { path: 'c.ts', status: 'modified' },
          { path: 'd.ts', status: 'modified' },
        ],
      })

      expect(useLayoutStore.getState().tabs).toContainEqual(
        expect.objectContaining({ id: 'filechanges-view', type: 'filechanges', title: '4 Files Changed' })
      )
      expect(useLayoutStore.getState().activeTabId).toBe('filechanges-view')
    })

    it('opens a review tab on submitted state and returns focus to chat on failure', () => {
      setProjectWorkflowState({ state: 'none' })

      setProjectWorkflowState({
        state: 'submitted',
        reviews: [{ number: 1, timestamp: '2026-03-25T00:00:00Z', approved: true, message: 'Looks good' }],
      })

      expect(useLayoutStore.getState().activeTabId).toBe('review-view')

      setProjectWorkflowState({ state: 'failed' })

      expect(useLayoutStore.getState().activeTabId).toBe('chat-default')
    })

    it('suppresses reactive workflow tabs in simple mode after hydration', () => {
      useViewModeStore.setState({ mode: 'simple', isFirstVisit: false })
      setProjectWorkflowState({ state: 'none' })

      setProjectWorkflowState({
        state: 'planned',
        task: { id: 'task-2', title: 'Spec first', state: 'planned', source: 'file' },
      })

      expect(useLayoutStore.getState().tabs).toEqual([
        expect.objectContaining({ id: 'chat-default', type: 'chat' }),
      ])
      expect(useLayoutStore.getState().activeTabId).toBe('chat-default')
    })

    it('re-evaluates workflow tabs when switching from simple to developer mode', () => {
      useViewModeStore.setState({ mode: 'simple', isFirstVisit: false })
      setProjectWorkflowState({
        state: 'planned',
        task: { id: 'task-3', title: 'Plan task', state: 'planned', source: 'file' },
      })
      expect(useLayoutStore.getState().tabs).toHaveLength(1)

      useViewModeStore.getState().setMode('developer')

      expect(useLayoutStore.getState().tabs).toContainEqual(
        expect.objectContaining({ id: 'spec-view', type: 'spec', title: 'Specification' })
      )
      expect(useLayoutStore.getState().activeTabId).toBe('spec-view')
    })

    it('closes stale workflow tabs when the task state regresses', () => {
      setProjectWorkflowState({
        state: 'submitted',
        reviews: [{ number: 2, timestamp: '2026-03-25T00:00:00Z', approved: false, message: 'Needs work' }],
      })
      expect(useLayoutStore.getState().tabs).toContainEqual(expect.objectContaining({ id: 'review-view', type: 'review' }))

      setProjectWorkflowState({
        state: 'loaded',
        task: { id: 'task-4', title: 'Retry task', state: 'loaded', source: 'file' },
      })

      const tabIds = useLayoutStore.getState().tabs.map(tab => tab.id)
      expect(tabIds).toContain('task-view')
      expect(tabIds).not.toContain('review-view')
      expect(useLayoutStore.getState().activeTabId).toBe('task-view')
    })
  })
})
