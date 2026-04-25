/**
 * E2E tests for the global Cmd+K command palette (#1330 PR 5.4).
 *
 * The palette is mounted from `App.vue` on every authenticated,
 * non-print, non-auth route. The hotkey wires through
 * `useKeyboardShortcuts({ key: 'k', modifiers: ['mod'] })` so it
 * matches Cmd+K on macOS and Ctrl+K everywhere else; Playwright's
 * `Control+KeyK` shortcut works on both platforms in CI thanks to the
 * cross-platform `mod` matcher.
 */
import { test } from '../fixtures/app-fixture.js'
import { expect } from '@playwright/test'

test.describe('Command palette — Cmd+K / Ctrl+K', () => {
  test('opens with the keyboard shortcut and closes with Escape', async ({ page, recorder }) => {
    await page.goto('/')
    await expect(page.locator('h1')).toBeVisible()

    // Open the palette.
    await page.keyboard.press('Control+KeyK')
    const palette = page.locator('[data-testid="command-palette"]')
    await expect(palette).toBeVisible()
    await recorder.takeScreenshot('command-palette-01-open')

    // The search input is autofocused.
    const search = palette.locator('input[role="searchbox"]')
    await expect(search).toBeFocused()

    // Initial hint is shown until the user types ≥ 2 characters.
    await expect(palette).toContainText('Type at least 2 characters to search.')

    // Esc closes the palette.
    await page.keyboard.press('Escape')
    await expect(palette).not.toBeVisible()
  })

  test('shows the empty-results message for queries that match nothing', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('h1')).toBeVisible()

    await page.keyboard.press('Control+KeyK')
    const palette = page.locator('[data-testid="command-palette"]')
    await expect(palette).toBeVisible()

    const search = palette.locator('input[role="searchbox"]')
    await search.fill('zzzzznosuchitem9999')

    // The debounced search settles after 300 ms; allow a generous window.
    await expect(palette).toContainText(/No results for/i, { timeout: 5000 })
  })
})
