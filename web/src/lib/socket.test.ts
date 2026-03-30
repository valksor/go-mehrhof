import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { SocketClient } from './socket'

// Mock the debug store so socket.ts can import it
vi.mock('../stores/debugStore', () => ({
  useDebugStore: {
    getState: () => ({ enabled: false, addLog: vi.fn() }),
  },
}))

// Store instances for test access
let mockWsInstances: MockWebSocket[] = []

// Mock WebSocket as a class that can be used with `new`
class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  readyState = MockWebSocket.CONNECTING
  url: string

  onopen: (() => void) | null = null
  onclose: ((event: { code: number; reason: string }) => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null

  send = vi.fn()
  close = vi.fn(() => {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ code: 1000, reason: '' })
  })

  constructor(url: string) {
    this.url = url
    mockWsInstances.push(this)
  }

  // Test helpers
  simulateOpen() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.()
  }

  simulateMessage(data: string) {
    this.onmessage?.({ data })
  }

  simulateError() {
    this.onerror?.()
  }

  simulateClose(code = 1000, reason = '') {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ code, reason })
  }
}

// Stub WebSocket globally with our mock class
vi.stubGlobal('WebSocket', MockWebSocket)

describe('SocketClient', () => {
  let client: SocketClient

  beforeEach(() => {
    mockWsInstances = []
    client = new SocketClient('ws://test')
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  const getLatestWs = () => mockWsInstances[mockWsInstances.length - 1]

  describe('connect', () => {
    it('creates WebSocket with correct URL', () => {
      void client.connect()
      expect(getLatestWs().url).toBe('ws://test')
    })

    it('resolves on successful connection', async () => {
      const connectPromise = client.connect()
      getLatestWs().simulateOpen()
      await expect(connectPromise).resolves.toBeUndefined()
    })

    it('rejects on error', async () => {
      const connectPromise = client.connect()
      getLatestWs().simulateError()
      await expect(connectPromise).rejects.toThrow('WebSocket error')
    })
  })

  describe('call', () => {
    beforeEach(async () => {
      const p = client.connect()
      getLatestWs().simulateOpen()
      await p
    })

    it('sends JSON-RPC request', async () => {
      const callPromise = client.call('test.method', { foo: 'bar' })

      expect(getLatestWs().send).toHaveBeenCalledTimes(1)
      const sentData = getLatestWs().send.mock.calls[0][0]
      const parsed = JSON.parse(sentData.replace('\n', ''))

      expect(parsed.jsonrpc).toBe('2.0')
      expect(parsed.method).toBe('test.method')
      expect(parsed.params).toEqual({ foo: 'bar' })
      expect(parsed.id).toBeDefined()

      // Clean up by resolving
      getLatestWs().simulateMessage(`{"jsonrpc":"2.0","id":"${parsed.id}","result":{}}\n`)
      await callPromise
    })

    it('resolves with result on success', async () => {
      const callPromise = client.call('test.method')

      const sentData = getLatestWs().send.mock.calls[0][0]
      const { id } = JSON.parse(sentData.replace('\n', ''))

      getLatestWs().simulateMessage(`{"jsonrpc":"2.0","id":"${id}","result":{"success":true}}\n`)

      await expect(callPromise).resolves.toEqual({ success: true })
    })

    it('rejects on RPC error', async () => {
      const callPromise = client.call('test.fail')

      const sentData = getLatestWs().send.mock.calls[0][0]
      const { id } = JSON.parse(sentData.replace('\n', ''))

      getLatestWs().simulateMessage(`{"jsonrpc":"2.0","id":"${id}","error":{"code":-1,"message":"Test error"}}\n`)

      await expect(callPromise).rejects.toThrow('Test error')
    })

    it('rejects when not connected', async () => {
      client.close()
      await expect(client.call('test')).rejects.toThrow('Not connected')
    })

    it('increments request ID', async () => {
      // First call
      void client.call('test1')
      const firstSent = getLatestWs().send.mock.calls[0][0]
      const firstId = JSON.parse(firstSent.replace('\n', '')).id

      // Second call
      void client.call('test2')
      const secondSent = getLatestWs().send.mock.calls[1][0]
      const secondId = JSON.parse(secondSent.replace('\n', '')).id

      expect(Number(secondId)).toBe(Number(firstId) + 1)

      // Clean up pending calls
      getLatestWs().simulateMessage(`{"jsonrpc":"2.0","id":"${firstId}","result":{}}\n`)
      getLatestWs().simulateMessage(`{"jsonrpc":"2.0","id":"${secondId}","result":{}}\n`)
    })

    it('handles multiple concurrent calls', async () => {
      const call1 = client.call('method1')
      const call2 = client.call('method2')

      const sent1 = JSON.parse(getLatestWs().send.mock.calls[0][0].replace('\n', ''))
      const sent2 = JSON.parse(getLatestWs().send.mock.calls[1][0].replace('\n', ''))

      // Respond in reverse order
      getLatestWs().simulateMessage(`{"jsonrpc":"2.0","id":"${sent2.id}","result":"result2"}\n`)
      getLatestWs().simulateMessage(`{"jsonrpc":"2.0","id":"${sent1.id}","result":"result1"}\n`)

      await expect(call1).resolves.toBe('result1')
      await expect(call2).resolves.toBe('result2')
    })

    it('rejects pending calls on connection close', async () => {
      const callPromise = client.call('test')
      getLatestWs().simulateClose()
      await expect(callPromise).rejects.toThrow('Connection closed')
    })

    it('times out after 30 seconds', async () => {
      vi.useFakeTimers()

      const callPromise = client.call('slow.method')

      vi.advanceTimersByTime(30001)

      await expect(callPromise).rejects.toThrow('Request timeout: slow.method')

      vi.useRealTimers()
    })
  })

  describe('subscribe', () => {
    beforeEach(async () => {
      const p = client.connect()
      getLatestWs().simulateOpen()
      await p
    })

    it('calls handler for non-RPC messages', () => {
      const handler = vi.fn()
      client.subscribe(handler)

      // Send a streaming event (no matching pending id)
      getLatestWs().simulateMessage('{"type":"stream","content":"hello"}\n')

      expect(handler).toHaveBeenCalledWith({ type: 'stream', content: 'hello' })
    })

    it('does not call handler for RPC responses', async () => {
      const handler = vi.fn()
      client.subscribe(handler)

      const callPromise = client.call('test')
      const sentData = getLatestWs().send.mock.calls[0][0]
      const { id } = JSON.parse(sentData.replace('\n', ''))

      getLatestWs().simulateMessage(`{"jsonrpc":"2.0","id":"${id}","result":"done"}\n`)
      await callPromise

      expect(handler).not.toHaveBeenCalled()
    })
  })

  describe('message buffering', () => {
    beforeEach(async () => {
      const p = client.connect()
      getLatestWs().simulateOpen()
      await p
    })

    it('handles fragmented messages', () => {
      const handler = vi.fn()
      client.subscribe(handler)

      // Send fragmented message
      getLatestWs().simulateMessage('{"type":"te')
      expect(handler).not.toHaveBeenCalled()

      getLatestWs().simulateMessage('st"}\n')
      expect(handler).toHaveBeenCalledWith({ type: 'test' })
    })

    it('handles multiple messages in one chunk', () => {
      const handler = vi.fn()
      client.subscribe(handler)

      getLatestWs().simulateMessage('{"type":"msg1"}\n{"type":"msg2"}\n')

      expect(handler).toHaveBeenCalledTimes(2)
      expect(handler).toHaveBeenNthCalledWith(1, { type: 'msg1' })
      expect(handler).toHaveBeenNthCalledWith(2, { type: 'msg2' })
    })

    it('skips empty lines', () => {
      const handler = vi.fn()
      client.subscribe(handler)

      getLatestWs().simulateMessage('\n\n{"type":"msg"}\n\n')

      expect(handler).toHaveBeenCalledTimes(1)
    })

    it('clears buffer on disconnect', () => {
      const handler = vi.fn()
      client.subscribe(handler)

      // Start a fragmented message
      getLatestWs().simulateMessage('{"partial":')

      // Disconnect
      getLatestWs().simulateClose()

      // Reconnect and send new message
      void client.connect()
      getLatestWs().simulateOpen()

      // New message should work without old buffer
      getLatestWs().simulateMessage('{"type":"new"}\n')
      expect(handler).toHaveBeenCalledWith({ type: 'new' })
    })
  })

  describe('setOnDisconnect', () => {
    it('calls disconnect handler on close', async () => {
      const onDisconnect = vi.fn()
      client.setOnDisconnect(onDisconnect)

      const p = client.connect()
      getLatestWs().simulateOpen()
      await p

      getLatestWs().simulateClose()

      expect(onDisconnect).toHaveBeenCalled()
    })
  })

  describe('close', () => {
    it('closes the WebSocket', async () => {
      const p = client.connect()
      getLatestWs().simulateOpen()
      await p

      client.close()

      expect(getLatestWs().close).toHaveBeenCalled()
    })

    it('handles close when not connected', () => {
      // Should not throw
      expect(() => client.close()).not.toThrow()
    })
  })
})
