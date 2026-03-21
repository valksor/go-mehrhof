import { test, expect } from '@playwright/test'

test.describe('View mode switching', () => {
  test('developer mode is active by default in demo', async ({ page }) => {
    await page.goto('/?demo')

    const toggle = page.getByTestId('view-mode-toggle')
    await expect(toggle).toBeVisible()

    // Developer button should be active
    const devButton = toggle.getByRole('button', { name: 'Developer' })
    await expect(devButton).toHaveClass(/btn-active/)
  })

  test('switching to simple mode shows SimpleProjectView', async ({ page }) => {
    await page.goto('/?demo')

    // Click Simple button
    const toggle = page.getByTestId('view-mode-toggle')
    await toggle.getByRole('button', { name: 'Simple' }).click()

    // SimpleProjectView shows a task input area with label
    await expect(page.getByText('Enter a GitHub issue URL')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Start' })).toBeVisible()
  })

  test('switching back to developer mode restores ProjectView', async ({ page }) => {
    await page.goto('/?demo')

    const toggle = page.getByTestId('view-mode-toggle')

    // Switch to simple
    await toggle.getByRole('button', { name: 'Simple' }).click()
    await expect(page.getByText('Enter a GitHub issue URL')).toBeVisible()

    // Switch back to developer
    await toggle.getByRole('button', { name: 'Developer' }).click()

    // Developer mode has sidebars
    await expect(page.getByRole('complementary', { name: 'Left sidebar' })).toBeVisible()
    await expect(page.getByRole('complementary', { name: 'Right sidebar' })).toBeVisible()
  })

  test('simple mode shows status banner', async ({ page }) => {
    await page.goto('/?demo')

    await page.getByTestId('view-mode-toggle').getByRole('button', { name: 'Simple' }).click()

    // Status banner shows "Ready to start" for none state
    await expect(page.getByText('Ready to start')).toBeVisible()
  })

  test('simple mode has back button', async ({ page }) => {
    await page.goto('/?demo')

    await page.getByTestId('view-mode-toggle').getByRole('button', { name: 'Simple' }).click()

    await expect(page.getByRole('button', { name: 'Back to projects' })).toBeVisible()
  })

  test('simple mode shows project name', async ({ page }) => {
    await page.goto('/?demo')

    await page.getByTestId('view-mode-toggle').getByRole('button', { name: 'Simple' }).click()

    await expect(page.getByRole('heading', { name: 'my-project' })).toBeVisible()
  })

  test('URL param overrides view mode', async ({ page }) => {
    await page.goto('/?demo&mode=simple')

    // Should land directly in simple mode
    await expect(page.getByText('Ready to start')).toBeVisible()
  })
})
