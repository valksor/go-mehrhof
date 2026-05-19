import type { StateCreator } from 'zustand'
import type {
  ProjectState,
  FilesSlice,
  TaskState,
  Review,
  ReviewDetail,
  SpecEntry,
  BrowseEntry,
  FilesEntry,
  GitLogEntry,
} from './projectStore.types'

export const createFilesSlice: StateCreator<ProjectState, [], [], FilesSlice> = (set, get) => ({
  checkpoints: [],
  redoStack: [],
  gitStatus: null,
  fileChanges: [],
  reviews: [],
  reviewDetails: {},

  undo: async (steps: number = 1) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })

    try {
      const result = await client.call<{ status: string; state: TaskState; steps: number }>('undo', { steps })
      set({ state: result.state })
      get().appendOutput(`Undo complete (${result.steps} step${result.steps > 1 ? 's' : ''})`)
      await get().refreshStatus()
      set({ loading: false })
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Undo failed' })
    }
  },

  redo: async (steps: number = 1) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })

    try {
      const result = await client.call<{ status: string; state: TaskState; steps: number }>('redo', { steps })
      set({ state: result.state })
      get().appendOutput(`Redo complete (${result.steps} step${result.steps > 1 ? 's' : ''})`)
      await get().refreshStatus()
      set({ loading: false })
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Redo failed' })
    }
  },

  goToCheckpoint: async (sha: string) => {
    const client = get().client
    if (!client) return

    set({ loading: true, error: null })
    get().appendOutput(`Navigating to checkpoint ${sha.slice(0, 8)}...`)

    try {
      await client.call<{ status: string; sha: string }>('checkpoint.goto', { sha })
      get().appendOutput(`Restored to checkpoint ${sha.slice(0, 8)}`)
      await get().refreshStatus()
      set({ loading: false })
    } catch (err) {
      set({ loading: false, error: err instanceof Error ? err.message : 'Checkpoint navigation failed' })
    }
  },

  previewCheckpoint: async (sha: string) => {
    const client = get().client
    if (!client) return null

    try {
      return await client.call<{ diff: string; stat: string }>('checkpoint.preview', { sha })
    } catch {
      return null
    }
  },

  refreshGitStatus: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{
        branch: string
        has_changes: boolean
        files: Array<{ path: string; status: 'added' | 'modified' | 'deleted' | 'renamed' }>
      }>('git.status', {})
      set({
        gitStatus: {
          branch: result.branch,
          hasChanges: result.has_changes,
        },
        fileChanges: (result.files || []).map((f) => ({
          path: f.path,
          status: f.status,
        })),
      })
    } catch (err) {
      // Git status may not be available
      console.warn('Could not fetch git status:', err)
    }
  },

  getGitDiff: async (cached: boolean = false): Promise<string> => {
    const client = get().client
    if (!client) return ''

    try {
      const result = await client.call<{ diff: string }>('git.diff', { cached })
      return result.diff || ''
    } catch (err) {
      console.warn('Could not fetch git diff:', err)
      return ''
    }
  },

  getGitLog: async (count: number = 10): Promise<GitLogEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ entries: GitLogEntry[] }>('git.log', { count })
      return result.entries || []
    } catch (err) {
      console.warn('Could not fetch git log:', err)
      return []
    }
  },

  loadReviews: async () => {
    const client = get().client
    if (!client) return

    try {
      const result = await client.call<{ reviews: Review[] }>('review.list', {})
      set({ reviews: result.reviews || [] })
    } catch (err) {
      console.warn('Could not fetch review history:', err)
    }
  },

  loadReview: async (number: number): Promise<ReviewDetail | null> => {
    const client = get().client
    if (!client) return null

    // Return cached if available
    const cached = get().reviewDetails[number]
    if (cached) return cached

    try {
      const result = await client.call<ReviewDetail>('review.view', { number })
      set((state) => ({
        reviewDetails: { ...state.reviewDetails, [number]: result },
      }))
      return result
    } catch (err) {
      console.warn('Could not fetch review detail:', err)
      return null
    }
  },

  loadSpec: async (): Promise<SpecEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ specifications: SpecEntry[] }>('show.spec', {})
      return result.specifications || []
    } catch (err) {
      console.warn('Could not load specifications:', err)
      return []
    }
  },

  loadPlan: async (): Promise<SpecEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ specifications: SpecEntry[] }>('show.plan', {})
      return result.specifications || []
    } catch (err) {
      console.warn('Could not load plan:', err)
      return []
    }
  },

  getGitDiffAgainst: async (ref: string, stat: boolean = false): Promise<string> => {
    const client = get().client
    if (!client) return ''

    try {
      const result = await client.call<{ diff: string }>('git.diff_against', { ref, stat })
      return result.diff || ''
    } catch (err) {
      console.warn('Could not fetch diff against ref:', err)
      return ''
    }
  },

  browseFiles: async (path?: string, filesOnly: boolean = false): Promise<BrowseEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ entries: BrowseEntry[] }>('browse', { path, files: filesOnly })
      return result.entries || []
    } catch (err) {
      console.warn('Could not browse files:', err)
      return []
    }
  },

  listFiles: async (path?: string, extensions?: string[], maxDepth?: number): Promise<FilesEntry[]> => {
    const client = get().client
    if (!client) return []

    try {
      const result = await client.call<{ files: FilesEntry[] }>('files.list', {
        path,
        extensions,
        max_depth: maxDepth,
      })
      return result.files || []
    } catch (err) {
      console.warn('Could not list files:', err)
      return []
    }
  },
})
