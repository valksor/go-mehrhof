/**
 * E2E tests for new features introduced in recent commits.
 * Tests run in demo mode (?demo) — no backend required.
 *
 * Covers: WorkflowBar (progress bar, autofix badge), ForkComparePanel tab,
 * and ReviewPanel (risk gauge).
 *
 * Note: GlobalView features (StatsWidget cache stats, TaskGroupPanel) are NOT
 * testable in demo mode because demo mode auto-selects a project, bypassing
 * GlobalView entirely. Those features are covered by integration tests instead.
 */

import { test, expect } from '@playwright/test'

test.describe('WorkflowBar — progress and autofix', () => {
  // WorkflowBar only renders when state !== 'none'.
  // Use ?demo&state=implementing to seed an active-phase state.
  test('progress bar renders with percentage and ETA', async ({ page }) => {
    await page.goto('/?demo&state=implementing')

    // Demo data seeds phaseProgress { percent: 65, eta: 135, calibrated: true }
    const progressText = page.getByText('65%')
    await expect(progressText).toBeVisible({ timeout: 5000 })
  })

  test('autofix badge shows attempt count', async ({ page }) => {
    await page.goto('/?demo&state=implementing')

    // Demo data seeds autoFixStatus { active: true, attempt: 1, maxAttempts: 3 }
    const autofixBadge = page.getByText('fix 1/3')
    await expect(autofixBadge).toBeVisible({ timeout: 5000 })
  })
})

test.describe('ForkComparePanel via tab', () => {
  test('fork tab can be opened and shows fork list', async ({ page }) => {
    await page.goto('/?demo')

    // Open add-tab menu (uses exact selectors from tabs.spec.ts)
    await page.getByRole('button', { name: 'Add new tab' }).click()
    const menu = page.getByRole('menu', { name: 'Tab types' })
    await expect(menu).toBeVisible()

    // Click the Forks menu item
    await menu.getByRole('menuitem', { name: 'Forks' }).click()

    // The panel should show our demo forks (pre-seeded in App.tsx)
    await expect(page.getByText('approach-a')).toBeVisible({ timeout: 5000 })
    await expect(page.getByText('approach-b')).toBeVisible({ timeout: 5000 })
  })
})

test.describe('ReviewPanel — risk gauge', () => {
  test('review tab opens and shows risk gauge', async ({ page }) => {
    await page.goto('/?demo')

    // Open review tab via add-tab menu
    await page.getByRole('button', { name: 'Add new tab' }).click()
    await page.getByRole('menu', { name: 'Tab types' }).getByRole('menuitem', { name: 'Review' }).click()

    // Demo data seeds riskScore { score: 0.35, level: 'low' }
    await expect(page.getByText('0.35')).toBeVisible({ timeout: 5000 })
    await expect(page.getByText(/low/i)).toBeVisible({ timeout: 5000 })
  })
})
