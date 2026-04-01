import type { ChatCommand } from './chatCommandTypes'
import { getState, isActive, worktreeClient, globalClient } from './chatCommandTypes'

// ── Governance & Quality ──────────────────────────────────────────────────

const governanceCommands: ChatCommand[] = [
  {
    name: '/approve',
    description: 'Approve workflow transition',
    isAvailable: () => getState().state === 'waiting',
    execute: async (args) => {
      const event = args.trim()
      if (!event) return 'Usage: /approve <event> (e.g. submit, implement)'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      await client.call('approve', { event })
      return `Approved: ${event}`
    },
  },
  {
    name: '/checklist check',
    description: 'Mark checklist item as checked',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const item = args.trim()
      if (!item) return 'Usage: /checklist check <item-name>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      await client.call('review.checklist.check', { item })
      return `Checked: ${item}`
    },
  },
  {
    name: '/checklist uncheck',
    description: 'Unmark checklist item',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const item = args.trim()
      if (!item) return 'Usage: /checklist uncheck <item-name>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      await client.call('review.checklist.uncheck', { item })
      return `Unchecked: ${item}`
    },
  },
  {
    name: '/checklist',
    description: 'Show review checklist',
    isAvailable: () => isActive(),
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ required: string[]; checked: string[] }>('review.checklist.get', {})
      const required = result.required || []
      if (required.length === 0) return 'No checklist items.'
      const checkedSet = new Set(result.checked || [])
      return required.map((item, i) => `${checkedSet.has(item) ? '✓' : '☐'} ${i + 1}. ${item}`).join('\n')
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
        out += '\n' + result.checks.map(c => `${c.status === 'passed' ? '✓' : '✗'} ${c.name}`).join('\n')
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

// ── Files & Code ──────────────────────────────────────────────────────────

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
  {
    name: '/codegraph callers',
    description: 'Find callers of a symbol',
    isAvailable: () => true,
    execute: async (args) => {
      const name = args.trim()
      if (!name) return 'Usage: /codegraph callers <symbol>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ callers: Array<{ name: string; file: string; line: number }> }>('codegraph.callers', { name })
      const callers = result.callers || []
      if (callers.length === 0) return 'No callers found.'
      return callers.map(c => `${c.name} — ${c.file}:${c.line}`).join('\n')
    },
  },
  {
    name: '/codegraph deps',
    description: 'Find dependencies of a symbol',
    isAvailable: () => true,
    execute: async (args) => {
      const name = args.trim()
      if (!name) return 'Usage: /codegraph deps <symbol>'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ deps: Array<{ name: string; kind: string; file: string }> }>('codegraph.deps', { name })
      const deps = result.deps || []
      if (deps.length === 0) return 'No dependencies found.'
      return deps.map(d => `${d.kind} ${d.name} — ${d.file}`).join('\n')
    },
  },
  {
    name: '/codegraph index',
    description: 'Index codebase for code graph',
    isAvailable: () => true,
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ symbols: number }>('codegraph.index', {})
      return `Indexed ${result.symbols ?? 0} symbols.`
    },
  },
  {
    name: '/codegraph stats',
    description: 'Show code graph statistics',
    isAvailable: () => true,
    execute: async () => {
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<Record<string, unknown>>('codegraph.stats', {})
      return JSON.stringify(result, null, 2)
    },
  },
]

// ── Memory & Cache ────────────────────────────────────────────────────────

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
    name: '/memory clear',
    description: 'Clear all memory entries',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('memory.clear', {})
      return 'Memory cleared.'
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

// ── Infrastructure & Utilities ────────────────────────────────────────────

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

const utilityCommands: ChatCommand[] = [
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
    name: '/export',
    description: 'Export task data',
    isAvailable: () => isActive(),
    execute: async (args) => {
      const format = args.trim() || 'json'
      const client = worktreeClient()
      if (!client) return 'Not connected.'
      const result = await client.call<{ data: string; path?: string }>('task.export', { format })
      if (result.path) return `Exported to ${result.path}`
      return result.data || 'Export complete.'
    },
  },
  {
    name: '/logs',
    description: 'View operation logs',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ entries: Array<{ timestamp: string; level: string; message: string; method: string }> }>('activity.query', { limit: 20 })
      const entries = result.entries || []
      if (entries.length === 0) return 'No log entries.'
      return entries.map(e => `[${e.timestamp}] [${e.level || 'INFO'}] ${e.message || e.method}`).join('\n')
    },
  },
  {
    name: '/access create',
    description: 'Create a new operator access token',
    isAvailable: () => true,
    execute: async (args) => {
      const label = args.trim()
      if (!label) return 'Usage: /access create <label>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ token: string }>('access.token.create', { role: 'operator', label })
      return `Token created:\n${result.token}\n\nSave this token — it cannot be shown again.`
    },
  },
  {
    name: '/access revoke',
    description: 'Revoke an access token',
    isAvailable: () => true,
    execute: async (args) => {
      const id = args.trim()
      if (!id) return 'Usage: /access revoke <token-id>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('access.token.revoke', { id })
      return `Token ${id} revoked.`
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
    name: '/workers add',
    description: 'Add a worker to the pool',
    isAvailable: () => true,
    execute: async (args) => {
      const name = args.trim()
      if (!name) return 'Usage: /workers add <agent-name>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('workers.add', { name })
      return `Worker "${name}" added.`
    },
  },
  {
    name: '/workers remove',
    description: 'Remove a worker from the pool',
    isAvailable: () => true,
    execute: async (args) => {
      const name = args.trim()
      if (!name) return 'Usage: /workers remove <worker-name>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('workers.remove', { name })
      return `Worker "${name}" removed.`
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
    name: '/config check',
    description: 'Check configuration for drift',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ drifts: Array<{ path: string; expected: unknown; actual: unknown }>; count: number }>('config.check', {})
      if (result.count === 0) return 'Configuration: no drift detected.'
      const drifts = result.drifts || []
      return 'Configuration drift:\n' + drifts.map(d => `  ${d.path}: expected=${String(d.expected)}, actual=${String(d.actual)}`).join('\n')
    },
  },
  {
    name: '/config show',
    description: 'Show effective configuration',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ effective: Record<string, unknown> }>('settings.get', {})
      return JSON.stringify(result.effective, null, 2)
    },
  },
  {
    name: '/config validate',
    description: 'Validate configuration and agent setup',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ valid: boolean; checks: Array<{ name: string; status: string; detail?: string; fix?: string }> }>('config.validate', {})
      const lines = result.checks.map(c => {
        const icon = c.status === 'ok' ? 'PASS' : c.status === 'warning' ? 'WARN' : 'FAIL'
        let line = `  [${icon}] ${c.name}`
        if (c.detail) line += ` — ${c.detail}`
        if (c.fix && c.status !== 'ok') line += `\n         Fix: ${c.fix}`
        return line
      })
      return `Configuration ${result.valid ? 'valid' : 'INVALID'}:\n${lines.join('\n')}`
    },
  },
  {
    name: '/strategy',
    description: 'List available agent reasoning strategies',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const names = await client.call<string[]>('strategy.list', {})
      if (!names || names.length === 0) return 'No strategies registered.'
      return 'Available strategies:\n' + names.map(n => `  - ${n}`).join('\n')
    },
  },
  {
    name: '/restore',
    description: 'Restore from a backup archive',
    isAvailable: () => true,
    execute: async (args: string) => {
      const archivePath = args.trim()
      if (!archivePath) return 'Usage: /restore <archive-path>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      await client.call('backup.restore', { archive_path: archivePath })
      return `Restored from ${archivePath}.`
    },
  },
  {
    name: '/catalog list',
    description: 'List available task templates',
    isAvailable: () => true,
    execute: async () => {
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const result = await client.call<{ templates: Array<{ name: string; description: string }> }>('catalog.list', {})
      const templates = result.templates || []
      if (templates.length === 0) return 'No templates in catalog.'
      return templates.map(t => `  ${t.name} — ${t.description}`).join('\n')
    },
  },
  {
    name: '/catalog use',
    description: 'Start a task from a catalog template',
    isAvailable: () => true,
    execute: async (args: string) => {
      const name = args.trim()
      if (!name) return 'Usage: /catalog use <template-name>'
      const client = globalClient()
      if (!client) return 'Not connected to global socket.'
      const tmpl = await client.call<{ name: string; source: string }>('catalog.get', { name })
      if (!tmpl.source) return `Template "${name}" has no source configured.`
      const wtClient = worktreeClient()
      if (!wtClient) return `Template "${name}" found (source: ${tmpl.source}). Connect to a project to start it.`
      await wtClient.call('start', { source: tmpl.source })
      return `Started task from template "${name}" (source: ${tmpl.source}).`
    },
  },
]

export const infraCommands: ChatCommand[] = [
  ...governanceCommands,
  ...fileCommands,
  ...memoryCommands,
  ...utilityCommands,
]
