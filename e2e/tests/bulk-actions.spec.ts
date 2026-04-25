/**
 * E2E smoke for the bulk-actions UX (#1330 PR 5.5).
 *
 * The full bulk-delete / bulk-move round-trip needs seeded entities,
 * which the existing CRUD spec set covers separately. This spec just
 * verifies the per-card checkbox + BulkActionsBar are present on the
 * commodity / file list views, the bar mounts only when a row is
 * selected, and the clear button puts it back into the empty state.
 */
import { test } from '../fixtures/app-fixture.js'
import { expect } from '@playwright/test'
import { navigateTo, TO_COMMODITIES, TO_HOME } from './includes/navigate.js'
import { navigateWithAuth } from './includes/auth.js'

test.describe('Bulk actions — selection UX', () => {
  test('Commodities list: selecting a card opens BulkActionsBar; clear closes it', async ({
    page,
    recorder,
  }) => {
    await navigateTo(page, recorder, TO_COMMODITIES)

    const bar = page.locator('[data-testid="bulk-actions-bar"]')
    await expect(bar).toBeHidden()

    const firstCheckbox = page.locator('[data-testid^="commodity-select-"]').first()
    const cardCount = await page.locator('[data-testid^="commodity-select-"]').count()
    test.skip(cardCount === 0, 'No commodities seeded — cannot exercise bulk-actions UX')

    // The checkbox is absolutely positioned over the CommodityCard; in
    // some viewports Playwright's strict visibility heuristic flags
    // absolute-positioned overlays as "not visible" even when they
    // render correctly. `force: true` bypasses that check and clicks
    // by element bounds — the underlying click handler still runs.
    await firstCheckbox.click({ force: true })

    await expect(bar).toBeVisible()
    await expect(bar.locator('[data-testid="bulk-actions-count"]')).toContainText('1')
    await expect(bar.locator('[data-testid="bulk-action-delete"]')).toBeVisible()
    await expect(bar.locator('[data-testid="bulk-action-move"]')).toBeVisible()

    await bar.locator('[data-testid="bulk-actions-clear"]').click()
    await expect(bar).toBeHidden()
  })

  test('Files list: selecting a card opens BulkActionsBar; clear closes it', async ({
    page,
    recorder,
  }) => {
    // Land inside a group first so /files is rewritten correctly.
    await navigateTo(page, recorder, TO_HOME)
    await navigateWithAuth(page, '/files', recorder)

    const bar = page.locator('[data-testid="bulk-actions-bar"]')
    await expect(bar).toBeHidden()

    const firstCheckbox = page.locator('[data-testid^="file-select-"]').first()
    const cardCount = await page.locator('[data-testid^="file-select-"]').count()
    test.skip(cardCount === 0, 'No files seeded — cannot exercise bulk-actions UX')

    await firstCheckbox.click({ force: true })

    await expect(bar).toBeVisible()
    await expect(bar.locator('[data-testid="bulk-actions-count"]')).toContainText('1')
    await expect(bar.locator('[data-testid="bulk-action-delete"]')).toBeVisible()

    await bar.locator('[data-testid="bulk-actions-clear"]').click()
    await expect(bar).toBeHidden()
  })
})
