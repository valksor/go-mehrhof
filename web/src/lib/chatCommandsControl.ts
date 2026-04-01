import type { ChatCommand } from './chatCommandTypes'
import { getState, isActive } from './chatCommandTypes'

export const controlCommands: ChatCommand[] = [
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
      return s !== 'none' && s !== 'loaded' && s !== 'submitted'
    },
    execute: async () => {
      await getState().abort()
      return 'Operation aborted.'
    },
  },
  {
    name: '/reset',
    description: 'Reset task to initial state',
    isAvailable: () => isActive(),
    execute: async () => {
      await getState().reset()
      return 'Task reset.'
    },
  },
  {
    name: '/retry',
    description: 'Re-run failed phase',
    isAvailable: () => getState().state === 'failed',
    execute: async () => {
      await getState().retry()
      return 'Retrying failed phase.'
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
]
