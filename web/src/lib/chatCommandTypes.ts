import { useProjectStore } from '../stores/projectStore'
import { useGlobalStore } from '../stores/globalStore'

export interface ChatCommand {
  name: string
  description: string
  isAvailable: () => boolean
  execute: (args: string) => Promise<string>  // Returns result message
}

export function getState() {
  return useProjectStore.getState()
}

export function getGlobal() {
  return useGlobalStore.getState()
}

export function isActive() {
  return getState().state !== 'none'
}

export function worktreeClient() {
  return getState().client
}

export function globalClient() {
  return getGlobal().client
}
