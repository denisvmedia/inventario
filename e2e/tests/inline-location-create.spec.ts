import { expect, type APIRequestContext, type Locator, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { navigateWithAuth } from './includes/auth.js'
import {
  createCommodityViaAPI,
  deleteAreaViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
  type ResolvedGroup,
} from './includes/commodities-api.js'

/**
 * Creating a location or an area from the item form's pickers (#2796):
 * "New location" and "New area" open the Locations page's create dialogs
 * over the form, select what they created, and leave the rest of the form
 * as it was typed. The e2e user owns its group, so it sees both entries.
 */

function jsonHeaders(auth: ApiAuth): Record<string, string> {
  return {
    'Content-Type': 'application/vnd.api+json',
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${auth.accessToken}`,
    'X-CSRF-Token': auth.csrfToken,
  }
}

async function itemAttrs(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<Record<string, unknown>> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
    { headers: jsonHeaders(auth) },
  )
  expect(resp.ok(), `GET /commodities/${id} → ${resp.status()}`).toBe(true)
  const body = (await resp.json()) as { data: { attributes: Record<string, unknown> } }
  return body.data.attributes
}

// The id of the area or location with this name, or null.
async function findIdByName(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  kind: 'locations' | 'areas',
  name: string,
): Promise<string | null> {
  const resp = await request.get(`/api/v1/g/${encodeURIComponent(slug)}/${kind}`, {
    headers: jsonHeaders(auth),
  })
  if (!resp.ok()) return null
  const body = (await resp.json()) as { data?: Array<{ id: string; attributes: { name?: string } }> }
  return body.data?.find((row) => row.attributes.name === name)?.id ?? null
}

interface Seeded {
  auth: ApiAuth
  group: ResolvedGroup
  suffix: string
  itemId: string
}

async function seed(page: Page, request: APIRequestContext): Promise<Seeded> {
  await navigateWithAuth(page, '/commodities')
  await expect(page.getByTestId('page-commodities')).toBeVisible()
  const auth = await extractApiAuth(page)
  const group = await resolveActiveGroup(request, auth)
  const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
  const suffix = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`
  const item = await createCommodityViaAPI(
    request,
    auth,
    group.slug,
    { name: `Inline ${suffix}`, shortName: 'inline', areaId },
    group.groupCurrency,
  )
  return { auth, group, suffix, itemId: item.id }
}

// Removes the item first: an area that still holds it cannot be deleted.
async function cleanup(
  request: APIRequestContext,
  s: Seeded,
  names: { location: string; area: string },
): Promise<void> {
  await deleteCommodityViaAPI(request, s.auth, s.group.slug, s.itemId)
  const areaId = await findIdByName(request, s.auth, s.group.slug, 'areas', names.area)
  if (areaId) await deleteAreaViaAPI(request, s.auth, s.group.slug, areaId)
  const locationId = await findIdByName(request, s.auth, s.group.slug, 'locations', names.location)
  if (locationId) {
    await request.delete(
      `/api/v1/g/${encodeURIComponent(s.group.slug)}/locations/${encodeURIComponent(locationId)}`,
      { headers: jsonHeaders(s.auth) },
    )
  }
}

// Opens a picker and chooses its create entry, then fills and saves the
// dialog it opens.
async function createFromPicker(
  page: Page,
  trigger: Locator,
  entryTestId: string,
  dialogTestId: string,
  nameTestId: string,
  submitTestId: string,
  name: string,
): Promise<void> {
  await trigger.click()
  await page.getByTestId(entryTestId).click()
  const dialog = page.getByTestId(dialogTestId)
  await expect(dialog).toBeVisible()
  await dialog.getByTestId(nameTestId).fill(name)
  await dialog.getByTestId(submitTestId).click()
  await expect(dialog).toBeHidden()
  await expect(trigger).toContainText(name)
}

test.describe('Creating a place from the item form', () => {
  test('the edit form creates a location and an area and keeps the typed edits', async ({
    page,
    request,
  }) => {
    const s = await seed(page, request)
    const names = { location: `Cottage ${s.suffix}`, area: `Attic ${s.suffix}` }
    try {
      await page.goto(`/g/${encodeURIComponent(s.group.slug)}/commodities/${s.itemId}/edit`)
      await expect(page.getByTestId('commodity-edit-form')).toBeVisible()
      await page.locator('#edit-serial').fill(`SN-${s.suffix}`)

      await createFromPicker(
        page,
        page.locator('#edit-location'),
        'edit-location-new',
        'location-form-dialog',
        'location-name-input',
        'location-form-submit',
        names.location,
      )
      // A new location starts without areas.
      await expect(page.locator('#edit-area')).toContainText('Pick an area')
      await createFromPicker(
        page,
        page.locator('#edit-area'),
        'edit-area-new',
        'area-form-dialog',
        'area-name-input',
        'area-form-submit',
        names.area,
      )

      // The form is still open with the edit typed before.
      await expect(page.getByTestId('commodity-edit-form')).toBeVisible()
      await expect(page.locator('#edit-serial')).toHaveValue(`SN-${s.suffix}`)
      await page.getByTestId('commodity-edit-save').click()
      await expect(page.getByTestId('commodity-edit-form')).toBeHidden()

      const areaId = await findIdByName(request, s.auth, s.group.slug, 'areas', names.area)
      expect(areaId).toBeTruthy()
      const attrs = await itemAttrs(request, s.auth, s.group.slug, s.itemId)
      expect(attrs.area_id).toBe(areaId)
      expect(attrs.serial_number).toBe(`SN-${s.suffix}`)
    } finally {
      await cleanup(request, s, names)
    }
  })

  test('the quick Location editor creates a place and saves it', async ({ page, request }) => {
    const s = await seed(page, request)
    const names = { location: `Shed ${s.suffix}`, area: `Shelf ${s.suffix}` }
    try {
      await page.goto(`/g/${encodeURIComponent(s.group.slug)}/commodities/${s.itemId}`)
      await expect(page.getByTestId('page-commodity-detail')).toBeVisible()
      await page.getByTestId('commodity-quick-edit-location').click()
      await expect(page.getByTestId('commodity-quick-location')).toBeVisible()

      await createFromPicker(
        page,
        page.locator('#quick-location'),
        'quick-location-new',
        'location-form-dialog',
        'location-name-input',
        'location-form-submit',
        names.location,
      )
      await createFromPicker(
        page,
        page.locator('#quick-area'),
        'quick-area-new',
        'area-form-dialog',
        'area-name-input',
        'area-form-submit',
        names.area,
      )
      await page.getByTestId('commodity-quick-location-save').click()
      await expect(page.getByTestId('commodity-quick-location')).toBeHidden()
      await expect(page.getByTestId('commodity-detail-location')).toContainText(names.area)

      const areaId = await findIdByName(request, s.auth, s.group.slug, 'areas', names.area)
      expect((await itemAttrs(request, s.auth, s.group.slug, s.itemId)).area_id).toBe(areaId)
    } finally {
      await cleanup(request, s, names)
    }
  })
})
