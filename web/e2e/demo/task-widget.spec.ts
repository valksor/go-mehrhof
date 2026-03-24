import { test, expect } from '@playwright/test'

test.describe('TaskWidget in demo mode', () => {
  test('shows three input mode tabs', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByRole('tab', { name: 'Quick Task' })).toBeVisible()
    await expect(page.getByRole('tab', { name: 'From File' })).toBeVisible()
    await expect(page.getByRole('tab', { name: 'From URL' })).toBeVisible()
  })

  test('quick task tab has textarea and load button', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByPlaceholder(/describe what you want/i)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Load Task' })).toBeVisible()
  })

  test('load button is disabled when textarea is empty', async ({ page }) => {
    await page.goto('/?demo')

    const loadButton = page.getByRole('button', { name: 'Load Task' })
    await expect(loadButton).toBeDisabled()
  })

  test('switching to From URL tab shows URL input', async ({ page }) => {
    await page.goto('/?demo')

    await page.getByRole('tab', { name: 'From URL' }).click()

    const urlInput = page.getByPlaceholder(/github\.com/i)
    await expect(urlInput).toBeVisible()
  })

  test('URL input detects GitHub provider', async ({ page }) => {
    await page.goto('/?demo')

    await page.getByRole('tab', { name: 'From URL' }).click()

    const urlInput = page.getByPlaceholder(/github\.com/i)
    await urlInput.fill('https://github.com/owner/repo/issues/123')

    // Provider badge should appear
    await expect(page.getByText('GH')).toBeVisible()
  })

  test('URL input detects GitLab provider', async ({ page }) => {
    await page.goto('/?demo')

    await page.getByRole('tab', { name: 'From URL' }).click()

    const urlInput = page.getByPlaceholder(/github\.com/i)
    await urlInput.fill('https://gitlab.com/group/project/-/issues/456')

    await expect(page.getByText('GL')).toBeVisible()
  })

  test('switching to From File tab shows browse button', async ({ page }) => {
    await page.goto('/?demo')

    await page.getByRole('tab', { name: 'From File' }).click()

    await expect(page.getByRole('button', { name: /browse/i })).toBeVisible()
  })

  test('quick fix toggle is present', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByText('Quick Fix')).toBeVisible()
    await expect(page.getByRole('checkbox', { name: /quick fix/i })).toBeVisible()
  })

  test('shows connection status', async ({ page }) => {
    await page.goto('/?demo')

    // In demo mode without backend, shows "Not connected"
    await expect(page.getByText('Not connected')).toBeVisible()
  })

  test('shows Ctrl+Enter hint', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByText('Ctrl+Enter to load')).toBeVisible()
  })
})
