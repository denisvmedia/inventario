/**
 * E2E coverage for first-class warranty tracking (#1367). Walks the
 * acceptance criterion in the issue:
 *
 *   1. Seed three commodities via the API — one expiring 65 days out
 *      (status=active), one expiring 30 days out (status=expiring),
 *      one expiring last week (status=expired).
 *   2. Open the dedicated /g/:slug/warranties page → assert each row
 *      lands on the right tab (the Active/Expiring/Expired filter
 *      hits the BE `warranty_status=` param shipped with #1367).
 *   3. Open one commodity's detail page, click the Warranty tab,
 *      and check the status pill matches the seeded date.
 *
 * The reminder-worker side of the acceptance ("tick clock 35 days
 * → reminder fires once; idempotent on re-run") lives in the Go
 * service unit test
 * `services.TestWarrantyReminderService_RemindOnce_TickClock` —
 * the time-injection logic doesn't translate to Playwright cleanly
 * and the unit test runs deterministically against the same
 * threshold list.
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

function dateOffsetISO(daysFromNow: number): string {
  const d = new Date()
  d.setUTCDate(d.getUTCDate() + daysFromNow)
  return d.toISOString().slice(0, 10)
}

test.describe('Warranties — list view + detail surface', () => {
  test('seeded warranties show up under the right tabs and the detail pill matches', async ({
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
      const activeName = `Warranty Active ${suffix}`
      const expiringName = `Warranty Expiring ${suffix}`
      const expiredName = `Warranty Expired ${suffix}`

      const { id: activeId } = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        {
          name: activeName,
          areaId,
          warrantyExpiresAt: dateOffsetISO(65),
        },
        group.groupCurrency,
      )
      seededIDs.push(activeId)

      const { id: expiringId } = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        {
          name: expiringName,
          areaId,
          warrantyExpiresAt: dateOffsetISO(30),
        },
        group.groupCurrency,
      )
      seededIDs.push(expiringId)

      const { id: expiredId } = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        {
          name: expiredName,
          areaId,
          warrantyExpiresAt: dateOffsetISO(-7),
        },
        group.groupCurrency,
      )
      seededIDs.push(expiredId)

      // 1) /warranties (default "All" tab) shows every seeded row.
      await page.goto(`/g/${encodeURIComponent(group.slug)}/warranties`)
      await expect(page.getByTestId('page-warranties')).toBeVisible()
      const allRowActive = page.locator(`[data-testid="warranties-row-${activeId}"]`)
      const allRowExpiring = page.locator(`[data-testid="warranties-row-${expiringId}"]`)
      const allRowExpired = page.locator(`[data-testid="warranties-row-${expiredId}"]`)
      await expect(allRowActive).toBeVisible({ timeout: 15000 })
      await expect(allRowExpiring).toBeVisible()
      await expect(allRowExpired).toBeVisible()

      // 2) "Active" tab — hits the BE `warranty_status=active` filter.
      await page.getByTestId('warranties-tab-active').click()
      await expect(page.getByTestId(`warranties-row-${activeId}`)).toBeVisible({ timeout: 15000 })
      await expect(page.locator(`[data-testid="warranties-row-${expiringId}"]`)).toHaveCount(0)
      await expect(page.locator(`[data-testid="warranties-row-${expiredId}"]`)).toHaveCount(0)

      // 3) "Expiring soon" tab — hits warranty_status=expiring.
      await page.getByTestId('warranties-tab-expiring').click()
      await expect(page.getByTestId(`warranties-row-${expiringId}`)).toBeVisible({ timeout: 15000 })
      await expect(page.locator(`[data-testid="warranties-row-${activeId}"]`)).toHaveCount(0)
      await expect(page.locator(`[data-testid="warranties-row-${expiredId}"]`)).toHaveCount(0)

      // 4) "Expired" tab — hits warranty_status=expired.
      await page.getByTestId('warranties-tab-expired').click()
      await expect(page.getByTestId(`warranties-row-${expiredId}`)).toBeVisible({ timeout: 15000 })
      await expect(page.locator(`[data-testid="warranties-row-${activeId}"]`)).toHaveCount(0)
      await expect(page.locator(`[data-testid="warranties-row-${expiringId}"]`)).toHaveCount(0)

      // 5) Detail page — Warranty tab shows the computed pill.
      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(expiringId)}`,
      )
      await page.getByRole('tab', { name: /^Warranty$/ }).click()
      await expect(page.getByTestId('commodity-detail-warranty')).toBeVisible()
      await expect(page.getByTestId('commodity-detail-warranty-status')).toContainText(
        /Expiring soon/i,
      )
    } finally {
      await cleanup()
    }
  })
})
