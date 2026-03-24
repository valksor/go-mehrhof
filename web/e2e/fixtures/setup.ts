/**
 * E2E Test Fixture Setup
 *
 * Creates an isolated git repository for integration testing.
 * This ensures tests never touch real user repos.
 *
 * The worktree socket is created on-demand by the main server when the
 * frontend connects - no need to start a standalone socket here.
 */

import { execFileSync } from 'child_process'
import { mkdtempSync, writeFileSync, cpSync, rmSync, existsSync, mkdirSync, realpathSync, readdirSync, readFileSync, statSync } from 'fs'
import { tmpdir, homedir } from 'os'
import { join, resolve } from 'path'
import { createHash } from 'crypto'

export interface TestFixture {
  repoPath: string
  taskPath: string
  socketPath: string
  cleanup: () => void
}

/**
 * Compute the worktree socket path for a given project directory.
 * Mirrors the Go implementation in pkg/socket/paths.go
 */
function getWorktreeSocketPath(projectDir: string): string {
  // Must use realpathSync to resolve symlinks (e.g., /var -> /private/var on macOS)
  // to match Go's filepath.Abs() behavior which also resolves symlinks
  const absPath = realpathSync(resolve(projectDir))
  const hash = createHash('sha256').update(absPath).digest('hex').slice(0, 16) // first 8 bytes = 16 hex chars
  return join(homedir(), '.valksor', 'kvelmo', 'worktrees', hash + '.sock')
}

/**
 * Creates a temporary git repository with a test task.
 * Call cleanup() when done to remove the temp directory.
 *
 * Note: This does NOT start a worktree socket. The main server creates
 * worktree sockets on-demand when the frontend connects, with access to
 * the worker pool for planning/implementation.
 */
export function createTestFixture(): TestFixture {
  // Create temp directory
  // Use realpathSync to get canonical path (e.g., /private/var instead of /var on macOS)
  // This is critical because Go's filepath.Abs resolves symlinks, so socket path hash must match
  const repoPath = realpathSync(mkdtempSync(join(tmpdir(), 'kvelmo-e2e-')))

  // Initialize git repo (using execFileSync for safety - no shell injection)
  execFileSync('git', ['init'], { cwd: repoPath, stdio: 'pipe' })
  execFileSync('git', ['config', 'user.email', 'test@example.com'], { cwd: repoPath, stdio: 'pipe' })
  execFileSync('git', ['config', 'user.name', 'E2E Test'], { cwd: repoPath, stdio: 'pipe' })

  // Create basic project structure
  writeFileSync(join(repoPath, 'package.json'), JSON.stringify({
    name: 'e2e-test-project',
    version: '1.0.0',
    type: 'module',
  }, null, 2))

  writeFileSync(join(repoPath, 'tsconfig.json'), JSON.stringify({
    compilerOptions: {
      target: 'ES2022',
      module: 'ESNext',
      moduleResolution: 'bundler',
      strict: true,
      outDir: 'dist',
    },
    include: ['src/**/*'],
  }, null, 2))

  mkdirSync(join(repoPath, 'src'), { recursive: true })
  writeFileSync(join(repoPath, 'src', 'index.ts'), '// Entry point\n')

  // Copy the test task file
  const fixturesDir = join(import.meta.dirname, '.')
  const taskPath = join(repoPath, 'task.md')
  cpSync(join(fixturesDir, 'task.md'), taskPath)

  // Create .valksor directory with agent config
  // Agent configurable via KVELMO_E2E_AGENT (default: ollama)
  const agentName = process.env.KVELMO_E2E_AGENT || 'ollama'
  const valksorDir = join(repoPath, '.valksor')
  mkdirSync(valksorDir, { recursive: true })
  writeFileSync(join(valksorDir, 'kvelmo.yaml'), [
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
  ].join('\n'))

  // Initial commit
  execFileSync('git', ['add', '-A'], { cwd: repoPath, stdio: 'pipe' })
  execFileSync('git', ['commit', '-m', 'Initial commit'], { cwd: repoPath, stdio: 'pipe' })

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
    cleanup: () => {
      // Unregister project from server (sync call via curl to avoid async issues)
      try {
        const hash = createHash('sha256').update(realpathSync(resolve(repoPath))).digest('hex').slice(0, 16)
        execFileSync('curl', [
          '-s', '-X', 'POST',
          '-H', 'Content-Type: application/json',
          '-d', JSON.stringify({ jsonrpc: '2.0', id: 'cleanup', method: 'projects.unregister', params: { id: hash } }),
          'http://localhost:6337/api/rpc'
        ], { stdio: 'pipe', timeout: 5000 })
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
          if ((worktreePath.startsWith('"') && worktreePath.endsWith('"')) ||
              (worktreePath.startsWith("'") && worktreePath.endsWith("'"))) {
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
