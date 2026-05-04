/**
 * E2E round-trip coverage for the commodities bulk + filter / sort /
 * search flows on the React `/commodities` list page (#1410). Closes
 * the parts of #1449 the existing `bulk-actions.spec.ts` only
 * smoke-tests (bar appears, clear button hides it):
 *
 *   - bulk-delete: seeds two commodities via the API helpers, walks
 *     the UI bulk-delete flow end to end, asserts both rows gone.
 *   - bulk-move: seeds two commodities in area A + a second area B,
 *     walks the bulk-move dialog, asserts both rows now live in B.
 *   - type filter: applies `type=electronics`, asserts URL state +
 *     only matching cards visible after refetch.
 *   - sort by `-registered_date`: seeds three rows sequentially,
 *     scopes via search, asserts the most-recent-first DOM order.
 *   - search: typing a unique substring narrows to the matching row
 *     only. URL gains `?q=...`.
 *
 * Each test runs an axe audit on the settled list state to satisfy
 * the #1449 AC ("All new assertions are axe-clean"). Dialog / dropdown
 * a11y is unit-tested separately with jest-axe.
 */
import { expect, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import {
  createAreaViaAPI,
  createCommodityViaAPI,
  deleteAreaViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
} from './includes/commodities-api.js'
import { navigateTo, TO_COMMODITIES } from './includes/navigate.js'
import { axeAudit } from '../utils/axe.js'

// Scope axe audits to the commodities-list page wrapper. The shared
// AppSidebar has known aria-hidden-focus + color-contrast issues that
// every authenticated page inherits; gating on them here would
// false-positive #1449 against pre-existing shell debt rather than
// the surfaces this spec actually exercises.
function auditList(page: Page): Promise<void> {
  return axeAudit(page, { include: '[data-testid="page-commodities"]' })
}

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

      // Axe-clean on the post-delete list (no bar, list refetched).
      await auditList(page)
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

      // Axe-clean on the filtered list state.
      await auditList(page)
    } finally {
      await cleanup()
    }
  })

  test('bulk-move two seeded commodities → both relocate to the target area', async ({
    page,
    request,
    recorder,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { locationId, areaId: sourceAreaId } = await ensureLocationAndArea(
      request,
      auth,
      group.slug,
    )

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    // Distinct target area so the post-move assertion can scope the
    // /commodities list to it via `?area=<id>` and see only the rows
    // we just moved (the source area likely contains pre-existing
    // seed data from other specs).
    const targetArea = await createAreaViaAPI(
      request,
      auth,
      group.slug,
      locationId,
      `Bulk move target ${suffix}`,
    )

    const seeded: { id: string; name: string }[] = []
    seeded.push(
      await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: `Bulk Move A ${suffix}`, areaId: sourceAreaId, type: 'electronics' },
        group.mainCurrency,
      ),
    )
    seeded.push(
      await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: `Bulk Move B ${suffix}`, areaId: sourceAreaId, type: 'electronics' },
        group.mainCurrency,
      ),
    )
    // Cleanup runs even on assertion failure: drop the commodities
    // first so the area DELETE doesn't 422 on "still owns rows".
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
      await deleteAreaViaAPI(request, auth, group.slug, targetArea.id).catch(() => {})
    }

    try {
      await navigateTo(page, recorder, TO_COMMODITIES)

      const cardA = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      const cardB = page.locator(`[data-commodity-id="${seeded[1].id}"]`)
      await expect(cardA).toBeVisible({ timeout: 15000 })
      await expect(cardB).toBeVisible()

      // Same checkbox-toggle workaround as the bulk-delete test.
      for (const card of [cardA, cardB]) {
        const cb = card.locator('[data-testid="commodity-select"]')
        await cb.scrollIntoViewIfNeeded()
        await cb.dispatchEvent('click')
      }

      const bar = page.locator('[data-testid="commodities-bulk-bar"]')
      await expect(bar).toBeVisible()
      await expect(bar).toContainText(/2 items? selected/)

      // Open the bulk-move dialog → pick target area → confirm.
      await bar.locator('[data-testid="commodities-bulk-move"]').click()
      const moveSelect = page.locator('[data-testid="bulk-move-area"]')
      await expect(moveSelect).toBeVisible()
      await moveSelect.selectOption(targetArea.id)

      const movePromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/commodities/bulk-move') && resp.request().method() === 'POST',
        { timeout: 15000 },
      )
      await page.locator('[data-testid="bulk-move-confirm"]').click()
      const moveResp = await movePromise
      expect(moveResp.status()).toBeLessThan(300)

      // Dialog auto-closes on success (handleBulkMove → setMoveOpen(false)).
      await expect(moveSelect).toBeHidden()

      // Filter to the target area and assert both seeded rows show
      // up there. The area filter URL param is `area=<id>`; the FE
      // remaps it to `area_id=` for the BE list call.
      const filterPromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/commodities') &&
          resp.url().includes(`area_id=${encodeURIComponent(targetArea.id)}`) &&
          resp.request().method() === 'GET',
        { timeout: 15000 },
      )
      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/commodities?area=${encodeURIComponent(targetArea.id)}`,
      )
      await filterPromise

      await expect(cardA).toBeVisible()
      await expect(cardB).toBeVisible()

      // Axe-clean on the filtered list (post-move).
      await auditList(page)
    } finally {
      await cleanup()
    }
  })

  test('sort by registered_date desc puts the most-recently-seeded row first', async ({
    page,
    request,
    recorder,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    // The BE schema for `registered_date` is `TEXT` with no
    // server-side default, so sequential POSTs that omit it all end
    // up with the same empty value and the `sort=-registered_date`
    // ordering is undefined. Pass distinct day-spaced values so the
    // expected order is deterministic.
    const seeds = [
      { tag: 'oldest', registeredDate: '2024-01-01' },
      { tag: 'middle', registeredDate: '2024-06-15' },
      { tag: 'newest', registeredDate: '2025-12-31' },
    ] as const
    const seeded: { id: string; name: string }[] = []
    for (const { tag, registeredDate } of seeds) {
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Sort ${tag} ${suffix}`, areaId, type: 'other', registeredDate },
          group.mainCurrency,
        ),
      )
    }
    const newest = seeded[2]

    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      await navigateTo(page, recorder, TO_COMMODITIES)

      // Scope the visible set to our seeded rows via the search box —
      // the e2e DB has unrelated rows from other specs that would
      // otherwise interleave with our three. The shared suffix is
      // unique per run, so `q=<suffix>` returns exactly our trio.
      const searchInput = page.locator('[data-testid="commodities-search"]')
      const searchPromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/commodities') &&
          resp.url().includes(`q=${encodeURIComponent(suffix)}`) &&
          resp.request().method() === 'GET',
        { timeout: 15000 },
      )
      await searchInput.fill(suffix)
      await searchPromise

      // Apply the registered_date sort. Date fields default to DESC
      // direction (CommoditiesListPage.setSort), so a single click is
      // all we need — URL becomes `sort=-registered_date`.
      await page.locator('[data-testid="commodities-sort"]').click()
      const sortPromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/commodities') &&
          resp.url().includes('sort=-registered_date') &&
          resp.request().method() === 'GET',
        { timeout: 15000 },
      )
      // English label for `registered_date` is "Date added" (see
      // commodities:sort.registered_date in the locale file). Match
      // by visible text rather than the BE field name.
      await page.getByRole('menuitemcheckbox', { name: /date added/i }).click()
      await sortPromise
      await page.keyboard.press('Escape')

      await expect(page).toHaveURL(/[?&]sort=-registered_date(?:&|$)/)

      // The first card in DOM order must be the most-recently-seeded.
      const firstCard = page.locator('[data-testid="commodity-card"]').first()
      await expect(firstCard).toHaveAttribute('data-commodity-id', newest.id, {
        timeout: 15000,
      })

      // Axe-clean on the sorted, search-scoped list.
      await auditList(page)
    } finally {
      await cleanup()
    }
  })

  test('search by partial name narrows the list to the matching row', async ({
    page,
    request,
    recorder,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    // Two rows that share the suffix but differ in a unique fragment
    // we'll search for. The fragment ("alpha") is what proves search
    // is doing the narrowing — not just the bulk filter on suffix.
    const matchName = `Search alpha-${suffix}`
    const otherName = `Search beta-${suffix}`

    const seeded: { id: string; name: string }[] = []
    seeded.push(
      await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: matchName, areaId, type: 'other' },
        group.mainCurrency,
      ),
    )
    seeded.push(
      await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: otherName, areaId, type: 'other' },
        group.mainCurrency,
      ),
    )
    const matchCard = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
    const otherCard = page.locator(`[data-commodity-id="${seeded[1].id}"]`)

    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      await navigateTo(page, recorder, TO_COMMODITIES)
      await expect(matchCard).toBeVisible({ timeout: 15000 })
      await expect(otherCard).toBeVisible()

      // Type the unique fragment. CommoditiesListPage debounces the
      // URL update by 300ms, so we wait on the GET that carries the
      // matching `q=` instead of racing on the DOM.
      const fragment = `alpha-${suffix}`
      const searchPromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/commodities') &&
          resp.url().includes(`q=${encodeURIComponent(fragment)}`) &&
          resp.request().method() === 'GET',
        { timeout: 15000 },
      )
      await page.locator('[data-testid="commodities-search"]').fill(fragment)
      await searchPromise

      await expect(page).toHaveURL(new RegExp(`[?&]q=${encodeURIComponent(fragment)}(?:&|$)`))
      await expect(matchCard).toBeVisible()
      await expect(otherCard).toHaveCount(0)

      // Axe-clean on the search-narrowed list.
      await auditList(page)
    } finally {
      await cleanup()
    }
  })
})
