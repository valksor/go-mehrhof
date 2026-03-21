import { test, expect } from '@playwright/test'

test.describe('ProjectView in demo mode', () => {
  test('shows project header with mock project name', async ({ page }) => {
    await page.goto('/?demo')

    // Demo mode injects a mock project at /Users/demo/workspace/my-project
    await expect(page.getByRole('heading', { name: 'my-project' })).toBeVisible()
  })

  test('displays back button to projects', async ({ page }) => {
    await page.goto('/?demo')

    // Back link shows "Projects" text (hidden on small screens, visible on SM+)
    await expect(page.getByText('Projects').first()).toBeVisible()
  })

  test('shows task widget in left sidebar', async ({ page }) => {
    await page.goto('/?demo')

    // Widget title "Task" in the left sidebar (rendered as span, uppercased by CSS)
    const leftSidebar = page.getByRole('complementary', { name: 'Left sidebar' })
    await expect(leftSidebar.getByText('Task', { exact: true })).toBeVisible()
  })

  test('shows agents widget in right sidebar', async ({ page }) => {
    await page.goto('/?demo')

    const rightSidebar = page.getByRole('complementary', { name: 'Right sidebar' })
    await expect(rightSidebar.getByText('Agents', { exact: true })).toBeVisible()
  })

  test('shows file changes widget', async ({ page }) => {
    await page.goto('/?demo')

    const leftSidebar = page.getByRole('complementary', { name: 'Left sidebar' })
    await expect(leftSidebar.getByText('File Changes', { exact: true })).toBeVisible()
  })

  test('shows checkpoints widget', async ({ page }) => {
    await page.goto('/?demo')

    const rightSidebar = page.getByRole('complementary', { name: 'Right sidebar' })
    await expect(rightSidebar.getByText('Checkpoints', { exact: true })).toBeVisible()
  })

  test('displays status badge with No Task state', async ({ page }) => {
    await page.goto('/?demo')

    // Demo mode state is 'none', which shows 'No Task' in the status badge
    await expect(page.getByText('No Task')).toBeVisible()
  })

  test('shows chat area with placeholder', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByText('Start a conversation')).toBeVisible()
    await expect(page.getByPlaceholder(/type a message/i)).toBeVisible()
  })

  test('settings button is present', async ({ page }) => {
    await page.goto('/?demo')

    const settingsButton = page.getByRole('button', { name: /settings/i })
    await expect(settingsButton).toBeVisible()
  })

  test('task widget has load task button', async ({ page }) => {
    await page.goto('/?demo')

    // In 'none' state, the Task widget shows a Load Task button
    await expect(page.getByRole('button', { name: 'Load Task' })).toBeVisible()
  })

  test('output panel is present', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByText('Output', { exact: true })).toBeVisible()
    await expect(page.getByText('No output yet')).toBeVisible()
  })
})
