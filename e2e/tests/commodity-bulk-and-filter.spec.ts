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
 * Test isolation
 * - Each test seeds with a unique per-run suffix (`Date.now()` +
 *   random base36) and lands on `/commodities?q=<suffix>` so the
 *   visible set is exactly the seeded rows regardless of whatever
 *   pre-existing data the shared e2e DB carries (default page size
 *   is 24, default sort is by name — an unbounded list would push
 *   our rows off page 1 otherwise).
 * - Cleanup is registered BEFORE any seeding so a partial-failure
 *   mid-seed still drops the rows that were already created.
 *
 * Axe coverage
 * - `auditList(page)` includes the page wrapper plus `[role="dialog"]`
 *   and `[role="menu"]` so portal-rendered overlays (radix Dialog +
 *   DropdownMenu) are audited when they're open. AppSidebar is
 *   excluded by scoping — it carries known aria-hidden-focus +
 *   color-contrast issues that every authenticated page inherits and
 *   that aren't on the surface this spec exercises.
 * - Each test that opens an overlay (sort/filter dropdowns, bulk
 *   confirm dialog, bulk-move dialog) audits while the overlay is
 *   open AND once more on the settled list state.
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
  type ApiAuth,
  type ResolvedGroup,
} from './includes/commodities-api.js'
import { axeAudit } from '../utils/axe.js'

// Scope axe audits to the commodities-list page wrapper plus any
// portal-rendered overlays (radix Dialog + DropdownMenu use
// role="dialog" / role="menu"). The shared AppSidebar carries known
// a11y issues every authenticated page inherits and isn't on the
// surface this spec exercises.
//
// The `[role="menu"]` include is gated on `data-state="open"` because
// Radix DropdownMenu keeps the content panel mounted with
// `data-state="closed"` for one animation frame after Escape; on
// webkit that frame can outlive the next axe pass and trip a
// color-contrast violation on the now-faded-out menu items (#1450
// CI run 25360622173 — pre-existing webkit-only flake on master too,
// independent of this PR's changes).
function auditList(page: Page): Promise<void> {
  return axeAudit(page, {
    include: [
      '[data-testid="page-commodities"]',
      '[role="dialog"]',
      '[role="menu"][data-state="open"]',
    ],
  })
}

// Land on /commodities pre-scoped to a search query so the visible
// set is exactly the seeded rows. Bypasses `navigateTo` because we
// already know the slug from `resolveActiveGroup` and the fixture
// has already authenticated; `page.goto` is enough.
async function gotoCommoditiesScoped(
  page: Page,
  group: ResolvedGroup,
  q: string,
): Promise<void> {
  await page.goto(
    `/g/${encodeURIComponent(group.slug)}/commodities?q=${encodeURIComponent(q)}`,
  )
  await expect(page.getByTestId('page-commodities')).toBeVisible()
}

interface TestContext {
  auth: ApiAuth
  group: ResolvedGroup
}

async function resolveContext(
  page: Page,
  request: import('@playwright/test').APIRequestContext,
): Promise<TestContext> {
  const auth = await extractApiAuth(page)
  const group = await resolveActiveGroup(request, auth)
  return { auth, group }
}

test.describe('Commodities — bulk + filter round-trips', () => {
  test('bulk-delete two seeded commodities → both gone from the list', async ({
    page,
    request,
  }) => {
    const { auth, group } = await resolveContext(page, request)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const seeded: { id: string; name: string }[] = []
    // Cleanup runs even if seeding throws partway through — the
    // helper is 404-tolerant so deleting an id that was never
    // created (or already gone) is a no-op.
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Bulk Delete A ${suffix}`, areaId, type: 'electronics' },
          group.groupCurrency,
        ),
      )
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Bulk Delete B ${suffix}`, areaId, type: 'electronics' },
          group.groupCurrency,
        ),
      )

      await gotoCommoditiesScoped(page, group, suffix)

      const cardA = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      const cardB = page.locator(`[data-commodity-id="${seeded[1].id}"]`)
      await expect(cardA).toBeVisible({ timeout: 15000 })
      await expect(cardB).toBeVisible()

      // Toggle the per-card checkbox on each row with a REAL click.
      // The checkbox carries `relative z-10` so it sits above the
      // card-wide overlay link (#1965); a real `.click()` therefore
      // lands on the checkbox (not the overlay) and selects the row.
      // (Before the #1965 fix this needed `dispatchEvent('click')` to
      // bypass the actionability gate — which also masked the bug that
      // a real user's click hit the overlay and opened the sheet.)
      for (const card of [cardA, cardB]) {
        const cb = card.locator('[data-testid="commodity-select"]')
        await cb.scrollIntoViewIfNeeded()
        await cb.click()
      }

      const bar = page.locator('[data-testid="commodities-bulk-bar"]')
      await expect(bar).toBeVisible()
      await expect(bar).toContainText(/2 items? selected/)

      // Open the confirm-dialog and audit while it's on screen,
      // before clicking accept (post-confirm the dialog unmounts).
      const deletePromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/commodities/bulk-delete') && resp.request().method() === 'POST',
        { timeout: 15000 },
      )
      await bar.locator('[data-testid="commodities-bulk-delete"]').click()
      await expect(page.getByTestId('confirm-dialog')).toBeVisible()
      await auditList(page)
      await page.getByTestId('confirm-accept').click()
      const deleteResponse = await deletePromise
      expect(deleteResponse.status()).toBeLessThan(300)

      await expect(cardA).toHaveCount(0, { timeout: 15000 })
      await expect(cardB).toHaveCount(0)
      await expect(bar).toBeHidden()

      await auditList(page)
    } finally {
      await cleanup()
    }
  })

  test('type filter narrows the list + the URL reflects the active filter', async ({
    page,
    request,
  }) => {
    const { auth, group } = await resolveContext(page, request)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const seeded: { id: string; name: string }[] = []
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Filter electronics ${suffix}`, areaId, type: 'electronics' },
          group.groupCurrency,
        ),
      )
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Filter furniture ${suffix}`, areaId, type: 'furniture' },
          group.groupCurrency,
        ),
      )

      await gotoCommoditiesScoped(page, group, suffix)

      const electronicsCard = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      const furnitureCard = page.locator(`[data-commodity-id="${seeded[1].id}"]`)
      await expect(electronicsCard).toBeVisible({ timeout: 15000 })
      await expect(furnitureCard).toBeVisible()

      // Open the filter menu, audit while it's on screen, then pick
      // Electronics. The trigger is a `<Button>` (not a `<select>`);
      // items render as `DropdownMenuCheckboxItem`s with
      // `role="menuitemcheckbox"`. After the change the URL should
      // carry `type=electronics`, only the electronics card stays
      // visible, and the BE re-fetched the narrower list.
      await page.getByTestId('commodities-filter-type').click()
      await auditList(page)
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

      await auditList(page)
    } finally {
      await cleanup()
    }
  })

  test('bulk-move two seeded commodities → both relocate to the target area', async ({
    page,
    request,
  }) => {
    const { auth, group } = await resolveContext(page, request)
    const { locationId, areaId: sourceAreaId } = await ensureLocationAndArea(
      request,
      auth,
      group.slug,
    )

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const seeded: { id: string; name: string }[] = []
    let targetAreaId: string | undefined
    // Cleanup must drop commodities first — the BE rejects DELETE on
    // areas that still own rows (422). Registered before any of the
    // POSTs so partial-failure setups still get cleaned up.
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
      if (targetAreaId) {
        await deleteAreaViaAPI(request, auth, group.slug, targetAreaId).catch(() => {})
      }
    }

    try {
      const targetArea = await createAreaViaAPI(
        request,
        auth,
        group.slug,
        locationId,
        `Bulk move target ${suffix}`,
      )
      targetAreaId = targetArea.id

      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Bulk Move A ${suffix}`, areaId: sourceAreaId, type: 'electronics' },
          group.groupCurrency,
        ),
      )
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Bulk Move B ${suffix}`, areaId: sourceAreaId, type: 'electronics' },
          group.groupCurrency,
        ),
      )

      await gotoCommoditiesScoped(page, group, suffix)

      const cardA = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      const cardB = page.locator(`[data-commodity-id="${seeded[1].id}"]`)
      await expect(cardA).toBeVisible({ timeout: 15000 })
      await expect(cardB).toBeVisible()

      // Real `.click()` — the checkbox sits above the overlay link
      // (`relative z-10`, #1965), so the click selects rather than
      // navigating to the detail sheet.
      for (const card of [cardA, cardB]) {
        const cb = card.locator('[data-testid="commodity-select"]')
        await cb.scrollIntoViewIfNeeded()
        await cb.click()
      }

      const bar = page.locator('[data-testid="commodities-bulk-bar"]')
      await expect(bar).toBeVisible()
      await expect(bar).toContainText(/2 items? selected/)

      // Open the bulk-move dialog and audit while it's on screen.
      // The areas dropdown reads from useAreas() which paginates at
      // 50; assert the freshly created target id is present before
      // calling selectOption (selectOption fails opaquely if the
      // option isn't in the DOM, which would mask a pagination
      // regression on groups with > 50 areas).
      await bar.locator('[data-testid="commodities-bulk-move"]').click()
      const moveSelect = page.locator('[data-testid="bulk-move-area"]')
      await expect(moveSelect).toBeVisible()
      await expect(
        moveSelect.locator(`option[value="${targetArea.id}"]`),
        'target area must be in the move dialog options — if this fails on a group with > 50 areas, useAreas() pagination needs to grow',
      ).toBeAttached()
      await auditList(page)
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
      // up there — and ONLY them. The area is freshly created so the
      // count check proves the rows actually moved, not just that
      // they're still visible from the source area.
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
      await expect(page.locator('[data-testid="commodity-card"]')).toHaveCount(2)

      await auditList(page)
    } finally {
      await cleanup()
    }
  })

  test('sort by registered_date desc puts the most-recently-seeded row first', async ({
    page,
    request,
  }) => {
    const { auth, group } = await resolveContext(page, request)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const seeded: { id: string; name: string }[] = []
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
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
      for (const { tag, registeredDate } of seeds) {
        seeded.push(
          await createCommodityViaAPI(
            request,
            auth,
            group.slug,
            { name: `Sort ${tag} ${suffix}`, areaId, type: 'other', registeredDate },
            group.groupCurrency,
          ),
        )
      }
      const newest = seeded[2]

      await gotoCommoditiesScoped(page, group, suffix)

      // Apply the registered_date sort. Date fields default to DESC
      // direction (CommoditiesListPage.setSort), so a single click is
      // all we need — URL becomes `sort=-registered_date`.
      await page.locator('[data-testid="commodities-sort"]').click()
      await auditList(page)
      const sortPromise = page.waitForResponse(
        (resp) =>
          resp.url().includes('/commodities') &&
          resp.url().includes('sort=-registered_date') &&
          resp.request().method() === 'GET',
        { timeout: 15000 },
      )
      // English label for `registered_date` is "Date added" (see
      // commodities:sort.registered_date). Match by visible text
      // rather than the BE field name.
      await page.getByRole('menuitemcheckbox', { name: /date added/i }).click()
      await sortPromise
      await page.keyboard.press('Escape')

      await expect(page).toHaveURL(/[?&]sort=-registered_date(?:&|$)/)

      // The first card in DOM order must be the most-recently-seeded.
      const firstCard = page.locator('[data-testid="commodity-card"]').first()
      await expect(firstCard).toHaveAttribute('data-commodity-id', newest.id, {
        timeout: 15000,
      })

      await auditList(page)
    } finally {
      await cleanup()
    }
  })

  test('search by partial name narrows the list to the matching row', async ({
    page,
    request,
  }) => {
    const { auth, group } = await resolveContext(page, request)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const fragment = `alpha-${suffix}`
    const seeded: { id: string; name: string }[] = []
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      // Two rows that share the suffix but differ in a unique
      // fragment we'll search for. Only the "alpha" row is expected
      // to remain visible after the search.
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Search ${fragment}`, areaId, type: 'other' },
          group.groupCurrency,
        ),
      )
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Search beta-${suffix}`, areaId, type: 'other' },
          group.groupCurrency,
        ),
      )

      // Land scoped to the suffix so both seeded rows are visible
      // before we narrow further with the fragment.
      await gotoCommoditiesScoped(page, group, suffix)

      const matchCard = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      const otherCard = page.locator(`[data-commodity-id="${seeded[1].id}"]`)
      await expect(matchCard).toBeVisible({ timeout: 15000 })
      await expect(otherCard).toBeVisible()

      // Replace the search-input value with the fragment.
      // CommoditiesListPage debounces the URL update by 300ms so we
      // wait on the GET that carries the matching `q=` instead of
      // racing on the DOM.
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
      await expect(page.locator('[data-testid="commodity-card"]')).toHaveCount(1)

      await auditList(page)
    } finally {
      await cleanup()
    }
  })

  // #1657 — the whole card is the click target via an absolute-Link
  // overlay; inert columns (thumb, area, price, purchase-date chip)
  // get `pointer-events-none` so clicks fall through. Click the thumb
  // (clearly outside the title) and assert the URL flips to the
  // commodity detail page.
  test('clicking outside the title navigates to the commodity detail (#1657)', async ({
    page,
    request,
  }) => {
    const { auth, group } = await resolveContext(page, request)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const seeded: { id: string; name: string }[] = []
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Whole-card click ${suffix}`, areaId, type: 'other' },
          group.groupCurrency,
        ),
      )

      await gotoCommoditiesScoped(page, group, suffix)

      const card = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      await expect(card).toBeVisible({ timeout: 15000 })

      // The thumb sits inside a `pointer-events-none` wrapper, with
      // the absolute overlay <Link> as a sibling under the card root.
      // We CAN'T use `locator.click()` here — Playwright's actionability
      // gate runs a hit-test before clicking, sees the overlay is the
      // topmost element at the thumb's center, and refuses. That's the
      // intended runtime behavior (a real user click also lands on the
      // overlay due to pointer-events), so we drive the click via
      // `page.mouse.click(x, y)` at coordinates inside the thumb. That
      // skips Playwright's gate and exercises the browser's actual
      // hit-testing — which respects `pointer-events: none` and routes
      // the click through to the overlay. If the rule regresses on
      // any inert column, the overlay won't receive the click and the
      // URL won't change.
      const thumb = card.locator('[data-testid="commodity-card-thumb"]')
      const thumbBox = await thumb.boundingBox()
      expect(thumbBox, 'thumb bounding box should be measurable').not.toBeNull()
      await page.mouse.click(
        thumbBox!.x + thumbBox!.width / 2,
        thumbBox!.y + thumbBox!.height / 2,
      )

      await expect(page).toHaveURL(new RegExp(`/commodities/${seeded[0].id}(?:$|[?#])`), {
        timeout: 15000,
      })
    } finally {
      await cleanup()
    }
  })

  // #1965 regression — the per-card selection checkbox sits beneath a
  // card-wide overlay <Link>. Before the fix a REAL click on the
  // checkbox landed on the overlay and opened the detail sheet instead
  // of selecting the row (only `dispatchEvent`, which skips hit-testing,
  // could toggle it — masking the bug). With `relative z-10` the
  // checkbox is the top hit target, so a genuine `.click()` selects and
  // never navigates. Both tests use a real `.click()` (NOT dispatchEvent)
  // and guard that the sheet stayed closed and the URL never left the list.
  test('grid: a real click on the card checkbox selects the row, not opens the sheet (#1965)', async ({
    page,
    request,
  }) => {
    const { auth, group } = await resolveContext(page, request)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const seeded: { id: string; name: string }[] = []
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Checkbox Select ${suffix}`, areaId, type: 'electronics' },
          group.groupCurrency,
        ),
      )
      await gotoCommoditiesScoped(page, group, suffix)

      const card = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      await expect(card).toBeVisible({ timeout: 15000 })
      const sheet = page.getByTestId('commodity-detail-sheet')
      const listPath = new URL(page.url()).pathname

      // REAL click — exercises the browser's actual hit-testing. If the
      // overlay link still covered the checkbox this would either error
      // ("intercepts pointer events") or open the sheet.
      const cb = card.locator('[data-testid="commodity-select"]')
      await cb.scrollIntoViewIfNeeded()
      await cb.click()

      const bar = page.getByTestId('commodities-bulk-bar')
      await expect(bar).toBeVisible()
      await expect(bar).toContainText(/1 item selected/i)
      await expect(sheet).toBeHidden()
      expect(new URL(page.url()).pathname, 'must stay on the list, not drill into the detail').toBe(
        listPath,
      )
    } finally {
      await cleanup()
    }
  })

  test('list: a real click on the row checkbox selects the row, not opens the sheet (#1965)', async ({
    page,
    request,
  }) => {
    const { auth, group } = await resolveContext(page, request)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const seeded: { id: string; name: string }[] = []
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(() => {})
      }
    }

    try {
      seeded.push(
        await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Row Select ${suffix}`, areaId, type: 'electronics' },
          group.groupCurrency,
        ),
      )
      await gotoCommoditiesScoped(page, group, suffix)
      // Flip to the list/table view so the row-checkbox variant is exercised.
      await page.getByTestId('commodities-view-list').click()
      await expect(page.getByTestId('commodities-table')).toBeVisible()

      const row = page.locator(`[data-commodity-id="${seeded[0].id}"]`)
      await expect(row).toBeVisible({ timeout: 15000 })
      const sheet = page.getByTestId('commodity-detail-sheet')
      const listPath = new URL(page.url()).pathname

      const cb = row.locator('[data-testid="commodity-row-select"]')
      await cb.scrollIntoViewIfNeeded()
      await cb.click()

      const bar = page.getByTestId('commodities-bulk-bar')
      await expect(bar).toBeVisible()
      await expect(bar).toContainText(/1 item selected/i)
      await expect(sheet).toBeHidden()
      expect(new URL(page.url()).pathname, 'must stay on the list, not drill into the detail').toBe(
        listPath,
      )
    } finally {
      await cleanup()
    }
  })
})
