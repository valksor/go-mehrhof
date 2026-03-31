import type { ChatCommand } from './chatCommandTypes'
import { getState, getGlobal, isActive, globalClient } from './chatCommandTypes'

async function listQueue(): Promise<string> {
  await getState().loadQueue()
  const queue = getState().taskQueue
  if (queue.length === 0) return 'Queue is empty.'
  return queue.map((t, i) => `${i + 1}. [${t.id.slice(0, 8)}] ${t.title || t.source}`).join('\n')
}

export const organizationCommands: ChatCommand[] = [
  {
    name: '/tag add',
    description: 'Add a tag to the task',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const tag = args.trim()
      if (!tag) return 'Usage: /tag add <name>'
      const client = getState().client
      if (!client) return 'Not connected.'
      await client.call('task.tag', { action: 'add', tag })
      return `Tag "${tag}" added.`
    },
  },
  {
    name: '/tag remove',
    description: 'Remove a tag from the task',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const tag = args.trim()
      if (!tag) return 'Usage: /tag remove <name>'
      const client = getState().client
      if (!client) return 'Not connected.'
      await client.call('task.tag', { action: 'remove', tag })
      return `Tag "${tag}" removed.`
    },
  },
  {
    name: '/tags',
    description: 'List current tags',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = getState().client
      if (!client) return 'Not connected.'
      const result = await client.call<{ tags: string[] }>('task.tag', { action: 'list' })
      const tags = result.tags || []
      return tags.length > 0 ? `Tags: ${tags.join(', ')}` : 'No tags.'
    },
  },
  {
    name: '/queue add',
    description: 'Add task to queue',
    isAvailable: () => true,
    execute: async (args) => {
      const source = args.trim()
      if (!source) return 'Usage: /queue add <source>'
      await getState().queueTask(source)
      return `Queued: ${source}`
    },
  },
  {
    name: '/queue remove',
    description: 'Remove task from queue',
    isAvailable: () => true,
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /queue remove <id>'
      await getState().dequeueTask(id)
      return `Removed ${id} from queue.`
    },
  },
  {
    name: '/queue list',
    description: 'List queued tasks',
    isAvailable: () => true,
    execute: listQueue,
  },
  {
    name: '/queue reorder',
    description: 'Move a queued task to a new position',
    isAvailable: () => true,
    execute: async (args) => {
      const parts = args.trim().split(/\s+/)
      if (parts.length !== 2) return 'Usage: /queue reorder <id> <position>'
      const [id, posStr] = parts
      const position = parseInt(posStr, 10)
      if (isNaN(position) || position < 1) return 'Position must be a positive number.'
      await getState().reorderQueue(id, position)
      return `Moved ${id} to position ${position}.`
    },
  },
  {
    name: '/queue',
    description: 'List queued tasks',
    isAvailable: () => true,
    execute: listQueue,
  },
  {
    name: '/fork create',
    description: 'Create a conversation fork',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const label = args.trim()
      if (!label) return 'Usage: /fork create <label>'
      await getState().createFork(label)
      return `Fork "${label}" created.`
    },
  },
  {
    name: '/fork list',
    description: 'List active forks',
    isAvailable: () => isActive(),
    execute: async () => {
      await getState().listForks()
      const forks = getState().forks
      if (forks.length === 0) return 'No active forks.'
      return forks.map(f => `${f.id.slice(0, 8)} — ${f.label} [${f.state}]`).join('\n')
    },
  },
  {
    name: '/fork compare',
    description: 'Compare forks',
    isAvailable: () => isActive(),
    execute: async () => {
      const result = await getState().compareForks()
      return JSON.stringify(result, null, 2)
    },
  },
  {
    name: '/fork select',
    description: 'Select a fork',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /fork select <fork-id>'
      await getState().selectFork(id)
      return `Switched to fork ${id.slice(0, 8)}.`
    },
  },
  {
    name: '/group create',
    description: 'Create cross-repo task group',
    isAvailable: () => true,
    execute: async (args) => {
      const label = args.trim()
      if (!label) return 'Usage: /group create <label>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ id: string }>('taskgroup.create', { label })
      return `Group created: ${result.id}`
    },
  },
  {
    name: '/group list',
    description: 'List task groups',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ groups: Array<{ id: string; label: string }> }>('taskgroup.list', {})
      const groups = result.groups || []
      if (groups.length === 0) return 'No groups.'
      return groups.map(g => `${g.id.slice(0, 8)} — ${g.label}`).join('\n')
    },
  },
  {
    name: '/group status',
    description: 'Show group status',
    isAvailable: () => true,
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /group status <group-id>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<Record<string, unknown>>('taskgroup.status', { id })
      return JSON.stringify(result, null, 2)
    },
  },
  {
    name: '/group add',
    description: 'Add task to group',
    isAvailable: () => true,
    execute: async (args) => {
      const parts = args.trim().split(/\s+/)
      if (parts.length < 2) return 'Usage: /group add <group-id> <task-id>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('taskgroup.add', { id: parts[0], task_id: parts[1] })
      return `Task added to group ${parts[0].slice(0, 8)}.`
    },
  },
  {
    name: '/group submit',
    description: 'Submit grouped tasks',
    isAvailable: () => true,
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /group submit <group-id>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('taskgroup.submit', { id })
      return `Group ${id.slice(0, 8)} submitted.`
    },
  },
  {
    name: '/group remove',
    description: 'Delete a task group',
    isAvailable: () => true,
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /group remove <group-id>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('taskgroup.remove', { id })
      return `Group ${id.slice(0, 8)} removed.`
    },
  },
  {
    name: '/batch',
    description: 'Run action across all active projects',
    isAvailable: () => true,
    execute: async (args) => {
      const action = args.trim()
      if (!action) return 'Usage: /batch <action> (plan, implement, review, submit, abort, reset, stop)'
      const result = await getGlobal().batchAction(action)
      return `Batch ${action}: ${result.succeeded}/${result.total} succeeded.`
    },
  },
]
