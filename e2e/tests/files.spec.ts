import * as fs from 'node:fs'
import * as path from 'node:path'

import { expect, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { navigateWithAuth } from './includes/auth.js'

/**
 * E2E coverage for the unified Files page (#1411).
 *
 * Covers AC #7 of the issue:
 *   - upload → list → detail → download → delete full flow
 *   - per-category content filter (an uploaded image surfaces in
 *     "All" and "Photos" tiles, not in "Invoices" or "Documents")
 * plus the original smoke (tile rendering, tile-click aria-selected,
 * upload-dialog open) which stays as a fast-failure layer above the
 * end-to-end flow.
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
    await expect(page.getByTestId('files-upload-input')).toBeAttached()
  })

  test('upload → list → detail → download URL → delete', async ({ page }) => {
    // Closes #1411 AC #7. Uses a unique filename per run because the
    // backend persists uploads and the test DB is shared with sibling
    // specs; the tail-end delete returns the row count to its
    // pre-test value, but the upload itself must not collide on path.
    const uniqueName = `e2e-upload-${Date.now()}.jpg`
    const fixtureBuffer = fs.readFileSync(path.join('fixtures', 'files', 'image.jpg'))

    await gotoFiles(page)

    // --- Upload --- (drag-drop + slot-gating + roundtrip) ----------
    await page.getByTestId('files-upload-cta').click()
    await expect(page.getByTestId('files-upload-dialog')).toBeVisible()
    await page.getByTestId('files-upload-input').setInputFiles({
      name: uniqueName,
      mimeType: 'image/jpeg',
      buffer: fixtureBuffer,
    })
    await page.getByTestId('files-upload-next').click()
    const [uploadResponse] = await Promise.all([
      page.waitForResponse(
        (resp) => resp.url().includes('/uploads/file') && resp.request().method() === 'POST',
        { timeout: 30000 }
      ),
      page.getByTestId('files-upload-start').click(),
    ])
    expect(uploadResponse.status()).toBe(201)
    await page.getByTestId('files-upload-close').click()
    await expect(page.getByTestId('files-upload-dialog')).toBeHidden()

    // --- List shows the new row --------------------------------------
    // Filter by Photos so we both prove the category-filter path AND
    // narrow the list to a slice that fits a single page even on a
    // shared DB. The card title falls back to the filename minus
    // extension when title is unset; we filter the locator by text.
    await page.getByTestId('files-tile-photos').click()
    await expect(page).toHaveURL(/category=photos/)
    const card = page
      .locator('[data-testid^="file-card-"][data-category="photos"]')
      .filter({ hasText: stripExt(uniqueName) })
      .first()
    await expect(card).toBeVisible({ timeout: 15000 })

    // --- Detail sheet opens with the right metadata ----------------
    // file-card-open-<id> is on the inner button; clicking via the
    // outer card locator hits it through the dom-tree, since the card
    // body is an <a> wrapping the same target.
    await card.getByTestId(/file-card-open-/).click()
    const sheet = page.getByTestId('file-detail-sheet')
    await expect(sheet).toBeVisible()
    await expect(sheet.getByTestId('file-detail-category')).toHaveText(/photos/i)
    await expect(sheet.getByTestId('file-detail-filename')).toContainText(stripExt(uniqueName))

    // --- Download link is present and points at a signed URL -------
    const downloadLink = sheet.getByTestId('file-detail-download')
    await expect(downloadLink).toBeVisible()
    const href = await downloadLink.getAttribute('href')
    expect(href).toBeTruthy()
    // Sanity: the signed URL is hosted under our origin (not an
    // external bucket) so a cross-domain navigation isn't required
    // to verify the download path.
    expect(href!).toContain('/files/')

    // --- Delete from the detail sheet ------------------------------
    await sheet.getByTestId('file-detail-delete').click()
    await expect(page.getByTestId('confirm-dialog')).toBeVisible()
    const [deleteResponse] = await Promise.all([
      page.waitForResponse(
        (resp) => resp.url().includes('/files/') && resp.request().method() === 'DELETE',
        { timeout: 15000 }
      ),
      page.getByTestId('confirm-accept').click(),
    ])
    expect(deleteResponse.status()).toBeLessThan(300)

    // --- After delete, the row is gone from the Photos slice -------
    await expect(
      page
        .locator('[data-testid^="file-card-"][data-category="photos"]')
        .filter({ hasText: stripExt(uniqueName) })
    ).toHaveCount(0, { timeout: 15000 })
  })

  test('per-category filter narrows the visible cards by category', async ({ page }) => {
    // This proves the BE /files?category=… filter wires through end
    // to end: an uploaded image must surface on All + Photos tiles
    // and NOT on Invoices / Documents. Cleans up after itself so
    // the row count returns to its pre-test value.
    const uniqueName = `e2e-cat-${Date.now()}.jpg`
    const fixtureBuffer = fs.readFileSync(path.join('fixtures', 'files', 'image.jpg'))

    await gotoFiles(page)

    // Upload one image (becomes category=photos by MIME derivation
    // in models.FileCategoryFromContext).
    await page.getByTestId('files-upload-cta').click()
    await page.getByTestId('files-upload-input').setInputFiles({
      name: uniqueName,
      mimeType: 'image/jpeg',
      buffer: fixtureBuffer,
    })
    await page.getByTestId('files-upload-next').click()
    await Promise.all([
      page.waitForResponse(
        (resp) => resp.url().includes('/uploads/file') && resp.request().method() === 'POST',
        { timeout: 30000 }
      ),
      page.getByTestId('files-upload-start').click(),
    ])
    await page.getByTestId('files-upload-close').click()
    await expect(page.getByTestId('files-upload-dialog')).toBeHidden()

    // Photos tile → must show our image.
    await page.getByTestId('files-tile-photos').click()
    const photosCard = page
      .locator('[data-testid^="file-card-"][data-category="photos"]')
      .filter({ hasText: stripExt(uniqueName) })
      .first()
    await expect(photosCard).toBeVisible({ timeout: 15000 })

    // Invoices tile → must NOT show our image.
    await page.getByTestId('files-tile-invoices').click()
    await expect(page).toHaveURL(/category=invoices/)
    await expect(
      page
        .locator('[data-testid^="file-card-"]')
        .filter({ hasText: stripExt(uniqueName) })
    ).toHaveCount(0)

    // Documents tile → must NOT show our image.
    await page.getByTestId('files-tile-documents').click()
    await expect(page).toHaveURL(/category=documents/)
    await expect(
      page
        .locator('[data-testid^="file-card-"]')
        .filter({ hasText: stripExt(uniqueName) })
    ).toHaveCount(0)

    // Cleanup: All tile → open the card we uploaded → delete via the
    // detail sheet so the row count is the same as before this test.
    await page.getByTestId('files-tile-all').click()
    const allCard = page
      .locator('[data-testid^="file-card-"]')
      .filter({ hasText: stripExt(uniqueName) })
      .first()
    await expect(allCard).toBeVisible({ timeout: 15000 })
    await allCard.getByTestId(/file-card-open-/).click()
    await page.getByTestId('file-detail-delete').click()
    await Promise.all([
      page.waitForResponse(
        (resp) => resp.url().includes('/files/') && resp.request().method() === 'DELETE',
        { timeout: 15000 }
      ),
      page.getByTestId('confirm-accept').click(),
    ])
  })
})

// stripExt drops the final ".ext" so card-title text matching can
// ignore the extension difference between filename + display title.
function stripExt(filename: string): string {
  const dot = filename.lastIndexOf('.')
  return dot > 0 ? filename.slice(0, dot) : filename
}
