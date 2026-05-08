/**
 * E2E coverage for issue #1554: a commodity with quantity > 1 is a
 * bundle of interchangeable units, not a single tracked instance.
 * The Warranty / Lend / Service tabs swap their bodies for an
 * empty-state hint that nudges the user to "split into separate
 * items" instead of letting them open dialogs that the BE would
 * reject with a 422.
 *
 * Locks the visible copy (translation key
 * `commodities:trackingRestrictions.*`) so future i18n edits don't
 * silently change the user-facing message.
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

test.describe('Issue #1554 — bundle commodities hide per-instance affordances', () => {
  test('Warranty / Lend / Service tabs render the bundle empty-state when count > 1', async ({
    page,
    request,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const seededIDs: string[] = []
    const cleanup = async () => {
      for (const id of seededIDs) {
        await deleteCommodityViaAPI(request, auth, group.slug, id).catch(() => {})
      }
    }

    try {
      const { id: bundleId } = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        {
          name: `Pack of bulbs ${suffix}`,
          areaId,
          count: 12,
        },
        group.groupCurrency,
      )
      seededIDs.push(bundleId)

      // Warranty tab — empty-state hint replaces the live status pill /
      // notes block. The pill in the page header is hidden too.
      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(bundleId)}?tab=warranty`,
      )
      await expect(page.getByTestId('commodity-detail-warranty')).toBeVisible()
      await expect(
        page.getByTestId('commodity-detail-warranty-bundle-empty-state'),
      ).toContainText(/quantity greater than 1/i)
      await expect(page.getByTestId('commodity-detail-warranty-pill')).toHaveCount(0)

      // Lend tab — bundle empty-state, no Lend button.
      await page.getByTestId('commodity-detail-tab-lend').click()
      await expect(page.getByTestId('lend-bundle-empty-state')).toBeVisible()
      await expect(page.getByTestId('commodity-detail-lend-button')).toHaveCount(0)

      // Service tab — bundle empty-state, no Send-for-service button.
      await page.getByTestId('commodity-detail-tab-service').click()
      await expect(page.getByTestId('service-bundle-empty-state')).toBeVisible()
      await expect(page.getByTestId('commodity-detail-service-button')).toHaveCount(0)
    } finally {
      await cleanup()
    }
  })
})
