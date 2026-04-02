import { describe, it, expect } from 'vitest'
import { configSettingSchema, taskStartSchema } from './schemas'

describe('configSettingSchema', () => {
  it('accepts valid config setting', () => {
    const result = configSettingSchema.safeParse({
      key: 'agent.default',
      value: 'claude',
      scope: 'global',
    })
    expect(result.success).toBe(true)
  })

  it('rejects empty key', () => {
    const result = configSettingSchema.safeParse({
      key: '',
      value: 'val',
      scope: 'global',
    })
    expect(result.success).toBe(false)
  })

  it('rejects invalid scope', () => {
    const result = configSettingSchema.safeParse({
      key: 'test',
      value: 'val',
      scope: 'invalid',
    })
    expect(result.success).toBe(false)
  })

  it('accepts project scope', () => {
    const result = configSettingSchema.safeParse({
      key: 'test',
      value: 'val',
      scope: 'project',
    })
    expect(result.success).toBe(true)
  })

  it('allows empty value', () => {
    const result = configSettingSchema.safeParse({
      key: 'test',
      value: '',
      scope: 'global',
    })
    expect(result.success).toBe(true)
  })
})

describe('taskStartSchema', () => {
  it('accepts valid task start', () => {
    const result = taskStartSchema.safeParse({
      source: 'github#123',
    })
    expect(result.success).toBe(true)
  })

  it('rejects empty source', () => {
    const result = taskStartSchema.safeParse({
      source: '',
    })
    expect(result.success).toBe(false)
  })

  it('accepts optional description', () => {
    const result = taskStartSchema.safeParse({
      source: 'fix bug',
      description: 'Fix the login bug',
    })
    expect(result.success).toBe(true)
  })

  it('accepts optional tags', () => {
    const result = taskStartSchema.safeParse({
      source: 'fix bug',
      tags: 'urgent,frontend',
    })
    expect(result.success).toBe(true)
  })

  it('works without optional fields', () => {
    const result = taskStartSchema.safeParse({
      source: 'implement feature',
    })
    expect(result.success).toBe(true)
  })
})
