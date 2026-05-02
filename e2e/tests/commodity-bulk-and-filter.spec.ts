/**
 * E2E round-trip coverage for the commodities bulk-delete + filter
 * flows on the React `/commodities` list page (#1410). Closes the
 * parts of #1449 the existing `bulk-actions.spec.ts` only smoke-tests:
 *
 *   - bulk-actions.spec.ts proves the BulkActionsBar appears + the
 *     clear button hides it. It does NOT actually delete anything.
 *   - This spec seeds two commodities via the API helpers in
 *     `commodities-api.ts`, walks the UI bulk-delete flow end to end,
 *     and asserts both rows are gone from the list.
 *   - A second test asserts the type filter narrows the visible set
 *     and the URL reflects the active filter (round-trip safe across
 *     reload).
 *
 * Bulk-move + sort + search round-trips are deliberately deferred to
 * follow-up commits — they need richer fixtures (multiple areas, a
 * couple of distinct registered_dates, etc.) and pile up review surface.
 */
import { expect } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import {
  createCommodityViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
} from './includes/commodities-api.js'
import { navigateTo, TO_COMMODITIES } from './includes/navigate.js'

test.describe('Commodities — bulk + filter round-trips', () => {
  test('bulk-delete two seeded commodities → both gone from the list', async ({
    page,
    request,
    recorder,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const aName = `Bulk Delete A ${suffix}`
    const bName = `Bulk Delete B ${suffix}`

    const seeded: { id: string; name: string }[] = []
    seeded.push(
      await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: aName, areaId, type: 'electronics' },
        group.mainCurrency,
      ),
    )
    seeded.push(
      await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: bName, areaId, type: 'electronics' },
        group.mainCurrency,
      ),
    )
    // Best-effort safety net: any seeded commodity that survives the
    // UI flow (because the test failed mid-way) is cleaned up here so
    // the next run starts from the same baseline. Successful runs
    // reach this with all rows already deleted; the API helper is
    // 404-tolerant and silently no-ops.
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      await navigateTo(page, recorder, TO_COMMODITIES)

      // Both seeded cards must be visible before we start selecting.
      const cardA = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      const cardB = page.locator(`[data-commodity-id="${seeded[1].id}"]`)
      await expect(cardA).toBeVisible({ timeout: 15000 })
      await expect(cardB).toBeVisible()

      // Toggle the per-card checkbox on each row. Reka's CheckboxRoot
      // is a `<button role="checkbox">` and Playwright's actionability
      // gate flakes on overlay-positioned checkboxes — `dispatchEvent`
      // fires the synthetic event without the visibility heuristic.
      // Same workaround the existing bulk-actions.spec uses.
      for (const card of [cardA, cardB]) {
        const cb = card.locator('[data-testid="commodity-select"]')
        await cb.scrollIntoViewIfNeeded()
        await cb.dispatchEvent('click')
      }

      const bar = page.locator('[data-testid="commodities-bulk-bar"]')
      await expect(bar).toBeVisible()
      // The count is a plain `<span>` inside the bar — no per-element
      // testid — so we match on the i18n string ("2 items selected").
      await expect(bar).toContainText(/2 items? selected/)

      // Click the bulk-delete CTA and accept the confirm. The bar
      // mounts the BE roundtrip; we wait for the DELETE response so a
      // slow CI run doesn't leave the assertions racing the network.
      const deletePromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/commodities/bulk-delete') && resp.request().method() === 'POST',
        { timeout: 15000 },
      )
      await bar.locator('[data-testid="commodities-bulk-delete"]').click()
      await expect(page.getByTestId('confirm-dialog')).toBeVisible()
      await page.getByTestId('confirm-accept').click()
      const deleteResponse = await deletePromise
      expect(deleteResponse.status()).toBeLessThan(300)

      // Both cards must be gone. The list query refetches on
      // mutation success — no need to navigate away and back.
      await expect(cardA).toHaveCount(0, { timeout: 15000 })
      await expect(cardB).toHaveCount(0)

      // BulkActionsBar collapses once the selection is empty.
      await expect(bar).toBeHidden()
    } finally {
      await cleanup()
    }
  })

  test('type filter narrows the list + the URL reflects the active filter', async ({
    page,
    request,
    recorder,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const electronicsName = `Filter electronics ${suffix}`
    const furnitureName = `Filter furniture ${suffix}`

    const seeded: { id: string; name: string }[] = []
    seeded.push(
      await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: electronicsName, areaId, type: 'electronics' },
        group.mainCurrency,
      ),
    )
    seeded.push(
      await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: furnitureName, areaId, type: 'furniture' },
        group.mainCurrency,
      ),
    )
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      await navigateTo(page, recorder, TO_COMMODITIES)

      const electronicsCard = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      const furnitureCard = page.locator(`[data-commodity-id="${seeded[1].id}"]`)
      await expect(electronicsCard).toBeVisible({ timeout: 15000 })
      await expect(furnitureCard).toBeVisible()

      // Apply the type=electronics filter via the DropdownMenu. The
      // trigger is a `<Button>` (not a `<select>`); items are
      // `DropdownMenuCheckboxItem`s rendered with `role="menuitemcheckbox"`.
      // We click the trigger to open the menu, then pick the
      // "Electronics" item by role+name. After the change the URL
      // should carry `type=electronics`, only the electronics card
      // stays visible, and the BE re-fetched the narrower list
      // (verified via waitForResponse on the matching GET so a stale
      // cache hit can't pass the assertion).
      await page.getByTestId('commodities-filter-type').click()
      const filterPromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/commodities') &&
          resp.url().includes('type=electronics') &&
          resp.request().method() === 'GET',
        { timeout: 15000 },
      )
      await page.getByRole('menuitemcheckbox', { name: /electronics/i }).click()
      await filterPromise
      // Close the menu (it stays open after toggling a CheckboxItem)
      // so the cards underneath are interactable for the assertions.
      await page.keyboard.press('Escape')

      await expect(page).toHaveURL(/[?&]type=electronics(?:&|$)/)
      await expect(electronicsCard).toBeVisible()
      await expect(furnitureCard).toHaveCount(0)
    } finally {
      await cleanup()
    }
  })
})
