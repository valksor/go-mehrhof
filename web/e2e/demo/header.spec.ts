import { test, expect } from '@playwright/test'

test.describe('ProjectView header in demo mode', () => {
  test('header toolbar buttons are present', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByRole('button', { name: 'View logs' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Settings' })).toBeVisible()
  })

  test('project path is displayed', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByText('/Users/demo/workspace/my-project')).toBeVisible()
  })

  test('status badge shows No Task', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByText('No Task')).toBeVisible()
  })

  test('theme toggle is in header', async ({ page }) => {
    await page.goto('/?demo')

    const themeBtn = page.getByRole('button', { name: /switch to/i })
    await expect(themeBtn).toBeVisible()
  })

  test('view mode toggle is in header', async ({ page }) => {
    await page.goto('/?demo')

    const toggle = page.getByTestId('view-mode-toggle')
    await expect(toggle).toBeVisible()
    await expect(toggle.getByRole('button', { name: 'Simple' })).toBeVisible()
    await expect(toggle.getByRole('button', { name: 'Developer' })).toBeVisible()
  })

  test('skip link is present for accessibility', async ({ page }) => {
    await page.goto('/?demo')

    const skipLink = page.getByRole('link', { name: 'Skip to main content' })
    await expect(skipLink).toBeAttached()
  })
})
