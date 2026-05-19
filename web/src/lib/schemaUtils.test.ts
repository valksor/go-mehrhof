import { describe, it, expect } from 'vitest'
import {
  getPath,
  setPath,
  evaluateShowWhen,
  validateField,
  validateSection,
  getEffectiveValue,
  isMaskedToken,
} from './schemaUtils'
import type { Field, Condition } from '../types/settings'

describe('getPath', () => {
  it('returns undefined for null object', () => {
    expect(getPath(null, 'foo')).toBeUndefined()
  })

  it('returns undefined for undefined object', () => {
    expect(getPath(undefined, 'foo')).toBeUndefined()
  })

  it('returns top-level value', () => {
    expect(getPath({ foo: 'bar' }, 'foo')).toBe('bar')
  })

  it('returns nested value with dot notation', () => {
    expect(getPath({ git: { auto_commit: true } }, 'git.auto_commit')).toBe(true)
  })

  it('returns deeply nested value', () => {
    const obj = { a: { b: { c: { d: 'deep' } } } }
    expect(getPath(obj, 'a.b.c.d')).toBe('deep')
  })

  it('returns undefined for missing path', () => {
    expect(getPath({ foo: 'bar' }, 'missing')).toBeUndefined()
  })

  it('returns undefined for missing nested path', () => {
    expect(getPath({ git: {} }, 'git.missing.deep')).toBeUndefined()
  })

  it('returns undefined when intermediate is not object', () => {
    expect(getPath({ git: 'string' }, 'git.auto_commit')).toBeUndefined()
  })

  it('handles array values', () => {
    expect(getPath({ items: [1, 2, 3] }, 'items')).toEqual([1, 2, 3])
  })

  it('handles null nested values', () => {
    expect(getPath({ foo: null }, 'foo.bar')).toBeUndefined()
  })
})

describe('setPath', () => {
  it('sets top-level value', () => {
    const result = setPath({ foo: 'bar' }, 'foo', 'baz')
    expect(result.foo).toBe('baz')
  })

  it('is immutable - does not modify original', () => {
    const original = { foo: 'bar' }
    const result = setPath(original, 'foo', 'baz')
    expect(original.foo).toBe('bar')
    expect(result.foo).toBe('baz')
  })

  it('sets nested value creating intermediate objects', () => {
    const result = setPath({}, 'git.auto_commit', true)
    expect(result).toEqual({ git: { auto_commit: true } })
  })

  it('preserves sibling values', () => {
    const result = setPath({ git: { branch: 'main' } }, 'git.auto_commit', true)
    expect(result).toEqual({ git: { branch: 'main', auto_commit: true } })
  })

  it('preserves other top-level keys', () => {
    const result = setPath({ a: 1, b: 2 }, 'a', 10)
    expect(result).toEqual({ a: 10, b: 2 })
  })

  it('overwrites non-object intermediate with object', () => {
    const result = setPath({ git: 'string' }, 'git.auto_commit', true)
    expect(result).toEqual({ git: { auto_commit: true } })
  })

  it('handles deeply nested paths', () => {
    const result = setPath({}, 'a.b.c.d', 'value')
    expect(result).toEqual({ a: { b: { c: { d: 'value' } } } })
  })
})

describe('evaluateShowWhen', () => {
  it('returns true when no condition', () => {
    expect(evaluateShowWhen(undefined, {})).toBe(true)
  })

  it('evaluates equals condition - match', () => {
    const condition: Condition = { field: 'provider', equals: 'github' }
    expect(evaluateShowWhen(condition, { provider: 'github' })).toBe(true)
  })

  it('evaluates equals condition - no match', () => {
    const condition: Condition = { field: 'provider', equals: 'github' }
    expect(evaluateShowWhen(condition, { provider: 'gitlab' })).toBe(false)
  })

  it('evaluates equals with nested field', () => {
    const condition: Condition = { field: 'git.provider', equals: 'github' }
    expect(evaluateShowWhen(condition, { git: { provider: 'github' } })).toBe(true)
  })

  it('evaluates notEquals condition - match', () => {
    const condition: Condition = { field: 'mode', notEquals: 'off' }
    expect(evaluateShowWhen(condition, { mode: 'on' })).toBe(true)
  })

  it('evaluates notEquals condition - no match', () => {
    const condition: Condition = { field: 'mode', notEquals: 'off' }
    expect(evaluateShowWhen(condition, { mode: 'off' })).toBe(false)
  })

  it('defaults to truthy check when no operator - true', () => {
    const condition: Condition = { field: 'enabled' }
    expect(evaluateShowWhen(condition, { enabled: true })).toBe(true)
  })

  it('defaults to truthy check when no operator - false', () => {
    const condition: Condition = { field: 'enabled' }
    expect(evaluateShowWhen(condition, { enabled: false })).toBe(false)
  })

  it('defaults to truthy check - empty string is falsy', () => {
    const condition: Condition = { field: 'value' }
    expect(evaluateShowWhen(condition, { value: '' })).toBe(false)
  })

  it('defaults to truthy check - non-empty string is truthy', () => {
    const condition: Condition = { field: 'value' }
    expect(evaluateShowWhen(condition, { value: 'text' })).toBe(true)
  })

  it('returns false for missing field', () => {
    const condition: Condition = { field: 'missing' }
    expect(evaluateShowWhen(condition, {})).toBe(false)
  })
})

describe('validateField', () => {
  const makeField = (overrides: Partial<Field> = {}): Field => ({
    path: 'test',
    type: 'string',
    label: 'Test Field',
    ...overrides,
  })

  it('returns undefined when no validation rules', () => {
    const field = makeField()
    expect(validateField(field, 'any')).toBeUndefined()
  })

  describe('required validation', () => {
    const field = makeField({ validation: { required: true } })

    it('returns error for empty string', () => {
      expect(validateField(field, '')).toBe('Test Field is required')
    })

    it('returns error for whitespace-only string', () => {
      expect(validateField(field, '   ')).toBe('Test Field is required')
    })

    it('returns error for null', () => {
      expect(validateField(field, null)).toBe('Test Field is required')
    })

    it('returns error for undefined', () => {
      expect(validateField(field, undefined)).toBe('Test Field is required')
    })

    it('returns error for empty array', () => {
      expect(validateField(field, [])).toBe('Test Field is required')
    })

    it('passes for valid string', () => {
      expect(validateField(field, 'valid')).toBeUndefined()
    })

    it('passes for non-empty array', () => {
      expect(validateField(field, ['item'])).toBeUndefined()
    })
  })

  describe('maxLength validation', () => {
    const field = makeField({ validation: { maxLength: 5 } })

    it('returns error when exceeding maxLength', () => {
      expect(validateField(field, 'toolong')).toBe('Test Field must be at most 5 characters')
    })

    it('passes at exactly maxLength', () => {
      expect(validateField(field, 'exact')).toBeUndefined()
    })

    it('passes under maxLength', () => {
      expect(validateField(field, 'ok')).toBeUndefined()
    })

    it('ignores maxLength for non-strings', () => {
      expect(validateField(field, 123456)).toBeUndefined()
    })
  })

  describe('numeric min/max validation', () => {
    const field = makeField({
      type: 'number',
      validation: { min: 1, max: 10 },
    })

    it('returns error when below min', () => {
      expect(validateField(field, 0)).toBe('Test Field must be at least 1')
    })

    it('returns error when above max', () => {
      expect(validateField(field, 11)).toBe('Test Field must be at most 10')
    })

    it('passes at min boundary', () => {
      expect(validateField(field, 1)).toBeUndefined()
    })

    it('passes at max boundary', () => {
      expect(validateField(field, 10)).toBeUndefined()
    })

    it('passes within range', () => {
      expect(validateField(field, 5)).toBeUndefined()
    })

    it('ignores min/max for strings', () => {
      expect(validateField(field, 'not a number')).toBeUndefined()
    })
  })

  describe('pattern validation', () => {
    it('returns default error for invalid pattern', () => {
      const field = makeField({ validation: { pattern: '^[a-z]+$' } })
      expect(validateField(field, '123')).toBe('Test Field has invalid format')
    })

    it('returns custom error message', () => {
      const field = makeField({
        validation: { pattern: '^[^@]+@[^@]+$', patternMessage: 'Invalid email' },
      })
      expect(validateField(field, 'invalid')).toBe('Invalid email')
    })

    it('passes for matching pattern', () => {
      const field = makeField({ validation: { pattern: '^[a-z]+$' } })
      expect(validateField(field, 'valid')).toBeUndefined()
    })

    it('ignores pattern for non-strings', () => {
      const field = makeField({ validation: { pattern: '^[a-z]+$' } })
      expect(validateField(field, 123)).toBeUndefined()
    })
  })

  describe('combined validations', () => {
    it('checks required before other validations', () => {
      const field = makeField({
        validation: { required: true, maxLength: 5 },
      })
      expect(validateField(field, '')).toBe('Test Field is required')
    })
  })
})

describe('validateSection', () => {
  it('returns empty object when all fields valid', () => {
    const fields: Field[] = [
      { path: 'name', type: 'string', label: 'Name' },
      { path: 'email', type: 'string', label: 'Email' },
    ]
    const values = { name: 'John', email: 'john@example.com' }
    expect(validateSection(fields, values)).toEqual({})
  })

  it('returns errors for invalid fields', () => {
    const fields: Field[] = [
      { path: 'name', type: 'string', label: 'Name', validation: { required: true } },
      { path: 'count', type: 'number', label: 'Count', validation: { min: 1 } },
    ]
    const values = { name: '', count: 0 }
    expect(validateSection(fields, values)).toEqual({
      name: 'Name is required',
      count: 'Count must be at least 1',
    })
  })

  it('handles nested paths', () => {
    const fields: Field[] = [{ path: 'git.token', type: 'string', label: 'Token', validation: { required: true } }]
    const values = { git: { token: '' } }
    expect(validateSection(fields, values)).toEqual({
      'git.token': 'Token is required',
    })
  })

  it('handles missing nested paths', () => {
    const fields: Field[] = [{ path: 'git.token', type: 'string', label: 'Token', validation: { required: true } }]
    const values = {}
    expect(validateSection(fields, values)).toEqual({
      'git.token': 'Token is required',
    })
  })
})

describe('getEffectiveValue', () => {
  const makeField = (defaultValue: unknown): Field => ({
    path: 'test',
    type: 'string',
    label: 'Test',
    default: defaultValue,
  })

  it('returns value when defined', () => {
    expect(getEffectiveValue(makeField('default'), 'actual')).toBe('actual')
  })

  it('returns default when value is undefined', () => {
    expect(getEffectiveValue(makeField('default'), undefined)).toBe('default')
  })

  it('returns default when value is null', () => {
    expect(getEffectiveValue(makeField('default'), null)).toBe('default')
  })

  it('returns default when value is empty string', () => {
    expect(getEffectiveValue(makeField('default'), '')).toBe('default')
  })

  it('returns 0 when value is 0 (falsy but valid)', () => {
    expect(getEffectiveValue(makeField(10), 0)).toBe(0)
  })

  it('returns false when value is false (falsy but valid)', () => {
    expect(getEffectiveValue(makeField(true), false)).toBe(false)
  })

  it('returns undefined default when no default specified', () => {
    const field: Field = { path: 'test', type: 'string', label: 'Test' }
    expect(getEffectiveValue(field, undefined)).toBeUndefined()
  })
})

describe('isMaskedToken', () => {
  it('returns true for value containing ***', () => {
    expect(isMaskedToken('ghp_***abc')).toBe(true)
  })

  it('returns true for just ***', () => {
    expect(isMaskedToken('***')).toBe(true)
  })

  it('returns true for *** at end', () => {
    expect(isMaskedToken('token***')).toBe(true)
  })

  it('returns false for unmasked token', () => {
    expect(isMaskedToken('ghp_realtoken123')).toBe(false)
  })

  it('returns false for empty string', () => {
    expect(isMaskedToken('')).toBe(false)
  })

  it('returns false for non-string number', () => {
    expect(isMaskedToken(123)).toBe(false)
  })

  it('returns false for non-string null', () => {
    expect(isMaskedToken(null)).toBe(false)
  })

  it('returns false for non-string undefined', () => {
    expect(isMaskedToken(undefined)).toBe(false)
  })

  it('returns false for non-string object', () => {
    expect(isMaskedToken({ token: '***' })).toBe(false)
  })
})
