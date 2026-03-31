import { create } from 'zustand'
import type { ProjectState } from './projectStore.types'
import { createTaskSlice } from './projectStoreTask'
import { createFilesSlice } from './projectStoreFiles'
import { createUISlice } from './projectStoreUI'

// Re-export all types so existing imports from './projectStore' continue to work
export type {
  ProjectState,
  TaskState,
  ContextItem,
  Task,
  Checkpoint,
  FileChange,
  GitStatus,
  GitLogEntry,
  BrowseEntry,
  FilesEntry,
  CacheStats,
  Review,
  ReviewDetail,
  ReviewOptions,
  SubmitOptions,
  SubmitPreview,
  QueuedTask,
  SpecEntry,
  FinishOptions,
  FinishResult,
  RefreshResult,
  UpdateResult,
  RecapData,
  TaskSlice,
  FilesSlice,
  UISlice,
} from './projectStore.types'

export const useProjectStore = create<ProjectState>()((...a) => ({
  ...createTaskSlice(...a),
  ...createFilesSlice(...a),
  ...createUISlice(...a),
}))
