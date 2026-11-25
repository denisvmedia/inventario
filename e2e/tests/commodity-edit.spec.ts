import { expect, type APIRequestContext, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { navigateWithAuth } from './includes/auth.js'
import {
  createCommodityViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
  type ResolvedGroup,
} from './includes/commodities-api.js'

/**
 * Editing items without the five-step dialog (#2795): the in-place edit
 * form with Previous / Next and "Save and next" over the Items list it was
 * opened from, quick edits on the Details tab, and Save on a middle step
 * of the dialog that "All fields" opens.
 *
 * Every item is seeded through the API with a name carrying a suffix
 * unique to the run, so the list searched for that suffix holds exactly
 * them, in name order.
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

async function deleteTag(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  tagSlug: string,
): Promise<void> {
  const base = `/api/v1/g/${encodeURIComponent(slug)}`
  const resp = await request.get(`${base}/tags?kind=commodity&per_page=100`, {
    headers: jsonHeaders(auth),
  })
  if (!resp.ok()) return
  const body = (await resp.json()) as { data?: Array<{ id: string; slug: string }> }
  for (const tag of body.data ?? []) {
    if (tag.slug !== tagSlug) continue
    await request.delete(`${base}/tags/${encodeURIComponent(tag.id)}?force=true`, {
      headers: jsonHeaders(auth),
    })
  }
}

interface Seeded {
  auth: ApiAuth
  group: ResolvedGroup
  suffix: string
  ids: string[]
}

// Seeds one item per letter, named `Edit <letter> <suffix>`. Short names
// leave the suffix out, so renaming an item takes it out of the search.
async function seed(page: Page, request: APIRequestContext, letters: string[]): Promise<Seeded> {
  await navigateWithAuth(page, '/commodities')
  await expect(page.getByTestId('page-commodities')).toBeVisible()
  const auth = await extractApiAuth(page)
  const group = await resolveActiveGroup(request, auth)
  const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
  // Parallel workers can seed within the same millisecond; the random part
  // keeps their searches apart.
  const suffix = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`
  const ids: string[] = []
  for (const letter of letters) {
    const item = await createCommodityViaAPI(
      request,
      auth,
      group.slug,
      { name: `Edit ${letter} ${suffix}`, shortName: `edit-${letter.toLowerCase()}`, areaId },
      group.groupCurrency,
    )
    ids.push(item.id)
  }
  return { auth, group, suffix, ids }
}

async function cleanup(request: APIRequestContext, s: Seeded): Promise<void> {
  for (const id of s.ids) {
    await deleteCommodityViaAPI(request, s.auth, s.group.slug, id)
  }
}

test.describe('Item editing', () => {
  test('Save and next walks the searched Items list', async ({ page, request }) => {
    const s = await seed(page, request, ['A', 'B', 'C'])
    const [a, b, c] = s.ids
    const slug = encodeURIComponent(s.group.slug)
    try {
      await page.goto(`/g/${slug}/commodities?q=${s.suffix}`)
      await page
        .getByTestId('commodity-card')
        .filter({ hasText: `Edit A ${s.suffix}` })
        .getByTestId('commodity-card-link')
        .click()
      const sheet = page.getByTestId('commodity-detail-sheet')
      await expect(sheet).toBeVisible()
      await expect(page.getByTestId('commodity-detail-position')).toHaveText('1 of 3')
      await page.getByTestId('commodity-detail-edit').click()

      // The edit form replaces the read view and knows the list.
      await expect(page).toHaveURL(new RegExp(`/commodities/${a}/edit\\?from=`))
      await expect(page.getByTestId('commodity-edit-form')).toBeVisible()
      await expect(page.getByTestId('commodity-edit-position')).toHaveText('1 of 3')
      await page.locator('#edit-serial').fill(`SN-A-${s.suffix}`)
      await page.getByTestId('commodity-edit-save-next').click()

      await expect(page).toHaveURL(new RegExp(`/commodities/${b}/edit\\?from=`))
      await expect(page.locator('#edit-name')).toHaveValue(`Edit B ${s.suffix}`)
      await expect(page.getByTestId('commodity-edit-position')).toHaveText('2 of 3')
      expect((await itemAttrs(request, s.auth, s.group.slug, a)).serial_number).toBe(
        `SN-A-${s.suffix}`,
      )

      // Renaming B takes it out of the search; the walk still goes on to
      // the item that followed it.
      await page.locator('#edit-name').fill('Edit B renamed')
      await page.getByTestId('commodity-edit-save-next').click()
      await expect(page).toHaveURL(new RegExp(`/commodities/${c}/edit\\?from=`))
      await expect(page.locator('#edit-name')).toHaveValue(`Edit C ${s.suffix}`)
      await expect(page.getByTestId('commodity-edit-position')).toHaveText('2 of 2')
      await expect(page.getByTestId('commodity-edit-save-next')).toBeDisabled()
      expect((await itemAttrs(request, s.auth, s.group.slug, b)).name).toBe('Edit B renamed')

      // Cancel with nothing changed returns to the read view, and closing
      // the panel shows the list as it was searched.
      await page.getByTestId('commodity-edit-cancel').click()
      await expect(page.getByTestId('commodity-edit-form')).toBeHidden()
      await expect(page).toHaveURL(new RegExp(`/commodities/${c}\\?from=`))
      await page.keyboard.press('Escape')
      await expect(sheet).toBeHidden()
      await expect(page).toHaveURL(new RegExp(`/commodities\\?q=${s.suffix}$`))
      await expect(page.getByTestId('commodity-card')).toHaveCount(2)
    } finally {
      await cleanup(request, s)
    }
  })

  test('quick edits on the Details tab save one field', async ({ page, request }) => {
    const s = await seed(page, request, ['Q'])
    const [id] = s.ids
    const tagLabel = `Quick ${s.suffix}`
    const tagSlug = `quick-${s.suffix}`
    try {
      await page.goto(`/g/${encodeURIComponent(s.group.slug)}/commodities/${id}`)
      await expect(page.getByTestId('page-commodity-detail')).toBeVisible()

      await page.getByTestId('commodity-quick-edit-tags').click()
      await page.getByTestId('commodity-quick-tags-palette-input').fill(tagLabel)
      await page.getByTestId('commodity-quick-tags-palette-input').press('Enter')
      await page.getByTestId('commodity-quick-tags-save').click()
      await expect(page.getByTestId('commodity-quick-tags')).toBeHidden()
      await expect(page.getByTestId('commodity-detail-tags')).toContainText(tagLabel)

      await page.getByTestId('commodity-quick-edit-notes').click()
      await page.getByTestId('commodity-quick-notes-input').fill('Keep the original box')
      await page.getByTestId('commodity-quick-notes-save').click()
      // The editor closes once the save is through.
      await expect(page.getByTestId('commodity-quick-notes')).toBeHidden()
      await expect(page.getByTestId('commodity-detail-notes')).toContainText(
        'Keep the original box',
      )

      const attrs = await itemAttrs(request, s.auth, s.group.slug, id)
      expect(attrs.tags).toEqual([tagSlug])
      expect(attrs.comments).toBe('Keep the original box')
      expect(attrs.name).toBe(`Edit Q ${s.suffix}`)
      // Quick edits never leave the read view.
      await expect(page).toHaveURL(new RegExp(`/commodities/${id}$`))
    } finally {
      await cleanup(request, s)
      await deleteTag(request, s.auth, s.group.slug, tagSlug)
    }
  })

  test('the dialog saves from a middle step', async ({ page, request }) => {
    const s = await seed(page, request, ['D'])
    const [id] = s.ids
    try {
      await page.goto(`/g/${encodeURIComponent(s.group.slug)}/commodities/${id}/edit`)
      await expect(page.getByTestId('commodity-edit-form')).toBeVisible()
      await page.getByTestId('commodity-edit-all-fields').click()
      const dialog = page.getByTestId('commodity-form-dialog')
      await expect(dialog).toBeVisible()

      // Every step is reachable from the stepper; Warranty is the third.
      await dialog.getByRole('button', { name: 'Warranty', exact: true }).click()
      await expect(page.getByTestId('commodity-form-warranty-step')).toBeVisible()
      await page.getByTestId('commodity-form-warranty-notes').fill('Two years, keep receipt')
      await page.getByTestId('commodity-form-submit').click()

      await expect(dialog).toBeHidden()
      await expect(page.getByTestId('commodity-edit-form')).toBeHidden()
      await expect(page).toHaveURL(new RegExp(`/commodities/${id}$`))
      expect((await itemAttrs(request, s.auth, s.group.slug, id)).warranty_notes).toBe(
        'Two years, keep receipt',
      )
    } finally {
      await cleanup(request, s)
    }
  })
})
