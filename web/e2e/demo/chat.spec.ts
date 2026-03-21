import { test, expect } from '@playwright/test'

test.describe('ChatWidget in demo mode', () => {
  test('shows empty state message', async ({ page }) => {
    await page.goto('/?demo')

    await expect(page.getByText('Start a conversation')).toBeVisible()
    await expect(page.getByText('Type @ to mention files')).toBeVisible()
  })

  test('chat input accepts text', async ({ page }) => {
    await page.goto('/?demo')

    const input = page.locator('[data-chat-input]')
    await expect(input).toBeVisible()

    await input.fill('Hello world')
    await expect(input).toHaveValue('Hello world')
  })

  test('send button is disabled when input is empty', async ({ page }) => {
    await page.goto('/?demo')

    const sendButton = page.getByRole('button', { name: 'Send message' })
    await expect(sendButton).toBeDisabled()
  })

  test('slash command triggers autocomplete', async ({ page }) => {
    await page.goto('/?demo')

    const input = page.locator('[data-chat-input]')
    await input.fill('/')

    // Command autocomplete should appear
    await expect(page.getByRole('listbox')).toBeVisible()
  })

  test('chat input has correct placeholder and aria attributes', async ({ page }) => {
    await page.goto('/?demo')

    const input = page.locator('[data-chat-input]')
    await expect(input).toBeVisible()
    await expect(input).toHaveAttribute('role', 'combobox')
    await expect(input).toHaveAttribute('aria-label', 'Message')
    await expect(input).toHaveAttribute('placeholder', 'Type a message... (@ files, / commands)')
  })
})
