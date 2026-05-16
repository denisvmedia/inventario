/**
 * E2E coverage for the commodities-list "Lent out" toolbar chip (#1510).
 *
 * Walks the full round-trip the issue's acceptance criterion lists:
 *   1. Seed two commodities (one will be lent, one stays at home).
 *   2. Lend the first one via the API (re-using the UI lend dialog adds
 *      churn the loans.spec already covers).
 *   3. Hit /commodities → toggle the "Lent out" chip → only the lent
 *      commodity is visible.
 *   4. Mark the loan returned via the API.
 *   5. The (still-toggled) chip empties — the just-returned commodity
 *      no longer matches the BE's `lent_out=true` filter.
 *
 * Per-step API seeding mirrors loans.spec — keyed by a per-run suffix
 * so two CI runs on the shared DB don't collide; cleanup hook runs
 * before any seed throws.
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

test.describe('Commodities list — Lent out filter (#1510)', () => {
  test('toggle filter shows only currently-lent items, returns flushes them', async ({
    page,
    request,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const lentName = `Lent Drill ${suffix}`
    const idleName = `Idle Drill ${suffix}`
    const seededIDs: string[] = []
    let loanID: string | undefined
    const cleanup = async () => {
      for (const id of seededIDs) {
        await deleteCommodityViaAPI(request, auth, group.slug, id).catch(() => {})
      }
    }

    try {
      const lent = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: lentName, areaId, type: 'equipment' },
        group.groupCurrency,
      )
      seededIDs.push(lent.id)

      const idle = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: idleName, areaId, type: 'equipment' },
        group.groupCurrency,
      )
      seededIDs.push(idle.id)

      // Open a loan on the first commodity. Using the API skips the
      // dialog churn — the lend-via-UI path is owned by loans.spec.
      const headers = {
        'Content-Type': 'application/vnd.api+json',
        Accept: 'application/vnd.api+json',
        Authorization: `Bearer ${auth.accessToken}`,
        'X-CSRF-Token': auth.csrfToken,
      }
      const seeded = await request.post(
        `/api/v1/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(lent.id)}/loans`,
        {
          headers,
          data: {
            data: {
              type: 'commodity_loans',
              attributes: {
                borrower_name: `Borrower ${suffix}`,
                lent_at: new Date().toISOString().slice(0, 10),
              },
            },
          },
        },
      )
      expect(seeded.ok()).toBeTruthy()
      loanID = (await seeded.json())?.data?.id as string | undefined
      expect(loanID, 'seeded loan response missing data.id').toBeTruthy()

      // Land on the list, narrow to our suffix so the page is clean,
      // and confirm both rows render before the toggle.
      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/commodities?q=${encodeURIComponent(suffix)}`,
      )
      await expect(page.getByTestId('page-commodities')).toBeVisible()
      const lentCard = page.locator(`[data-commodity-id="${lent.id}"]`)
      const idleCard = page.locator(`[data-commodity-id="${idle.id}"]`)
      await expect(lentCard).toBeVisible({ timeout: 15000 })
      await expect(idleCard).toBeVisible({ timeout: 15000 })

      // Toggle the chip → only the lent row stays.
      const chip = page.getByTestId('commodities-filter-lent-out')
      await expect(chip).toHaveAttribute('aria-pressed', 'false')
      await chip.click()
      await expect(chip).toHaveAttribute('aria-pressed', 'true')
      await expect(idleCard).toHaveCount(0, { timeout: 15000 })
      await expect(lentCard).toBeVisible()

      // Close the loan via the dedicated return endpoint. PATCH only
      // mutates borrower fields + due_back_at; the loan service exposes
      // POST .../loans/{id}/return for the close transition. Sending
      // an empty body lets the BE default returned_at to today.
      const returnResp = await request.post(
        `/api/v1/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(lent.id)}/loans/${encodeURIComponent(loanID!)}/return`,
        { headers },
      )
      expect(returnResp.ok()).toBeTruthy()

      // Re-issue the list with the chip still toggled (URL state
      // survives the navigation). Both rows must now be absent — the
      // returned commodity dropped out of `lent_out=true` and the
      // idle row was already excluded.
      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/commodities?q=${encodeURIComponent(suffix)}&lent_out=1`,
      )
      await expect(page.getByTestId('page-commodities')).toBeVisible()
      await expect(page.getByTestId('commodities-filter-lent-out')).toHaveAttribute(
        'aria-pressed',
        'true',
      )
      await expect(page.locator(`[data-commodity-id="${lent.id}"]`)).toHaveCount(0, {
        timeout: 15000,
      })
      await expect(page.locator(`[data-commodity-id="${idle.id}"]`)).toHaveCount(0)
    } finally {
      await cleanup()
    }
  })
})
