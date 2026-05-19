import type { Condition, Field } from '../types/settings'

/**
 * Get a nested value from an object using dot-notation path.
 * Example: getPath(obj, 'git.auto_commit') => obj.git.auto_commit
 */
export function getPath(obj: Record<string, unknown> | null | undefined, path: string): unknown {
  if (!obj) return undefined

  const keys = path.split('.')
  let current: unknown = obj

  for (const key of keys) {
    if (current == null || typeof current !== 'object') return undefined
    current = (current as Record<string, unknown>)[key]
  }

  return current
}

/**
 * Set a nested value in an object using dot-notation path (immutable).
 * Returns a new object with the value set.
 */
export function setPath(obj: Record<string, unknown>, path: string, value: unknown): Record<string, unknown> {
  const keys = path.split('.')
  const result = { ...obj }
  let current: Record<string, unknown> = result

  for (let i = 0; i < keys.length - 1; i++) {
    const key = keys[i]
    const existing = current[key]
    current[key] = existing && typeof existing === 'object' ? { ...(existing as Record<string, unknown>) } : {}
    current = current[key] as Record<string, unknown>
  }

  const finalKey = keys[keys.length - 1]
  current[finalKey] = value

  return result
}

/**
 * Evaluate a showWhen condition against current values.
 * Returns true if the field should be visible.
 */
export function evaluateShowWhen(condition: Condition | undefined, values: Record<string, unknown>): boolean {
  if (!condition) return true

  const fieldValue = getPath(values, condition.field)

  if (condition.equals !== undefined) {
    return fieldValue === condition.equals
  }
  if (condition.notEquals !== undefined) {
    return fieldValue !== condition.notEquals
  }

  // Default: show if field is truthy
  return Boolean(fieldValue)
}

/**
 * Validate a field value against its rules.
 * Returns error message or undefined if valid.
 */
export function validateField(field: Field, value: unknown): string | undefined {
  const rules = field.validation
  if (!rules) return undefined

  // Required check
  if (rules.required && isEmpty(value)) {
    return `${field.label} is required`
  }

  // String length check
  if (typeof value === 'string' && rules.maxLength) {
    if (value.length > rules.maxLength) {
      return `${field.label} must be at most ${rules.maxLength} characters`
    }
  }

  // Numeric range checks
  if (typeof value === 'number') {
    if (rules.min !== undefined && value < rules.min) {
      return `${field.label} must be at least ${rules.min}`
    }
    if (rules.max !== undefined && value > rules.max) {
      return `${field.label} must be at most ${rules.max}`
    }
  }

  // Pattern check
  if (typeof value === 'string' && rules.pattern) {
    const regex = new RegExp(rules.pattern)
    if (!regex.test(value)) {
      return rules.patternMessage || `${field.label} has invalid format`
    }
  }

  return undefined
}

/**
 * Check if a value is empty.
 */
function isEmpty(value: unknown): boolean {
  if (value === null || value === undefined) return true
  if (typeof value === 'string') return value.trim() === ''
  if (Array.isArray(value)) return value.length === 0
  return false
}

/**
 * Get the effective value for a field, considering defaults.
 */
export function getEffectiveValue(field: Field, value: unknown): unknown {
  if (value !== undefined && value !== null && value !== '') {
    return value
  }
  return field.default
}

/**
 * Validate all fields in a section.
 * Returns a map of path -> error message.
 */
export function validateSection(fields: Field[], values: Record<string, unknown>): Record<string, string> {
  const errors: Record<string, string> = {}

  for (const field of fields) {
    const value = getPath(values, field.path)
    const error = validateField(field, value)
    if (error) {
      errors[field.path] = error
    }
  }

  return errors
}

/**
 * Check if a token value is masked (contains ***).
 */
export function isMaskedToken(value: unknown): boolean {
  return typeof value === 'string' && value.includes('***')
}
