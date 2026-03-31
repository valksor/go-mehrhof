import { useChatStore } from '../stores/chatStore'
import type { ChatCommand } from './chatCommandTypes'
import { getState, isActive, worktreeClient, globalClient } from './chatCommandTypes'

export const inspectCommands: ChatCommand[] = [
  {
    name: '/status',
    description: 'Show current task state',
    isAvailable: () => true,
    execute: async () => {
      const { state, task } = getState()
      if (state === 'none') return 'No active task.'
      const title = task?.title ? ` — ${task.title}` : ''
      return `Current state: ${state}${title}`
    },
  },
  {
    name: '/explain',
    description: 'Ask agent to explain last action',
    isAvailable: () => {
      const s = getState().state
      return isActive() && s !== 'loaded'
    },
    execute: async () => {
      const wtId = getState().worktreeId
      await useChatStore.getState().sendMessage(
        'Explain what you did in the last action, why you made those choices, and any assumptions or constraints you encountered.',
        wtId || undefined
      )
      return ''
    },
  },
  {
    name: '/checkpoints',
    description: 'List git checkpoints',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ checkpoints: Array<{ sha: string; message: string; timestamp: string }> }>('checkpoints', {})
      const cps = result.checkpoints || []
      if (cps.length === 0) return 'No checkpoints.'
      return cps.map((cp, i) => `${i + 1}. ${cp.sha.slice(0, 7)} — ${cp.message}`).join('\n')
    },
  },
  {
    name: '/checkpoints goto',
    description: 'Jump to a checkpoint by SHA',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const sha = args.trim()
      if (!sha) return 'Usage: /checkpoints goto <sha>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      await client.call('checkpoint.goto', { sha })
      return `Jumped to checkpoint ${sha.slice(0, 7)}.`
    },
  },
  {
    name: '/recap',
    description: 'Summarize current task state for resuming',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ recap: string }>('recap', {})
      return result.recap || 'No recap available.'
    },
  },
  {
    name: '/diff',
    description: 'Show file changes from agent work',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ diff: string }>('git.diff', {})
      return result.diff || 'No changes.'
    },
  },
  {
    name: '/show spec',
    description: 'Show current specification',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ specifications: Array<{ content: string }> }>('show.spec', {})
      const specs = result.specifications || []
      if (specs.length === 0) return 'No specification available.'
      return specs.map(s => s.content).join('\n---\n\n')
    },
  },
  {
    name: '/show plan',
    description: 'Show current plan',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ plans: Array<{ content: string }> }>('show.plan', {})
      const plans = result.plans || []
      if (plans.length === 0) return 'No plan available.'
      return plans.map(s => s.content).join('\n---\n\n')
    },
  },
  {
    name: '/list search',
    description: 'Search task history',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const query = args.trim()
      if (!query) return 'Usage: /list search <query>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ tasks: Array<{ id: string; title: string; state: string }> }>('task.search', { query })
      const tasks = result.tasks || []
      if (tasks.length === 0) return 'No matching tasks.'
      return tasks.map(t => `${t.id.slice(0, 8)} [${t.state}] ${t.title}`).join('\n')
    },
  },
  {
    name: '/list',
    description: 'List task history',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ tasks: Array<{ id: string; title: string; state: string }> }>('task.history', {})
      const tasks = result.tasks || []
      if (tasks.length === 0) return 'No task history.'
      return tasks.map(t => `${t.id.slice(0, 8)} [${t.state}] ${t.title}`).join('\n')
    },
  },
  {
    name: '/eventlog',
    description: 'View task lifecycle events',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ events: Array<{ type: string; timestamp: string; message?: string }> }>('eventlog.query', { limit: 20 })
      const events = result.events || []
      if (events.length === 0) return 'No events.'
      return events.map(e => `[${e.timestamp}] ${e.type}${e.message ? ': ' + e.message : ''}`).join('\n')
    },
  },
  {
    name: '/jobs',
    description: 'List job queue',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ jobs: Array<{ id: string; type: string; status: string }> }>('jobs.list', {})
      const jobs = result.jobs || []
      if (jobs.length === 0) return 'No jobs.'
      return jobs.map(j => `${j.id.slice(0, 8)} [${j.status}] ${j.type}`).join('\n')
    },
  },
  {
    name: '/stats',
    description: 'Show real-time metrics',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<Record<string, unknown>>('metrics', {})
      return JSON.stringify(result, null, 2)
    },
  },
]
