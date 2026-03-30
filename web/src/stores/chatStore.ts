import { create } from 'zustand'
import { useGlobalStore } from './globalStore'
import { useProjectStore } from './projectStore'
import { cliCmd } from '../meta'
import { chatEvents } from '../lib/events'

export type MessageRole = 'user' | 'assistant' | 'system' | 'subagent' | 'permission'
export type MessageStatus = 'pending' | 'streaming' | 'complete' | 'error'

// Subagent status for display
export type SubagentStatus = 'started' | 'completed' | 'failed'

// Danger level for permission requests
export type DangerLevel = 'safe' | 'caution' | 'dangerous'

export interface UIChatMessage {
  id: string
  role: MessageRole
  content: string
  timestamp: Date
  status: MessageStatus
  jobId?: string
  mentions?: string[]  // File paths mentioned with @
  actions?: Array<{
    id: string
    label: string
    type: 'approve' | 'reject' | 'custom'
  }>
  // Subagent info (when role='subagent')
  subagent?: {
    subagentId: string
    type: string
    description: string
    status: SubagentStatus
    duration?: number  // ms
    exitReason?: string
  }
  // Permission info (when role='permission')
  permission?: {
    requestId: string
    tool: string
    dangerLevel: DangerLevel
    dangerReason?: string
    approved?: boolean
  }
}

interface BackendMessage {
  id: string
  role: string
  content: string
  mentions?: string[]
  timestamp?: string
  job_id?: string
}

// ChatEvent is a streaming event pushed by the server for chat jobs
interface ChatEvent {
  type: string        // job_started, stream, job_completed, job_failed, subagent, permission
  job_id: string      // The job this event relates to
  content?: string    // Streaming content or message
  result?: string     // Final result on completion
  error?: string      // Error message on failure
  timestamp?: string
  // Subagent event fields
  subagent?: {
    id: string
    type: string
    description: string
    status: SubagentStatus
    duration?: number
    exit_reason?: string
  }
  // Permission event fields
  permission_request?: {
    id: string
    tool: string
    danger_level?: DangerLevel
    danger_reason?: string
  }
}

interface ChatState {
  messages: UIChatMessage[]
  isTyping: boolean
  error: string | null
  activeJobId: string | null
  taskId: string | null      // Current task ID for chat persistence
  isDisabled: boolean        // True when no active task

  // Actions
  sendMessage: (content: string, worktreeId?: string) => Promise<void>
  addMessage: (message: Omit<UIChatMessage, 'id' | 'timestamp'>) => string
  updateMessage: (id: string, updates: Partial<UIChatMessage>) => void
  appendToMessage: (id: string, content: string) => void
  setTyping: (typing: boolean) => void
  clearMessages: () => Promise<void>
  loadHistory: (worktreeId: string) => Promise<void>
  setTaskId: (taskId: string | null) => void
  handleAction: (messageId: string, actionId: string) => void
  cancelOperation: () => void
}

let messageIdCounter = 0
const generateId = () => `msg-${++messageIdCounter}-${Date.now()}`

// Module-scoped abort controller for cancellable agent operations
let activeAbort: AbortController | null = null

// Convert backend message to frontend format
const convertMessage = (msg: BackendMessage): UIChatMessage => ({
  id: msg.id || generateId(),
  role: msg.role as MessageRole,
  content: msg.content,
  timestamp: msg.timestamp ? new Date(msg.timestamp) : new Date(),
  status: 'complete',
  mentions: msg.mentions,
  jobId: msg.job_id
})

export const useChatStore = create<ChatState>((set, get) => ({
  messages: [],
  isTyping: false,
  error: null,
  activeJobId: null,
  taskId: null,
  isDisabled: true,  // Disabled by default until task is loaded

  loadHistory: async (worktreeId: string) => {
    const client = useGlobalStore.getState().client
    if (!client) return

    try {
      const result = await client.call<{ messages: BackendMessage[]; task_id: string }>('chat.history', {
        worktree_id: worktreeId
      })

      const messages = (result.messages || []).map(convertMessage)
      set({
        messages,
        taskId: result.task_id,
        isDisabled: false,
        error: null
      })
    } catch (err) {
      // If error is "no active task", disable chat
      const errorMsg = err instanceof Error ? err.message : 'Failed to load history'
      if (errorMsg.includes('no active task')) {
        set({
          messages: [],
          taskId: null,
          isDisabled: true,
          error: null
        })
      } else {
        set({ error: errorMsg })
      }
    }
  },

  setTaskId: (taskId: string | null) => {
    set({
      taskId,
      isDisabled: !taskId,
      messages: taskId ? get().messages : []
    })
  },

  sendMessage: async (content: string, worktreeId?: string) => {
    if (!content.trim()) return

    const { isDisabled } = get()
    if (isDisabled) {
      set({ error: `No active task. Start a task first with ${cliCmd('start')}.` })
      return
    }

    const client = useGlobalStore.getState().client
    if (!client) {
      set({ error: 'Not connected to server' })
      return
    }

    // Add user message
    get().addMessage({
      role: 'user',
      content: content.trim(),
      status: 'complete'
    })

    // Abort any previous operation before starting a new one
    activeAbort?.abort()
    const abort = new AbortController()
    activeAbort = abort

    set({ isTyping: true, error: null })

    try {
      // Submit chat to backend
      const result = await client.call<{ job_id: string; status: string }>('chat.send', {
        message: content.trim(),
        worktree_id: worktreeId
      })

      const jobId = result.job_id
      set({ activeJobId: jobId })

      // Create assistant message for streaming
      const assistantMsgId = get().addMessage({
        role: 'assistant',
        content: '',
        status: 'streaming',
        jobId
      })

      // Subscribe to streaming events from the server
      // Events are pushed via WebSocket instead of polling
      const unsubscribe = client.subscribe((data: unknown) => {
        // Stop processing if this operation was cancelled
        if (abort.signal.aborted) {
          unsubscribe()
          return
        }

        // Dispatch to typed event emitter so typed handlers receive chat events
        chatEvents.dispatch(data)

        const event = data as ChatEvent

        // Only handle events for our job
        if (event.job_id !== jobId) return

        switch (event.type) {
          case 'job_started':
            get().updateMessage(assistantMsgId, {
              content: event.content || 'Processing...',
              status: 'streaming'
            })
            break

          case 'stream':
          case 'assistant':
            // Append streaming content
            get().appendToMessage(assistantMsgId, event.content || '')
            break

          case 'job_completed':
            get().updateMessage(assistantMsgId, {
              content: event.result || get().messages.find(m => m.id === assistantMsgId)?.content || 'Task completed.',
              status: 'complete',
              actions: [
                { id: 'approve', label: 'Approve', type: 'approve' },
                { id: 'reject', label: 'Reject', type: 'reject' }
              ]
            })
            set({ isTyping: false, activeJobId: null })
            if (activeAbort === abort) activeAbort = null
            unsubscribe()
            break

          case 'job_failed':
            get().updateMessage(assistantMsgId, {
              content: event.error || 'Job failed',
              status: 'error'
            })
            set({ isTyping: false, activeJobId: null, error: event.error || 'Job failed' })
            if (activeAbort === abort) activeAbort = null
            unsubscribe()
            break

          case 'subagent':
            // Add a subagent status message to chat
            if (event.subagent) {
              const subagent = event.subagent
              const statusText = subagent.status === 'started'
                ? `Starting ${subagent.type}: "${subagent.description}"`
                : subagent.status === 'completed'
                ? `Completed ${subagent.type}: "${subagent.description}" (${(subagent.duration || 0) / 1000}s)`
                : `Failed ${subagent.type}: "${subagent.description}" - ${subagent.exit_reason || 'unknown error'}`

              get().addMessage({
                role: 'subagent',
                content: statusText,
                status: 'complete',
                jobId,
                subagent: {
                  subagentId: subagent.id,
                  type: subagent.type,
                  description: subagent.description,
                  status: subagent.status,
                  duration: subagent.duration,
                  exitReason: subagent.exit_reason
                }
              })
            }
            break

          case 'permission':
            // Add permission request to chat (with danger warning if applicable)
            if (event.permission_request) {
              const perm = event.permission_request
              const dangerLevel = perm.danger_level || 'safe'
              let content = `Permission requested: ${perm.tool}`
              if (dangerLevel !== 'safe' && perm.danger_reason) {
                content += `\n⚠️ ${dangerLevel.toUpperCase()}: ${perm.danger_reason}`
              }

              get().addMessage({
                role: 'permission',
                content,
                status: 'complete',
                jobId,
                permission: {
                  requestId: perm.id,
                  tool: perm.tool,
                  dangerLevel,
                  dangerReason: perm.danger_reason
                }
              })
            }
            break
        }
      })

    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Failed to send message'

      // Check if error indicates no active task
      if (errorMsg.includes('no active task')) {
        set({
          isTyping: false,
          isDisabled: true,
          error: 'No active task. Start a task first.',
          activeJobId: null
        })
      } else {
        set({
          isTyping: false,
          error: errorMsg,
          activeJobId: null
        })
      }

      get().addMessage({
        role: 'system',
        content: `Error: ${errorMsg}`,
        status: 'error'
      })
    }
  },

  addMessage: (message) => {
    const id = generateId()
    const newMessage: UIChatMessage = {
      ...message,
      id,
      timestamp: new Date()
    }
    set(state => ({
      messages: [...state.messages, newMessage]
    }))
    return id
  },

  updateMessage: (id, updates) => {
    set(state => ({
      messages: state.messages.map(msg =>
        msg.id === id ? { ...msg, ...updates } : msg
      )
    }))
  },

  appendToMessage: (id, content) => {
    set(state => ({
      messages: state.messages.map(msg =>
        msg.id === id ? { ...msg, content: msg.content + content } : msg
      )
    }))
  },

  setTyping: (typing) => {
    set({ isTyping: typing })
  },

  clearMessages: async () => {
    const client = useGlobalStore.getState().client
    const worktreeId = useProjectStore.getState().worktreeId

    // Try to clear on backend first
    if (client && worktreeId) {
      try {
        await client.call('chat.clear', { worktree_id: worktreeId })
      } catch (err) {
        // Log but don't fail - still clear local state
        console.warn('Failed to clear chat on backend:', err)
      }
    }

    set({ messages: [], error: null, activeJobId: null })
  },

  handleAction: (messageId, actionId) => {
    void (async () => {
    const message = get().messages.find(m => m.id === messageId)
    if (!message) return

    const action = message.actions?.find(a => a.id === actionId)
    if (!action) return

    // Remove actions immediately so buttons disappear
    get().updateMessage(messageId, { actions: undefined })

    // Quality gate response: actionId is "quality:<promptId>:yes|no"
    if (actionId.startsWith('quality:')) {
      const parts = actionId.split(':')
      if (parts.length === 3) {
        const promptId = parts[1]
        const answer = parts[2] === 'yes'
        get().addMessage({
          role: 'system',
          content: `Quality gate: ${answer ? 'Yes, proceed' : 'No, skip'}`,
          status: 'complete'
        })
        await useProjectStore.getState().respondToPrompt(promptId, answer)
      }
      return
    }

    // Workflow action buttons (from state-transition messages)
    if (actionId.startsWith('workflow:')) {
      const ps = useProjectStore.getState()
      switch (actionId) {
        case 'workflow:refresh':
          await ps.refresh()
          break
        case 'workflow:approve':
          await ps.approveRemote()
          break
        case 'workflow:merge':
          await ps.mergeRemote('rebase')
          break
        case 'workflow:finish':
          await ps.finish()
          break
        case 'workflow:undo':
          await ps.undo()
          break
      }
      return
    }

    // Add system message confirming the action
    get().addMessage({
      role: 'system',
      content: `Action "${action.label}" ${action.type === 'approve' ? 'approved' : action.type === 'reject' ? 'rejected' : 'executed'}.`,
      status: 'complete'
    })

    // Send approve/reject to the review socket
    if (action.type === 'approve') {
      await useProjectStore.getState().review({ approve: true })
    } else if (action.type === 'reject') {
      await useProjectStore.getState().review({ reject: true })
    }
    })()
  },

  cancelOperation: () => {
    if (activeAbort) {
      activeAbort.abort()
      activeAbort = null
    }
    // Also stop the active job on the backend
    const { activeJobId } = get()
    if (activeJobId) {
      const worktreeId = useProjectStore.getState().worktreeId
      if (worktreeId) {
        void useGlobalStore.getState().stopChat(worktreeId, activeJobId)
      }
    }
    set({ isTyping: false, activeJobId: null })
  }
}))

// Subscribe to project state changes to load chat history, surface quality prompts,
// and inject workflow transition messages
let prevWorktreeId: string | null = null
let prevQualityPromptId: string | null = null
let prevTaskState: string | null = null
useProjectStore.subscribe((state) => {
  const worktreeId = state.worktreeId
  if (worktreeId !== prevWorktreeId) {
    prevWorktreeId = worktreeId
    if (worktreeId) {
      void useChatStore.getState().loadHistory(worktreeId)
    } else {
      useChatStore.setState({
        messages: [],
        taskId: null,
        isDisabled: true,
        error: null
      })
    }
  }

  // Surface quality gate prompts as chat messages with yes/no buttons
  const qp = state.qualityPrompt
  const newId = qp?.id ?? null
  if (newId !== prevQualityPromptId && newId !== null && qp) {
    prevQualityPromptId = newId
    useChatStore.getState().addMessage({
      role: 'system',
      content: `Quality gate: ${qp.question}`,
      status: 'complete',
      actions: [
        { id: `quality:${qp.id}:yes`, label: 'Yes, proceed', type: 'custom' },
        { id: `quality:${qp.id}:no`, label: 'No, skip', type: 'custom' }
      ]
    })
  }
  if (newId === null) {
    prevQualityPromptId = null
  }

  // Inject workflow state-transition messages with action buttons
  const taskState = state.state
  if (taskState !== prevTaskState && prevTaskState !== null) {
    const { addMessage } = useChatStore.getState()

    if (taskState === 'submitted') {
      const msgId = addMessage({
        role: 'system',
        content: 'PR submitted.',
        status: 'complete',
        actions: [
          { id: 'workflow:refresh', label: 'Check Status', type: 'custom' },
          { id: 'workflow:approve', label: 'Approve', type: 'approve' },
          { id: 'workflow:merge', label: 'Merge', type: 'custom' },
          { id: 'workflow:finish', label: 'Finish', type: 'custom' },
        ]
      })
      // Store msgId for action handling (handled via handleAction)
      void msgId
    }

    if (taskState === 'failed' && state.error) {
      addMessage({
        role: 'system',
        content: `Action failed: ${state.error}`,
        status: 'error',
        actions: [
          { id: 'workflow:undo', label: 'Undo', type: 'custom' },
        ]
      })
    }

    if (taskState === 'planned') {
      addMessage({
        role: 'system',
        content: 'Plan complete. Type /implement or click Implement in the workflow bar.',
        status: 'complete',
      })
    }

    if (taskState === 'implemented') {
      addMessage({
        role: 'system',
        content: 'Implementation complete. Type /review, /simplify, or /optimize.',
        status: 'complete',
      })
    }
  }
  prevTaskState = taskState
})
