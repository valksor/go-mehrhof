import { useProjectStore } from '../stores/projectStore'
import { useChatStore } from '../stores/chatStore'

export interface ChatCommand {
  name: string
  description: string
  isAvailable: () => boolean
  execute: (args: string) => Promise<string>  // Returns result message
}

function getState() {
  return useProjectStore.getState()
}

function isActive() {
  return getState().state !== 'none'
}

export const COMMANDS: ChatCommand[] = [
  {
    name: '/quick',
    description: 'Quick fix: load, implement, and submit in one step',
    isAvailable: () => getState().state === 'none',
    execute: async (args) => {
      const source = args.trim()
      if (!source) return 'Usage: /quick <source> (e.g. github:owner/repo#123 or file:task.md)'
      await getState().quickStart(source)
      return 'Quick fix started — will auto-advance through implement and submit.'
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
  {
    name: '/undo',
    description: 'Undo to previous checkpoint',
    isAvailable: () => getState().checkpoints.length > 0,
    execute: async () => {
      await getState().undo()
      return 'Undone to previous checkpoint.'
    },
  },
  {
    name: '/redo',
    description: 'Redo to next checkpoint',
    isAvailable: () => getState().redoStack.length > 0,
    execute: async () => {
      await getState().redo()
      return 'Redone to next checkpoint.'
    },
  },
  {
    name: '/stop',
    description: 'Stop current operation (preserves state)',
    isAvailable: () => {
      const s = getState().state
      return ['planning', 'implementing', 'simplifying', 'optimizing', 'reviewing'].includes(s)
    },
    execute: async () => {
      await getState().stop()
      return 'Operation stopped.'
    },
  },
  {
    name: '/abort',
    description: 'Abort current operation',
    isAvailable: () => {
      const s = getState().state
      return s !== 'none' && s !== 'submitted'
    },
    execute: async () => {
      await getState().abort()
      return 'Operation aborted.'
    },
  },
  {
    name: '/update',
    description: 'Update task from source',
    isAvailable: () => {
      const s = getState().state
      return s === 'loaded' || s === 'planned' || s === 'implemented'
    },
    execute: async () => {
      const result = await getState().update()
      if (result.changed) {
        return result.specification_generated
          ? 'Task updated from source — new specification generated.'
          : 'Task content updated from source.'
      }
      return 'Task is already up to date.'
    },
  },
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
      return '' // sendMessage handles the chat flow
    },
  },
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
]

// Returns modal ID if the command should open a modal instead of executing directly.
// The ChatWidget handles these specially.
export type ModalCommand = 'submit' | 'finish' | 'abandon' | 'delete'

export interface ModalCommandDef {
  name: string
  description: string
  modal: ModalCommand
  isAvailable: () => boolean
}

export const MODAL_COMMANDS: ModalCommandDef[] = [
  {
    name: '/submit',
    description: 'Submit pull request',
    modal: 'submit',
    isAvailable: () => getState().state === 'reviewing',
  },
  {
    name: '/finish',
    description: 'Finish and clean up after merge',
    modal: 'finish',
    isAvailable: () => getState().state === 'submitted',
  },
  {
    name: '/abandon',
    description: 'Abandon current task',
    modal: 'abandon',
    isAvailable: () => isActive(),
  },
  {
    name: '/delete',
    description: 'Delete task permanently',
    modal: 'delete',
    isAvailable: () => isActive(),
  },
]

export interface ParsedCommand {
  type: 'action' | 'modal' | 'unknown'
  command?: ChatCommand
  modalCommand?: ModalCommandDef
  args: string
  input: string
}

export function parseCommand(input: string): ParsedCommand | null {
  if (!input.startsWith('/')) return null

  // Try modal commands first (they have priority for exact matches)
  for (const mc of MODAL_COMMANDS) {
    if (input === mc.name || input.startsWith(mc.name + ' ')) {
      return {
        type: 'modal',
        modalCommand: mc,
        args: input.slice(mc.name.length).trim(),
        input,
      }
    }
  }

  // Try action commands — match longest first to handle "/review fix" vs "/review"
  const sorted = [...COMMANDS].sort((a, b) => b.name.length - a.name.length)
  for (const cmd of sorted) {
    if (input === cmd.name || input.startsWith(cmd.name + ' ')) {
      return {
        type: 'action',
        command: cmd,
        args: input.slice(cmd.name.length).trim(),
        input,
      }
    }
  }

  return { type: 'unknown', args: '', input }
}

export function getAvailableCommands(filter: string): Array<ChatCommand | ModalCommandDef> {
  const query = filter.toLowerCase()
  const all: Array<ChatCommand | ModalCommandDef> = [...COMMANDS, ...MODAL_COMMANDS]
  return all.filter(cmd => {
    if (!cmd.isAvailable()) return false
    if (!query) return true
    return cmd.name.toLowerCase().includes(query) || cmd.description.toLowerCase().includes(query)
  })
}
