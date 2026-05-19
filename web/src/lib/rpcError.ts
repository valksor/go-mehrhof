/**
 * Typed RPC error classification for the socket layer.
 */

export type RPCErrorType = 'connection' | 'timeout' | 'state' | 'notFound' | 'rateLimited' | 'shutdown' | 'general'

/**
 * An error carrying an RPC error type classification.
 */
export class RPCError extends Error {
  readonly type: RPCErrorType
  readonly code: number

  constructor(message: string, code: number) {
    super(message)
    this.name = 'RPCError'
    this.code = code
    this.type = classifyErrorCode(code)
  }
}

/**
 * Map JSON-RPC error codes to semantic error types.
 *
 * Standard JSON-RPC codes:
 *   -32700  Parse error
 *   -32600  Invalid request
 *   -32601  Method not found
 *   -32602  Invalid params
 *   -32603  Internal error
 *
 * Application codes (kvelmo):
 *   -32000  Timeout
 *   -32001  Shutdown
 *   -32002  Rate limited
 *   -32003  State error (invalid transition)
 *   -32004  Not found
 */
function classifyErrorCode(code: number): RPCErrorType {
  switch (code) {
    case -32000:
      return 'timeout'
    case -32001:
      return 'shutdown'
    case -32002:
      return 'rateLimited'
    case -32003:
      return 'state'
    case -32004:
    case -32601: // Method not found
      return 'notFound'
    default:
      return 'general'
  }
}

/**
 * Whether this error type is transient (should be shown as a toast, not inline).
 */
export function isTransientError(type: RPCErrorType): boolean {
  return type === 'timeout' || type === 'rateLimited' || type === 'connection'
}
