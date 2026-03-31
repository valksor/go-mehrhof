import { workflowCommands } from './chatCommandsWorkflow'
import { controlCommands } from './chatCommandsControl'
import { inspectCommands } from './chatCommandsInspect'
import { organizationCommands } from './chatCommandsOrganization'
import { infraCommands } from './chatCommandsInfra'
import { getState, isActive } from './chatCommandTypes'

export type { ChatCommand } from './chatCommandTypes'

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

// ── All Commands ────────────────────────────────────────────────────────────

export const COMMANDS = [
  ...workflowCommands,
  ...controlCommands,
  ...inspectCommands,
  ...organizationCommands,
  ...infraCommands,
]

export interface ParsedCommand {
  type: 'action' | 'modal' | 'unknown'
  command?: (typeof COMMANDS)[number]
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

export function getAvailableCommands(filter: string): Array<(typeof COMMANDS)[number] | ModalCommandDef> {
  const query = filter.toLowerCase()
  const all: Array<(typeof COMMANDS)[number] | ModalCommandDef> = [...COMMANDS, ...MODAL_COMMANDS]
  return all.filter(cmd => {
    if (!cmd.isAvailable()) return false
    if (!query) return true
    return cmd.name.toLowerCase().includes(query) || cmd.description.toLowerCase().includes(query)
  })
}
