import { test, expect } from '@playwright/test'

test.describe('Keyboard shortcuts in demo mode', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/?demo')
    // Wait for the app to fully mount before testing keyboard shortcuts
    await expect(page.getByRole('heading', { name: 'my-project' })).toBeVisible()
  })

  test('Ctrl+/ opens keyboard shortcuts help', async ({ page }) => {
    await page.keyboard.press('Control+/')

    await expect(page.getByText('Keyboard Shortcuts')).toBeVisible()
  })

  test('Escape closes keyboard shortcuts help', async ({ page }) => {
    // Open help
    await page.keyboard.press('Control+/')
    await expect(page.getByText('Keyboard Shortcuts')).toBeVisible()

    // Close help
    await page.keyboard.press('Escape')
    await expect(page.getByText('Keyboard Shortcuts')).not.toBeVisible()
  })

  test('shortcuts help shows navigation section', async ({ page }) => {
    await page.keyboard.press('Control+/')

    await expect(page.getByText('Navigation')).toBeVisible()
    await expect(page.getByText('Focus chat input')).toBeVisible()
  })

  test('shortcuts help shows workflow section in developer mode', async ({ page }) => {
    await page.keyboard.press('Control+/')

    await expect(page.getByText('Workflow')).toBeVisible()
  })

  test('Ctrl+Shift+V toggles view mode', async ({ page }) => {
    // Ensure focus is on main (non-input, has tabindex=-1)
    await page.locator('main').focus()

    // Toggle to simple using keyboard
    await page.keyboard.press('Control+Shift+v')

    // Simple mode shows status banner
    await expect(page.getByText('Ready to start')).toBeVisible()
  })
})
