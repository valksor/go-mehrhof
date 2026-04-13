/**
 * Hand-written TypeScript interfaces for Go types that are referenced
 * across package boundaries. tygo cannot resolve cross-package types,
 * so these are maintained manually and imported via tygo frontmatter.
 *
 * Source structs:
 *   - conductor.ContextItem  → internal/conductor/context.go
 *   - conductor.PhaseMetrics → internal/conductor/state.go
 *   - git.FileStatus         → internal/git/git.go
 */

/** Lightweight reference to contextual information attached to a task. */
export interface ContextItem {
  type: string
  ref: string
  label: string
}

/** Metrics captured for a single workflow phase execution. */
export interface PhaseMetrics {
  agent?: string
  duration: number
  input_tokens?: number
  output_tokens?: number
  total_tokens?: number
  est_cost_usd?: number
  recording_path?: string
  checkpoint_sha?: string
}

/** Status of a single file in a git diff. */
export interface FileStatus {
  path: string
  status: string
}
