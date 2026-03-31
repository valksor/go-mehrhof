import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useChatStore } from './chatStore'
import { useGlobalStore as mockGlobalStore } from './globalStore'
import { useProjectStore as mockProjectStore } from './projectStore'

// Mock globalStore to avoid real WebSocket connections
vi.mock('./globalStore', () => ({
  useGlobalStore: {
    getState: vi.fn().mockReturnValue({
      client: null,
    })
  }
}))

// Mock projectStore to avoid real WebSocket connections
vi.mock('./projectStore', () => ({
  useProjectStore: {
    getState: vi.fn().mockReturnValue({
      worktreeId: null,
      respondToPrompt: vi.fn(),
      review: vi.fn(),
    }),
    subscribe: vi.fn().mockReturnValue(() => {}),
  }
}))

// Helpers
type EventHandler = (data: unknown) => void

// Creates a mutable handler ref that works with TypeScript's control flow analysis
const createHandlerRef = () => {
  const ref: { current: EventHandler | null } = { current: null }
  return ref
}

const makeClient = (overrides: Record<string, ReturnType<typeof vi.fn>> = {}) => ({
  call: vi.fn().mockResolvedValue({}),
  subscribe: vi.fn(),
  connect: vi.fn().mockResolvedValue(undefined),
  close: vi.fn(),
  setOnDisconnect: vi.fn(),
  ...overrides,
})

const setClient = (client: ReturnType<typeof makeClient> | null) => {
  (mockGlobalStore.getState as ReturnType<typeof vi.fn>).mockReturnValue({ client } as never)
}

const setWorktreeId = (id: string | null) => {
  (mockProjectStore.getState as ReturnType<typeof vi.fn>).mockReturnValue({
    worktreeId: id,
    respondToPrompt: vi.fn(),
    review: vi.fn(),
  } as never)
}

const initialState = {
  messages: [],
  isTyping: false,
  error: null,
  activeJobId: null,
  taskId: null,
  isDisabled: true,
}

describe('chatStore', () => {
  beforeEach(() => {
    useChatStore.setState(initialState)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  describe('initial state', () => {
    it('starts with empty messages', () => {
      expect(useChatStore.getState().messages).toEqual([])
    })

    it('starts not typing', () => {
      expect(useChatStore.getState().isTyping).toBe(false)
    })

    it('starts with no error', () => {
      expect(useChatStore.getState().error).toBeNull()
    })

    it('starts with no active job', () => {
      expect(useChatStore.getState().activeJobId).toBeNull()
    })

    it('starts with null taskId', () => {
      expect(useChatStore.getState().taskId).toBeNull()
    })

    it('starts disabled', () => {
      expect(useChatStore.getState().isDisabled).toBe(true)
    })
  })

  describe('addMessage', () => {
    it('adds a message and returns its id', () => {
      const id = useChatStore.getState().addMessage({
        role: 'user',
        content: 'Hello!',
        status: 'complete',
      })
      expect(typeof id).toBe('string')
      expect(id).toBeTruthy()
    })

    it('adds message to messages array', () => {
      useChatStore.getState().addMessage({
        role: 'user',
        content: 'Test message',
        status: 'complete',
      })
      expect(useChatStore.getState().messages).toHaveLength(1)
      expect(useChatStore.getState().messages[0].content).toBe('Test message')
    })

    it('assigns a unique id to each message', () => {
      const id1 = useChatStore.getState().addMessage({ role: 'user', content: 'msg1', status: 'complete' })
      const id2 = useChatStore.getState().addMessage({ role: 'user', content: 'msg2', status: 'complete' })
      expect(id1).not.toBe(id2)
    })

    it('preserves message role', () => {
      useChatStore.getState().addMessage({ role: 'assistant', content: 'reply', status: 'streaming' })
      expect(useChatStore.getState().messages[0].role).toBe('assistant')
    })

    it('preserves message status', () => {
      useChatStore.getState().addMessage({ role: 'user', content: 'msg', status: 'pending' })
      expect(useChatStore.getState().messages[0].status).toBe('pending')
    })

    it('adds a timestamp to the message', () => {
      useChatStore.getState().addMessage({ role: 'user', content: 'msg', status: 'complete' })
      expect(useChatStore.getState().messages[0].timestamp).toBeInstanceOf(Date)
    })

    it('appends messages in order', () => {
      useChatStore.getState().addMessage({ role: 'user', content: 'first', status: 'complete' })
      useChatStore.getState().addMessage({ role: 'assistant', content: 'second', status: 'complete' })
      const msgs = useChatStore.getState().messages
      expect(msgs[0].content).toBe('first')
      expect(msgs[1].content).toBe('second')
    })

    it('preserves optional jobId', () => {
      useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'reply',
        status: 'streaming',
        jobId: 'job-123',
      })
      expect(useChatStore.getState().messages[0].jobId).toBe('job-123')
    })

    it('can add permission message with permission info', () => {
      useChatStore.getState().addMessage({
        role: 'permission',
        content: 'Permission requested: bash',
        status: 'complete',
        permission: {
          requestId: 'req-1',
          tool: 'bash',
          dangerLevel: 'caution',
        },
      })
      const msg = useChatStore.getState().messages[0]
      expect(msg.role).toBe('permission')
      expect(msg.permission?.tool).toBe('bash')
      expect(msg.permission?.dangerLevel).toBe('caution')
    })

    it('can add subagent message with subagent info', () => {
      useChatStore.getState().addMessage({
        role: 'subagent',
        content: 'Starting search: find files',
        status: 'complete',
        subagent: {
          subagentId: 'sa-1',
          type: 'search',
          description: 'find files',
          status: 'started',
        },
      })
      const msg = useChatStore.getState().messages[0]
      expect(msg.role).toBe('subagent')
      expect(msg.subagent?.type).toBe('search')
      expect(msg.subagent?.status).toBe('started')
    })
  })

  describe('updateMessage', () => {
    it('updates message content', () => {
      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'initial',
        status: 'streaming',
      })
      useChatStore.getState().updateMessage(id, { content: 'updated content' })
      const msg = useChatStore.getState().messages.find(m => m.id === id)
      expect(msg?.content).toBe('updated content')
    })

    it('updates message status', () => {
      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: '',
        status: 'streaming',
      })
      useChatStore.getState().updateMessage(id, { status: 'complete' })
      const msg = useChatStore.getState().messages.find(m => m.id === id)
      expect(msg?.status).toBe('complete')
    })

    it('does not affect other messages', () => {
      const id1 = useChatStore.getState().addMessage({ role: 'user', content: 'msg1', status: 'complete' })
      const id2 = useChatStore.getState().addMessage({ role: 'user', content: 'msg2', status: 'complete' })
      useChatStore.getState().updateMessage(id1, { content: 'changed' })
      const msg2 = useChatStore.getState().messages.find(m => m.id === id2)
      expect(msg2?.content).toBe('msg2')
    })

    it('can update actions', () => {
      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'done',
        status: 'complete',
      })
      useChatStore.getState().updateMessage(id, {
        actions: [{ id: 'approve', label: 'Approve', type: 'approve' }]
      })
      const msg = useChatStore.getState().messages.find(m => m.id === id)
      expect(msg?.actions).toHaveLength(1)
      expect(msg?.actions?.[0].id).toBe('approve')
    })

    it('preserves other fields when updating', () => {
      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'hello',
        status: 'streaming',
        jobId: 'job-abc',
      })
      useChatStore.getState().updateMessage(id, { status: 'complete' })
      const msg = useChatStore.getState().messages.find(m => m.id === id)
      expect(msg?.jobId).toBe('job-abc')
      expect(msg?.content).toBe('hello')
    })
  })

  describe('appendToMessage', () => {
    it('appends content to existing message', () => {
      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'Hello',
        status: 'streaming',
      })
      useChatStore.getState().appendToMessage(id, ' world')
      const msg = useChatStore.getState().messages.find(m => m.id === id)
      expect(msg?.content).toBe('Hello world')
    })

    it('handles multiple appends', () => {
      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'A',
        status: 'streaming',
      })
      useChatStore.getState().appendToMessage(id, 'B')
      useChatStore.getState().appendToMessage(id, 'C')
      useChatStore.getState().appendToMessage(id, 'D')
      const msg = useChatStore.getState().messages.find(m => m.id === id)
      expect(msg?.content).toBe('ABCD')
    })

    it('does not affect other messages', () => {
      const id1 = useChatStore.getState().addMessage({ role: 'user', content: 'msg1', status: 'complete' })
      const id2 = useChatStore.getState().addMessage({ role: 'assistant', content: 'reply', status: 'streaming' })
      useChatStore.getState().appendToMessage(id2, ' more')
      const msg1 = useChatStore.getState().messages.find(m => m.id === id1)
      expect(msg1?.content).toBe('msg1')
    })
  })

  describe('setTyping', () => {
    it('sets isTyping to true', () => {
      useChatStore.getState().setTyping(true)
      expect(useChatStore.getState().isTyping).toBe(true)
    })

    it('sets isTyping to false', () => {
      useChatStore.setState({ isTyping: true })
      useChatStore.getState().setTyping(false)
      expect(useChatStore.getState().isTyping).toBe(false)
    })
  })

  describe('setTaskId', () => {
    it('sets taskId', () => {
      useChatStore.getState().setTaskId('task-123')
      expect(useChatStore.getState().taskId).toBe('task-123')
    })

    it('enables chat when taskId is set', () => {
      useChatStore.getState().setTaskId('task-123')
      expect(useChatStore.getState().isDisabled).toBe(false)
    })

    it('disables chat when taskId is null', () => {
      useChatStore.setState({ taskId: 'task-123', isDisabled: false })
      useChatStore.getState().setTaskId(null)
      expect(useChatStore.getState().isDisabled).toBe(true)
    })

    it('clears messages when taskId is null', () => {
      useChatStore.getState().addMessage({ role: 'user', content: 'hi', status: 'complete' })
      useChatStore.getState().setTaskId(null)
      expect(useChatStore.getState().messages).toEqual([])
    })

    it('preserves messages when setting a new taskId', () => {
      useChatStore.getState().addMessage({ role: 'user', content: 'hi', status: 'complete' })
      useChatStore.getState().setTaskId('task-456')
      expect(useChatStore.getState().messages).toHaveLength(1)
    })
  })

  describe('clearMessages', () => {
    it('clears all messages', async () => {
      useChatStore.getState().addMessage({ role: 'user', content: 'msg1', status: 'complete' })
      useChatStore.getState().addMessage({ role: 'assistant', content: 'reply', status: 'complete' })
      await useChatStore.getState().clearMessages()
      expect(useChatStore.getState().messages).toEqual([])
    })

    it('clears error state', async () => {
      useChatStore.setState({ error: 'some error' })
      await useChatStore.getState().clearMessages()
      expect(useChatStore.getState().error).toBeNull()
    })

    it('clears activeJobId', async () => {
      useChatStore.setState({ activeJobId: 'job-123' })
      await useChatStore.getState().clearMessages()
      expect(useChatStore.getState().activeJobId).toBeNull()
    })

    it('works when messages are already empty', async () => {
      await useChatStore.getState().clearMessages()
      expect(useChatStore.getState().messages).toEqual([])
    })
  })

  describe('sendMessage', () => {
    it('does nothing when content is empty', async () => {
      await useChatStore.getState().sendMessage('')
      expect(useChatStore.getState().messages).toEqual([])
    })

    it('does nothing when content is only whitespace', async () => {
      await useChatStore.getState().sendMessage('   ')
      expect(useChatStore.getState().messages).toEqual([])
    })

    it('sets error when chat is disabled', async () => {
      useChatStore.setState({ isDisabled: true })
      await useChatStore.getState().sendMessage('hello')
      expect(useChatStore.getState().error).toBeTruthy()
    })

    it('sets error when not connected (client is null)', async () => {
      useChatStore.setState({ isDisabled: false })
      await useChatStore.getState().sendMessage('hello')
      expect(useChatStore.getState().error).toBe('Not connected to server')
    })
  })

  describe('handleAction', () => {
    it('does nothing for unknown message id', () => {
      // Should not throw
      expect(() => {
        useChatStore.getState().handleAction('unknown-id', 'approve')
      }).not.toThrow()
    })

    it('does nothing when message has no matching action', () => {
      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'done',
        status: 'complete',
        actions: [{ id: 'approve', label: 'Approve', type: 'approve' }],
      })
      // Should not throw for unknown action id
      expect(() => {
        useChatStore.getState().handleAction(id, 'nonexistent-action')
      }).not.toThrow()
    })

    it('removes actions from message when action is found', async () => {
      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'done',
        status: 'complete',
        actions: [
          { id: 'approve', label: 'Approve', type: 'approve' },
          { id: 'reject', label: 'Reject', type: 'reject' },
        ],
      })
      useChatStore.getState().handleAction(id, 'approve')
      const msg = useChatStore.getState().messages.find(m => m.id === id)
      expect(msg?.actions).toBeUndefined()
    })

    it('adds confirmation system message when action is handled', async () => {
      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'done',
        status: 'complete',
        actions: [{ id: 'approve', label: 'Approve', type: 'approve' }],
      })
      useChatStore.getState().handleAction(id, 'approve')
      const systemMsg = useChatStore.getState().messages.find(m => m.role === 'system')
      expect(systemMsg).toBeTruthy()
      expect(systemMsg?.content).toContain('Approve')
    })

    it('handles quality gate "yes" action', async () => {
      const projectState = {
        worktreeId: 'wt-1',
        respondToPrompt: vi.fn().mockResolvedValue(undefined),
        review: vi.fn(),
      }
      ;(mockProjectStore.getState as ReturnType<typeof vi.fn>).mockReturnValue(projectState as never)

      const id = useChatStore.getState().addMessage({
        role: 'system',
        content: 'Quality gate: Proceed?',
        status: 'complete',
        actions: [{ id: 'quality:prompt-1:yes', label: 'Yes, proceed', type: 'custom' }],
      })
      useChatStore.getState().handleAction(id, 'quality:prompt-1:yes')
      expect(projectState.respondToPrompt).toHaveBeenCalledWith('prompt-1', true)
    })

    it('handles quality gate "no" action', async () => {
      const projectState = {
        worktreeId: 'wt-1',
        respondToPrompt: vi.fn().mockResolvedValue(undefined),
        review: vi.fn(),
      }
      ;(mockProjectStore.getState as ReturnType<typeof vi.fn>).mockReturnValue(projectState as never)

      const id = useChatStore.getState().addMessage({
        role: 'system',
        content: 'Quality gate: Proceed?',
        status: 'complete',
        actions: [{ id: 'quality:prompt-2:no', label: 'No, skip', type: 'custom' }],
      })
      useChatStore.getState().handleAction(id, 'quality:prompt-2:no')
      expect(projectState.respondToPrompt).toHaveBeenCalledWith('prompt-2', false)
    })

    it('calls review approve for approve action', async () => {
      const projectState = {
        worktreeId: 'wt-1',
        respondToPrompt: vi.fn(),
        review: vi.fn().mockResolvedValue(undefined),
      }
      ;(mockProjectStore.getState as ReturnType<typeof vi.fn>).mockReturnValue(projectState as never)

      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'done',
        status: 'complete',
        actions: [{ id: 'approve', label: 'Approve', type: 'approve' }],
      })
      useChatStore.getState().handleAction(id, 'approve')
      expect(projectState.review).toHaveBeenCalledWith({ approve: true })
    })

    it('calls review reject for reject action', async () => {
      const projectState = {
        worktreeId: 'wt-1',
        respondToPrompt: vi.fn(),
        review: vi.fn().mockResolvedValue(undefined),
      }
      ;(mockProjectStore.getState as ReturnType<typeof vi.fn>).mockReturnValue(projectState as never)

      const id = useChatStore.getState().addMessage({
        role: 'assistant',
        content: 'done',
        status: 'complete',
        actions: [{ id: 'reject', label: 'Reject', type: 'reject' }],
      })
      useChatStore.getState().handleAction(id, 'reject')
      expect(projectState.review).toHaveBeenCalledWith({ reject: true })
    })
  })

  // ---------------------------------------------------------------------------
  // loadHistory
  // ---------------------------------------------------------------------------

  describe('loadHistory', () => {
    it('loads messages and sets taskId on success', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({
        messages: [
          { id: 'msg-1', role: 'user', content: 'Hello', timestamp: '2026-01-01T00:00:00Z' },
          { id: 'msg-2', role: 'assistant', content: 'Hi there', timestamp: '2026-01-01T00:00:01Z' },
        ],
        task_id: 'task-abc',
      })
      setClient(client)

      await useChatStore.getState().loadHistory('wt-1')

      expect(useChatStore.getState().messages).toHaveLength(2)
      expect(useChatStore.getState().taskId).toBe('task-abc')
      expect(useChatStore.getState().isDisabled).toBe(false)
      expect(useChatStore.getState().error).toBeNull()
    })

    it('calls chat.history with worktree_id', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ messages: [], task_id: 't1' })
      setClient(client)

      await useChatStore.getState().loadHistory('wt-42')
      expect(client.call).toHaveBeenCalledWith('chat.history', { worktree_id: 'wt-42' })
    })

    it('converts backend messages to frontend format', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({
        messages: [{ id: 'be-1', role: 'user', content: 'Question', job_id: 'j1' }],
        task_id: 't1',
      })
      setClient(client)

      await useChatStore.getState().loadHistory('wt-1')
      const msg = useChatStore.getState().messages[0]
      expect(msg.status).toBe('complete')
      expect(msg.jobId).toBe('j1')
    })

    it('disables chat and clears messages on "no active task" error', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('no active task found'))
      setClient(client)

      useChatStore.getState().addMessage({ role: 'user', content: 'hi', status: 'complete' })
      await useChatStore.getState().loadHistory('wt-1')

      expect(useChatStore.getState().isDisabled).toBe(true)
      expect(useChatStore.getState().taskId).toBeNull()
      expect(useChatStore.getState().messages).toEqual([])
      expect(useChatStore.getState().error).toBeNull()
    })

    it('sets error on other failures', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('connection timeout'))
      setClient(client)

      await useChatStore.getState().loadHistory('wt-1')
      expect(useChatStore.getState().error).toBe('connection timeout')
    })

    it('does nothing when no client', async () => {
      setClient(null)
      await useChatStore.getState().loadHistory('wt-1')
      expect(useChatStore.getState().messages).toEqual([])
    })

    it('handles empty messages array', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ messages: [], task_id: 't1' })
      setClient(client)

      await useChatStore.getState().loadHistory('wt-1')
      expect(useChatStore.getState().messages).toEqual([])
      expect(useChatStore.getState().taskId).toBe('t1')
    })
  })

  // ---------------------------------------------------------------------------
  // sendMessage — with connected client
  // ---------------------------------------------------------------------------

  describe('sendMessage with client', () => {
    it('adds user message before calling API', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-1', status: 'started' })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('Hello server')
      const userMsg = useChatStore.getState().messages.find(m => m.role === 'user')
      expect(userMsg?.content).toBe('Hello server')
    })

    it('trims whitespace from message', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-1', status: 'started' })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('  hello  ')
      expect(client.call).toHaveBeenCalledWith(
        'chat.send',
        expect.objectContaining({ message: 'hello' })
      )
    })

    it('creates assistant streaming message after submit', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-99', status: 'started' })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('test')
      const assistantMsg = useChatStore.getState().messages.find(m => m.role === 'assistant')
      expect(assistantMsg).toBeTruthy()
      expect(assistantMsg?.status).toBe('streaming')
      expect(assistantMsg?.jobId).toBe('job-99')
    })

    it('sets activeJobId after submit', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-7', status: 'started' })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('work')
      expect(useChatStore.getState().activeJobId).toBe('job-7')
    })

    it('includes worktreeId in call when provided', async () => {
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'j1', status: 'started' })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('hi', 'wt-abc')
      expect(client.call).toHaveBeenCalledWith('chat.send', {
        message: 'hi',
        worktree_id: 'wt-abc',
      })
    })

    it('handles no active task error from backend', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('no active task'))
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('hi')
      expect(useChatStore.getState().isDisabled).toBe(true)
      expect(useChatStore.getState().isTyping).toBe(false)
    })

    it('handles generic backend error', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('internal error'))
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('hi')
      expect(useChatStore.getState().error).toBe('internal error')
      expect(useChatStore.getState().isTyping).toBe(false)
      expect(useChatStore.getState().activeJobId).toBeNull()
    })

    it('adds error system message on backend failure', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('boom'))
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('hi')
      const errMsg = useChatStore.getState().messages.find(m => m.role === 'system' && m.status === 'error')
      expect(errMsg?.content).toContain('boom')
    })
  })

  // ---------------------------------------------------------------------------
  // subscribe callback — streaming events
  // ---------------------------------------------------------------------------

  describe('streaming event handling via subscribe', () => {
    it('handles stream event by appending content', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-stream', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('ping')

      if (handlerRef.current) {
        handlerRef.current({ type: 'stream', job_id: 'job-stream', content: 'Hello' })
        handlerRef.current({ type: 'stream', job_id: 'job-stream', content: ' world' })
        const assistantMsg = useChatStore.getState().messages.find(m => m.role === 'assistant')
        expect(assistantMsg?.content).toBe('Hello world')
      }
    })

    it('ignores events for other job ids', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-mine', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('ping')

      if (handlerRef.current) {
        handlerRef.current({ type: 'stream', job_id: 'job-other', content: 'should be ignored' })
        const assistantMsg = useChatStore.getState().messages.find(m => m.role === 'assistant')
        expect(assistantMsg?.content).toBe('')
      }
    })

    it('handles job_completed event', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-done', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('go')

      if (handlerRef.current) {
        handlerRef.current({ type: 'job_completed', job_id: 'job-done', result: 'Final answer' })
        const assistantMsg = useChatStore.getState().messages.find(m => m.role === 'assistant')
        expect(assistantMsg?.status).toBe('complete')
        expect(assistantMsg?.content).toBe('Final answer')
        expect(useChatStore.getState().isTyping).toBe(false)
        expect(useChatStore.getState().activeJobId).toBeNull()
      }
    })

    it('job_completed adds approve/reject actions', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-done', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('go')

      if (handlerRef.current) {
        handlerRef.current({ type: 'job_completed', job_id: 'job-done', result: 'Done' })
        const assistantMsg = useChatStore.getState().messages.find(m => m.role === 'assistant')
        expect(assistantMsg?.actions).toHaveLength(2)
        expect(assistantMsg?.actions?.map(a => a.id)).toContain('approve')
        expect(assistantMsg?.actions?.map(a => a.id)).toContain('reject')
      }
    })

    it('handles job_failed event', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-fail', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('fail me')

      if (handlerRef.current) {
        handlerRef.current({ type: 'job_failed', job_id: 'job-fail', error: 'out of memory' })
        const assistantMsg = useChatStore.getState().messages.find(m => m.role === 'assistant')
        expect(assistantMsg?.status).toBe('error')
        expect(useChatStore.getState().error).toBe('out of memory')
        expect(useChatStore.getState().isTyping).toBe(false)
      }
    })

    it('handles subagent started event', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-sub', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('search')

      if (handlerRef.current) {
        handlerRef.current({
          type: 'subagent',
          job_id: 'job-sub',
          subagent: { id: 'sa-1', type: 'search', description: 'find files', status: 'started' }
        })
        const subMsg = useChatStore.getState().messages.find(m => m.role === 'subagent')
        expect(subMsg).toBeTruthy()
        expect(subMsg?.subagent?.type).toBe('search')
        expect(subMsg?.subagent?.status).toBe('started')
        expect(subMsg?.content).toContain('Starting search')
      }
    })

    it('handles subagent completed event', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-sub', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('search')

      if (handlerRef.current) {
        handlerRef.current({
          type: 'subagent',
          job_id: 'job-sub',
          subagent: { id: 'sa-1', type: 'search', description: 'find files', status: 'completed', duration: 3000 }
        })
        const subMsg = useChatStore.getState().messages.find(m => m.role === 'subagent')
        expect(subMsg?.content).toContain('Completed search')
        expect(subMsg?.content).toContain('3s')
      }
    })

    it('handles subagent failed event', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-sub', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('search')

      if (handlerRef.current) {
        handlerRef.current({
          type: 'subagent',
          job_id: 'job-sub',
          subagent: { id: 'sa-2', type: 'bash', description: 'run tests', status: 'failed', exit_reason: 'timeout' }
        })
        const subMsg = useChatStore.getState().messages.find(m => m.role === 'subagent')
        expect(subMsg?.content).toContain('Failed bash')
        expect(subMsg?.content).toContain('timeout')
        expect(subMsg?.subagent?.exitReason).toBe('timeout')
      }
    })

    it('handles permission event', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-perm', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('run')

      if (handlerRef.current) {
        handlerRef.current({
          type: 'permission',
          job_id: 'job-perm',
          permission_request: { id: 'req-1', tool: 'bash', danger_level: 'caution', danger_reason: 'executes shell' }
        })
        const permMsg = useChatStore.getState().messages.find(m => m.role === 'permission')
        expect(permMsg).toBeTruthy()
        expect(permMsg?.permission?.tool).toBe('bash')
        expect(permMsg?.permission?.dangerLevel).toBe('caution')
        expect(permMsg?.permission?.dangerReason).toBe('executes shell')
        expect(permMsg?.content).toContain('CAUTION')
      }
    })

    it('handles safe permission event without danger warning', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-safe', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('read')

      if (handlerRef.current) {
        handlerRef.current({
          type: 'permission',
          job_id: 'job-safe',
          permission_request: { id: 'req-2', tool: 'read_file' }
        })
        const permMsg = useChatStore.getState().messages.find(m => m.role === 'permission')
        expect(permMsg?.permission?.dangerLevel).toBe('safe')
        expect(permMsg?.content).not.toContain('⚠️')
      }
    })

    it('handles job_started event', async () => {
      const handlerRef = createHandlerRef()
      const client = makeClient()
      client.call.mockResolvedValueOnce({ job_id: 'job-start', status: 'started' })
      client.subscribe.mockImplementationOnce((handler: (data: unknown) => void) => {
        handlerRef.current = handler
        return () => {}
      })
      setClient(client)
      useChatStore.setState({ isDisabled: false })

      await useChatStore.getState().sendMessage('begin')

      if (handlerRef.current) {
        handlerRef.current({ type: 'job_started', job_id: 'job-start', content: 'Processing your request' })
        const assistantMsg = useChatStore.getState().messages.find(m => m.role === 'assistant')
        expect(assistantMsg?.content).toBe('Processing your request')
        expect(assistantMsg?.status).toBe('streaming')
      }
    })
  })

  // ---------------------------------------------------------------------------
  // clearMessages — with client
  // ---------------------------------------------------------------------------

  describe('clearMessages with client', () => {
    it('calls chat.clear on backend when client and worktreeId are available', async () => {
      const client = makeClient()
      setClient(client)
      setWorktreeId('wt-clear')

      await useChatStore.getState().clearMessages()
      expect(client.call).toHaveBeenCalledWith('chat.clear', { worktree_id: 'wt-clear' })
    })

    it('still clears local state even if backend call fails', async () => {
      const client = makeClient()
      client.call.mockRejectedValueOnce(new Error('backend error'))
      setClient(client)
      setWorktreeId('wt-1')

      useChatStore.getState().addMessage({ role: 'user', content: 'old message', status: 'complete' })
      await useChatStore.getState().clearMessages()

      expect(useChatStore.getState().messages).toEqual([])
      expect(useChatStore.getState().activeJobId).toBeNull()
    })

    it('skips backend call when no worktreeId', async () => {
      const client = makeClient()
      setClient(client)
      setWorktreeId(null)

      await useChatStore.getState().clearMessages()
      expect(client.call).not.toHaveBeenCalled()
      expect(useChatStore.getState().messages).toEqual([])
    })
  })
})
