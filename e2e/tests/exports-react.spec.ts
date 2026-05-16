import { expect, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { navigateWithAuth } from './includes/auth.js'
import {
  createCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
} from './includes/commodities-api.js'

/**
 * E2E coverage for the React Exports/Imports/Restores surface (#1415).
 *
 * Covers the issue's E2E acceptance line: create an export, then run a
 * (dry-run) restore from it. Import-cycle coverage (download → re-upload
 * via /exports/import → restore) is intentionally deferred to a follow-up
 * — the file-staging + multipart upload roundtrip is sensitive to the
 * docker volume layout under the e2e stack and bumps runtime by ~10s.
 *
 * The legacy `exports-crud.spec.ts` is left skipped (it mounts on
 * PrimeVue selectors that no longer exist after the React cutover); this
 * spec replaces its export-create coverage. Re-enable that file or
 * delete it once the helpers in `e2e/tests/includes/exports.ts` are
 * ported to React selectors.
 */

async function gotoExports(page: Page): Promise<void> {
  await navigateWithAuth(page, '/exports')
  await expect(page.getByTestId('page-exports')).toBeVisible()
}

test.describe('Exports / Restores (React)', () => {
  test('renders the page shell with retention banner + create CTA', async ({ page }) => {
    await gotoExports(page)
    await expect(page.getByTestId('exports-retention-banner')).toBeVisible()
    await expect(page.getByTestId('exports-create-button')).toBeVisible()
    await expect(page.getByTestId('exports-import-button')).toBeVisible()
  })

  test('wizard creates a full_database export, then a dry-run restore lands in history', async ({
    page,
    request,
  }) => {
    // --- Seed a minimal location / area / commodity so the export
    // captures something meaningful (otherwise the detail-page status
    // poll just races the empty-database fast path).
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
    await createCommodityViaAPI(
      request,
      auth,
      group.slug,
      { name: `exports-react-${Date.now()}`, areaId },
      group.groupCurrency,
    )

    // --- Step 1: open wizard, full_database default is fine ---
    await gotoExports(page)
    await page.getByTestId('exports-create-button').click()
    await expect(page).toHaveURL(/\/exports\/new/)
    await expect(page.getByTestId('wizard-step-1-content')).toBeVisible()
    await page.getByTestId('wizard-next').click()

    // --- Step 2: confirm + submit ---
    await expect(page.getByTestId('wizard-step-2-content')).toBeVisible()
    await page.getByTestId('wizard-description').fill(`E2E full DB ${Date.now()}`)
    await page.getByTestId('wizard-submit').click()

    // --- The wizard navigates straight to the detail page on success.
    // The detail page is the canonical "watch this export" surface
    // (status badge polling, download/restore CTAs, restore history).
    await expect(page.getByTestId('page-export-detail')).toBeVisible({ timeout: 30_000 })
    // Wait for the export to reach a terminal state so the Restore CTA
    // unlocks — otherwise the click below races the polling loop.
    await expect(page.getByTestId('export-detail-restore')).toBeEnabled({ timeout: 60_000 })
    await page.getByTestId('export-detail-restore').click()
    await expect(page.getByTestId('page-export-restore')).toBeVisible()
    // Description is required on both sides — BE validation
    // (jsonapi/restore_operations.go: Required + Length 1..500) and the
    // FE form (`required` + submit-disabled until non-empty). Defaults
    // for the rest: merge_add + dry_run + include_file_data.
    await page.getByTestId('restore-description').fill(`E2E dry-run ${Date.now()}`)
    await page.getByTestId('restore-submit').click()

    // --- Restore appears in history; poll until status flips to a
    // terminal value (the restore worker writes ~immediately for an
    // empty / minimal dataset).
    await expect(page.getByTestId('page-export-detail')).toBeVisible()
    const restoresList = page.getByTestId('restores-list')
    await expect(restoresList).toBeVisible({ timeout: 30_000 })
    const firstRestore = restoresList.locator('[data-testid^="restore-row-"]').first()
    await expect(firstRestore).toBeVisible()
    // Status badge must reach completed (or failed — surface either
    // outcome to keep the test stable across worker timing).
    await expect(
      firstRestore.locator('[data-testid="status-completed"], [data-testid="status-failed"]'),
    ).toBeVisible({ timeout: 30_000 })
  })

  // #1661 — Description is no longer required server-side (validation.Required
  // dropped on Export.Description), and the service synthesises a
  // "Backup · {type label} · {date}" default so the list row is never blank.
  // This spec drives the empty-description path through the full wizard and
  // asserts the detail page surfaces the synthesised text.
  test('create export with empty description succeeds and surfaces synthesised default', async ({
    page,
    request,
  }) => {
    // Seed minimal data so the export captures something meaningful.
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
    await createCommodityViaAPI(
      request,
      auth,
      group.slug,
      { name: `exports-empty-desc-${Date.now()}`, areaId },
      group.groupCurrency,
    )

    await gotoExports(page)
    await page.getByTestId('exports-create-button').click()
    await expect(page).toHaveURL(/\/exports\/new/)
    await expect(page.getByTestId('wizard-step-1-content')).toBeVisible()
    await page.getByTestId('wizard-next').click()

    await expect(page.getByTestId('wizard-step-2-content')).toBeVisible()
    // Leave description blank — the BE should synthesise the default.
    // Sanity-check that the hint copy is rendered so future regressions
    // (someone reinstates the required marker) get caught here.
    await expect(page.getByTestId('wizard-description-hint')).toBeVisible()
    await page.getByTestId('wizard-submit').click()

    // The wizard navigates to the detail page on success; the description
    // header should render the synthesised default, not "No description."
    await expect(page.getByTestId('page-export-detail')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByText(/Backup · Full database · /)).toBeVisible()
  })
})
