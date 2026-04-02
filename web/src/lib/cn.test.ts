import { describe, it, expect } from 'vitest'
import { cn } from './cn'

describe('cn', () => {
  it('returns empty string for no arguments', () => {
    expect(cn()).toBe('')
  })

  it('passes through a single class', () => {
    expect(cn('foo')).toBe('foo')
  })

  it('joins multiple classes', () => {
    expect(cn('foo', 'bar')).toBe('foo bar')
  })

  it('filters out falsy values', () => {
    expect(cn('foo', false, null, undefined, 'bar')).toBe('foo bar')
  })

  it('handles conditional classes via object syntax', () => {
    expect(cn({ foo: true, bar: false, baz: true })).toBe('foo baz')
  })

  it('handles array syntax', () => {
    expect(cn(['foo', 'bar'])).toBe('foo bar')
  })

  it('merges conflicting tailwind classes (last wins)', () => {
    expect(cn('p-4', 'p-2')).toBe('p-2')
  })

  it('merges conflicting tailwind color classes', () => {
    expect(cn('text-red-500', 'text-blue-500')).toBe('text-blue-500')
  })

  it('keeps non-conflicting tailwind classes', () => {
    expect(cn('p-4', 'mx-2')).toBe('p-4 mx-2')
  })

  it('handles mixed inputs', () => {
    expect(cn('foo', { bar: true }, ['baz'])).toBe('foo bar baz')
  })
})
