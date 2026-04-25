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

    // The checkbox is an absolutely-positioned overlay on top of the
    // CommodityCard; if the card lives below the fold, the click hits
    // an offscreen point and Playwright bails. Scroll into view first
    // (force:true bypasses the visibility heuristic that flags
    // absolute-positioned children of `position:relative` parents).
    // Reka's CheckboxRoot is a `<button role="checkbox">`; webkit's
    // strict actionability check rejects clicks even after a successful
    // `scrollIntoViewIfNeeded` because the absolutely-positioned overlay
    // sits at the same coordinates as the parent CommodityCard and
    // Playwright's "stable + visible" gate flakes. `dispatchEvent('click')`
    // skips every actionability check and just fires the synthetic event;
    // the underlying `@update:model-value` handler still runs.
    await firstCheckbox.scrollIntoViewIfNeeded()
    await firstCheckbox.dispatchEvent('click')

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

    // Reka's CheckboxRoot is a `<button role="checkbox">`; webkit's
    // strict actionability check rejects clicks even after a successful
    // `scrollIntoViewIfNeeded` because the absolutely-positioned overlay
    // sits at the same coordinates as the parent CommodityCard and
    // Playwright's "stable + visible" gate flakes. `dispatchEvent('click')`
    // skips every actionability check and just fires the synthetic event;
    // the underlying `@update:model-value` handler still runs.
    await firstCheckbox.scrollIntoViewIfNeeded()
    await firstCheckbox.dispatchEvent('click')

    await expect(bar).toBeVisible()
    await expect(bar.locator('[data-testid="bulk-actions-count"]')).toContainText('1')
    await expect(bar.locator('[data-testid="bulk-action-delete"]')).toBeVisible()

    await bar.locator('[data-testid="bulk-actions-clear"]').click()
    await expect(bar).toBeHidden()
  })
})
