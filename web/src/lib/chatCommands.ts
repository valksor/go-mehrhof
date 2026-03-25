import { useProjectStore } from '../stores/projectStore'
import { useGlobalStore } from '../stores/globalStore'
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

function getGlobal() {
  return useGlobalStore.getState()
}

function isActive() {
  return getState().state !== 'none'
}

function worktreeClient() {
  return getState().client
}

function globalClient() {
  return getGlobal().client
}

// ── Workflow ────────────────────────────────────────────────────────────────

const workflowCommands: ChatCommand[] = [
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

// ── Workflow Control ────────────────────────────────────────────────────────

const controlCommands: ChatCommand[] = [
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

// ── Information & Inspection ────────────────────────────────────────────────

const infoCommands: ChatCommand[] = [
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
      return ''
    },
  },
  {
    name: '/checkpoints',
    description: 'List git checkpoints',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ checkpoints: Array<{ sha: string; message: string; timestamp: string }> }>('checkpoints', {})
      const cps = result.checkpoints || []
      if (cps.length === 0) return 'No checkpoints.'
      return cps.map((cp, i) => `${i + 1}. ${cp.sha.slice(0, 7)} — ${cp.message}`).join('\n')
    },
  },
  {
    name: '/checkpoints goto',
    description: 'Jump to a checkpoint by SHA',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const sha = args.trim()
      if (!sha) return 'Usage: /checkpoints goto <sha>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      await client.call('checkpoint.goto', { sha })
      return `Jumped to checkpoint ${sha.slice(0, 7)}.`
    },
  },
  {
    name: '/recap',
    description: 'Summarize current task state for resuming',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ recap: string }>('recap', {})
      return result.recap || 'No recap available.'
    },
  },
  {
    name: '/diff',
    description: 'Show file changes from agent work',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ diff: string }>('git.diff', {})
      return result.diff || 'No changes.'
    },
  },
  {
    name: '/show spec',
    description: 'Show current specification',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ specifications: Array<{ content: string }> }>('show.spec', {})
      const specs = result.specifications || []
      if (specs.length === 0) return 'No specification available.'
      return specs.map(s => s.content).join('\n---\n\n')
    },
  },
  {
    name: '/show plan',
    description: 'Show current plan',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ specifications: Array<{ content: string }> }>('show.plan', {})
      const specs = result.specifications || []
      if (specs.length === 0) return 'No plan available.'
      return specs.map(s => s.content).join('\n---\n\n')
    },
  },
  {
    name: '/list search',
    description: 'Search task history',
    isAvailable: () => true,
    execute: async (args) => {
      const query = args.trim()
      if (!query) return 'Usage: /list search <query>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ tasks: Array<{ id: string; title: string; state: string }> }>('task.search', { query })
      const tasks = result.tasks || []
      if (tasks.length === 0) return 'No matching tasks.'
      return tasks.map(t => `${t.id.slice(0, 8)} [${t.state}] ${t.title}`).join('\n')
    },
  },
  {
    name: '/list',
    description: 'List task history',
    isAvailable: () => true,
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ tasks: Array<{ id: string; title: string; state: string }> }>('task.history', {})
      const tasks = result.tasks || []
      if (tasks.length === 0) return 'No task history.'
      return tasks.map(t => `${t.id.slice(0, 8)} [${t.state}] ${t.title}`).join('\n')
    },
  },
  {
    name: '/eventlog',
    description: 'View task lifecycle events',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ events: Array<{ type: string; timestamp: string; message?: string }> }>('eventlog.query', { limit: 20 })
      const events = result.events || []
      if (events.length === 0) return 'No events.'
      return events.map(e => `[${e.timestamp}] ${e.type}${e.message ? ': ' + e.message : ''}`).join('\n')
    },
  },
  {
    name: '/jobs',
    description: 'List job queue',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ jobs: Array<{ id: string; type: string; status: string }> }>('jobs.list', {})
      const jobs = result.jobs || []
      if (jobs.length === 0) return 'No jobs.'
      return jobs.map(j => `${j.id.slice(0, 8)} [${j.status}] ${j.type}`).join('\n')
    },
  },
  {
    name: '/stats',
    description: 'Show real-time metrics',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<Record<string, unknown>>('metrics', {})
      return JSON.stringify(result, null, 2)
    },
  },
]

// ── Organization ────────────────────────────────────────────────────────────

async function listQueue(): Promise<string> {
  await getState().loadQueue()
  const queue = getState().taskQueue
  if (queue.length === 0) return 'Queue is empty.'
  return queue.map((t, i) => `${i + 1}. [${t.id.slice(0, 8)}] ${t.title || t.source}`).join('\n')
}

const orgCommands: ChatCommand[] = [
  {
    name: '/tag add',
    description: 'Add a tag to the task',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const tag = args.trim()
      if (!tag) return 'Usage: /tag add <name>'
      const client = worktreeClient()
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
      const client = worktreeClient()
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
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ tags: string[] }>('task.tag', { action: 'list' })
      const tags = result.tags || []
      return tags.length > 0 ? `Tags: ${tags.join(', ')}` : 'No tags.'
    },
  },
  {
    name: '/queue add',
    description: 'Add task to queue',
    isAvailable: () => true,
    execute: async (args) => {
      const source = args.trim()
      if (!source) return 'Usage: /queue add <source>'
      await getState().queueTask(source)
      return `Queued: ${source}`
    },
  },
  {
    name: '/queue remove',
    description: 'Remove task from queue',
    isAvailable: () => true,
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /queue remove <id>'
      await getState().dequeueTask(id)
      return `Removed ${id} from queue.`
    },
  },
  {
    name: '/queue list',
    description: 'List queued tasks',
    isAvailable: () => true,
    execute: listQueue,
  },
  {
    name: '/queue',
    description: 'List queued tasks',
    isAvailable: () => true,
    execute: listQueue,
  },
  {
    name: '/fork create',
    description: 'Create a conversation fork',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const label = args.trim()
      if (!label) return 'Usage: /fork create <label>'
      await getState().createFork(label)
      return `Fork "${label}" created.`
    },
  },
  {
    name: '/fork list',
    description: 'List active forks',
    isAvailable: () => isActive(),
    execute: async () => {
      await getState().listForks()
      const forks = getState().forks
      if (forks.length === 0) return 'No active forks.'
      return forks.map(f => `${f.id.slice(0, 8)} — ${f.label} [${f.state}]`).join('\n')
    },
  },
  {
    name: '/fork compare',
    description: 'Compare forks',
    isAvailable: () => isActive(),
    execute: async () => {
      const result = await getState().compareForks()
      return JSON.stringify(result, null, 2)
    },
  },
  {
    name: '/fork select',
    description: 'Select a fork',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /fork select <fork-id>'
      await getState().selectFork(id)
      return `Switched to fork ${id.slice(0, 8)}.`
    },
  },
  {
    name: '/group create',
    description: 'Create cross-repo task group',
    isAvailable: () => true,
    execute: async (args) => {
      const label = args.trim()
      if (!label) return 'Usage: /group create <label>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ id: string }>('taskgroup.create', { label })
      return `Group created: ${result.id}`
    },
  },
  {
    name: '/group list',
    description: 'List task groups',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ groups: Array<{ id: string; label: string }> }>('taskgroup.list', {})
      const groups = result.groups || []
      if (groups.length === 0) return 'No groups.'
      return groups.map(g => `${g.id.slice(0, 8)} — ${g.label}`).join('\n')
    },
  },
  {
    name: '/group status',
    description: 'Show group status',
    isAvailable: () => true,
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /group status <group-id>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<Record<string, unknown>>('taskgroup.status', { id })
      return JSON.stringify(result, null, 2)
    },
  },
  {
    name: '/group add',
    description: 'Add task to group',
    isAvailable: () => true,
    execute: async (args) => {
      const parts = args.trim().split(/\s+/)
      if (parts.length < 2) return 'Usage: /group add <group-id> <task-id>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('taskgroup.add', { id: parts[0], task_id: parts[1] })
      return `Task added to group ${parts[0].slice(0, 8)}.`
    },
  },
  {
    name: '/group submit',
    description: 'Submit grouped tasks',
    isAvailable: () => true,
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /group submit <group-id>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('taskgroup.submit', { id })
      return `Group ${id.slice(0, 8)} submitted.`
    },
  },
  {
    name: '/group remove',
    description: 'Delete a task group',
    isAvailable: () => true,
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /group remove <group-id>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('taskgroup.remove', { id })
      return `Group ${id.slice(0, 8)} removed.`
    },
  },
  {
    name: '/batch',
    description: 'Run action across all active projects',
    isAvailable: () => true,
    execute: async (args) => {
      const action = args.trim()
      if (!action) return 'Usage: /batch <action> (plan, implement, review, submit, abort, reset, stop)'
      const result = await getGlobal().batchAction(action)
      return `Batch ${action}: ${result.succeeded}/${result.total} succeeded.`
    },
  },
  {
    name: '/activity',
    description: 'View RPC activity log',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ entries: Array<{ method: string; timestamp: string; duration_ms: number }> }>('activity.query', { limit: 20 })
      const entries = result.entries || []
      if (entries.length === 0) return 'No activity.'
      return entries.map(e => `[${e.timestamp}] ${e.method} (${e.duration_ms}ms)`).join('\n')
    },
  },
  {
    name: '/audit',
    description: 'View audit trail',
    isAvailable: () => true,
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ entries: Array<{ action: string; timestamp: string; details?: string }> }>('task.export', { format: 'audit' })
      return JSON.stringify(result, null, 2)
    },
  },
  {
    name: '/report',
    description: 'Generate compliance report',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ report: string }>('report.generate', {})
      return result.report || 'Report generated.'
    },
  },
  {
    name: '/backup',
    description: 'Create state backup',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ path: string }>('backup.create', {})
      return `Backup created: ${result.path}`
    },
  },
  {
    name: '/access',
    description: 'List access tokens',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ tokens: Array<{ id: string; name: string; created: string }> }>('access.token.list', {})
      const tokens = result.tokens || []
      if (tokens.length === 0) return 'No access tokens.'
      return tokens.map(t => `${t.id.slice(0, 8)} — ${t.name} (${t.created})`).join('\n')
    },
  },
]

// ── Governance & Quality ────────────────────────────────────────────────────

const governanceCommands: ChatCommand[] = [
  {
    name: '/approve',
    description: 'Approve workflow transition',
    isAvailable: () => getState().state === 'waiting',
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      await client.call('approve', {})
      return 'Approved.'
    },
  },
  {
    name: '/checklist check',
    description: 'Mark checklist item as checked',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const index = parseInt(args.trim(), 10)
      if (isNaN(index)) return 'Usage: /checklist check <number>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      await client.call('review.checklist.check', { index })
      return `Checklist item ${index} checked.`
    },
  },
  {
    name: '/checklist uncheck',
    description: 'Unmark checklist item',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const index = parseInt(args.trim(), 10)
      if (isNaN(index)) return 'Usage: /checklist uncheck <number>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      await client.call('review.checklist.uncheck', { index })
      return `Checklist item ${index} unchecked.`
    },
  },
  {
    name: '/checklist',
    description: 'Show review checklist',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ items: Array<{ label: string; checked: boolean }> }>('review.checklist.get', {})
      const items = result.items || []
      if (items.length === 0) return 'No checklist items.'
      return items.map((item, i) => `${item.checked ? '✓' : '☐'} ${i + 1}. ${item.label}`).join('\n')
    },
  },
  {
    name: '/quality',
    description: 'Run code quality gates',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ status: string; findings?: Array<{ message: string; severity: string }> }>('quality.respond', { action: 'run' })
      if (!result.findings || result.findings.length === 0) return `Quality: ${result.status}`
      return `Quality: ${result.status}\n` + result.findings.map(f => `[${f.severity}] ${f.message}`).join('\n')
    },
  },
  {
    name: '/ci',
    description: 'Show CI pipeline status',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ status: string; url?: string; checks?: Array<{ name: string; status: string }> }>('ci.status', {})
      let out = `CI: ${result.status}`
      if (result.url) out += ` (${result.url})`
      if (result.checks && result.checks.length > 0) {
        out += '\n' + result.checks.map(c => `  ${c.status === 'passed' ? '✓' : '✗'} ${c.name}`).join('\n')
      }
      return out
    },
  },
  {
    name: '/policy',
    description: 'Check workflow policy compliance',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ compliant: boolean; violations?: Array<{ rule: string; message: string }> }>('policy.check', {})
      if (result.compliant) return 'Policy: compliant.'
      const violations = result.violations || []
      return 'Policy violations:\n' + violations.map(v => `  • ${v.rule}: ${v.message}`).join('\n')
    },
  },
]

// ── Files & Code ────────────────────────────────────────────────────────────

const fileCommands: ChatCommand[] = [
  {
    name: '/files search',
    description: 'Search project files',
    isAvailable: () => true,
    execute: async (args) => {
      const pattern = args.trim()
      if (!pattern) return 'Usage: /files search <pattern>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ files: string[] }>('files.search', { pattern })
      const files = result.files || []
      if (files.length === 0) return 'No matching files.'
      return files.join('\n')
    },
  },
  {
    name: '/files',
    description: 'List project files',
    isAvailable: () => true,
    execute: async (args) => {
      const path = args.trim() || '.'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ files: string[] }>('files.list', { path })
      const files = result.files || []
      if (files.length === 0) return 'No files.'
      return files.join('\n')
    },
  },
  {
    name: '/git status',
    description: 'Show git status',
    isAvailable: () => true,
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ branch: string; has_changes: boolean; summary?: string }>('git.status', {})
      let out = `Branch: ${result.branch}`
      if (result.summary) out += '\n' + result.summary
      else out += result.has_changes ? ' (has changes)' : ' (clean)'
      return out
    },
  },
  {
    name: '/git log',
    description: 'Show recent commits',
    isAvailable: () => true,
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ entries: Array<{ sha: string; message: string }> }>('git.log', { limit: 10 })
      const entries = result.entries || []
      if (entries.length === 0) return 'No commits.'
      return entries.map(e => `${e.sha.slice(0, 7)} ${e.message}`).join('\n')
    },
  },
  {
    name: '/codegraph search',
    description: 'Search code symbols',
    isAvailable: () => true,
    execute: async (args) => {
      const name = args.trim()
      if (!name) return 'Usage: /codegraph search <symbol>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ symbols: Array<{ name: string; kind: string; file: string; line: number }> }>('codegraph.search', { name })
      const symbols = result.symbols || []
      if (symbols.length === 0) return 'No symbols found.'
      return symbols.map(s => `${s.kind} ${s.name} — ${s.file}:${s.line}`).join('\n')
    },
  },
]

// ── Memory & Cache ──────────────────────────────────────────────────────────

const memoryCommands: ChatCommand[] = [
  {
    name: '/memory search',
    description: 'Search semantic memory',
    isAvailable: () => true,
    execute: async (args) => {
      const query = args.trim()
      if (!query) return 'Usage: /memory search <query>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ results: Array<{ content: string; score: number }> }>('memory.search', { query, limit: 5 })
      const results = result.results || []
      if (results.length === 0) return 'No results.'
      return results.map((r, i) => `${i + 1}. (${r.score.toFixed(2)}) ${r.content}`).join('\n')
    },
  },
  {
    name: '/memory stats',
    description: 'Show memory store statistics',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<Record<string, unknown>>('memory.stats', {})
      return JSON.stringify(result, null, 2)
    },
  },
  {
    name: '/cache stats',
    description: 'Show response cache statistics',
    isAvailable: () => true,
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<Record<string, unknown>>('cache.stats', {})
      return JSON.stringify(result, null, 2)
    },
  },
  {
    name: '/cache clear',
    description: 'Clear response cache',
    isAvailable: () => true,
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      await client.call('cache.clear', {})
      return 'Cache cleared.'
    },
  },
]

// ── Infrastructure ──────────────────────────────────────────────────────────

/** Parse "source..target [note]" into parts. */
function parseChangelogArgs(args: string): { source: string; target: string; note: string } {
  const input = args.trim()
  const spaceIdx = input.indexOf(' ')
  const refPart = spaceIdx === -1 ? input : input.slice(0, spaceIdx)
  const note = spaceIdx === -1 ? '' : input.slice(spaceIdx + 1).trim()
  const parts = refPart.split('..')
  if (parts.length !== 2 || !parts[0] || !parts[1]) {
    return { source: '', target: '', note: '' }
  }
  return { source: parts[0], target: parts[1], note }
}

const infraCommands: ChatCommand[] = [
  {
    name: '/changelog',
    description: 'Generate changelog between git refs (source..target [note])',
    isAvailable: () => true,
    execute: async (args) => {
      const { source, target, note } = parseChangelogArgs(args)
      if (!source || !target) {
        return 'Usage: /changelog <source>..<target> [note] (e.g. v1.0..v2.0 only frontend)'
      }
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const params: Record<string, unknown> = { source, target }
      if (note) params.note = note
      const result = await client.call<{ markdown: string }>('changelog.generate', params)
      return result.markdown || `No commits between ${source} and ${target}`
    },
  },
  {
    name: '/changelog full',
    description: 'Generate changelog with full descriptions',
    isAvailable: () => true,
    execute: async (args) => {
      const { source, target, note } = parseChangelogArgs(args)
      if (!source || !target) {
        return 'Usage: /changelog full <source>..<target> [note]'
      }
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const params: Record<string, unknown> = { source, target, full: true }
      if (note) params.note = note
      const result = await client.call<{ markdown: string }>('changelog.generate', params)
      return result.markdown || `No commits between ${source} and ${target}`
    },
  },
  {
    name: '/workers',
    description: 'List worker pool',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ workers: Array<{ name: string; state: string }> }>('workers.list', {})
      const workers = result.workers || []
      if (workers.length === 0) return 'No workers.'
      return workers.map(w => `${w.name} [${w.state}]`).join('\n')
    },
  },
  {
    name: '/discover',
    description: 'Scan project for available commands',
    isAvailable: () => true,
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ commands: Array<{ name: string; source: string }> }>('discovery.scan', {})
      const commands = result.commands || []
      if (commands.length === 0) return 'No project commands found.'
      return commands.map(c => `[${c.source}] ${c.name}`).join('\n')
    },
  },
  {
    name: '/diagnose',
    description: 'Run system diagnostics',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ checks: Array<{ name: string; status: string; detail?: string }> }>('system.diagnose', {})
      const checks = result.checks || []
      if (checks.length === 0) return 'Diagnostics: OK'
      return checks.map(c => `${c.status === 'passed' ? '✓' : '✗'} ${c.name}${c.detail ? ': ' + c.detail : ''}`).join('\n')
    },
  },
  {
    name: '/security scan',
    description: 'Run security scan',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ issues: Array<{ severity: string; message: string }> }>('security.scan', {})
      const issues = result.issues || []
      if (issues.length === 0) return 'No security issues found.'
      return issues.map(i => `[${i.severity}] ${i.message}`).join('\n')
    },
  },
  {
    name: '/remote approve',
    description: 'Approve pull request',
    isAvailable: () => getState().state === 'submitted',
    execute: async () => {
      await getState().approveRemote()
      return 'PR approved.'
    },
  },
  {
    name: '/remote merge',
    description: 'Merge pull request',
    isAvailable: () => getState().state === 'submitted',
    execute: async () => {
      await getState().mergeRemote()
      return 'PR merged.'
    },
  },
  {
    name: '/onboarding reset',
    description: 'Reset onboarding guide',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('onboarding.reset', {})
      return 'Onboarding reset. The guide will show again on next visit.'
    },
  },
  {
    name: '/config check',
    description: 'Check configuration for drift',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ drifted: boolean; diffs?: Array<{ key: string; expected: string; actual: string }> }>('config.check', {})
      if (!result.drifted) return 'Configuration: no drift detected.'
      const diffs = result.diffs || []
      return 'Configuration drift:\n' + diffs.map(d => `  ${d.key}: expected=${d.expected}, actual=${d.actual}`).join('\n')
    },
  },
]

// ── All Commands ────────────────────────────────────────────────────────────

export const COMMANDS: ChatCommand[] = [
  ...workflowCommands,
  ...controlCommands,
  ...infoCommands,
  ...orgCommands,
  ...governanceCommands,
  ...fileCommands,
  ...memoryCommands,
  ...infraCommands,
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
