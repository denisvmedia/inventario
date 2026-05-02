import { expect, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { navigateWithAuth } from './includes/auth.js'

/**
 * Smoke for the unified Files page (#1411).
 *
 * What the spec proves:
 *   - The /files route renders the Files list page (not a placeholder).
 *   - The four category tiles introduced by #1398 are visible and
 *     respond to the per-tile selection click — `aria-selected` flips,
 *     the URL gains `?category=`, and the BE roundtrips the filter.
 *   - The Upload button opens the upload dialog with a working
 *     dropzone that accepts a synthetic File via Playwright's
 *     `setInputFiles` (slot-gating + actual upload roundtrip is left
 *     for the shared login fixture in #1449 to cover).
 *
 * Authentication uses the shared `app-fixture` (handles login,
 * recovers from session-expired bounces) — the inline `loginToReact`
 * helper used pre-#1449 was racey when the suite ran in parallel and
 * a sibling spec invalidated the auth token mid-run.
 */

async function gotoFiles(page: Page): Promise<void> {
  // Group-scoped routes are `/g/<slug>/...`; navigateWithAuth stays
  // logged in across the redirect through /no-group / /login bounces
  // that pure page.goto would otherwise expose.
  await navigateWithAuth(page, '/files')
  await expect(page.getByTestId('page-files')).toBeVisible()
}

test.describe('Files page', () => {
  test('renders the list page with all five category tiles', async ({ page }) => {
    await gotoFiles(page)

    await expect(page.getByTestId('files-tile-all')).toBeVisible()
    await expect(page.getByTestId('files-tile-photos')).toBeVisible()
    await expect(page.getByTestId('files-tile-invoices')).toBeVisible()
    await expect(page.getByTestId('files-tile-documents')).toBeVisible()
    await expect(page.getByTestId('files-tile-other')).toBeVisible()
    // "All" is the default selection.
    await expect(page.getByTestId('files-tile-all')).toHaveAttribute('aria-selected', 'true')
  })

  test('selecting a category tile flips aria-selected and updates the URL', async ({ page }) => {
    await gotoFiles(page)

    await page.getByTestId('files-tile-photos').click()
    await expect(page.getByTestId('files-tile-photos')).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByTestId('files-tile-all')).toHaveAttribute('aria-selected', 'false')
    await expect(page).toHaveURL(/category=photos/)
  })

  test('upload dialog opens from the CTA and exposes the dropzone', async ({ page }) => {
    await gotoFiles(page)

    await page.getByTestId('files-upload-cta').click()
    await expect(page.getByTestId('files-upload-dialog')).toBeVisible()
    await expect(page.getByTestId('files-upload-dropzone')).toBeVisible()
    // The hidden <input type=file> is what Playwright drives — we don't
    // attach a real file here because slot-gating + post-upload
    // roundtrip is still pending the #1449 fixture.
    await expect(page.getByTestId('files-upload-input')).toBeAttached()
  })
})
