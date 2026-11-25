/**
 * Trash round trip: deleting an item moves it to the trash, the Trash
 * page restores it with the same id, and "Delete now" removes it for
 * good. A second case covers the Undo action on the delete toast.
 *
 * Items are seeded through the API with a per-run suffix, and cleanup
 * deletes and purges them so no row stays in the shared e2e database.
 */
import { expect, type APIRequestContext, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import {
  createCommodityViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  purgeTrashedCommodityViaAPI,
  resolveActiveGroup,
  type ApiAuth,
  type ResolvedGroup,
} from './includes/commodities-api.js'
import { axeAudit } from '../utils/axe.js'

interface Seeded {
  auth: ApiAuth
  group: ResolvedGroup
  item: { id: string; name: string }
}

async function seedItem(
  page: Page,
  request: APIRequestContext,
  label: string,
): Promise<Seeded> {
  const auth = await extractApiAuth(page)
  const group = await resolveActiveGroup(request, auth)
  const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  const item = await createCommodityViaAPI(
    request,
    auth,
    group.slug,
    { name: `${label} ${suffix}`, areaId },
    group.groupCurrency,
  )
  return { auth, group, item }
}

async function cleanup(request: APIRequestContext, seeded: Seeded): Promise<void> {
  const { auth, group, item } = seeded
  await deleteCommodityViaAPI(request, auth, group.slug, item.id).catch(() => {})
  await purgeTrashedCommodityViaAPI(request, auth, group.slug, item.id).catch(() => {})
}

async function liveStatus(
  request: APIRequestContext,
  seeded: Seeded,
): Promise<number> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(seeded.group.slug)}/commodities/${encodeURIComponent(seeded.item.id)}`,
    {
      headers: {
        Authorization: `Bearer ${seeded.auth.accessToken}`,
        Accept: 'application/vnd.api+json',
      },
    },
  )
  return resp.status()
}

async function deleteFromDetailPage(page: Page, seeded: Seeded): Promise<void> {
  await page.goto(
    `/g/${encodeURIComponent(seeded.group.slug)}/commodities/${encodeURIComponent(seeded.item.id)}`,
  )
  await page.getByTestId('commodity-detail-delete').click()
  const dialog = page.getByTestId('confirm-dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog).toContainText(/trash/i)
  await page.getByTestId('confirm-accept').click()
  await expect(dialog).toBeHidden()
}

test.describe('Trash', () => {
  test('a deleted item is restored from the trash, then deleted for good', async ({
    page,
    request,
    recorder,
  }) => {
    const seeded = await seedItem(page, request, 'Trash round trip')
    try {
      recorder.log('Step 1: delete the item from its page')
      await deleteFromDetailPage(page, seeded)
      await expect.poll(() => liveStatus(request, seeded)).toBe(404)

      recorder.log('Step 2: find it on the Trash page and restore it')
      await page.goto(`/g/${encodeURIComponent(seeded.group.slug)}/trash`)
      await expect(page.getByTestId('page-trash')).toBeVisible()
      const row = page.getByTestId(`trash-item-${seeded.item.id}`)
      await expect(row).toBeVisible({ timeout: 15000 })
      await expect(row).toContainText(seeded.item.name)
      await axeAudit(page, { include: ['[data-testid="page-trash"]'] })
      await recorder.takeScreenshot('trash-01-item-in-trash')

      await page.getByTestId(`trash-item-${seeded.item.id}-restore`).click()
      await expect(row).toHaveCount(0, { timeout: 15000 })
      await expect.poll(() => liveStatus(request, seeded)).toBe(200)

      recorder.log('Step 3: delete it again and delete it now from the trash')
      await deleteCommodityViaAPI(request, seeded.auth, seeded.group.slug, seeded.item.id)
      await page.reload()
      await expect(row).toBeVisible({ timeout: 15000 })
      await page.getByTestId(`trash-item-${seeded.item.id}-purge`).click()
      const dialog = page.getByTestId('confirm-dialog')
      await expect(dialog).toBeVisible()
      await axeAudit(page, { include: ['[role="dialog"]'] })
      await page.getByTestId('confirm-accept').click()
      await expect(row).toHaveCount(0, { timeout: 15000 })

      await page.reload()
      await expect(page.getByTestId('page-trash')).toBeVisible()
      await expect(page.getByTestId(`trash-item-${seeded.item.id}`)).toHaveCount(0)
    } finally {
      await cleanup(request, seeded)
    }
  })

  test('Undo on the delete toast brings the item back', async ({ page, request, recorder }) => {
    const seeded = await seedItem(page, request, 'Trash undo')
    try {
      recorder.log('Step 1: delete the item and click Undo on the toast')
      await deleteFromDetailPage(page, seeded)
      await page.getByRole('button', { name: 'Undo' }).click()

      recorder.log('Step 2: the item is live again under the same id')
      await expect.poll(() => liveStatus(request, seeded)).toBe(200)
    } finally {
      await cleanup(request, seeded)
    }
  })
})
