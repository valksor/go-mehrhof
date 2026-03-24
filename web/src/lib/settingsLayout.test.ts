import { describe, expect, it } from 'vitest'

import { getSectionFieldGroups } from './settingsLayout'
import type { Field, Section } from '../types/settings'

function makeField(path: string, overrides: Partial<Field> = {}): Field {
  return {
    path,
    type: 'string',
    label: path,
    ...overrides,
  }
}

describe('getSectionFieldGroups', () => {
  it('splits the agent section into agent-specific cards', () => {
    const section: Section = {
      id: 'agent',
      title: 'Agent',
      category: 'core',
      fields: [],
    }

    const groups = getSectionFieldGroups(section, [
      makeField('agent.default'),
      makeField('agent.allowed'),
      makeField('agent.openai.api_key', { sensitive: true, type: 'password' }),
      makeField('agent.openai.base_url'),
      makeField('agent.ollama.base_url'),
    ])

    expect(groups.map(group => group.id)).toEqual(['general', 'openai', 'ollama'])
    expect(groups[0].title).toBe('Agent selection')
    expect(groups[1].description).toContain('OpenAI')
    expect(groups[1].badge).toBe('API agent')
    expect(groups[1].fields.map(field => field.path)).toEqual([
      'agent.openai.api_key',
      'agent.openai.base_url',
    ])
  })

  it('separates provider defaults from provider-specific credentials', () => {
    const section: Section = {
      id: 'providers',
      title: 'Providers',
      category: 'providers',
      fields: [],
    }

    const groups = getSectionFieldGroups(section, [
      makeField('providers.default'),
      makeField('providers.github.token', { sensitive: true, type: 'password' }),
      makeField('providers.github.base_url'),
      makeField('providers.linear.token', { sensitive: true, type: 'password' }),
    ])

    expect(groups.map(group => group.id)).toEqual([
      'providers-general',
      'providers-github',
      'providers-linear',
    ])
    expect(groups[0].title).toBe('Provider defaults')
    expect(groups[1].title).toBe('GitHub')
    expect(groups[1].badge).toBe('Credentials')
    expect(groups[1].description).toContain('credentials, endpoint')
    expect(groups[2].description).toContain('credentials')
  })

  it('returns empty groups for agent section with no fields', () => {
    const section: Section = { id: 'agent', title: 'Agent', category: 'core', fields: [] }
    const groups = getSectionFieldGroups(section, [])
    expect(groups).toEqual([])
  })

  it('returns empty groups for providers section with no fields', () => {
    const section: Section = { id: 'providers', title: 'Providers', category: 'providers', fields: [] }
    const groups = getSectionFieldGroups(section, [])
    expect(groups).toEqual([])
  })

  it('puts unrecognized agent keys into the general group', () => {
    const section: Section = { id: 'agent', title: 'Agent', category: 'core', fields: [] }
    const groups = getSectionFieldGroups(section, [
      makeField('agent.unknown_thing'),
    ])
    expect(groups.map(g => g.id)).toEqual(['general'])
    expect(groups[0].fields.map(f => f.path)).toEqual(['agent.unknown_thing'])
  })

  it('puts unrecognized provider keys into the general group', () => {
    const section: Section = { id: 'providers', title: 'Providers', category: 'providers', fields: [] }
    const groups = getSectionFieldGroups(section, [
      makeField('providers.unknown'),
    ])
    expect(groups.map(g => g.id)).toEqual(['providers-general'])
    expect(groups[0].fields.map(f => f.path)).toEqual(['providers.unknown'])
  })

  it('handles a single agent field', () => {
    const section: Section = { id: 'agent', title: 'Agent', category: 'core', fields: [] }
    const groups = getSectionFieldGroups(section, [
      makeField('agent.ollama.base_url'),
    ])
    expect(groups.map(g => g.id)).toEqual(['ollama'])
    expect(groups[0].title).toBe('Ollama')
  })

  it('falls through to a flat group for non-agent non-provider sections', () => {
    const section: Section = { id: 'general', title: 'General', category: 'core', fields: [] }
    const groups = getSectionFieldGroups(section, [
      makeField('general.theme'),
      makeField('general.language'),
    ])
    expect(groups).toHaveLength(1)
    expect(groups[0].id).toBe('general')
    expect(groups[0].fields).toHaveLength(2)
  })
})