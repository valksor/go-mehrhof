import { useState, useRef, useEffect } from 'react'
import { useChatStore } from '../stores/chatStore'
import { ChatMessageContent } from './ChatMessage'

export function SimpleChatWidget() {
  const { messages, isTyping, sendMessage } = useChatStore()
  const [input, setInput] = useState('')
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, isTyping])

  // Auto-resize textarea
  useEffect(() => {
    const el = inputRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 120)}px`
  }, [input])

  const handleSend = async () => {
    const trimmed = input.trim()
    if (!trimmed || isTyping) return
    setInput('')
    try {
      await sendMessage(trimmed)
    } catch (err) {
      console.error('Failed to send message:', err)
      setInput(trimmed)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      void handleSend()
    }
  }

  return (
    <div className="flex flex-col h-full">
      {/* Messages */}
      <div className="flex-1 overflow-y-auto space-y-3 px-1 mb-3 max-h-64">
        {messages.length === 0 && !isTyping && (
          <p className="text-center text-base-content/40 text-sm py-4">
            Ask the AI a question or give instructions
          </p>
        )}
        {messages.map((msg) => (
          <div
            key={msg.id}
            className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
          >
            <div
              className={`rounded-xl px-3 py-2 max-w-[85%] text-sm ${
                msg.role === 'user'
                  ? 'bg-primary text-primary-content'
                  : 'bg-base-300 text-base-content'
              }`}
            >
              <ChatMessageContent content={msg.content} isUser={msg.role === 'user'} />
            </div>
          </div>
        ))}
        {isTyping && (
          <div className="flex justify-start">
            <div className="bg-base-300 rounded-xl px-3 py-2">
              <span className="loading loading-dots loading-xs"></span>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="flex gap-2 items-end">
        <textarea
          ref={inputRef}
          data-chat-input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Ask or instruct the AI..."
          className="textarea textarea-bordered flex-1 text-sm resize-none min-h-[2.5rem] max-h-[7.5rem]"
          rows={1}
          disabled={isTyping}
        />
        <button
          onClick={handleSend}
          disabled={!input.trim() || isTyping}
          className="btn btn-primary btn-sm"
          aria-label="Send message"
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
          </svg>
        </button>
      </div>
    </div>
  )
}
