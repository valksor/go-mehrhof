import { RPCError } from './rpcError'
import { useDebugStore } from '../stores/debugStore'

export interface RPCRequest {
  jsonrpc: '2.0'
  id: string
  method: string
  params?: unknown
}

export interface RPCResponse {
  jsonrpc: '2.0'
  id: string
  result?: unknown
  error?: { code: number; message: string }
}

export class SocketClient {
  private ws: WebSocket | null = null
  private pending = new Map<string, { resolve: (v: unknown) => void; reject: (e: Error) => void; method: string }>()
  private nextId = 1
  private messageHandlers = new Set<(data: unknown) => void>()
  private onDisconnect?: () => void
  private buffer = '' // Buffer for fragmented messages

  private url: string

  constructor(url: string) {
    this.url = url
  }

  setOnDisconnect(handler: () => void) {
    this.onDisconnect = handler
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.url)

      this.ws.onopen = () => resolve()
      this.ws.onerror = () => reject(new Error('WebSocket error'))

      this.ws.onmessage = (event) => {
        if (import.meta.env.DEV) {
          console.log('[SocketClient] Received message:', String(event.data).slice(0, 200))
        }
        // Append to buffer (handles fragmented messages)
        this.buffer += event.data

        // Process complete lines (newline-delimited JSON)
        const lines = this.buffer.split('\n')
        // Keep the last incomplete line in buffer
        this.buffer = lines.pop() || ''

        for (const line of lines) {
          if (!line.trim()) continue
          try {
            const msg = JSON.parse(line) as RPCResponse
            if (msg.id && this.pending.has(msg.id)) {
              const { resolve, reject, method } = this.pending.get(msg.id)!
              this.pending.delete(msg.id)

              // Debug logging for responses
              this.debugLog('response', line, method)

              if (msg.error) {
                reject(new RPCError(msg.error.message, msg.error.code))
              } else {
                resolve(msg.result)
              }
            } else if (this.messageHandlers.size > 0) {
              for (const handler of this.messageHandlers) {
                handler(msg)
              }
            }
          } catch (e) {
            console.error('[SocketClient] Parse error:', e, 'line:', line.slice(0, 100))
          }
        }
      }

      this.ws.onclose = (event) => {
        if (import.meta.env.DEV) {
          console.log('[SocketClient] WebSocket closed, code:', event.code, 'reason:', event.reason)
        }
        this.ws = null
        this.buffer = '' // Clear buffer on disconnect
        // Reject all pending calls
        for (const [id, { reject }] of this.pending) {
          reject(new RPCError('Connection closed', -1))
          this.pending.delete(id)
        }
        // Notify disconnection
        if (this.onDisconnect) {
          this.onDisconnect()
        }
      }
    })
  }

  call<T>(method: string, params?: unknown): Promise<T> {
    return new Promise((resolve, reject) => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        reject(new RPCError('Not connected', -1))
        return
      }

      const id = String(this.nextId++)
      const req: RPCRequest = {
        jsonrpc: '2.0',
        id,
        method,
        params
      }

      const timeoutId = setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id)
          reject(new RPCError(`Request timeout: ${method}`, -32000))
        }
      }, 30_000)

      this.pending.set(id, {
        resolve: (v: unknown) => { clearTimeout(timeoutId); (resolve as (v: unknown) => void)(v) },
        reject: (e: Error) => { clearTimeout(timeoutId); reject(e) },
        method
      })

      const payload = JSON.stringify(req) + '\n'

      // Debug logging for requests
      this.debugLog('request', payload, method)

      this.ws.send(payload)
    })
  }

  subscribe(handler: (data: unknown) => void): () => void {
    this.messageHandlers.add(handler)
    // Return unsubscribe function
    return () => {
      this.messageHandlers.delete(handler)
    }
  }

  close() {
    this.ws?.close()
  }

  /** Log RPC traffic when debug mode is enabled */
  private debugLog(direction: 'request' | 'response', data: string, method?: string) {
    const debugStore = useDebugStore.getState()
    if (!debugStore.enabled) return
    debugStore.addLog({
      direction,
      method,
      data: data.slice(0, 500),
    })
  }
}
