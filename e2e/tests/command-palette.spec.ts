/**
 * E2E tests for the global Cmd+K command palette (#1330 PR 5.4).
 *
 * The palette queries `/api/v1/search`, which is mounted by the backend
 * only inside the group-scoped router (`/api/v1/g/{slug}/search`). The
 * frontend axios interceptor rewrites the flat URL when the current
 * route carries a `:groupSlug` param. We therefore navigate into a
 * group-scoped page (`/locations`, which `gotoScoped` rewrites to
 * `/g/{slug}/locations`) before opening the palette — opening it from
 * `/` would hit a non-existent endpoint and 404.
 *
 * Keystroke note: `ControlOrMeta+KeyK` is the cross-platform shortcut
 * spelling. The `useKeyboardShortcuts` composable's `mod` modifier
 * matches `metaKey` on macOS / `ctrlKey` everywhere else; webkit on
 * macOS-latest runners refused a `Control+KeyK` press because Reka
 * sees `ctrlKey=true` but the composable wants `metaKey=true`.
 */
import { test } from '../fixtures/app-fixture.js'
import { expect } from '@playwright/test'
import { navigateWithAuth } from './includes/auth.js'

test.describe('Command palette — Cmd+K / Ctrl+K', () => {
  test('opens with the keyboard shortcut and closes with Escape', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder)
    await expect(page.locator('h1')).toBeVisible()

    // Open the palette.
    await page.keyboard.press('ControlOrMeta+KeyK')
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

  test('shows the empty-results message for queries that match nothing', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder)
    await expect(page.locator('h1')).toBeVisible()

    await page.keyboard.press('ControlOrMeta+KeyK')
    const palette = page.locator('[data-testid="command-palette"]')
    await expect(palette).toBeVisible()

    const search = palette.locator('input[role="searchbox"]')
    await search.fill('zzzzznosuchitem9999')

    // The debounced search settles after 300 ms; allow a generous window.
    await expect(palette).toContainText(/No results for/i, { timeout: 5000 })
  })
})
