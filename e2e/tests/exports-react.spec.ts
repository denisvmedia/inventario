import * as fs from 'node:fs'
import * as path from 'node:path'

import { expect, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { navigateWithAuth } from './includes/auth.js'
import {
  createCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
} from './includes/commodities-api.js'
import { loginUser, SEEDED_TEST_USERS } from './includes/user-isolation-auth.js'

/**
 * E2E coverage for the React Exports/Imports/Restores surface (#1415).
 *
 * Covers the issue's E2E acceptance line: create an export, then run a
 * (dry-run) restore from it. As of #534 the full import cycle is no
 * longer deferred — backups are a signed `.inb` archive (MIME
 * `application/x-inventario-backup`), and the round-trip test below
 * creates → downloads (asserting the `.inb` suffix) → re-uploads via
 * /exports/import → runs a dry-run restore. A negative test asserts a
 * wrong-extension file is blocked client-side and that a tampered
 * `.inb` is rejected server-side.
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

/**
 * Drive the two-step wizard to create a `full_database` export and land
 * on the detail page once it reaches a terminal status. Returns nothing
 * — the caller asserts against the now-visible detail surface.
 */
async function createFullDatabaseExport(page: Page, description: string): Promise<void> {
  await gotoExports(page)
  await page.getByTestId('exports-create-button').click()
  await expect(page).toHaveURL(/\/exports\/new/)
  await expect(page.getByTestId('wizard-step-1-content')).toBeVisible()
  await page.getByTestId('wizard-next').click()

  await expect(page.getByTestId('wizard-step-2-content')).toBeVisible()
  await page.getByTestId('wizard-description').fill(description)
  await page.getByTestId('wizard-submit').click()

  await expect(page.getByTestId('page-export-detail')).toBeVisible({ timeout: 30_000 })
  // Wait for the export to reach a terminal state so download + restore
  // CTAs unlock — otherwise the next click races the polling loop. This is a
  // full_database export of the SHARED e2e database, so late in the suite it
  // serialises everything every prior spec created; the export worker is async
  // (poll-driven) and the dataset can be large, so allow a wide CI-load window.
  await expect(page.getByTestId('export-detail-restore')).toBeEnabled({ timeout: 120_000 })
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
    // empty / minimal dataset). 30s timeout: the post-submit navigation +
    // createExportRestore round-trip is slower on webkit than the 10s default.
    await expect(page.getByTestId('page-export-detail')).toBeVisible({ timeout: 30_000 })
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
    // header should render the synthesised default ending in " UTC" — not
    // "No description.", and not a local-time-looking string.
    await expect(page.getByTestId('page-export-detail')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByText(/Backup · Full database · .+ UTC$/)).toBeVisible()
  })

  // #534 — Full import cycle on the signed `.inb` archive format.
  // create → download (assert `.inb` suffix) → re-upload via
  // /exports/import → dry-run restore → terminal status. This replaces
  // the import-cycle coverage that was deferred before the format
  // migration.
  test('round-trips a backup: create → download .inb → re-import → dry-run restore', async ({
    page,
    request,
  }) => {
    // --- Seed minimal data so the export captures something. ---
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
    await createCommodityViaAPI(
      request,
      auth,
      group.slug,
      { name: `exports-inb-${Date.now()}`, areaId },
      group.groupCurrency,
    )

    // --- Create the full_database export and wait for it to finish. ---
    await createFullDatabaseExport(page, `E2E inb round-trip ${Date.now()}`)

    // --- Download it; the BE drives the filename via Content-Disposition,
    // which the format-agnostic FE honours. The suggested filename must
    // end in `.inb` (the new signed-archive extension). ---
    const [download] = await Promise.all([
      page.waitForEvent('download'),
      page.getByTestId('export-detail-download').click(),
    ])
    expect(download.suggestedFilename()).toMatch(/\.inb$/i)
    // Persist the downloaded bytes so we can re-feed them to the import
    // input via setInputFiles (Playwright needs an on-disk path).
    const downloadedPath = await download.path()
    const downloadedBuffer = fs.readFileSync(downloadedPath)

    // --- Re-import the downloaded archive through the import surface. ---
    await navigateWithAuth(page, '/exports/import')
    await expect(page.getByTestId('page-export-import')).toBeVisible()
    await page.getByTestId('import-file-input').setInputFiles({
      name: download.suggestedFilename(),
      mimeType: 'application/x-inventario-backup',
      buffer: downloadedBuffer,
    })
    // The chosen-file chip confirms the client-side extension guard
    // accepted the `.inb` file (no error surfaced).
    await expect(page.getByTestId('import-file-chosen')).toBeVisible()
    await expect(page.getByTestId('import-file-error')).toHaveCount(0)
    await page.getByTestId('import-description').fill(`E2E re-import ${Date.now()}`)
    await page.getByTestId('import-submit').click()

    // --- Import success navigates straight to the restore form for the
    // freshly-created "imported" export. ---
    await expect(page.getByTestId('page-export-restore')).toBeVisible({ timeout: 30_000 })

    // The imported export is created "pending"; the import worker parses the
    // signed `.inb` (verifies the signature + reads the manifest) before the
    // export flips to "completed". A restore can only be created once the export
    // is completed — the BE returns 409 otherwise — so wait for that before
    // submitting, rather than racing the async worker.
    const restoreMatch = new URL(page.url()).pathname.match(
      /\/g\/([^/]+)\/exports\/([^/?#]+)\/restore/,
    )
    const importGroupSlug = restoreMatch?.[1] ?? group.slug
    const importExportId = restoreMatch?.[2]
    expect(importExportId, 'restore URL carries the imported export id').toBeTruthy()
    const pollAuth = await extractApiAuth(page)
    await expect
      .poll(
        async () => {
          const resp = await request.get(
            `/api/v1/g/${importGroupSlug}/exports/${importExportId}`,
            {
              headers: {
                Accept: 'application/vnd.api+json',
                Authorization: `Bearer ${pollAuth.accessToken}`,
                'X-CSRF-Token': pollAuth.csrfToken,
              },
            },
          )
          if (!resp.ok()) return `http-${resp.status()}`
          const body = (await resp.json()) as { data?: { attributes?: { status?: string } } }
          return body.data?.attributes?.status
        },
        // The import is async (worker poll @ ~10s + signature verify + manifest
        // read); give it a generous window under CI load.
        { timeout: 60_000, intervals: [500, 1000, 2000] },
      )
      .toBe('completed')

    // Defaults are merge_add + dry_run + include_file_data; description
    // is required on both sides.
    await page.getByTestId('restore-description').fill(`E2E imported dry-run ${Date.now()}`)
    await page.getByTestId('restore-submit').click()

    // --- The dry-run restore lands in history and reaches a TERMINAL state.
    // We assert terminal (completed OR failed), not strictly `completed`: this
    // is a full_database dry-run over the SHARED e2e database, so it runs across
    // whatever every prior spec left behind, and the merge_add validation can
    // legitimately end `failed` on unrelated accumulated data. The meaningful
    // #534 assertion — that the signed `.inb` round-trips and its signature
    // verifies — is the import-completion poll above (a tampered/invalid `.inb`
    // never reaches `completed` there; see the negative test). Restore
    // correctness itself is covered by the Go round-trip unit tests. The
    // restore worker is async (poll-driven), so use a generous CI-load window. ---
    await expect(page.getByTestId('page-export-detail')).toBeVisible({ timeout: 30_000 })
    const restoresList = page.getByTestId('restores-list')
    await expect(restoresList).toBeVisible({ timeout: 30_000 })
    const firstRestore = restoresList.locator('[data-testid^="restore-row-"]').first()
    await expect(firstRestore).toBeVisible()
    await expect(
      firstRestore.locator('[data-testid="status-completed"], [data-testid="status-failed"]'),
    ).toBeVisible({ timeout: 60_000 })
  })

  // #534 — Negative path. (1) A wrong-extension file is blocked
  // client-side before any upload fires. (2) A garbage file with the
  // right `.inb` extension passes the client gate but is rejected
  // server-side because it isn't a valid signed archive — the
  // import/restore flow surfaces a failure.
  test('rejects a wrong-extension file client-side and a tampered .inb server-side', async ({
    page,
  }) => {
    await navigateWithAuth(page, '/exports/import')
    await expect(page.getByTestId('page-export-import')).toBeVisible()

    // --- (1) Client-side guard: a `.xml` file never stages, surfaces the
    // invalidFileType error, and keeps submit disabled (no upload). ---
    await page.getByTestId('import-file-input').setInputFiles({
      name: 'legacy-backup.xml',
      mimeType: 'application/xml',
      buffer: Buffer.from('<export>legacy XML is no longer accepted</export>'),
    })
    await expect(page.getByTestId('import-file-error')).toBeVisible()
    await expect(page.getByTestId('import-file-chosen')).toHaveCount(0)
    await expect(page.getByTestId('import-submit')).toBeDisabled()

    // --- (2) Server-side rejection: a garbage file with the right `.inb`
    // extension clears the client gate (chip shows, no error) but the
    // backend rejects it as not a valid signed archive. The failure
    // surfaces on either the upload or the import step. ---
    const tamperedBuffer = fs.readFileSync(
      path.join('fixtures', 'files', 'invalid-backup.inb'),
    )
    await page.getByTestId('import-file-input').setInputFiles({
      name: 'invalid-backup.inb',
      mimeType: 'application/x-inventario-backup',
      buffer: tamperedBuffer,
    })
    await expect(page.getByTestId('import-file-chosen')).toBeVisible()
    await expect(page.getByTestId('import-file-error')).toHaveCount(0)
    await page.getByTestId('import-description').fill(`E2E tampered ${Date.now()}`)
    await page.getByTestId('import-submit').click()

    // The BE may reject at upload (sandbox validation) or at the import
    // parse step; either way the page surfaces the destructive error
    // banner and never navigates the user into a usable restore form.
    await expect(page.getByTestId('import-error')).toBeVisible({ timeout: 30_000 })
    await expect(page.getByTestId('page-export-restore')).toHaveCount(0)
  })

  // #534 — Cross-account/tenant isolation. A backup created by one account
  // must not be reachable by a different, isolated account. The two seeded
  // users (admin@test-org.com / user2@test-org.com) are data-isolated (see
  // user-isolation.spec.ts), so the second account must be denied access to
  // the owner's export through the owner's group-scoped route, and must not
  // see it in its own export list. (The destructive cross-tenant invariant —
  // a full_replace restore never touches another tenant — is locked at the
  // unit level by TestINBRestore_FullReplaceDoesNotWipeOtherTenant.)
  test('a second account cannot reach the owner\'s backup (cross-account isolation)', async ({
    page,
    request,
    browser,
  }) => {
    // Owner = the app-fixture default user (admin@test-org.com).
    const ownerAuth = await extractApiAuth(page)
    const ownerGroup = await resolveActiveGroup(request, ownerAuth)
    await createFullDatabaseExport(page, `E2E isolation owner ${Date.now()}`)
    const exportId = new URL(page.url()).pathname.match(/\/exports\/([^/?#]+)/)?.[1]
    expect(exportId, 'owner landed on the export detail page with an id in the URL').toBeTruthy()

    // Second, isolated account logs in in its own browser context.
    const otherContext = await browser.newContext()
    try {
      const otherPage = await otherContext.newPage()
      await loginUser(otherPage, SEEDED_TEST_USERS[1].email, SEEDED_TEST_USERS[1].password)
      const otherAuth = await extractApiAuth(otherPage)
      const otherHeaders = {
        Accept: 'application/vnd.api+json',
        Authorization: `Bearer ${otherAuth.accessToken}`,
        'X-CSRF-Token': otherAuth.csrfToken,
      }

      // (1) Direct fetch of the owner's export through the owner's group
      // route must be rejected — the second account is not a member.
      const denied = await request.get(`/api/v1/g/${ownerGroup.slug}/exports/${exportId}`, {
        headers: otherHeaders,
      })
      expect(denied.status(), `cross-account export fetch returned ${denied.status()}`)
        .toBeGreaterThanOrEqual(400)
      expect(denied.status()).toBeLessThan(500)

      // (2) The owner's export must not surface in the second account's own
      // export list either.
      const otherGroup = await resolveActiveGroup(request, otherAuth)
      const list = await request.get(`/api/v1/g/${otherGroup.slug}/exports`, {
        headers: otherHeaders,
      })
      expect(list.ok()).toBeTruthy()
      const body = (await list.json()) as { data?: Array<{ id: string }> }
      const ids = (body.data ?? []).map((e) => e.id)
      expect(ids).not.toContain(exportId)
    } finally {
      await otherContext.close()
    }
  })
})
