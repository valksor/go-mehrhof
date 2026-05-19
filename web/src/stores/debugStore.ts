import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { storeName } from '../meta'

export interface RPCLogEntry {
  id: number
  timestamp: Date
  direction: 'request' | 'response'
  method?: string
  data: string // truncated JSON
}

const MAX_LOG_ENTRIES = 200

let entryIdCounter = 0

interface DebugState {
  enabled: boolean
  logs: RPCLogEntry[]

  setEnabled: (enabled: boolean) => void
  addLog: (entry: Omit<RPCLogEntry, 'id' | 'timestamp'>) => void
  clearLogs: () => void
}

export const useDebugStore = create<DebugState>()(
  persist(
    (set) => ({
      enabled: false,
      logs: [],

      setEnabled: (enabled) => set({ enabled, logs: [] }),

      addLog: (entry) => {
        set((state) => {
          if (!state.enabled) return state
          const newEntry: RPCLogEntry = {
            ...entry,
            id: ++entryIdCounter,
            timestamp: new Date(),
          }
          const logs = [...state.logs, newEntry]
          // Keep only last MAX_LOG_ENTRIES
          if (logs.length > MAX_LOG_ENTRIES) {
            logs.splice(0, logs.length - MAX_LOG_ENTRIES)
          }
          return { logs }
        })
      },

      clearLogs: () => set({ logs: [] }),
    }),
    {
      name: storeName('debug'),
      partialize: (state) => ({ enabled: state.enabled }),
    },
  ),
)
