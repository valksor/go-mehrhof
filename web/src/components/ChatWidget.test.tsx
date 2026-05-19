import { render, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ChatWidget } from './ChatWidget'
import type { UIChatMessage } from '../stores/chatStore'

const mockSendMessage = vi.fn()
const mockClearMessages = vi.fn()
const mockHandleAction = vi.fn()
const mockAddMessage = vi.fn()

let mockChatState: Record<string, unknown> = {}
let mockGlobalState: Record<string, unknown> = {}
let mockProjectState: Record<string, unknown> = {}
let mockScreenshotState: Record<string, unknown> = {}

vi.mock('../stores/chatStore', () => ({
  useChatStore: (selector?: (s: Record<string, unknown>) => unknown) =>
    selector ? selector(mockChatState) : mockChatState,
}))

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: (selector?: (s: Record<string, unknown>) => unknown) =>
    selector ? selector(mockGlobalState) : mockGlobalState,
}))

vi.mock('../stores/projectStore', () => ({
  useProjectStore: Object.assign(
    (selector?: (s: Record<string, unknown>) => unknown) => (selector ? selector(mockProjectState) : mockProjectState),
    { getState: () => mockProjectState },
  ),
}))

vi.mock('../stores/screenshotStore', () => ({
  useScreenshotStore: (selector?: (s: Record<string, unknown>) => unknown) =>
    selector ? selector(mockScreenshotState) : mockScreenshotState,
  getScreenshotById: () => null,
  formatScreenshotRef: (id: string) => `[screenshot:${id}]`,
}))

vi.mock('./ChatMessage', () => ({
  ChatMessageContent: ({ content }: { content: string }) => <span>{content}</span>,
}))

const mockParseCommand = vi.fn().mockReturnValue(null)
const mockGetAvailableCommands = vi.fn().mockReturnValue([])

vi.mock('../lib/chatCommands', () => ({
  parseCommand: (...args: unknown[]) => mockParseCommand(...args),
  getAvailableCommands: (...args: unknown[]) => mockGetAvailableCommands(...args),
}))

const mockDownloadJSON = vi.fn()

vi.mock('../lib/export', () => ({
  downloadJSON: (...args: unknown[]) => mockDownloadJSON(...args),
}))

function makeMessage(overrides: Partial<UIChatMessage> = {}): UIChatMessage {
  return {
    id: `msg-${Math.random().toString(36).slice(2)}`,
    role: 'user',
    content: 'Hello',
    timestamp: new Date('2026-01-01T12:00:00'),
    status: 'complete',
    ...overrides,
  }
}

function resetStores() {
  mockChatState = {
    messages: [],
    isTyping: false,
    sendMessage: mockSendMessage,
    clearMessages: mockClearMessages,
    handleAction: mockHandleAction,
    addMessage: mockAddMessage,
  }
  mockGlobalState = {
    client: { call: vi.fn() },
    connected: true,
  }
  mockProjectState = {
    start: vi.fn(),
  }
  mockScreenshotState = {
    attachedIds: [],
    clearAttached: vi.fn(),
    detach: vi.fn(),
  }
}

describe('ChatWidget', () => {
  beforeEach(() => {
    resetStores()
    vi.clearAllMocks()
  })

  it('renders without crashing', () => {
    const { container } = render(<ChatWidget />)
    expect(container).toBeTruthy()
  })

  it('shows Chat heading when not embedded', () => {
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Chat')).toBeInTheDocument()
  })

  it('does not show Chat heading when embedded', () => {
    const { queryByText } = render(<ChatWidget embedded />)
    expect(queryByText('Chat')).not.toBeInTheDocument()
  })

  it('shows empty state when no messages', () => {
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Start a conversation')).toBeInTheDocument()
    expect(getByText(/Type @ to mention files/)).toBeInTheDocument()
  })

  it('renders user messages', () => {
    mockChatState.messages = [makeMessage({ role: 'user', content: 'Hi there' })]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Hi there')).toBeInTheDocument()
    expect(getByText('You')).toBeInTheDocument()
  })

  it('renders assistant messages', () => {
    mockChatState.messages = [makeMessage({ role: 'assistant', content: 'Hello!' })]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Hello!')).toBeInTheDocument()
    expect(getByText('AI')).toBeInTheDocument()
  })

  it('renders system messages centered', () => {
    mockChatState.messages = [makeMessage({ role: 'system', content: 'System notice' })]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('System notice')).toBeInTheDocument()
  })

  it('shows typing indicator when isTyping', () => {
    mockChatState.isTyping = true
    const { container } = render(<ChatWidget />)
    // The typing indicator has bouncing dots
    const bounceDots = container.querySelectorAll('.animate-bounce')
    expect(bounceDots.length).toBe(3)
  })

  it('does not show typing indicator when not typing', () => {
    mockChatState.isTyping = false
    const { container } = render(<ChatWidget />)
    const bounceDots = container.querySelectorAll('.animate-bounce')
    expect(bounceDots.length).toBe(0)
  })

  it('has a message input textarea', () => {
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    expect(input).toBeInTheDocument()
  })

  it('has a send button', () => {
    const { getByRole } = render(<ChatWidget />)
    expect(getByRole('button', { name: 'Send message' })).toBeInTheDocument()
  })

  it('disables send button when input is empty', () => {
    const { getByRole } = render(<ChatWidget />)
    expect(getByRole('button', { name: 'Send message' })).toBeDisabled()
  })

  it('enables send button when input has text', () => {
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: 'Hello', selectionStart: 5 } })
    expect(getByRole('button', { name: 'Send message' })).not.toBeDisabled()
  })

  it('calls sendMessage on form submit', () => {
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: 'Test message', selectionStart: 12 } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(mockSendMessage).toHaveBeenCalledWith('Test message')
  })

  it('clears input after sending', () => {
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' }) as HTMLTextAreaElement
    fireEvent.change(input, { target: { value: 'Test', selectionStart: 4 } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(input.value).toBe('')
  })

  it('does not send on Shift+Enter', () => {
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: 'Hello', selectionStart: 5 } })
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true })
    expect(mockSendMessage).not.toHaveBeenCalled()
  })

  it('disables input when typing', () => {
    mockChatState.isTyping = true
    const { getByRole } = render(<ChatWidget />)
    expect(getByRole('combobox', { name: 'Message' })).toBeDisabled()
  })

  it('shows Clear chat and Export chat buttons when messages exist', () => {
    mockChatState.messages = [makeMessage()]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Clear chat')).toBeInTheDocument()
    expect(getByText('Export chat')).toBeInTheDocument()
  })

  it('does not show Clear/Export when no messages', () => {
    const { queryByText } = render(<ChatWidget />)
    expect(queryByText('Clear chat')).not.toBeInTheDocument()
    expect(queryByText('Export chat')).not.toBeInTheDocument()
  })

  it('calls clearMessages when Clear chat is clicked', () => {
    mockChatState.messages = [makeMessage()]
    const { getByText } = render(<ChatWidget />)
    getByText('Clear chat').click()
    expect(mockClearMessages).toHaveBeenCalled()
  })

  it('shows character count when input has text', () => {
    const { getByRole, getByText } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: 'Hello', selectionStart: 5 } })
    expect(getByText('5')).toBeInTheDocument()
  })

  it('shows task source detection hint for GitHub URLs', () => {
    const { getByRole, getByText } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: 'https://github.com/owner/repo/issues/42', selectionStart: 39 } })
    expect(getByText(/Task source detected/)).toBeInTheDocument()
  })

  it('renders multiple messages in order', () => {
    mockChatState.messages = [
      makeMessage({ id: '1', role: 'user', content: 'First' }),
      makeMessage({ id: '2', role: 'assistant', content: 'Second' }),
      makeMessage({ id: '3', role: 'user', content: 'Third' }),
    ]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('First')).toBeInTheDocument()
    expect(getByText('Second')).toBeInTheDocument()
    expect(getByText('Third')).toBeInTheDocument()
  })

  it('shows attached screenshots indicator', () => {
    mockScreenshotState.attachedIds = ['ss-1']
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Attached:')).toBeInTheDocument()
    expect(getByText('Clear all')).toBeInTheDocument()
  })

  it('calls detach when screenshot remove button is clicked', () => {
    mockScreenshotState.attachedIds = ['ss-1']
    const { getByLabelText } = render(<ChatWidget />)
    getByLabelText(/Remove screenshot/).click()
    expect(mockScreenshotState.detach).toHaveBeenCalledWith('ss-1')
  })

  it('calls clearAttached when Clear all is clicked', () => {
    mockScreenshotState.attachedIds = ['ss-1']
    const { getByText } = render(<ChatWidget />)
    getByText('Clear all').click()
    expect(mockScreenshotState.clearAttached).toHaveBeenCalled()
  })

  it('appends screenshot refs to message when sending with attachments', () => {
    mockScreenshotState.attachedIds = ['ss-1']
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: 'Check this', selectionStart: 10 } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(mockSendMessage).toHaveBeenCalledWith('Check this\n\n[screenshot:ss-1]')
    expect(mockScreenshotState.clearAttached).toHaveBeenCalled()
  })

  it('exports chat as JSON when Export chat is clicked', () => {
    mockChatState.messages = [makeMessage({ role: 'user', content: 'Hi' })]
    const { getByText } = render(<ChatWidget />)
    getByText('Export chat').click()
    expect(mockDownloadJSON).toHaveBeenCalledTimes(1)
    const exported = mockDownloadJSON.mock.calls[0][0]
    expect(exported).toHaveLength(1)
    expect(exported[0].role).toBe('user')
    expect(exported[0].content).toBe('Hi')
  })

  it('renders subagent messages with started status', () => {
    mockChatState.messages = [
      makeMessage({
        role: 'subagent' as UIChatMessage['role'],
        content: '',
        subagent: { subagentId: 'sa-1', status: 'started', type: 'planner', description: 'Planning task' },
      }),
    ]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('planner')).toBeInTheDocument()
  })

  it('renders subagent completed status with duration', () => {
    mockChatState.messages = [
      makeMessage({
        role: 'subagent' as UIChatMessage['role'],
        content: '',
        subagent: {
          subagentId: 'sa-2',
          status: 'completed',
          type: 'builder',
          description: 'Building code',
          duration: 3500,
        },
      }),
    ]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('builder')).toBeInTheDocument()
    expect(getByText('(3.5s)')).toBeInTheDocument()
  })

  it('renders permission message with danger styling', () => {
    mockChatState.messages = [
      makeMessage({
        role: 'permission' as UIChatMessage['role'],
        content: '',
        permission: {
          requestId: 'perm-1',
          tool: 'file_write',
          dangerLevel: 'dangerous',
          dangerReason: 'Writes to system files',
        },
      }),
    ]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Permission: file_write')).toBeInTheDocument()
    expect(getByText(/Writes to system files/)).toBeInTheDocument()
  })

  it('renders permission message with safe level', () => {
    mockChatState.messages = [
      makeMessage({
        role: 'permission' as UIChatMessage['role'],
        content: '',
        permission: { requestId: 'perm-2', tool: 'read_file', dangerLevel: 'safe', dangerReason: '' },
      }),
    ]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Permission: read_file')).toBeInTheDocument()
  })

  it('shows streaming cursor on streaming messages', () => {
    mockChatState.messages = [makeMessage({ role: 'assistant', content: 'Typing...', status: 'streaming' })]
    const { container } = render(<ChatWidget />)
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument()
  })

  it('renders action buttons on system messages', () => {
    mockChatState.messages = [
      makeMessage({
        role: 'system',
        content: 'Approve changes?',
        actions: [
          { id: 'approve', label: 'Approve', type: 'approve' },
          { id: 'reject', label: 'Reject', type: 'reject' },
        ],
      }),
    ]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Approve')).toBeInTheDocument()
    expect(getByText('Reject')).toBeInTheDocument()
  })

  it('calls handleAction when action button is clicked on system message', () => {
    const msg = makeMessage({
      id: 'msg-action',
      role: 'system',
      content: 'Approve?',
      actions: [{ id: 'approve', label: 'Approve', type: 'approve' }],
    })
    mockChatState.messages = [msg]
    const { getByText } = render(<ChatWidget />)
    getByText('Approve').click()
    expect(mockHandleAction).toHaveBeenCalledWith('msg-action', 'approve')
  })

  it('renders action buttons on assistant messages', () => {
    mockChatState.messages = [
      makeMessage({
        role: 'assistant',
        content: 'Result',
        actions: [{ id: 'retry', label: 'Retry', type: 'custom' }],
      }),
    ]
    const { getByText } = render(<ChatWidget />)
    expect(getByText('Retry')).toBeInTheDocument()
  })

  it('triggers command autocomplete when typing /', () => {
    mockGetAvailableCommands.mockReturnValue([
      { name: '/plan', description: 'Start planning', isAvailable: () => true },
    ])
    const { getByRole, getByText } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/pl', selectionStart: 3 } })
    expect(getByText('/plan')).toBeInTheDocument()
  })

  it('triggers file autocomplete when typing @', () => {
    const { getByRole, getByRole: getRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: 'check @src', selectionStart: 10 } })
    // The autocomplete dropdown should appear (listbox role)
    expect(getRole('listbox', { name: 'File suggestions' })).toBeInTheDocument()
  })

  it('closes autocomplete on Escape key', () => {
    mockGetAvailableCommands.mockReturnValue([
      { name: '/plan', description: 'Start planning', isAvailable: () => true },
    ])
    const { getByRole, queryByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/p', selectionStart: 2 } })
    expect(queryByRole('listbox')).toBeInTheDocument()
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('handles ArrowDown in autocomplete', () => {
    mockGetAvailableCommands.mockReturnValue([
      { name: '/plan', description: 'Plan', isAvailable: () => true },
      { name: '/review', description: 'Review', isAvailable: () => true },
    ])
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/', selectionStart: 1 } })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    // Second item should now be selected
    const secondOption = document.getElementById('autocomplete-item-1')
    expect(secondOption?.getAttribute('aria-selected')).toBe('true')
  })

  it('handles ArrowUp in autocomplete', () => {
    mockGetAvailableCommands.mockReturnValue([
      { name: '/plan', description: 'Plan', isAvailable: () => true },
      { name: '/review', description: 'Review', isAvailable: () => true },
    ])
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/', selectionStart: 1 } })
    // Move down then up
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'ArrowUp' })
    const firstOption = document.getElementById('autocomplete-item-0')
    expect(firstOption?.getAttribute('aria-selected')).toBe('true')
  })

  it('selects command on Tab key in autocomplete', () => {
    mockGetAvailableCommands.mockReturnValue([{ name: '/plan', description: 'Plan', isAvailable: () => true }])
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' }) as HTMLTextAreaElement
    fireEvent.change(input, { target: { value: '/p', selectionStart: 2 } })
    fireEvent.keyDown(input, { key: 'Tab' })
    expect(input.value).toBe('/plan ')
  })

  it('executes unknown slash command and shows error', async () => {
    mockParseCommand.mockReturnValue({ type: 'unknown', args: '', input: '/bogus' })
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/bogus', selectionStart: 6 } })
    // Escape to dismiss autocomplete, then submit via form
    fireEvent.keyDown(input, { key: 'Escape' })
    fireEvent.submit(input.closest('form')!)
    await vi.waitFor(() => {
      expect(mockAddMessage).toHaveBeenCalledWith(
        expect.objectContaining({ content: expect.stringContaining('Unknown command') }),
      )
    })
  })

  it('executes modal slash command and calls onModalCommand', async () => {
    const mockOnModal = vi.fn()
    const mockModal = { name: '/submit', description: 'Submit', modal: 'submit' as const, isAvailable: () => true }
    mockParseCommand.mockReturnValue({ type: 'modal', modalCommand: mockModal, args: '', input: '/submit' })
    const { getByRole } = render(<ChatWidget onModalCommand={mockOnModal} />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/submit', selectionStart: 7 } })
    fireEvent.keyDown(input, { key: 'Escape' })
    fireEvent.submit(input.closest('form')!)
    await vi.waitFor(() => {
      expect(mockOnModal).toHaveBeenCalledWith('submit')
    })
  })

  it('shows unavailable message for unavailable modal command', async () => {
    const mockModal = { name: '/submit', description: 'Submit', modal: 'submit' as const, isAvailable: () => false }
    mockParseCommand.mockReturnValue({ type: 'modal', modalCommand: mockModal, args: '', input: '/submit' })
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/submit', selectionStart: 7 } })
    fireEvent.keyDown(input, { key: 'Escape' })
    fireEvent.submit(input.closest('form')!)
    await vi.waitFor(() => {
      expect(mockAddMessage).toHaveBeenCalledWith(
        expect.objectContaining({ content: expect.stringContaining('not available') }),
      )
    })
  })

  it('executes action command successfully', async () => {
    const mockCmd = {
      name: '/plan',
      description: 'Plan',
      isAvailable: () => true,
      execute: vi.fn().mockResolvedValue('Plan created'),
    }
    mockParseCommand.mockReturnValue({ type: 'action', command: mockCmd, args: '', input: '/plan' })
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/plan', selectionStart: 5 } })
    fireEvent.keyDown(input, { key: 'Escape' })
    fireEvent.submit(input.closest('form')!)
    await vi.waitFor(() => {
      expect(mockCmd.execute).toHaveBeenCalledWith('')
      expect(mockAddMessage).toHaveBeenCalledWith(
        expect.objectContaining({ content: expect.stringContaining('Running /plan') }),
      )
    })
  })

  it('shows error when action command fails', async () => {
    const mockCmd = {
      name: '/plan',
      description: 'Plan',
      isAvailable: () => true,
      execute: vi.fn().mockRejectedValue(new Error('RPC failed')),
    }
    mockParseCommand.mockReturnValue({ type: 'action', command: mockCmd, args: '', input: '/plan' })
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/plan', selectionStart: 5 } })
    fireEvent.keyDown(input, { key: 'Escape' })
    fireEvent.submit(input.closest('form')!)
    await vi.waitFor(() => {
      expect(mockAddMessage).toHaveBeenCalledWith(
        expect.objectContaining({ content: expect.stringContaining('RPC failed'), status: 'error' }),
      )
    })
  })

  it('shows unavailable message for unavailable action command', async () => {
    const mockCmd = { name: '/plan', description: 'Plan', isAvailable: () => false, execute: vi.fn() }
    mockParseCommand.mockReturnValue({ type: 'action', command: mockCmd, args: '', input: '/plan' })
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/plan', selectionStart: 5 } })
    fireEvent.keyDown(input, { key: 'Escape' })
    fireEvent.submit(input.closest('form')!)
    await vi.waitFor(() => {
      expect(mockAddMessage).toHaveBeenCalledWith(
        expect.objectContaining({ content: expect.stringContaining('not available') }),
      )
    })
  })

  it('does not submit when input is empty', () => {
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(mockSendMessage).not.toHaveBeenCalled()
  })

  it('does not submit when isTyping is true', () => {
    mockChatState.isTyping = true
    const { getByRole } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' }) as HTMLTextAreaElement
    // Manually set value since input is disabled
    Object.defineProperty(input, 'value', { writable: true, value: 'test' })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(mockSendMessage).not.toHaveBeenCalled()
  })

  it('detects source shorthand patterns', () => {
    const { getByRole, getByText } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: 'github:org/repo#42', selectionStart: 18 } })
    expect(getByText(/Task source detected/)).toBeInTheDocument()
  })

  it('shows no matching commands message in command autocomplete', () => {
    mockGetAvailableCommands.mockReturnValue([])
    const { getByRole, getByText } = render(<ChatWidget />)
    const input = getByRole('combobox', { name: 'Message' })
    fireEvent.change(input, { target: { value: '/zzz', selectionStart: 4 } })
    expect(getByText('No matching commands')).toBeInTheDocument()
  })
})
