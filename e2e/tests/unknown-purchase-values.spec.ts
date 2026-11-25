import { expect, type APIRequestContext, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { navigateWithAuth } from './includes/auth.js'
import { createCommodity } from './includes/commodities.js'
import {
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
  type ResolvedGroup,
} from './includes/commodities-api.js'
import { navigateTo, TO_COMMODITIES, TO_HOME } from './includes/navigate.js'

/**
 * A reviewed item can keep its purchase date and prices unknown, and
 * unknown is not zero: a gift has no price, a free item has a price of 0.
 * Covers creating such an item through the dialog, making its price a
 * known zero later, and the dashboard total saying how many items it
 * leaves out.
 */

function jsonHeaders(auth: ApiAuth): Record<string, string> {
  return {
    'Content-Type': 'application/vnd.api+json',
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${auth.accessToken}`,
    'X-CSRF-Token': auth.csrfToken,
  }
}

function uniqueSuffix(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

async function apiContext(page: Page, request: APIRequestContext) {
  const auth = await extractApiAuth(page)
  const group = await resolveActiveGroup(request, auth)
  return { auth, group }
}

async function itemAttrs(
  request: APIRequestContext,
  auth: ApiAuth,
  group: ResolvedGroup,
  id: string,
): Promise<Record<string, unknown>> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(id)}`,
    { headers: jsonHeaders(auth) },
  )
  expect(resp.ok(), `GET /commodities/${id} → ${resp.status()}`).toBe(true)
  const body = (await resp.json()) as { data: { attributes: Record<string, unknown> } }
  return body.data.attributes
}

// Seeds a reviewed item with only the prices given; every other price and
// the purchase date are left out, so they are stored as unknown.
async function seedItem(
  request: APIRequestContext,
  auth: ApiAuth,
  group: ResolvedGroup,
  areaId: string,
  name: string,
  prices: Record<string, string>,
): Promise<string> {
  const resp = await request.post(`/api/v1/g/${encodeURIComponent(group.slug)}/commodities`, {
    headers: jsonHeaders(auth),
    data: {
      data: {
        type: 'commodities',
        attributes: {
          name,
          short_name: name.slice(0, 20),
          type: 'other',
          status: 'in_use',
          area_id: areaId,
          count: 1,
          original_price_currency: group.groupCurrency,
          draft: false,
          ...prices,
        },
      },
    },
  })
  expect(resp.ok(), `POST /commodities → ${resp.status()} ${await resp.text()}`).toBe(true)
  const body = (await resp.json()) as { data: { id: string } }
  return body.data.id
}

test.describe('Unknown purchase date and prices', () => {
  test('a reviewed gift keeps its purchase info unknown until a zero is entered', async ({
    page,
    request,
    recorder,
  }) => {
    const name = `Gift clock ${uniqueSuffix()}`
    await navigateTo(page, recorder, TO_COMMODITIES)
    // Basics only: the purchase step is left blank and the item is saved
    // as reviewed, not as a draft.
    const url = await createCommodity(page, recorder, {
      name,
      shortName: 'Clock',
      type: 'Other',
      count: 1,
    })
    const id = url.match(/\/commodities\/([0-9a-fA-F-]{36})/)?.[1]
    expect(id, `commodity id in ${url}`).toBeTruthy()
    const { auth, group } = await apiContext(page, request)

    try {
      const detail = page.getByTestId('page-commodity-detail')
      await expect(detail.getByTestId('commodity-detail-purchase-date')).toContainText('—')
      await expect(detail.getByTestId('commodity-detail-original-price')).toContainText('—')
      await expect(detail.getByTestId('commodity-detail-current-price')).toContainText('—')

      let attrs = await itemAttrs(request, auth, group, id!)
      expect(attrs.draft).toBe(false)
      expect(attrs.original_price ?? null).toBeNull()
      expect(attrs.current_price ?? null).toBeNull()
      expect(attrs.purchase_date ?? null).toBeNull()

      // Learning that it was free makes the price a known zero, shown as
      // a zero rather than as unknown.
      await page.getByTestId('commodity-detail-edit').click()
      const form = page.getByTestId('commodity-edit-form')
      await form.waitFor({ state: 'visible', timeout: 10000 })
      await page.fill('#edit-price', '0')
      await page.getByTestId('commodity-edit-save').click()
      await form.waitFor({ state: 'hidden', timeout: 30000 })

      const originalPrice = detail.getByTestId('commodity-detail-original-price')
      await expect(originalPrice).not.toContainText('—')
      await expect(originalPrice).toContainText('0')
      await expect(detail.getByTestId('commodity-detail-current-price')).toContainText('—')

      attrs = await itemAttrs(request, auth, group, id!)
      expect(attrs.original_price).toBe('0')
      expect(attrs.current_price ?? null).toBeNull()
      expect(attrs.draft).toBe(false)
    } finally {
      await deleteCommodityViaAPI(request, auth, group.slug, id!)
    }
  })

  test('the dashboard total says how many items it leaves out', async ({ page, request, recorder }) => {
    await navigateWithAuth(page, '/commodities', recorder)
    const { auth, group } = await apiContext(page, request)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
    const suffix = uniqueSuffix()
    const ids = [
      // A gift: nothing known, so no value.
      await seedItem(request, auth, group, areaId, `Gift ${suffix}`, {}),
      // Only a current value, from an appraisal.
      await seedItem(request, auth, group, areaId, `Appraised ${suffix}`, { current_price: '1200' }),
      // Free: a known zero is a value.
      await seedItem(request, auth, group, areaId, `Free ${suffix}`, {
        original_price: '0',
        current_price: '0',
      }),
    ]

    try {
      const gift = await itemAttrs(request, auth, group, ids[0])
      expect(gift.current_price ?? null).toBeNull()
      const free = await itemAttrs(request, auth, group, ids[2])
      expect(free.original_price).toBe('0')
      expect(free.current_price).toBe('0')

      await navigateTo(page, recorder, TO_HOME)
      // Other specs share the group, so the count is at least the gift.
      await expect(page.getByTestId('dashboard-total-value')).toContainText(
        /Excludes \d+ items? without a value/,
      )
    } finally {
      for (const id of ids) {
        await deleteCommodityViaAPI(request, auth, group.slug, id)
      }
    }
  })
})
