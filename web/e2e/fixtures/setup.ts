/**
 * E2E Test Fixture Setup
 *
 * Clones the e2e test repository from GitHub for integration testing.
 * This ensures tests exercise the same flow a real user would have —
 * pointing kvelmo at an existing project with a task.
 *
 * The worktree socket is created on-demand by the main server when the
 * frontend connects - no need to start a standalone socket here.
 *
 * Requires:
 * - E2E_KVELMO_TOKEN: GitHub personal access token
 * - E2E_GITHUB_REPO: Repository in "owner/repo" format (default: ozo2003/e2e-test)
 */

import { execFileSync } from 'child_process'
import {
  mkdtempSync,
  writeFileSync,
  rmSync,
  existsSync,
  mkdirSync,
  realpathSync,
  readdirSync,
  readFileSync,
  statSync,
  cpSync,
} from 'fs'
import { tmpdir, homedir } from 'os'
import { join, resolve } from 'path'
import { createHash } from 'crypto'

export interface TestFixture {
  repoPath: string
  taskPath: string
  socketPath: string
  token: string
  repo: string
  cleanup: () => void
}

/**
 * Compute the worktree socket path for a given project directory.
 * Mirrors the Go implementation in internal/socket/paths.go
 */
function getWorktreeSocketPath(projectDir: string): string {
  // Must use realpathSync to resolve symlinks (e.g., /var -> /private/var on macOS)
  // to match Go's filepath.Abs() behavior which also resolves symlinks
  const absPath = realpathSync(resolve(projectDir))
  const hash = createHash('sha256').update(absPath).digest('hex').slice(0, 16) // first 8 bytes = 16 hex chars
  return join(homedir(), '.valksor', 'kvelmo', 'worktrees', hash + '.sock')
}

/**
 * Checks if GitHub credentials are configured for e2e tests.
 */
export function isGitHubConfigured(): boolean {
  return !!process.env.E2E_KVELMO_TOKEN
}

/**
 * Creates a test fixture by cloning the e2e test repository from GitHub.
 * Call cleanup() when done to remove the temp directory.
 *
 * Note: This does NOT start a worktree socket. The main server creates
 * worktree sockets on-demand when the frontend connects, with access to
 * the worker pool for planning/implementation.
 */
export function createTestFixture(): TestFixture {
  const token = process.env.E2E_KVELMO_TOKEN
  if (!token) {
    throw new Error('E2E_KVELMO_TOKEN must be set for integration tests')
  }

  const repo = process.env.E2E_GITHUB_REPO || 'ozo2003/e2e-test'
  const [owner, repoName] = repo.split('/')
  if (!owner || !repoName) {
    throw new Error(`E2E_GITHUB_REPO must be in "owner/repo" format, got: ${repo}`)
  }

  // Clone to temp directory
  const repoPath = realpathSync(mkdtempSync(join(tmpdir(), 'kvelmo-e2e-')))
  const repoURL = `https://x-access-token:${token}@github.com/${owner}/${repoName}.git`
  execFileSync('git', ['clone', repoURL, repoPath], { stdio: 'pipe' })

  // Configure git user
  execFileSync('git', ['config', 'user.email', 'test@example.com'], { cwd: repoPath, stdio: 'pipe' })
  execFileSync('git', ['config', 'user.name', 'E2E Test'], { cwd: repoPath, stdio: 'pipe' })

  // Copy the test task file
  const fixturesDir = join(import.meta.dirname, '.')
  const taskPath = join(repoPath, 'task.md')
  cpSync(join(fixturesDir, 'task.md'), taskPath)

  // Create .valksor directory with agent config
  const agentName = process.env.KVELMO_E2E_AGENT || 'ollama'
  const valksorDir = join(repoPath, '.valksor')
  mkdirSync(valksorDir, { recursive: true })
  writeFileSync(
    join(valksorDir, 'kvelmo.yaml'),
    [
      'agent:',
      `  default: ${agentName}`,
      '  ollama:',
      '    model: llama3.1',
      'storage:',
      '  save_in_project: true',
      'workflow:',
      '  external_review:',
      '    mode: never',
      '',
    ].join('\n'),
  )

  // Commit task and config so the agent starts with a clean worktree
  execFileSync('git', ['add', '-A'], { cwd: repoPath, stdio: 'pipe' })
  execFileSync('git', ['commit', '-m', 'Add e2e test task and config'], { cwd: repoPath, stdio: 'pipe' })

  // Compute the expected socket path (server will create this on-demand)
  const socketPath = getWorktreeSocketPath(repoPath)
  console.log('Test fixture created:')
  console.log('  Repo path:', repoPath)
  console.log('  Task path:', taskPath)
  console.log('  Expected socket path:', socketPath)

  return {
    repoPath,
    taskPath,
    socketPath,
    token,
    repo,
    cleanup: () => {
      // Unregister project from server (sync call via curl to avoid async issues)
      try {
        const hash = createHash('sha256')
          .update(realpathSync(resolve(repoPath)))
          .digest('hex')
          .slice(0, 16)
        execFileSync(
          'curl',
          [
            '-s',
            '-X',
            'POST',
            '-H',
            'Content-Type: application/json',
            '-d',
            JSON.stringify({ jsonrpc: '2.0', id: 'cleanup', method: 'projects.unregister', params: { id: hash } }),
            'http://localhost:6337/api/rpc',
          ],
          { stdio: 'pipe', timeout: 5000 },
        )
      } catch {
        // Server may not be running during cleanup - that's OK
      }

      // Remove temp directory
      if (existsSync(repoPath)) {
        rmSync(repoPath, { recursive: true, force: true })
      }

      // Remove worktree socket if it exists
      if (existsSync(socketPath)) {
        rmSync(socketPath, { force: true })
      }

      // Clean up orphaned task state from global storage
      cleanupOrphanedTaskState()
    },
  }
}

/**
 * Removes task state for tasks whose worktree_path no longer exists.
 * This prevents state pollution between test runs.
 */
function cleanupOrphanedTaskState(): void {
  const workDir = join(homedir(), '.valksor', 'kvelmo', 'work')
  if (!existsSync(workDir)) return

  try {
    const entries = readdirSync(workDir)
    for (const entry of entries) {
      const taskDir = join(workDir, entry)
      if (!statSync(taskDir).isDirectory()) continue

      const taskYaml = join(taskDir, 'task.yaml')
      if (!existsSync(taskYaml)) continue

      try {
        const content = readFileSync(taskYaml, 'utf-8')
        // Simple YAML parsing for worktree_path (avoid dependency)
        const match = content.match(/^worktree_path:\s*(.+)$/m)
        if (match) {
          let worktreePath = match[1].trim()
          // Strip surrounding quotes if present (YAML may quote paths with spaces)
          if (
            (worktreePath.startsWith('"') && worktreePath.endsWith('"')) ||
            (worktreePath.startsWith("'") && worktreePath.endsWith("'"))
          ) {
            worktreePath = worktreePath.slice(1, -1)
          }
          // If worktree_path doesn't exist, this is orphaned state
          if (worktreePath && !existsSync(worktreePath)) {
            console.log(`Cleaning up orphaned task state: ${entry}`)
            rmSync(taskDir, { recursive: true, force: true })
          }
        }
      } catch {
        // Ignore parse errors
      }
    }
  } catch {
    // Ignore errors during cleanup
  }
}

/**
 * Checks if Claude CLI is available
 */
export function isClaudeAvailable(): boolean {
  try {
    execFileSync('claude', ['--version'], { stdio: 'pipe' })
    return true
  } catch {
    return false
  }
}

/**
 * Checks if Ollama server is reachable at localhost:11434
 */
export function isOllamaAvailable(): boolean {
  try {
    execFileSync('curl', ['-sf', 'http://localhost:11434/api/tags'], { stdio: 'pipe', timeout: 5000 })
    return true
  } catch {
    return false
  }
}

/**
 * Checks if kvelmo backend is running
 */
export async function isBackendRunning(port = 6337): Promise<boolean> {
  try {
    const response = await fetch(`http://localhost:${port}/api/health`)
    return response.ok
  } catch {
    return false
  }
}
