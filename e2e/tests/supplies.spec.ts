/**
 * E2E coverage for the per-commodity Supply Links feature (#1369).
 *
 * Acceptance criteria from the issue:
 *   - Create a commodity, add three supply links, reorder, click one,
 *     redirect to the external URL.
 *
 * The flow:
 *   1. Open a fresh commodity → Supplies tab → empty state.
 *   2. Add three supply links via the dialog.
 *   3. Verify all three rows render in creation order (sort_order 0..2).
 *   4. Reorder: move the third link to the top via the "Move up"
 *      buttons. Verify the new order persists across a reload.
 *   5. Click one row's "Open" button → it has the right external URL.
 *   6. Delete one row via the confirm dialog → row disappears.
 *
 * Per-step API seeding mirrors loans.spec.ts: data is keyed by a
 * per-run suffix so two CI runs hitting the shared DB don't collide,
 * and the cleanup hook runs before any seed throws so partial failures
 * still get cleaned up.
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

test.describe('Commodity supply links (#1369)', () => {
  test('add three links, reorder, click out, delete one', async ({ page, request }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const commodityName = `Supplies Coffee Machine ${suffix}`
    const seededIDs: string[] = []
    const cleanup = async () => {
      for (const id of seededIDs) {
        await deleteCommodityViaAPI(request, auth, group.slug, id).catch(() => {})
      }
    }

    try {
      const { id: commodityID } = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: commodityName, areaId, type: 'equipment' },
        group.groupCurrency,
      )
      seededIDs.push(commodityID)

      // 1) Open the detail page → Supplies tab → empty state.
      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(commodityID)}`,
      )
      await expect(page.getByTestId('commodity-detail-tabs')).toBeVisible()
      await page.getByRole('tab', { name: /^Supplies$/ }).click()
      await expect(page.getByTestId('supplies-tab')).toBeVisible()
      await expect(page.getByTestId('supplies-empty')).toBeVisible()

      // 2) Add three supply links. The first becomes sort_order=0, the
      //    second sort_order=1, the third sort_order=2 — the SupplyLinkService
      //    defaults sort_order to the current list length on Create.
      const seedLinks = [
        { label: 'Water filter', url: 'https://example.com/water-filter' },
        { label: 'Descaler', url: 'https://example.com/descaler' },
        { label: 'Espresso beans', url: 'https://example.com/beans' },
      ]
      for (const link of seedLinks) {
        await page.getByTestId('supplies-add').click()
        const dialog = page.getByTestId('supply-link-dialog')
        await expect(dialog).toBeVisible()
        await dialog.getByTestId('supply-link-label-input').fill(link.label)
        await dialog.getByTestId('supply-link-url-input').fill(link.url)
        await dialog.getByTestId('supply-link-submit').click()
        await expect(dialog).toBeHidden()
      }

      // 3) Verify the three rows render in creation order. The list
      //    item carries `data-testid="supplies-row"` and a
      //    `data-supply-id` attribute the test uses for ordering checks.
      const rows = page.locator('[data-testid="supplies-row"]')
      await expect(rows).toHaveCount(3)
      const labelsBefore = await page
        .locator('[data-testid="supplies-row-label"]')
        .allInnerTexts()
      expect(labelsBefore).toEqual([
        'Water filter',
        'Descaler',
        'Espresso beans',
      ])

      // 4) Reorder: bring "Espresso beans" (row index 2) up twice so it
      //    becomes row 0. Each click triggers a /reorder POST and
      //    invalidates the cache.
      await page
        .locator('[data-testid="supplies-row"]')
        .nth(2)
        .getByTestId('supplies-move-up')
        .click()
      await expect
        .poll(async () =>
          page.locator('[data-testid="supplies-row-label"]').allInnerTexts(),
        )
        .toEqual(['Water filter', 'Espresso beans', 'Descaler'])

      await page
        .locator('[data-testid="supplies-row"]')
        .nth(1)
        .getByTestId('supplies-move-up')
        .click()
      await expect
        .poll(async () =>
          page.locator('[data-testid="supplies-row-label"]').allInnerTexts(),
        )
        .toEqual(['Espresso beans', 'Water filter', 'Descaler'])

      // Reload to confirm the order persisted server-side (the
      // sort_order column was densely renumbered 0..N-1).
      await page.reload()
      await page.getByRole('tab', { name: /^Supplies$/ }).click()
      await expect
        .poll(async () =>
          page.locator('[data-testid="supplies-row-label"]').allInnerTexts(),
        )
        .toEqual(['Espresso beans', 'Water filter', 'Descaler'])

      // 5) Click the "Open" anchor on the top row. It points at the
      //    external URL with target="_blank" — assert the href rather
      //    than waiting for a new tab so the test stays hermetic.
      const topOpen = page
        .locator('[data-testid="supplies-row"]')
        .first()
        .getByTestId('supplies-row-open')
      await expect(topOpen).toHaveAttribute('href', 'https://example.com/beans')
      await expect(topOpen).toHaveAttribute('target', '_blank')

      // 6) Delete the middle row ("Water filter"). The confirm dialog
      //    uses the shared confirm-accept testid.
      await page
        .locator('[data-testid="supplies-row"]')
        .nth(1)
        .getByTestId('supplies-delete')
        .click()
      await page.getByTestId('confirm-accept').click()
      await expect
        .poll(async () =>
          page.locator('[data-testid="supplies-row-label"]').allInnerTexts(),
        )
        .toEqual(['Espresso beans', 'Descaler'])
    } finally {
      await cleanup()
    }
  })
})
