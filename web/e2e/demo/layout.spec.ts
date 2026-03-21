import { test, expect } from '@playwright/test'

test.describe('Panel layout in demo mode', () => {
  test('three-panel layout is visible', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByRole('complementary', { name: 'Left sidebar' })).toBeVisible()
    await expect(page.getByRole('complementary', { name: 'Right sidebar' })).toBeVisible()
  })

  test('left resize handle is present', async ({ page }) => {
    await page.goto('/?demo')

    const handle = page.getByRole('slider', { name: 'Resize left sidebar' })
    await expect(handle).toBeVisible()
  })

  test('right resize handle is present', async ({ page }) => {
    await page.goto('/?demo')

    const handle = page.getByRole('slider', { name: 'Resize right sidebar' })
    await expect(handle).toBeVisible()
  })

  test('output panel has collapse button', async ({ page }) => {
    await page.goto('/?demo')

    const collapseBtn = page.getByRole('button', { name: 'Collapse output panel' })
    await expect(collapseBtn).toBeVisible()
  })

  test('output panel can be collapsed', async ({ page }) => {
    await page.goto('/?demo')

    // Collapse the output panel
    await page.getByRole('button', { name: 'Collapse output panel' }).click()

    // "No output yet" should no longer be visible
    await expect(page.getByText('No output yet')).not.toBeVisible()

    // "Show Output Panel" expand button should appear
    await expect(page.getByText('Show Output Panel')).toBeVisible()
  })

  test('output panel has clear button', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByRole('button', { name: 'Clear' })).toBeVisible()
  })
})
