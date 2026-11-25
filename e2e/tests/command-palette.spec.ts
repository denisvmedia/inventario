/**
 * E2E tests for the global command palette (#1330 PR 5.4, #2801).
 *
 * #2801 added a search entry to the TopBar, so the palette opens by click
 * or tap as well as by Cmd/Ctrl+K, and lists the matches by kind with a
 * "show items" action for areas and tags.
 *
 * The palette queries `/api/v1/search`, which is mounted by the backend
 * only inside the group-scoped router (`/api/v1/g/{slug}/search`). The
 * frontend axios interceptor rewrites the flat URL when the current
 * route carries a `:groupSlug` param. We therefore navigate into a
 * group-scoped page (`/locations`, which `gotoScoped` rewrites to
 * `/g/{slug}/locations`) before opening the palette — opening it from
 * `/` would hit a non-existent endpoint and 404.
 *
 * Keystroke note: `ControlOrMeta+KeyK` is the cross-platform shortcut
 * spelling. The `useKeyboardShortcuts` composable's `mod` modifier
 * matches `metaKey` on macOS / `ctrlKey` everywhere else; webkit on
 * macOS-latest runners refused a `Control+KeyK` press because Reka
 * sees `ctrlKey=true` but the composable wants `metaKey=true`.
 */
import { test } from '../fixtures/app-fixture.js'
import { expect, type APIRequestContext } from '@playwright/test'
import { navigateWithAuth } from './includes/auth.js'
import {
  createAreaViaAPI,
  createCommodityViaAPI,
  deleteAreaViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
} from './includes/commodities-api.js'

test.describe('Command palette — Cmd+K / Ctrl+K', () => {
  test('opens with the keyboard shortcut and closes with Escape', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder)
    await expect(page.locator('h1')).toBeVisible()

    // Open the palette.
    await page.keyboard.press('ControlOrMeta+KeyK')
    const palette = page.locator('[data-testid="command-palette"]')
    await expect(palette).toBeVisible()
    await recorder.takeScreenshot('command-palette-01-open')

    // The search input is autofocused.
    // cmdk's Input hardcodes role="combobox" (after spreading the wrapper's
    // role) — that's the canonical ARIA combobox pattern for a search-as-you-
    // type listbox. Driver matches the rendered role rather than fighting the
    // library; the input still acts as the palette's text-entry affordance.
    const search = palette.locator('input[role="combobox"]')
    await expect(search).toBeFocused()

    // Initial hint is shown until the user types ≥ 2 characters.
    await expect(palette).toContainText('Type at least 2 characters to search.')

    // Esc closes the palette.
    await page.keyboard.press('Escape')
    await expect(palette).not.toBeVisible()
  })

  test('shows the empty-results message for queries that match nothing', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder)
    await expect(page.locator('h1')).toBeVisible()

    await page.keyboard.press('ControlOrMeta+KeyK')
    const palette = page.locator('[data-testid="command-palette"]')
    await expect(palette).toBeVisible()

    // cmdk's Input hardcodes role="combobox" (after spreading the wrapper's
    // role) — that's the canonical ARIA combobox pattern for a search-as-you-
    // type listbox. Driver matches the rendered role rather than fighting the
    // library; the input still acts as the palette's text-entry affordance.
    const search = palette.locator('input[role="combobox"]')
    await search.fill('zzzzznosuchitem9999')

    // The debounced search settles after 300 ms; allow a generous window.
    await expect(palette).toContainText(/No results for/i, { timeout: 5000 })
  })
})

// Best-effort cleanup of the item tags a test created through its items.
async function deleteTagsMatching(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  text: string,
): Promise<void> {
  const headers = {
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${auth.accessToken}`,
    'X-CSRF-Token': auth.csrfToken,
  }
  const base = `/api/v1/g/${encodeURIComponent(slug)}/tags`
  const resp = await request.get(`${base}?kind=commodity&per_page=100&q=${encodeURIComponent(text)}`, {
    headers,
  })
  if (!resp.ok()) return
  const body = (await resp.json()) as { data?: Array<{ id: string }> }
  for (const tag of body.data ?? []) {
    await request.delete(`${base}/${encodeURIComponent(tag.id)}?force=true`, { headers }).catch(() => {})
  }
}

test.describe('Command palette — header search entry (#2801)', () => {
  test('opens from the header search entry by click', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder)
    await expect(page.locator('h1')).toBeVisible()

    await page.getByTestId('header-search').click()
    const palette = page.getByTestId('command-palette')
    await expect(palette).toBeVisible()
    await expect(palette.locator('input[role="combobox"]')).toBeFocused()

    // Escape returns focus to the entry that opened the palette.
    await page.keyboard.press('Escape')
    await expect(palette).not.toBeVisible()
    await expect(page.getByTestId('header-search')).toBeFocused()
  })

  test('opens from the compact search button on a narrow screen', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder)
    await expect(page.locator('h1')).toBeVisible()
    await page.setViewportSize({ width: 390, height: 800 })

    await expect(page.getByTestId('header-search')).toBeHidden()
    await page.getByTestId('header-search-compact').click()
    const palette = page.getByTestId('command-palette')
    await expect(palette).toBeVisible()
    await expect(palette).toContainText('Type at least 2 characters to search.')
  })

  test.describe('grouped results', () => {
    let auth: ApiAuth
    let slug: string
    let run: string
    let areaId = ''
    let commodityId = ''

    test.beforeEach(async ({ page, request }) => {
      auth = await extractApiAuth(page)
      const group = await resolveActiveGroup(request, auth)
      slug = group.slug
      run = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`
      const { locationId } = await ensureLocationAndArea(request, auth, slug)
      areaId = (await createAreaViaAPI(request, auth, slug, locationId, `Loft ${run}`)).id
      commodityId = (
        await createCommodityViaAPI(
          request,
          auth,
          slug,
          { name: `Reading lamp ${run}`, shortName: 'Lamp', areaId, tags: [`Attic ${run}`] },
          group.groupCurrency,
        )
      ).id
    })

    test.afterEach(async ({ request }) => {
      if (commodityId) await deleteCommodityViaAPI(request, auth, slug, commodityId).catch(() => {})
      if (areaId) await deleteAreaViaAPI(request, auth, slug, areaId).catch(() => {})
      await deleteTagsMatching(request, auth, slug, run)
      commodityId = ''
      areaId = ''
    })

    test('lists items, areas and tags, and shows the items of an area', async ({ page, recorder }) => {
      await page.goto(`/g/${encodeURIComponent(slug)}/locations`)
      await page.getByTestId('header-search').click()
      const palette = page.getByTestId('command-palette')
      await palette.locator('input[role="combobox"]').fill(run)

      await expect(palette.getByTestId(`palette-result-commodity-${commodityId}`)).toContainText(
        `Reading lamp ${run}`,
      )
      await expect(palette.getByTestId(`palette-result-area-${areaId}`)).toContainText(`Loft ${run}`)
      await expect(palette.getByTestId('palette-group-tags')).toContainText(`Attic ${run}`)
      await recorder.takeScreenshot('command-palette-02-grouped-results')

      await palette.getByTestId(`palette-area-items-${areaId}`).click()
      await expect(palette).not.toBeVisible()
      await expect(page).toHaveURL(new RegExp(`[?&]area=${areaId}`))
      await expect(page.getByTestId('commodity-card')).toHaveCount(1)
      await expect(page.getByTestId('commodity-card').first()).toContainText(`Reading lamp ${run}`)
    })

    test('hands the query to the search page and remembers opened results', async ({ page }) => {
      await page.goto(`/g/${encodeURIComponent(slug)}/locations`)
      await page.getByTestId('header-search').click()
      let palette = page.getByTestId('command-palette')
      await palette.locator('input[role="combobox"]').fill(run)
      await expect(palette.getByTestId(`palette-result-commodity-${commodityId}`)).toBeVisible()

      // The first row hands the query to the search page.
      await page.keyboard.press('Enter')
      await expect(page).toHaveURL(new RegExp(`/search\\?q=${run}`))
      await expect(page.getByTestId(`result-area-${areaId}`)).toBeVisible()
      await expect(page.getByTestId('group-tags')).toContainText(`Attic ${run}`)

      // Open the item from the palette; the empty palette then lists it.
      await page.getByTestId('header-search').click()
      palette = page.getByTestId('command-palette')
      await palette.locator('input[role="combobox"]').fill(run)
      await palette.getByTestId(`palette-result-commodity-${commodityId}`).click()
      await expect(page).toHaveURL(new RegExp(`/commodities/${commodityId}`))

      await page.getByTestId('header-search').click()
      await expect(
        page.getByTestId('command-palette').getByTestId(`palette-recent-commodity-${commodityId}`),
      ).toContainText(`Reading lamp ${run}`)
    })
  })
})
