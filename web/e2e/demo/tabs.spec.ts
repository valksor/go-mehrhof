import { test, expect } from '@playwright/test'

test.describe('Tab management in demo mode', () => {
  test('default Chat tab is present', async ({ page }) => {
    await page.goto('/?demo')

    const tablist = page.getByRole('tablist', { name: 'Open tabs' })
    await expect(tablist).toBeVisible()

    const chatTab = tablist.getByRole('tab', { name: 'Chat' })
    await expect(chatTab).toBeVisible()
    await expect(chatTab).toHaveAttribute('aria-selected', 'true')
  })

  test('add tab button shows dropdown menu', async ({ page }) => {
    await page.goto('/?demo')

    const addButton = page.getByRole('button', { name: 'Add new tab' })
    await expect(addButton).toBeVisible()

    await addButton.click()

    const menu = page.getByRole('menu', { name: 'Tab types' })
    await expect(menu).toBeVisible()

    // Check available tab types
    await expect(menu.getByRole('menuitem', { name: 'Chat' })).toBeVisible()
    await expect(menu.getByRole('menuitem', { name: 'Spec' })).toBeVisible()
    await expect(menu.getByRole('menuitem', { name: 'Screenshots' })).toBeVisible()
    await expect(menu.getByRole('menuitem', { name: 'Jobs' })).toBeVisible()
    await expect(menu.getByRole('menuitem', { name: 'Files' })).toBeVisible()
    await expect(menu.getByRole('menuitem', { name: 'Browser' })).toBeVisible()
  })

  test('adding a new tab creates it and selects it', async ({ page }) => {
    await page.goto('/?demo')

    // Open add menu and add a Spec tab
    await page.getByRole('button', { name: 'Add new tab' }).click()
    await page.getByRole('menuitem', { name: 'Spec' }).click()

    // New tab should be selected
    const tablist = page.getByRole('tablist', { name: 'Open tabs' })
    const specTab = tablist.getByRole('tab', { name: 'Spec' })
    await expect(specTab).toBeVisible()
    await expect(specTab).toHaveAttribute('aria-selected', 'true')
  })

  test('closing a tab removes it', async ({ page }) => {
    await page.goto('/?demo')

    // Add a new tab
    await page.getByRole('button', { name: 'Add new tab' }).click()
    await page.getByRole('menuitem', { name: 'Jobs' }).click()

    const tablist = page.getByRole('tablist', { name: 'Open tabs' })
    await expect(tablist.getByRole('tab', { name: 'Jobs' })).toBeVisible()

    // Close it
    await page.getByRole('button', { name: 'Close Jobs' }).click()

    // Should be gone
    await expect(tablist.getByRole('tab', { name: 'Jobs' })).not.toBeVisible()
  })

  test('clicking a tab switches to it', async ({ page }) => {
    await page.goto('/?demo')

    // Add a second tab
    await page.getByRole('button', { name: 'Add new tab' }).click()
    await page.getByRole('menuitem', { name: 'Screenshots' }).click()

    const tablist = page.getByRole('tablist', { name: 'Open tabs' })

    // Click back to Chat
    await tablist.getByRole('tab', { name: 'Chat' }).click()
    await expect(tablist.getByRole('tab', { name: 'Chat' })).toHaveAttribute('aria-selected', 'true')
    await expect(tablist.getByRole('tab', { name: 'Screenshots' })).toHaveAttribute('aria-selected', 'false')
  })

  test('add menu closes when clicking outside', async ({ page }) => {
    await page.goto('/?demo')

    await page.getByRole('button', { name: 'Add new tab' }).click()
    await expect(page.getByRole('menu', { name: 'Tab types' })).toBeVisible()

    // Click outside the menu
    await page.locator('main').click({ position: { x: 10, y: 10 } })

    await expect(page.getByRole('menu', { name: 'Tab types' })).not.toBeVisible()
  })
})
