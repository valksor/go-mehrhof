import type { Field, Section } from '../types/settings'

export interface FieldGroup {
  id: string
  title: string
  description: string
  badge?: string
  fields: Field[]
}

const agentGroupMeta: Record<string, Omit<FieldGroup, 'fields'>> = {
  general: {
    id: 'general',
    title: 'Agent selection',
    description: 'Choose which agents are available and how kvelmo routes work by default.',
  },
  consensus: {
    id: 'consensus',
    title: 'Consensus review',
    description: 'Control multi-agent review behavior and agreement thresholds.',
    badge: 'Review',
  },
  openai: {
    id: 'openai',
    title: 'OpenAI',
    description: 'API credentials, endpoint, and model for the OpenAI-backed agent.',
    badge: 'API agent',
  },
  anthropic: {
    id: 'anthropic',
    title: 'Anthropic',
    description: 'API credentials, endpoint, and model for the Anthropic-backed agent.',
    badge: 'API agent',
  },
  ollama: {
    id: 'ollama',
    title: 'Ollama',
    description: 'Server URL and model for the local Ollama agent.',
    badge: 'Local agent',
  },
}

const providerNames: Record<string, string> = {
  github: 'GitHub',
  gitlab: 'GitLab',
  wrike: 'Wrike',
  linear: 'Linear',
  jira: 'Jira',
  azuredevops: 'Azure DevOps',
  file: 'File',
}

export function getSectionFieldGroups(section: Section, fields: Field[]): FieldGroup[] {
  if (section.id === 'agent') {
    return buildAgentGroups(fields)
  }

  if (section.id === 'providers') {
    return buildProviderGroups(fields)
  }

  return [{
    id: section.id,
    title: section.title,
    description: section.description ?? '',
    fields,
  }]
}

const agentGroupOrder = ['general', 'consensus', 'openai', 'anthropic', 'ollama']

function buildAgentGroups(fields: Field[]): FieldGroup[] {
  const groups = new Map<string, Field[]>()

  for (const field of fields) {
    const parts = field.path.split('.')
    const key = parts[1] ?? 'general'
    const groupId = key === 'consensus' || key === 'openai' || key === 'anthropic' || key === 'ollama'
      ? key
      : 'general'

    if (!groups.has(groupId)) groups.set(groupId, [])
    groups.get(groupId)!.push(field)
  }

  return agentGroupOrder
    .filter(groupId => groups.has(groupId))
    .map(groupId => {
      const groupFields = groups.get(groupId)!
      const meta = agentGroupMeta[groupId]!

      return {
        ...meta,
        fields: groupFields,
      }
    })
}

const providerOrder = Object.keys(providerNames)

function buildProviderGroups(fields: Field[]): FieldGroup[] {
  const generalFields: Field[] = []
  const providerGroups = new Map<string, Field[]>()

  for (const field of fields) {
    const parts = field.path.split('.')
    const providerKey = parts[1]

    if (!providerKey || !providerNames[providerKey]) {
      generalFields.push(field)
      continue
    }

    if (!providerGroups.has(providerKey)) providerGroups.set(providerKey, [])
    providerGroups.get(providerKey)!.push(field)
  }

  const groups: FieldGroup[] = []

  if (generalFields.length > 0) {
    groups.push({
      id: 'providers-general',
      title: 'Provider defaults',
      description: 'Pick the task source kvelmo should prefer when a task does not specify one.',
      fields: generalFields,
    })
  }

  for (const providerKey of providerOrder) {
    const groupFields = providerGroups.get(providerKey)
    if (!groupFields) continue

    const title = providerNames[providerKey]
    const hasCredential = groupFields.some(field => field.sensitive)
    const hasEndpoint = groupFields.some(field => field.path.endsWith('.base_url'))

    let description = `${title} connection details and project/task import settings.`
    if (hasCredential && hasEndpoint) {
      description = `${title} credentials, endpoint, and import behavior.`
    } else if (hasCredential) {
      description = `${title} credentials and import behavior.`
    } else if (hasEndpoint) {
      description = `${title} endpoint and import behavior.`
    }

    groups.push({
      id: `providers-${providerKey}`,
      title,
      description,
      badge: hasCredential ? 'Credentials' : undefined,
      fields: groupFields,
    })
  }

  return groups
}
