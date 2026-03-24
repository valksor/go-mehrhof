import { describe, it, expect } from 'vitest'
import { RPCError, isTransientError, type RPCErrorType } from './rpcError'

describe('RPCError', () => {
  it('sets name to RPCError', () => {
    const err = new RPCError('test', -32000)
    expect(err.name).toBe('RPCError')
  })

  it('extends Error', () => {
    const err = new RPCError('test', -32000)
    expect(err).toBeInstanceOf(Error)
  })

  it('stores message and code', () => {
    const err = new RPCError('something broke', -32603)
    expect(err.message).toBe('something broke')
    expect(err.code).toBe(-32603)
  })

  it.each<[number, RPCErrorType]>([
    [-32000, 'timeout'],
    [-32001, 'shutdown'],
    [-32002, 'rateLimited'],
    [-32003, 'state'],
    [-32004, 'notFound'],
    [-32601, 'notFound'],
    [-32603, 'general'],
    [-32700, 'general'],
    [-99999, 'general'],
    [0, 'general'],
  ])('classifies code %i as %s', (code, expectedType) => {
    const err = new RPCError('test', code)
    expect(err.type).toBe(expectedType)
  })
})

describe('isTransientError', () => {
  it.each<[RPCErrorType, boolean]>([
    ['timeout', true],
    ['rateLimited', true],
    ['connection', true],
    ['state', false],
    ['notFound', false],
    ['shutdown', false],
    ['general', false],
  ])('type %s returns %s', (type, expected) => {
    expect(isTransientError(type)).toBe(expected)
  })
})
