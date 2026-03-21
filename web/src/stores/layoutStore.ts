import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { storeName } from '../meta'
import { useProjectStore } from './projectStore'
import { useViewModeStore, isViewModeHydrated } from './viewModeStore'

type TaskState =
  | 'none'
  | 'loaded'
  | 'planning'
  | 'planned'
  | 'implementing'
  | 'implemented'
  | 'simplifying'
  | 'optimizing'
  | 'reviewing'
  | 'submitted'
  | 'failed'
  | 'waiting'
  | 'paused'

// Widget configuration
export type WidgetId =
  | 'task'
  | 'files'
  | 'output'
  | 'checkpoints'
  | 'chat'
  | 'agents'

export type PanelId = 'left' | 'right' | 'bottom' | 'main'

export interface WidgetState {
  collapsed: boolean
  visible: boolean
}

// Tab configuration
export type TabType = 'file' | 'spec' | 'agent' | 'output' | 'chat' | 'diff' | 'screenshots' | 'jobs' | 'files' | 'browser' | 'task' | 'review' | 'filechanges'

export interface Tab {
  id: string
  type: TabType
  title: string
  icon?: string
  data?: Record<string, unknown>
  closeable?: boolean
}

// Panel sizes (percentages)
export interface PanelSizes {
  left: number
  right: number
  bottom: number
}

interface LayoutState {
  // Widget management
  panels: Record<PanelId, WidgetId[]>
  widgetStates: Record<WidgetId, WidgetState>

  // Tab management
  tabs: Tab[]
  activeTabId: string | null

  // Panel sizes
  panelSizes: PanelSizes
  bottomPanelVisible: boolean

  // Widget actions
  toggleWidgetCollapsed: (widgetId: WidgetId) => void
  setWidgetVisible: (widgetId: WidgetId, visible: boolean) => void
  moveWidget: (widgetId: WidgetId, toPanel: PanelId, index?: number) => void
  reorderWidgets: (panelId: PanelId, widgets: WidgetId[]) => void

  // Tab actions
  openTab: (tab: Tab) => void
  closeTab: (tabId: string) => void
  setActiveTab: (tabId: string) => void
  reorderTabs: (tabs: Tab[]) => void

  // Panel actions
  setPanelSize: (panelId: keyof PanelSizes, size: number) => void
  toggleBottomPanel: () => void

  // Modal command (triggered by chat slash commands)
  modalCommand: 'submit' | 'finish' | 'abandon' | 'delete' | null
  setModalCommand: (modal: 'submit' | 'finish' | 'abandon' | 'delete' | null) => void

  // Reset
  resetLayout: () => void
}

const DEFAULT_WIDGET_STATES: Record<WidgetId, WidgetState> = {
  task: { collapsed: false, visible: true },
  files: { collapsed: false, visible: true },
  output: { collapsed: false, visible: true },
  checkpoints: { collapsed: false, visible: true },
  chat: { collapsed: false, visible: true },
  agents: { collapsed: false, visible: true },
}

const DEFAULT_PANELS: Record<PanelId, WidgetId[]> = {
  left: ['task', 'files'],
  right: ['checkpoints'],
  bottom: ['output'],
  main: [],
}

const DEFAULT_PANEL_SIZES: PanelSizes = {
  left: 25,
  right: 25,
  bottom: 30,
}

const DEFAULT_TABS: Tab[] = [
  { id: 'chat-default', type: 'chat', title: 'Chat', closeable: false },
]

export const useLayoutStore = create<LayoutState>()(
  persist(
    (set) => ({
      // Initial state
      panels: DEFAULT_PANELS,
      widgetStates: DEFAULT_WIDGET_STATES,
      tabs: DEFAULT_TABS,
      activeTabId: 'chat-default',
      panelSizes: DEFAULT_PANEL_SIZES,
      bottomPanelVisible: true,
      modalCommand: null,

      // Modal command
      setModalCommand: (modal) => set({ modalCommand: modal }),

      // Widget actions
      toggleWidgetCollapsed: (widgetId) => {
        set((state) => ({
          widgetStates: {
            ...state.widgetStates,
            [widgetId]: {
              ...state.widgetStates[widgetId],
              collapsed: !state.widgetStates[widgetId].collapsed,
            },
          },
        }))
      },

      setWidgetVisible: (widgetId, visible) => {
        set((state) => ({
          widgetStates: {
            ...state.widgetStates,
            [widgetId]: {
              ...state.widgetStates[widgetId],
              visible,
            },
          },
        }))
      },

      moveWidget: (widgetId, toPanel, index) => {
        set((state) => {
          // Find current panel and remove widget from it
          const newPanels = { ...state.panels }

          for (const [panelId, widgets] of Object.entries(newPanels)) {
            const widgetIndex = widgets.indexOf(widgetId)
            if (widgetIndex !== -1) {
              newPanels[panelId as PanelId] = widgets.filter((w) => w !== widgetId)
              break
            }
          }

          // Add to new panel
          const targetWidgets = [...newPanels[toPanel]]
          if (index !== undefined && index >= 0) {
            targetWidgets.splice(index, 0, widgetId)
          } else {
            targetWidgets.push(widgetId)
          }
          newPanels[toPanel] = targetWidgets

          return { panels: newPanels }
        })
      },

      reorderWidgets: (panelId, widgets) => {
        set((state) => ({
          panels: {
            ...state.panels,
            [panelId]: widgets,
          },
        }))
      },

      // Tab actions
      openTab: (tab) => {
        set((state) => {
          // Check if tab already exists
          // Re-opening an existing tab activates it without updating data.
          // Stale tab cleanup on state regression (below) handles the common case.
          const existingTab = state.tabs.find((t) => t.id === tab.id)
          if (existingTab) {
            return { activeTabId: tab.id }
          }
          return {
            tabs: [...state.tabs, tab],
            activeTabId: tab.id,
          }
        })
      },

      closeTab: (tabId) => {
        set((state) => {
          const tab = state.tabs.find((t) => t.id === tabId)
          if (tab && tab.closeable === false) {
            return state // Don't close non-closeable tabs
          }

          const newTabs = state.tabs.filter((t) => t.id !== tabId)
          let newActiveTabId = state.activeTabId

          // If closing active tab, switch to another
          if (state.activeTabId === tabId && newTabs.length > 0) {
            const closedIndex = state.tabs.findIndex((t) => t.id === tabId)
            const newIndex = Math.min(closedIndex, newTabs.length - 1)
            newActiveTabId = newTabs[newIndex].id
          } else if (newTabs.length === 0) {
            newActiveTabId = null
          }

          return { tabs: newTabs, activeTabId: newActiveTabId }
        })
      },

      setActiveTab: (tabId) => {
        set({ activeTabId: tabId })
      },

      reorderTabs: (tabs) => {
        set({ tabs })
      },

      // Panel actions
      setPanelSize: (panelId, size) => {
        set((state) => ({
          panelSizes: {
            ...state.panelSizes,
            [panelId]: Math.max(10, Math.min(50, size)), // Clamp between 10-50%
          },
        }))
      },

      toggleBottomPanel: () => {
        set((state) => ({ bottomPanelVisible: !state.bottomPanelVisible }))
      },

      // Reset
      resetLayout: () => {
        set({
          panels: DEFAULT_PANELS,
          widgetStates: DEFAULT_WIDGET_STATES,
          tabs: DEFAULT_TABS,
          activeTabId: 'chat-default',
          panelSizes: DEFAULT_PANEL_SIZES,
          bottomPanelVisible: true,
        })
      },
    }),
    {
      name: storeName('layout'),
      version: 3, // Bumped: removed actions widget, chat-first UI
    }
  )
)

// Reactive tabs: Subscribe to projectStore state changes
// Initialize from current state to avoid triggering on page load
let prevTaskState: TaskState | null = useProjectStore.getState().state

// Track unsubscribe handle to prevent HMR double-subscribe
let unsubscribeReactiveTabs: (() => void) | null = null

function setupReactiveTabsSubscription() {
  // Clean up existing subscription (for HMR)
  if (unsubscribeReactiveTabs) {
    unsubscribeReactiveTabs()
  }

  unsubscribeReactiveTabs = useProjectStore.subscribe((projectState) => {
    const { state: taskState, task, fileChanges, reviews } = projectState

    // Only react to state changes
    if (taskState === prevTaskState) return
    prevTaskState = taskState

    const { openTab, setActiveTab, closeTab, tabs } = useLayoutStore.getState()

    // Only suppress reactive tabs in simple mode AFTER hydration.
    // Before hydration, fall through to developer behavior (the default).
    if (isViewModeHydrated() && useViewModeStore.getState().mode === 'simple') return

    // Close stale workflow tabs when state regresses (re-entry from submitted/implemented).
    // This prevents old review/diff tabs from lingering while re-planning.
    const staleTabTypes: Record<string, string[]> = {
      loaded: ['spec', 'diff', 'filechanges', 'review'],
      planning: ['diff', 'filechanges', 'review'],
      planned: ['diff', 'filechanges', 'review'],
      implementing: ['review'],
      simplifying: ['review'],
      optimizing: ['review'],
    }
    const typesToClose = staleTabTypes[taskState as string]
    if (typesToClose) {
      for (const tab of tabs) {
        if (tab.closeable && typesToClose.includes(tab.type)) {
          closeTab(tab.id)
        }
      }
    }

    switch (taskState) {
      case 'loaded':
        // Open Task tab when task is loaded
        if (task) {
          openTab({
            id: 'task-view',
            type: 'task',
            title: task.title || 'Task',
            data: { task },
            closeable: true,
          })
          setActiveTab('task-view')
        }
        break

      case 'planned':
        // Open Spec tab when planning completes — loads content from show.spec RPC
        openTab({
          id: 'spec-view',
          type: 'spec',
          title: 'Specification',
          data: { mode: 'spec' },
          closeable: true,
        })
        setActiveTab('spec-view')
        break

      case 'implemented':
        // Open diff tabs or file changes list based on file count
        if (fileChanges.length > 0 && fileChanges.length <= 3) {
          // Open all diff tabs first, then focus the first one
          const firstTabId = `diff-${fileChanges[0].path}`
          for (const fc of fileChanges) {
            const fileName = fc.path.split('/').pop() || fc.path
            openTab({
              id: `diff-${fc.path}`,
              type: 'diff',
              title: fileName,
              data: { path: fc.path, status: fc.status },
              closeable: true,
            })
          }
          // Focus first tab after all tabs are opened
          setActiveTab(firstTabId)
        } else if (fileChanges.length > 3) {
          // Open file changes list tab
          openTab({
            id: 'filechanges-view',
            type: 'filechanges',
            title: `${fileChanges.length} Files Changed`,
            data: { fileChanges },
            closeable: true,
          })
          setActiveTab('filechanges-view')
        }
        break

      case 'submitted':
        // Open Review tab after submission
        if (reviews.length > 0) {
          openTab({
            id: 'review-view',
            type: 'review',
            title: 'Review',
            data: { reviews },
            closeable: true,
          })
          setActiveTab('review-view')
        }
        break

      case 'failed': {
        // Focus Chat tab on failure so user sees error
        const chatTab = tabs.find(t => t.type === 'chat')
        if (chatTab) {
          setActiveTab(chatTab.id)
        }
        break
      }
    }
  })
}

// Initialize subscription
setupReactiveTabsSubscription()

// Subscribe to viewModeStore: on simple->developer switch, re-evaluate tabs
let unsubscribeViewMode: (() => void) | null = null

function handleViewModeChange(state: { mode: string }, prev: { mode: string }) {
  if (prev.mode === 'simple' && state.mode === 'developer') {
    // Reset tracker so reactive tabs fire on next state change
    prevTaskState = null
    // Re-read current taskState and trigger tab opening
    const { state: taskState, task, fileChanges, reviews } = useProjectStore.getState()
    const { openTab, tabs } = useLayoutStore.getState()
    // Only auto-focus if user hasn't opened any workflow tabs yet
    // (DEFAULT_TABS always includes the non-closeable chat tab, so length > 0)
    const noWorkflowTabs = tabs.every(t => t.type === 'chat')

    switch (taskState) {
      case 'loaded':
        if (task) {
          openTab({ id: 'task-view', type: 'task', title: task.title || 'Task', data: { task }, closeable: true })
          if (noWorkflowTabs) useLayoutStore.getState().setActiveTab('task-view')
        }
        break
      case 'planned':
        openTab({ id: 'spec-view', type: 'spec', title: 'Specification', data: { mode: 'spec' }, closeable: true })
        if (noWorkflowTabs) useLayoutStore.getState().setActiveTab('spec-view')
        break
      case 'implemented':
        if (fileChanges.length > 0 && fileChanges.length <= 3) {
          const firstTabId = `diff-${fileChanges[0].path}`
          for (const fc of fileChanges) {
            const fileName = fc.path.split('/').pop() || fc.path
            openTab({ id: `diff-${fc.path}`, type: 'diff', title: fileName, data: { path: fc.path, status: fc.status }, closeable: true })
          }
          if (noWorkflowTabs) useLayoutStore.getState().setActiveTab(firstTabId)
        } else if (fileChanges.length > 3) {
          openTab({ id: 'filechanges-view', type: 'filechanges', title: `${fileChanges.length} Files Changed`, data: { fileChanges }, closeable: true })
          if (noWorkflowTabs) useLayoutStore.getState().setActiveTab('filechanges-view')
        }
        break
      case 'submitted':
        if (reviews.length > 0) {
          openTab({ id: 'review-view', type: 'review', title: 'Review', data: { reviews }, closeable: true })
          if (noWorkflowTabs) useLayoutStore.getState().setActiveTab('review-view')
        }
        break
      case 'failed': {
        const chatTab = tabs.find(t => t.type === 'chat')
        if (chatTab) {
          useLayoutStore.getState().setActiveTab(chatTab.id)
        }
        break
      }
    }

    prevTaskState = taskState
  }
}

unsubscribeViewMode = useViewModeStore.subscribe(handleViewModeChange)

// Support HMR by cleaning up old subscriptions before module replacement
if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    if (unsubscribeReactiveTabs) unsubscribeReactiveTabs()
    if (unsubscribeViewMode) unsubscribeViewMode()
  })
  import.meta.hot.accept(() => {
    setupReactiveTabsSubscription()
    unsubscribeViewMode = useViewModeStore.subscribe(handleViewModeChange)
  })
}
