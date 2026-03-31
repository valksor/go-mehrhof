import type { ChatCommand } from './chatCommandTypes'
import { getState } from './chatCommandTypes'

export const workflowCommands: ChatCommand[] = [
  {
    name: '/quick',
    description: 'Quick fix: load and implement, skipping planning',
    isAvailable: () => getState().state === 'none',
    execute: async (args) => {
      const source = args.trim()
      if (!source) return 'Usage: /quick <source> (e.g. github:owner/repo#123 or file:task.md)'
      await getState().quickStart(source)
      return 'Quick fix started — skipping plan, auto-advancing through implement and review.'
    },
  },
  {
    name: '/plan',
    description: 'Run planning phase',
    isAvailable: () => getState().state === 'loaded',
    execute: async () => {
      await getState().plan()
      return 'Planning started.'
    },
  },
  {
    name: '/plan!',
    description: 'Re-run planning',
    isAvailable: () => getState().state === 'planned',
    execute: async () => {
      await getState().plan()
      return 'Re-planning started.'
    },
  },
  {
    name: '/implement',
    description: 'Run implementation phase',
    isAvailable: () => getState().state === 'planned',
    execute: async () => {
      await getState().implement()
      return 'Implementation started.'
    },
  },
  {
    name: '/implement!',
    description: 'Re-run implementation',
    isAvailable: () => getState().state === 'implemented',
    execute: async () => {
      await getState().implement()
      return 'Re-implementation started.'
    },
  },
  {
    name: '/simplify',
    description: 'Run code simplification pass',
    isAvailable: () => getState().state === 'implemented',
    execute: async () => {
      await getState().simplify()
      return 'Simplification started.'
    },
  },
  {
    name: '/optimize',
    description: 'Run optimization pass',
    isAvailable: () => getState().state === 'implemented',
    execute: async () => {
      await getState().optimize()
      return 'Optimization started.'
    },
  },
  {
    name: '/review',
    description: 'Review and approve implementation',
    isAvailable: () => getState().state === 'implemented',
    execute: async () => {
      await getState().review({ approve: true })
      return 'Review started.'
    },
  },
  {
    name: '/review fix',
    description: 'Review with automatic fixes',
    isAvailable: () => getState().state === 'implemented',
    execute: async () => {
      await getState().review({ fix: true })
      return 'Review with fixes started.'
    },
  },
]
