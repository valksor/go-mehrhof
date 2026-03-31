import type { SocketClient } from '../lib/socket'
import type { FailureClass } from '../lib/events'
import type { PhaseMetrics, ForkInfo } from '../types/conductor'

export type TaskState =
  | 'none'
  | 'loaded'
  | 'planning'
  | 'planned'
  | 'implementing'
  | 'implemented'
  | 'simplifying'
  | 'optimizing'
  | 'reviewing'
  | 'submitted'
  | 'failed'
  | 'waiting'
  | 'paused'

export interface ContextItem {
  type: string
  ref: string
  label: string
}

export interface Task {
  id: string
  title: string
  state: TaskState
  source: string
  description?: string
  branch?: string
  worktreePath?: string
  contextItems?: ContextItem[]
}

export interface Checkpoint {
  sha: string
  message: string
  timestamp: string
}

export interface FileChange {
  path: string
  status: 'added' | 'modified' | 'deleted' | 'renamed'
}

export interface GitStatus {
  branch: string
  hasChanges: boolean
}

export interface GitLogEntry {
  sha: string
  message: string
  author: string
  date: string
}

export interface BrowseEntry {
  name: string
  path: string
  is_dir: boolean
  size?: number
  modified?: string
}

export interface FilesEntry {
  path: string
  size?: number
  modified?: string
}

export interface CacheStats {
  enabled: boolean
  entries: number
  hits: number
  misses: number
  hit_rate: number
  tokens_saved: number
}

export interface Review {
  number: number
  timestamp: string
  approved: boolean
  message: string
}

export interface ReviewDetail {
  number: number
  timestamp: string
  approved: boolean
  message: string
  content: string
  findings: string[]
}

export interface ReviewOptions {
  approve?: boolean
  reject?: boolean
  message?: string
  fix?: boolean
}

export interface SubmitOptions {
  title?: string
  body?: string
  draft?: boolean
  reviewers?: string[]
  labels?: string[]
  delete_branch?: boolean
  dry_run?: boolean
  sections?: Record<string, string>
}

export interface SubmitPreview {
  title: string
  body: string
  branch: string
  base_branch: string
  diff_stat?: string
  checkpoints: number
  specifications: number
}

export interface QueuedTask {
  id: string
  source: string
  title: string
  added_at: string
  position: number
}

export interface SpecEntry {
  path: string
  content: string
}

export interface FinishOptions {
  delete_remote?: boolean
  force?: boolean
}

export interface FinishResult {
  previous_branch: string
  current_branch: string
  branch_deleted: boolean
  remote_branch_deleted: boolean
}

export interface RefreshResult {
  task_id: string
  branch: string
  pr_status: string
  pr_merged: boolean
  pr_url: string
  commits_behind_base: number
  action: string
  message: string
}

export interface UpdateResult {
  changed: boolean
  specification_generated: boolean
}

export interface RecapData {
  state: TaskState
  path: string
  task: { id: string; title: string; source: string; branch?: string } | null
  last_checkpoint: { sha: string; message: string; timestamp: string } | null
  checkpoint_count: number
  files_changed: FileChange[] | undefined
  phase_metrics: Record<string, PhaseMetrics> | null
  tags: string[]
  last_activity: string
  next_action: string
  last_error: string
}

// Slice interfaces — each slice owns a subset of the full state

export interface TaskSlice {
  // Task state
  task: Task | null
  state: TaskState

  // Task queue
  taskQueue: QueuedTask[]

  // Quality gate prompt
  qualityPrompt: { id: string; question: string } | null

  // Phase failure classification
  phaseError: { message: string; class: FailureClass; phase: string } | null

  // Phase metrics
  phaseMetrics: Record<string, PhaseMetrics> | null

  // Recovery state
  needsRecovery: string | null

  // Skip phases
  skipPhases: string[]

  // Tags
  tags: string[]

  // Pending graph node approvals
  pendingNodeApprovals: { nodeId: string; message: string }[]

  // Risk evaluation
  riskScore: { score: number; factors: Record<string, number>; level: string } | null
  evaluateRisk: () => Promise<void>

  // CI fix loop status
  ciFixStatus: { active: boolean; attempt?: number; maxAttempts?: number; result?: 'success' | 'failed' } | null

  // Quality gate auto-fix loop status
  autoFixStatus: { active: boolean; attempt?: number; maxAttempts?: number; result?: 'success' | 'failed' } | null

  // Phase progress estimation
  phaseProgress: { percent: number; eta: number; calibrated: boolean } | null

  // Conversation forks
  forks: ForkInfo[]
  createFork: (label: string) => Promise<ForkInfo | null>
  listForks: () => Promise<void>
  compareForks: () => Promise<Record<string, unknown> | null>
  selectFork: (forkId: string) => Promise<void>

  // Recap
  recap: RecapData | null
  loadRecap: () => Promise<void>

  // Response cache stats
  cacheStats: CacheStats | null
  loadCacheStats: () => Promise<void>
  clearCache: () => Promise<void>

  // Task actions
  start: (source: string, autoAdvance?: boolean, contextItems?: ContextItem[]) => Promise<void>
  quickStart: (source: string) => Promise<void>
  plan: () => Promise<void>
  implement: () => Promise<void>
  simplify: () => Promise<void>
  optimize: () => Promise<void>
  review: (options?: ReviewOptions) => Promise<void>
  submit: (options?: SubmitOptions) => Promise<void>
  stop: () => Promise<void>
  abort: () => Promise<void>
  reset: () => Promise<void>
  retry: () => Promise<void>
  abandon: (keepBranch?: boolean) => Promise<void>
  deleteTask: () => Promise<void>
  update: () => Promise<UpdateResult>
  finish: (options?: FinishOptions) => Promise<FinishResult | null>
  refresh: () => Promise<RefreshResult | null>
  approveTransition: (event: string) => Promise<void>
  approveNode: (nodeId: string) => Promise<void>
  rejectNode: (nodeId: string) => Promise<void>
  approveRemote: (comment?: string) => Promise<void>
  mergeRemote: (method?: string) => Promise<void>

  // Queue actions
  queueTask: (source: string, title?: string) => Promise<QueuedTask | null>
  dequeueTask: (id: string) => Promise<void>
  loadQueue: () => Promise<void>
  reorderQueue: (id: string, position: number) => Promise<void>

  // Tags
  loadTags: () => Promise<void>
  addTag: (tag: string) => Promise<void>
  removeTag: (tag: string) => Promise<void>

  // Quality gate
  respondToPrompt: (promptId: string, answer: boolean) => Promise<void>
}

export interface FilesSlice {
  // Git state
  checkpoints: Checkpoint[]
  redoStack: Checkpoint[]
  gitStatus: GitStatus | null
  fileChanges: FileChange[]

  // Review history
  reviews: Review[]
  reviewDetails: Record<number, ReviewDetail>

  // Checkpoint navigation
  undo: (steps?: number) => Promise<void>
  redo: (steps?: number) => Promise<void>
  goToCheckpoint: (sha: string) => Promise<void>
  previewCheckpoint: (sha: string) => Promise<{ diff: string; stat: string } | null>

  // Git operations
  refreshGitStatus: () => Promise<void>
  getGitDiff: (cached?: boolean) => Promise<string>
  getGitLog: (count?: number) => Promise<GitLogEntry[]>

  // Review history
  loadReviews: () => Promise<void>
  loadReview: (number: number) => Promise<ReviewDetail | null>

  // Specifications & plans
  loadSpec: () => Promise<SpecEntry[]>
  loadPlan: () => Promise<SpecEntry[]>

  // Git diff against branch
  getGitDiffAgainst: (ref: string, stat?: boolean) => Promise<string>

  // File browser
  browseFiles: (path?: string, filesOnly?: boolean) => Promise<BrowseEntry[]>
  listFiles: (path?: string, extensions?: string[], maxDepth?: number) => Promise<FilesEntry[]>
}

export interface UISlice {
  // Connection
  connected: boolean
  connecting: boolean
  reconnectAttempt: number
  reconnectTimeoutId: ReturnType<typeof setTimeout> | null
  connectionVersion: number
  worktreeId: string | null
  client: SocketClient | null
  unsubscribeSocket: (() => void) | null

  // Output stream
  output: string[]
  lastSeq: number

  // UI state
  loading: boolean
  error: string | null

  // Dry-run mode
  dryRunMode: boolean
  toggleDryRun: () => void

  // PR body override
  prBodyOverride: string | null
  setPrBodyOverride: (body: string | null) => void

  // Connection
  connect: (worktreeId: string) => Promise<void>
  disconnect: () => void

  // Output
  appendOutput: (line: string) => void
  clearOutput: () => void

  // Status refresh
  refreshStatus: () => Promise<void>
}

export type ProjectState = TaskSlice & FilesSlice & UISlice
