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
    (selector?: (s: Record<string, unknown>) => unknown) =>
      selector ? selector(mockProjectState) : mockProjectState,
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

vi.mock('../lib/export', () => ({
  downloadJSON: vi.fn(),
}))

vi.mock('../lib/chatCommands', () => ({
  parseCommand: () => null,
  getAvailableCommands: () => [],
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
})
