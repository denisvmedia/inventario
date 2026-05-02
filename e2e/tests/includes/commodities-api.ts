/**
 * API-level helpers for commodity / location / area seeding in e2e
 * specs. Pairs with the UI-driven `commodities.ts` helpers — those
 * exercise the multi-step Add-Item dialog (which is what you want when
 * the dialog itself is the unit under test); these drive the BE
 * directly so bulk / filter / sort / search specs can seed two-dozen
 * commodities cheaply without repeatedly walking the wizard.
 *
 * Auth is captured from the page's `localStorage` / `sessionStorage`
 * after the canonical `app-fixture.ts` log-in, so the helpers run as
 * the authenticated user the spec was given. CSRF is required for any
 * write — the BE rejects mutations without it.
 */

import { expect, type Page, type APIRequestContext } from '@playwright/test'

export interface ApiAuth {
  accessToken: string
  csrfToken: string
}

/**
 * Read the auth pair set by the React app after a successful login.
 * Throws when either piece is missing — that's a sign the spec didn't
 * use the `app-fixture.ts` page (or that the storage shape changed).
 */
export async function extractApiAuth(page: Page): Promise<ApiAuth> {
  const accessToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '')
  const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '')
  if (!accessToken) {
    throw new Error(
      'extractApiAuth: no `inventario_token` in localStorage — was the page authenticated via app-fixture.ts?',
    )
  }
  if (!csrfToken) {
    throw new Error(
      'extractApiAuth: no `inventario_csrf_token` in sessionStorage — login completed but CSRF mint missed',
    )
  }
  return { accessToken, csrfToken }
}

function authHeaders(auth: ApiAuth): Record<string, string> {
  return {
    'Content-Type': 'application/vnd.api+json',
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${auth.accessToken}`,
    'X-CSRF-Token': auth.csrfToken,
  }
}

/**
 * Resolve the active group's slug + main currency. The React http
 * client rewrites `/api/v1/...` to `/api/v1/g/{slug}/...` per request;
 * Playwright's request fixture skips that wrapping, so callers need
 * the slug explicitly to build the right URL.
 */
export interface ResolvedGroup {
  id: string
  slug: string
  mainCurrency: string
}

export async function resolveActiveGroup(
  request: APIRequestContext,
  auth: ApiAuth,
): Promise<ResolvedGroup> {
  const resp = await request.get('/api/v1/groups', { headers: authHeaders(auth) })
  if (!resp.ok()) {
    throw new Error(`resolveActiveGroup: GET /groups → ${resp.status()} ${await resp.text()}`)
  }
  const body = (await resp.json()) as {
    data?: Array<{ id: string; attributes: { slug?: string; main_currency?: string } }>
  }
  const group = body.data?.[0]
  if (!group?.attributes?.slug) {
    throw new Error('resolveActiveGroup: user has no usable group slug')
  }
  return {
    id: group.id,
    slug: group.attributes.slug,
    mainCurrency: group.attributes.main_currency ?? 'USD',
  }
}

/**
 * Find the first existing location/area pair in the active group, or
 * create one if the group is empty. The bulk / filter specs don't
 * care about the location/area names — they just need a target for
 * `area_id` on the commodities they seed.
 */
export async function ensureLocationAndArea(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
): Promise<{ locationId: string; areaId: string }> {
  const apiBase = `/api/v1/g/${encodeURIComponent(slug)}`
  const headers = authHeaders(auth)

  // Locations
  const locationsResp = await request.get(`${apiBase}/locations`, { headers })
  const locationsBody = (await locationsResp.json()) as { data?: Array<{ id: string }> }
  let locationId: string
  if (locationsBody.data && locationsBody.data.length > 0) {
    locationId = locationsBody.data[0].id
  } else {
    const created = await request.post(`${apiBase}/locations`, {
      headers,
      data: {
        data: {
          type: 'locations',
          attributes: { name: 'API helper location', address: 'API helper address' },
        },
      },
    })
    if (!created.ok()) {
      throw new Error(
        `ensureLocationAndArea: POST /locations → ${created.status()} ${await created.text()}`,
      )
    }
    locationId = (await created.json()).data.id
  }

  // Areas (flat list, location_id on each row)
  const areasResp = await request.get(`${apiBase}/areas`, { headers })
  const areasBody = (await areasResp.json()) as { data?: Array<{ id: string }> }
  let areaId: string
  if (areasBody.data && areasBody.data.length > 0) {
    areaId = areasBody.data[0].id
  } else {
    const created = await request.post(`${apiBase}/areas`, {
      headers,
      data: {
        data: {
          type: 'areas',
          attributes: { name: 'API helper area', location_id: locationId },
        },
      },
    })
    if (!created.ok()) {
      throw new Error(
        `ensureLocationAndArea: POST /areas → ${created.status()} ${await created.text()}`,
      )
    }
    areaId = (await created.json()).data.id
  }

  return { locationId, areaId }
}

export interface CreateCommodityParams {
  name: string
  shortName?: string
  type?: string
  status?: 'in_use' | 'sold' | 'lost' | 'disposed' | 'written_off'
  areaId: string
  count?: number
  originalPrice?: number
  /** ISO-4217. Defaults to the group's main currency. */
  currency?: string
  draft?: boolean
}

/**
 * POST /commodities with the same JSON:API envelope the
 * CommodityFormDialog submits. Returns the new commodity's id.
 *
 * Defaults: count=1, status=in_use, type=other, originalPrice=0,
 * currency = group's main currency. The BE enforces
 * converted_original_price=0 when purchase currency matches main, so
 * we pass 0 explicitly.
 */
export async function createCommodityViaAPI(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  params: CreateCommodityParams,
  groupMainCurrency = 'USD',
): Promise<{ id: string; name: string }> {
  const currency = params.currency ?? groupMainCurrency
  const resp = await request.post(`/api/v1/g/${encodeURIComponent(slug)}/commodities`, {
    headers: authHeaders(auth),
    data: {
      data: {
        type: 'commodities',
        attributes: {
          name: params.name,
          short_name: params.shortName ?? params.name.slice(-20),
          type: params.type ?? 'other',
          status: params.status ?? 'in_use',
          area_id: params.areaId,
          count: params.count ?? 1,
          purchase_date: '2026-01-01',
          original_price: params.originalPrice ?? 0,
          original_price_currency: currency,
          current_price: params.originalPrice ?? 0,
          converted_original_price: 0,
          draft: params.draft ?? false,
        },
      },
    },
  })
  if (!resp.ok()) {
    throw new Error(
      `createCommodityViaAPI: POST /commodities → ${resp.status()} ${await resp.text()}`,
    )
  }
  const body = await resp.json()
  expect(body?.data?.id, 'createCommodityViaAPI: response missing data.id').toBeTruthy()
  return { id: body.data.id as string, name: params.name }
}

/**
 * DELETE a commodity by id. Best-effort cleanup — silent on 404 so
 * test teardown doesn't blow up if a previous step already removed
 * the row.
 */
export async function deleteCommodityViaAPI(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<void> {
  const resp = await request.delete(
    `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
    { headers: authHeaders(auth) },
  )
  if (resp.status() === 404) return
  if (!resp.ok() && resp.status() !== 204) {
    throw new Error(
      `deleteCommodityViaAPI: DELETE /commodities/${id} → ${resp.status()} ${await resp.text()}`,
    )
  }
}
