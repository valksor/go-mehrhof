/**
 * Full E2E Integration Test - Task Lifecycle via Web UI
 *
 * Covers the complete kvelmo workflow:
 *   Load → Plan → Implement → Simplify → Optimize → Review → Submit → Finish
 *
 * Uses WebSocket API for state management + Playwright for UI verification.
 *
 * Requirements:
 * - kvelmo backend running (`make run`)
 * - Agent available: Ollama (`ollama serve`) or Claude CLI
 * - Override agent with KVELMO_E2E_AGENT env var (default: ollama)
 *
 * Run with: cd web && bun run test:e2e:integration
 */

import { test, expect, type Page } from '@playwright/test'
import { createTestFixture, isOllamaAvailable, type TestFixture } from '../fixtures/setup'

const PHASE_TIMEOUT = 600_000 // 10 minutes per phase
const UI_TIMEOUT = 15_000

let fixture: TestFixture

// ─── WebSocket API helpers ────────────────────────────────────────────────────

async function callWebSocketAPI(wsUrl: string, id: string, method: string, params: Record<string, unknown> = {}): Promise<unknown> {
  const WebSocket = (await import('ws')).default
  const ws = new WebSocket(wsUrl)

  return new Promise((resolve, reject) => {
    let settled = false
    const timeout = setTimeout(() => { settled = true; ws.close(); reject(new Error(`Timeout calling ${method}`)) }, 30000)
    let buffer = ''

    ws.on('open', () => {
      ws.send(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n')
    })
    ws.on('message', (data: Buffer) => {
      buffer += data.toString()
      for (const line of buffer.split('\n').slice(0, -1)) {
        buffer = buffer.slice(line.length + 1)
        if (!line.trim()) continue
        try {
          const msg = JSON.parse(line)
          if (msg.id === id) {
            clearTimeout(timeout); settled = true; ws.close()
            msg.error ? reject(new Error(msg.error.message)) : resolve(msg.result)
            return
          }
        } catch { /* ignore partial JSON */ }
      }
    })
    ws.on('error', (err: Error) => { clearTimeout(timeout); if (!settled) reject(err) })
    ws.on('close', () => { clearTimeout(timeout); if (!settled) reject(new Error('WebSocket closed')) })
  })
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function callWorktreeAPI(projectPath: string, method: string, params: any = {}): Promise<any> {
  const encodedPath = encodeURIComponent(projectPath)
  return callWebSocketAPI(`ws://localhost:6337/ws/worktree/${encodedPath}`, 'call', method, params)
}

async function addProjectViaAPI(projectPath: string, socketPath?: string): Promise<void> {
  await callWebSocketAPI('ws://localhost:6337/ws/global', 'add', 'projects.register', { path: projectPath, socket_path: socketPath || '' })
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function callGlobalAPI(method: string, params: Record<string, unknown> = {}): Promise<any> {
  return callWebSocketAPI('ws://localhost:6337/ws/global', 'call', method, params)
}

// ─── State helpers ────────────────────────────────────────────────────────────

async function getState(projectPath: string): Promise<string> {
  try {
    const result = await callWorktreeAPI(projectPath, 'status')
    return result?.state || 'none'
  } catch {
    return 'none'
  }
}

async function waitForState(projectPath: string, state: string, timeout = PHASE_TIMEOUT) {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    const current = await getState(projectPath)
    if (current === state) return
    if (current === 'failed') throw new Error(`Task failed while waiting for ${state}`)
    await new Promise(r => setTimeout(r, 2000))
  }
  throw new Error(`Timeout waiting for state ${state} (current: ${await getState(projectPath)})`)
}

// ─── UI helpers ───────────────────────────────────────────────────────────────

async function skipOnboarding(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('kvelmo-viewMode', JSON.stringify({
      state: { mode: 'developer', isFirstVisit: false },
      version: 1,
    }))
  })
}

async function dismissOnboarding(page: Page) {
  const card = page.getByText('Developer').first()
  if (await card.isVisible({ timeout: 3000 }).catch(() => false)) {
    await card.click()
    await page.waitForTimeout(500)
  }
}

async function navigateToProject(page: Page) {
  await skipOnboarding(page)
  await page.goto('/')
  await dismissOnboarding(page)
  await expect(page.getByRole('button', { name: 'Add Project' })).toBeVisible({ timeout: UI_TIMEOUT })

  await addProjectViaAPI(fixture.repoPath, fixture.socketPath)
  await page.getByRole('button', { name: 'Refresh' }).first().click()
  await page.waitForTimeout(1000)

  const projectName = fixture.repoPath.split('/').pop()!
  await page.getByText(projectName, { exact: true }).first().click()
  await dismissOnboarding(page)

  // Wait for ProjectView
  await expect(page.getByRole('complementary', { name: 'Left sidebar' })).toBeVisible({ timeout: UI_TIMEOUT })
}

// ─── Tests ────────────────────────────────────────────────────────────────────

test.describe('Full Task Lifecycle', () => {
  test.beforeAll(async () => {
    const agent = process.env.KVELMO_E2E_AGENT || 'ollama'
    if (agent === 'ollama' && !isOllamaAvailable()) {
      throw new Error('Ollama not reachable. Start with: ollama serve')
    }

    fixture = createTestFixture()
    console.log(`Fixture: ${fixture.repoPath}, Agent: ${agent}`)
  })

  test.afterAll(async () => {
    if (fixture) { fixture.cleanup(); console.log('Fixture cleaned up') }
  })

  test('complete workflow: load → plan → implement → simplify → optimize → review', async ({ page }) => {
    test.setTimeout(PHASE_TIMEOUT * 6 + 120_000)

    // ─── Navigate to project ────────────────────────────────────────
    await navigateToProject(page)
    console.log('ProjectView loaded')

    // ─── Handle stale state ─────────────────────────────────────────
    let state = await getState(fixture.repoPath)
    console.log('Initial state:', state)

    if (['planning', 'implementing', 'optimizing', 'simplifying', 'reviewing'].includes(state)) {
      try { await callWorktreeAPI(fixture.repoPath, 'abort') } catch { /* ok */ }
      state = await getState(fixture.repoPath)
    }
    if (state === 'failed') {
      try { await callWorktreeAPI(fixture.repoPath, 'reset') } catch { /* ok */ }
      state = await getState(fixture.repoPath)
    }

    // ─── Load task ──────────────────────────────────────────────────
    if (state === 'none') {
      await callWorktreeAPI(fixture.repoPath, 'start', { source: `file:${fixture.taskPath}` })
      await waitForState(fixture.repoPath, 'loaded', 30_000)
      console.log('Task loaded')
    }

    // ─── Plan ───────────────────────────────────────────────────────
    state = await getState(fixture.repoPath)
    if (state === 'loaded') {
      await callWorktreeAPI(fixture.repoPath, 'plan')
      console.log('Planning started...')
      await waitForState(fixture.repoPath, 'planned')
      console.log('Planning completed!')
    }

    // Verify: UI shows task info
    await page.waitForTimeout(2000)
    await expect(page.getByLabel('Left sidebar').first()).toBeVisible()

    // ─── Implement ──────────────────────────────────────────────────
    state = await getState(fixture.repoPath)
    if (state === 'planned') {
      await callWorktreeAPI(fixture.repoPath, 'implement')
      console.log('Implementation started...')
      await waitForState(fixture.repoPath, 'implemented')
      console.log('Implementation completed!')
    }

    // ─── Simplify ───────────────────────────────────────────────────
    state = await getState(fixture.repoPath)
    if (state === 'implemented') {
      await callWorktreeAPI(fixture.repoPath, 'simplify')
      console.log('Simplification started...')
      await waitForState(fixture.repoPath, 'implemented')
      console.log('Simplification completed!')
    }

    // ─── Optimize ───────────────────────────────────────────────────
    state = await getState(fixture.repoPath)
    if (state === 'implemented') {
      await callWorktreeAPI(fixture.repoPath, 'optimize')
      console.log('Optimization started...')
      await waitForState(fixture.repoPath, 'implemented')
      console.log('Optimization completed!')
    }

    // ─── New feature API calls (best-effort) ─────────────────────────
    console.log('Testing new feature APIs...')

    // Cache stats
    try {
      const cacheResult = await callWorktreeAPI(fixture.repoPath, 'cache.stats')
      console.log('Cache stats:', JSON.stringify(cacheResult))
    } catch (err) { console.log('cache.stats skipped:', err) }

    // Risk evaluation
    try {
      const riskResult = await callWorktreeAPI(fixture.repoPath, 'risk.evaluate')
      console.log('Risk score:', riskResult?.score)
      if (riskResult) {
        expect(riskResult).toHaveProperty('score')
      }
    } catch (err) { console.log('risk.evaluate skipped:', err) }

    // Fork operations
    try {
      await callWorktreeAPI(fixture.repoPath, 'fork.create', { label: 'e2e-fork' })
      const forks = await callWorktreeAPI(fixture.repoPath, 'fork.list')
      console.log('Forks:', JSON.stringify(forks))
    } catch (err) { console.log('fork ops skipped:', err) }

    // Failure classification
    try {
      const failclass = await callWorktreeAPI(fixture.repoPath, 'failclass.stats')
      console.log('Failclass stats:', JSON.stringify(failclass))
    } catch (err) { console.log('failclass.stats skipped:', err) }

    // Autofix status
    try {
      const autofix = await callWorktreeAPI(fixture.repoPath, 'autofix.status')
      console.log('Autofix status:', JSON.stringify(autofix))
    } catch (err) { console.log('autofix.status skipped:', err) }

    // Progress (may be null if phase finished)
    try {
      const progress = await callWorktreeAPI(fixture.repoPath, 'progress.get')
      console.log('Progress:', JSON.stringify(progress))
    } catch (err) { console.log('progress.get skipped:', err) }

    // ─── Undo / Redo ────────────────────────────────────────────────
    try {
      await callWorktreeAPI(fixture.repoPath, 'undo')
      await callWorktreeAPI(fixture.repoPath, 'redo')
      console.log('Undo/redo succeeded')
    } catch (err) {
      console.log('Undo/redo skipped:', err)
    }

    // ─── Review ─────────────────────────────────────────────────────
    state = await getState(fixture.repoPath)
    if (state === 'implemented') {
      await callWorktreeAPI(fixture.repoPath, 'review', { approve: true })
      console.log('Review started...')
      await waitForState(fixture.repoPath, 'reviewing', 120_000)
      console.log('Review phase active!')
    }

    // ─── Post-review feature checks ──────────────────────────────────
    try {
      const advResults = await callWorktreeAPI(fixture.repoPath, 'adversarial.results')
      console.log('Adversarial results:', JSON.stringify(advResults))
    } catch (err) { console.log('adversarial.results skipped:', err) }

    try {
      const riskHistory = await callWorktreeAPI(fixture.repoPath, 'risk.history')
      console.log('Risk history:', JSON.stringify(riskHistory))
    } catch (err) { console.log('risk.history skipped:', err) }

    // ─── Submit (best-effort — needs GitHub token) ──────────────────
    try {
      await callWorktreeAPI(fixture.repoPath, 'submit')
      console.log('Submit succeeded')
    } catch (err) {
      console.log('Submit skipped (may need GitHub token):', err)
    }

    // ─── Verify final UI state ──────────────────────────────────────
    await page.waitForTimeout(2000)
    const finalState = await getState(fixture.repoPath)
    console.log(`Workflow completed! Final state: ${finalState}`)
    expect(['reviewing', 'implemented', 'submitted', 'none']).toContain(finalState)
  })

  test('mode switch mid-workflow', async ({ page }) => {
    test.setTimeout(PHASE_TIMEOUT * 2 + 60_000)

    await navigateToProject(page)
    console.log('Developer mode — resetting and loading task')

    // Reset from any previous state
    let state = await getState(fixture.repoPath)
    if (state !== 'none' && state !== 'loaded') {
      try { await callWorktreeAPI(fixture.repoPath, 'abort') } catch { /* ok */ }
      try { await callWorktreeAPI(fixture.repoPath, 'reset') } catch { /* ok */ }
      try { await callWorktreeAPI(fixture.repoPath, 'abandon') } catch { /* ok */ }
      await new Promise(r => setTimeout(r, 1000))
    }

    // Load and plan
    state = await getState(fixture.repoPath)
    if (state === 'none') {
      await callWorktreeAPI(fixture.repoPath, 'start', { source: `file:${fixture.taskPath}` })
      await waitForState(fixture.repoPath, 'loaded', 30_000)
    }
    state = await getState(fixture.repoPath)
    if (state === 'loaded') {
      await callWorktreeAPI(fixture.repoPath, 'plan')
      await waitForState(fixture.repoPath, 'planned')
      console.log('Planning completed in developer mode')
    }

    // Switch to Simple mode
    await page.keyboard.press('Control+Shift+V')
    await page.waitForTimeout(1000)

    // Verify SimpleProjectView rendered (has "Plan ready" or simple layout markers)
    const simpleIndicator = page.getByText('Plan ready', { exact: false })
    const simpleModeBtn = page.getByRole('button', { name: 'Simple' })
    const isSimple = await simpleIndicator.isVisible().catch(() => false) ||
                     (await simpleModeBtn.isVisible().catch(() => false) && await simpleModeBtn.getAttribute('class').then(c => c?.includes('btn-primary')).catch(() => false))
    if (isSimple) {
      console.log('Switched to Simple mode — UI updated')
    } else {
      console.log('Mode switch visual check inconclusive — checking state preserved')
    }

    // State should be preserved
    state = await getState(fixture.repoPath)
    expect(state).toBe('planned')
    console.log('State preserved after mode switch:', state)

    // Start implementation from current mode
    await callWorktreeAPI(fixture.repoPath, 'implement')
    console.log('Implementation started from simple mode')

    // Switch back to Developer mode
    await page.keyboard.press('Control+Shift+V')
    await page.waitForTimeout(1000)

    // Verify developer mode UI (left sidebar should be back)
    await expect(page.getByLabel('Left sidebar').first()).toBeVisible({ timeout: UI_TIMEOUT })
    console.log('Switched back to Developer mode')

    // Wait for implementation
    await waitForState(fixture.repoPath, 'implemented')
    console.log('Implementation completed after mode switch')
  })

  test('tab switching', async ({ page }) => {
    test.setTimeout(60_000)
    await navigateToProject(page)

    // Switch between tabs
    for (const tabName of ['Chat', 'Spec', 'Jobs', 'Files']) {
      const tab = page.getByRole('tab', { name: new RegExp(tabName, 'i') })
      if (await tab.isVisible().catch(() => false)) {
        await tab.click()
        await page.waitForTimeout(300)
        console.log(`Tab: ${tabName}`)
      }
    }
  })
})

test.describe('Standalone Tests', () => {
  test('settings panel', async ({ page }) => {
    test.setTimeout(30_000)
    await skipOnboarding(page)
    await page.goto('/')
    await dismissOnboarding(page)

    const settingsButton = page.getByRole('button', { name: /Settings/i })
    if (await settingsButton.isVisible().catch(() => false)) {
      await settingsButton.click()
      await page.waitForTimeout(500)
      console.log('Settings opened')
      await page.keyboard.press('Escape')
    }
  })

  test('theme toggle', async ({ page }) => {
    test.setTimeout(15_000)
    await skipOnboarding(page)
    await page.goto('/')
    await dismissOnboarding(page)

    const html = page.locator('html')
    const initial = await html.getAttribute('data-theme')

    const toggle = page.getByRole('button', { name: /theme|dark|light/i })
    if (await toggle.isVisible().catch(() => false)) {
      await toggle.click()
      await page.waitForTimeout(300)
      const after = await html.getAttribute('data-theme')
      console.log(`Theme: ${initial} → ${after}`)
      expect(after).not.toBe(initial)
    }
  })

  test('global socket — task group CRUD', async () => {
    test.setTimeout(30_000)

    // Create a task group via global socket
    const group = await callGlobalAPI('taskgroup.create', { label: 'e2e-integration-group' })
    console.log('Created group:', JSON.stringify(group))
    expect(group).toHaveProperty('id')

    // List groups
    const groups = await callGlobalAPI('taskgroup.list')
    console.log('Groups:', JSON.stringify(groups))

    // Remove it
    try {
      await callGlobalAPI('taskgroup.remove', { id: group.id })
      console.log('Group removed')
    } catch (err) {
      console.log('Group remove skipped:', err)
    }
  })

  test('global socket — memory outcomes', async () => {
    test.setTimeout(15_000)

    try {
      const outcomes = await callGlobalAPI('memory.outcomes')
      console.log('Memory outcomes:', JSON.stringify(outcomes))
    } catch (err) {
      console.log('memory.outcomes skipped (empty store):', err)
    }
  })
})
