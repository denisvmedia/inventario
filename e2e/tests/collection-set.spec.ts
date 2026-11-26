/**
 * E2E coverage for the Collection set (#1607): a group created as a
 * Collection gets the set's item types and custom fields and hides
 * Warranties; an item of a set type gets a Condition; removing the set
 * from the group settings lists that type and field as staying, deletes
 * the rest, and keeps the used ones.
 *
 * The spec works in a group of its own, created through the create-group
 * page, and deletes it at the end.
 */
import { expect, type APIRequestContext } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import {
  createCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  type ApiAuth,
} from './includes/commodities-api.js'

const ADMIN_PASSWORD = 'TestPassword123'

interface Group {
  id: string
  name: string
  slug: string
}

interface SetRow {
  key: string
  name: string
  state: string
  item_type_id?: string
  custom_field_id?: string
}

function headers(auth: ApiAuth): Record<string, string> {
  return {
    'Content-Type': 'application/vnd.api+json',
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${auth.accessToken}`,
    'X-CSRF-Token': auth.csrfToken,
  }
}

async function deleteGroup(request: APIRequestContext, auth: ApiAuth, group: Group) {
  await request.delete(`/api/v1/groups/${group.id}`, {
    headers: headers(auth),
    data: { confirm_word: group.name, password: ADMIN_PASSWORD },
  })
}

async function collectionSet(
  request: APIRequestContext,
  auth: ApiAuth,
  group: Group,
): Promise<{ types: SetRow[]; fields: SetRow[] }> {
  const resp = await request.get(`/api/v1/groups/${group.id}/collection-set`, {
    headers: headers(auth),
  })
  expect(resp.ok(), `GET /collection-set → ${resp.status()}`).toBe(true)
  const body = await resp.json()
  return {
    types: body.data.attributes.item_types as SetRow[],
    fields: body.data.attributes.custom_fields as SetRow[],
  }
}

async function itemTypeSlug(
  request: APIRequestContext,
  auth: ApiAuth,
  group: Group,
  id: string,
): Promise<string> {
  const resp = await request.get(`/api/v1/g/${encodeURIComponent(group.slug)}/item-types`, {
    headers: headers(auth),
  })
  expect(resp.ok(), `GET /item-types → ${resp.status()}`).toBe(true)
  const body = (await resp.json()) as { data: Array<{ id: string; attributes: { slug: string } }> }
  const row = body.data.find((t) => t.id === id)
  expect(row, `item type ${id} listed`).toBeTruthy()
  return row!.attributes.slug
}

test.describe('Collection set (#1607)', () => {
  test('a Collection group gets the set, and removing it keeps what an item uses', async ({
    page,
    request,
  }) => {
    const auth = await extractApiAuth(page)
    const name = `Collection ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    let group: Group | undefined

    try {
      // Create the group as a Collection.
      await page.goto('/groups/new')
      await page.getByTestId('group-name-input').fill(name)
      await page.getByTestId('group-kind-collection').click()
      const created = page.waitForResponse(
        (r) => r.url().endsWith('/api/v1/groups') && r.request().method() === 'POST',
      )
      await page.getByTestId('create-group-submit').click()
      const resp = await created
      expect(resp.status()).toBe(201)
      const body = await resp.json()
      group = { id: body.data.id as string, name, slug: body.data.attributes.slug as string }
      expect(body.data.attributes.hidden_sections).toEqual(['warranties'])
      expect(body.meta.collection_set).toEqual({
        added: true,
        created_item_types: 7,
        created_custom_fields: 8,
      })
      await page.waitForURL(`**/g/${encodeURIComponent(group.slug)}`)
      await expect(
        page.getByTestId('sidebar-inventory-group').getByRole('link', { name: 'Warranties' }),
      ).toHaveCount(0)

      // The types and fields are there.
      const set = await collectionSet(request, auth, group)
      const coins = set.types.find((t) => t.key === 'coins')!
      const art = set.types.find((t) => t.key === 'art')!
      const condition = set.fields.find((f) => f.key === 'condition')!
      const provenance = set.fields.find((f) => f.key === 'provenance')!
      const coinsSlug = await itemTypeSlug(request, auth, group, coins.item_type_id!)
      const artSlug = await itemTypeSlug(request, auth, group, art.item_type_id!)

      await page.goto(`/g/${encodeURIComponent(group.slug)}/item-types`)
      await expect(page.getByTestId(`item-type-row-${coinsSlug}`)).toContainText('Coins')
      await expect(page.getByTestId(`item-type-row-${artSlug}`)).toContainText('Art')

      await page.goto(`/g/${encodeURIComponent(group.slug)}/custom-fields`)
      await expect(page.getByTestId(`custom-field-row-${condition.custom_field_id}`)).toContainText(
        'Condition',
      )
      await expect(page.getByTestId(`custom-field-row-${provenance.custom_field_id}`)).toContainText(
        'Provenance',
      )

      // An item of the Coins type gets a Condition.
      const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
      const item = await createCommodityViaAPI(request, auth, group.slug, {
        name: 'Maria Theresa thaler',
        shortName: 'Thaler',
        type: coinsSlug,
        areaId,
      })
      await page.goto(`/g/${encodeURIComponent(group.slug)}/commodities/${item.id}/edit`)
      await expect(page.getByTestId('commodity-edit-form')).toBeVisible()
      await page.getByTestId('custom-field-add').click()
      await page.getByTestId(`custom-field-add-${condition.custom_field_id}`).click()
      await page.getByTestId(`custom-field-${condition.custom_field_id}-input`).click()
      await page.getByRole('option', { name: 'Mint' }).click()
      await page.getByTestId('commodity-edit-save').click()
      await expect(page.getByTestId('commodity-edit-form')).toBeHidden()
      const detail = page.getByTestId(`commodity-detail-custom-field-${condition.custom_field_id}`)
      await expect(detail).toContainText('Condition')
      await expect(detail).toContainText('Mint')

      // Removing the set lists the used type and field as staying.
      await page.goto(`/groups/${group.id}/settings?section=sections`)
      const card = page.getByTestId('collection-set-card')
      await expect(card.getByTestId('collection-set-state')).toHaveText('Added')
      await card.getByTestId('collection-set-remove').click()
      const dialog = page.getByTestId('collection-set-remove-dialog')
      await expect(dialog).toBeVisible()
      await expect(dialog.getByTestId('collection-set-stays-type-coins')).toContainText(
        '1 item has this type',
      )
      await expect(dialog.getByTestId('collection-set-stays-field-condition')).toContainText(
        '1 item has a value of this field',
      )
      await expect(dialog.getByTestId('collection-set-remove-goes')).toContainText('Art')
      const removed = page.waitForResponse(
        (r) =>
          r.url().endsWith(`/api/v1/groups/${group!.id}/collection-set`) &&
          r.request().method() === 'DELETE',
      )
      await dialog.getByTestId('collection-set-remove-confirm').click()
      expect((await removed).status()).toBe(200)
      await expect(dialog).toBeHidden()
      await expect(card.getByTestId('collection-set-state')).toHaveText('Partly added')

      // The used type and field stay; the rest is gone.
      await page.goto(`/g/${encodeURIComponent(group.slug)}/item-types`)
      await expect(page.getByTestId(`item-type-row-${coinsSlug}`)).toContainText('Coins')
      await expect(page.getByTestId(`item-type-row-${artSlug}`)).toHaveCount(0)
      await page.goto(`/g/${encodeURIComponent(group.slug)}/custom-fields`)
      await expect(page.getByTestId(`custom-field-row-${condition.custom_field_id}`)).toBeVisible()
      await expect(page.getByTestId(`custom-field-row-${provenance.custom_field_id}`)).toHaveCount(0)
    } finally {
      if (group) await deleteGroup(request, auth, group)
    }
  })
})
