import { test, expect } from '@playwright/test'

test.describe('Widget collapse/expand behavior', () => {
  test('task widget can be toggled', async ({ page }) => {
    await page.goto('/?demo')

    const leftSidebar = page.getByRole('complementary', { name: 'Left sidebar' })
    const taskWidget = leftSidebar.locator('[data-widget-id="task"]')

    // Find the toggle button (could say Collapse or Expand depending on initial state)
    const toggleBtn = taskWidget.getByRole('button', { name: /Collapse|Expand/ }).first()
    await expect(toggleBtn).toBeVisible()

    const initialState = await toggleBtn.getAttribute('aria-expanded')

    // Click to toggle
    await toggleBtn.click()

    // State should flip
    const newState = await toggleBtn.getAttribute('aria-expanded')
    expect(newState).not.toBe(initialState)
  })

  test('file changes widget is visible in left sidebar', async ({ page }) => {
    await page.goto('/?demo')

    const leftSidebar = page.getByRole('complementary', { name: 'Left sidebar' })
    const fcWidget = leftSidebar.locator('[data-widget-id="files"]')
    await expect(fcWidget).toBeVisible()
  })

  test('review history widget starts collapsed', async ({ page }) => {
    await page.goto('/?demo')

    const rightSidebar = page.getByRole('complementary', { name: 'Right sidebar' })
    const reviewWidget = rightSidebar.locator('[data-widget-id="review-history"]')

    // Use aria-expanded=false on the widget-level chevron (not the review item button)
    const expandBtn = reviewWidget.locator('button[aria-expanded="false"][aria-controls]').first()
    await expect(expandBtn).toBeVisible()
    await expect(expandBtn).toHaveAttribute('aria-label', 'Expand')
  })

  test('task history widget starts collapsed', async ({ page }) => {
    await page.goto('/?demo')

    const rightSidebar = page.getByRole('complementary', { name: 'Right sidebar' })
    const historyWidget = rightSidebar.locator('[data-widget-id="history"]')

    const expandBtn = historyWidget.getByRole('button', { name: 'Expand' })
    await expect(expandBtn).toBeVisible()
  })

  test('expanding a collapsed widget reveals content', async ({ page }) => {
    await page.goto('/?demo')

    const rightSidebar = page.getByRole('complementary', { name: 'Right sidebar' })
    const reviewWidget = rightSidebar.locator('[data-widget-id="review-history"]')

    // Expand using the widget-level chevron (has aria-controls pointing to content ID)
    await reviewWidget.locator('button[aria-expanded="false"][aria-controls]').first().click()

    // Content should be visible now (demo seeds a rejected review)
    await expect(reviewWidget.getByText('Rejected #1')).toBeVisible()

    // Chevron should now say Collapse (re-query since aria-expanded changed)
    const collapseBtn = reviewWidget.locator('button[aria-expanded="true"][aria-controls]').first()
    await expect(collapseBtn).toHaveAttribute('aria-label', 'Collapse')
  })
})

test.describe('Widget empty states', () => {
  test('file changes widget shows empty state', async ({ page }) => {
    await page.goto('/?demo')

    const leftSidebar = page.getByRole('complementary', { name: 'Left sidebar' })
    const filesWidget = leftSidebar.locator('[data-widget-id="files"]')
    await expect(filesWidget.getByText('No changes yet')).toBeVisible()
  })

  test('checkpoints widget shows empty state', async ({ page }) => {
    await page.goto('/?demo')

    const rightSidebar = page.getByRole('complementary', { name: 'Right sidebar' })
    const checkpointsWidget = rightSidebar.locator('[data-widget-id="checkpoints"]')
    await expect(checkpointsWidget.getByText('No checkpoints yet')).toBeVisible()
  })

  test('agents widget shows no workers', async ({ page }) => {
    await page.goto('/?demo')

    const rightSidebar = page.getByRole('complementary', { name: 'Right sidebar' })
    const agentsWidget = rightSidebar.locator('[data-widget-id="agents"]')
    await expect(agentsWidget.getByText('No workers running')).toBeVisible()
  })
})
