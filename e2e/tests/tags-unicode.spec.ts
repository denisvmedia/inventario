import { expect, type APIRequestContext } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { createCommodity, type TestCommodity } from './includes/commodities.js'
import {
  deleteCommodityViaAPI,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
} from './includes/commodities-api.js'
import { navigateTo, TO_COMMODITIES } from './includes/navigate.js'

/**
 * Tags typed in any script survive a save (#2791).
 *
 * The item is created through the Add-item wizard with tags typed into
 * the chip input, the way a user enters them. After a reload the detail
 * page must show every tag under the text the user typed, each tag must
 * be its own row on the Tags page, and the item must reference one slug
 * per tag. "USB кабель" and "USB хаб" share their Latin prefix, which is
 * the case that used to collapse into a single tag.
 */

function headers(auth: ApiAuth): Record<string, string> {
  return {
    'Content-Type': 'application/vnd.api+json',
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${auth.accessToken}`,
    'X-CSRF-Token': auth.csrfToken,
  }
}

async function commodityTags(
  request: APIRequestContext,
  auth: ApiAuth,
  groupSlug: string,
  id: string,
): Promise<string[]> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(groupSlug)}/commodities/${encodeURIComponent(id)}`,
    { headers: headers(auth) },
  )
  expect(resp.ok(), `GET /commodities/${id} → ${resp.status()}`).toBe(true)
  const body = (await resp.json()) as { data: { attributes: { tags?: string[] } } }
  return body.data.attributes.tags ?? []
}

// Best-effort cleanup of the tags this spec auto-created, so re-runs
// against the same stack do not pile up rows on the Tags page.
async function deleteTagsBySlug(
  request: APIRequestContext,
  auth: ApiAuth,
  groupSlug: string,
  slugs: string[],
): Promise<void> {
  const base = `/api/v1/g/${encodeURIComponent(groupSlug)}/tags`
  const resp = await request.get(`${base}?kind=commodity&per_page=100`, { headers: headers(auth) })
  if (!resp.ok()) return
  const body = (await resp.json()) as { data?: Array<{ id: string; slug: string }> }
  for (const tag of body.data ?? []) {
    if (!slugs.includes(tag.slug)) continue
    await request.delete(`${base}/${encodeURIComponent(tag.id)}?force=true`, {
      headers: headers(auth),
    })
  }
}

test.describe('Tags in any script', () => {
  test('an item keeps its Cyrillic and mixed-script tags after a reload', async ({
    page,
    request,
    recorder,
  }) => {
    const suffix = `${Date.now()}`
    const typed = [`На продажу ${suffix}`, `USB кабель ${suffix}`, `USB хаб ${suffix}`]
    const slugs = [`на-продажу-${suffix}`, `usb-кабель-${suffix}`, `usb-хаб-${suffix}`]
    const item: TestCommodity = {
      name: `Unicode tags ${suffix}`,
      shortName: 'Unicode',
      type: 'Electronics',
      count: 1,
      originalPrice: 100,
      originalPriceCurrency: 'CZK',
      purchaseDate: new Date().toISOString().split('T')[0],
      tags: typed,
    }

    await navigateTo(page, recorder, TO_COMMODITIES)
    const detailUrl = await createCommodity(page, recorder, item)
    const id = /\/commodities\/([0-9a-fA-F-]{36})/.exec(new URL(detailUrl).pathname)?.[1]
    expect(id, `commodity id in ${detailUrl}`).toBeTruthy()

    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    try {
      await page.reload()
      const details = page.getByTestId('commodity-detail-details')
      await expect(details).toBeVisible({ timeout: 10000 })
      for (const label of typed) {
        await expect(details).toContainText(label)
      }

      expect(await commodityTags(request, auth, group.slug, id!)).toEqual(slugs)

      // Filter by the run suffix so the rows are on the first page however
      // many tags the group already has.
      await page.goto(`/g/${encodeURIComponent(group.slug)}/tags?q=${suffix}`)
      await expect(page.getByTestId('page-tags')).toBeVisible()
      for (const slug of slugs) {
        await expect(page.getByTestId(`tag-row-${slug}`)).toBeVisible()
      }
    } finally {
      await deleteCommodityViaAPI(request, auth, group.slug, id!)
      await deleteTagsBySlug(request, auth, group.slug, slugs)
    }
  })
})
