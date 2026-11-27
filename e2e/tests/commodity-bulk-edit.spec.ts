/**
 * E2E coverage for bulk edits on the Items list (#2798): add and remove
 * tags, set the type, and change the status of the selected items, then
 * select every item a query matches across pages and act on all of them.
 *
 * Each test seeds rows with a per-run suffix and opens
 * `/commodities?q=<suffix>`, so the list holds exactly the seeded rows
 * whatever else the shared e2e database carries. Results are read back
 * through the API: the list hides items that leave in_use. Cleanup is
 * registered before seeding, purges the rows from Trash too, and deletes
 * the tags the run created, which the dialog lists for the whole group.
 */
import { expect, type APIRequestContext, type Locator, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import {
  createCommodityViaAPI,
  deleteCommodityTagsViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  getCommodityViaAPI,
  purgeTrashedCommodityViaAPI,
  resolveActiveGroup,
  type ApiAuth,
  type ResolvedGroup,
} from './includes/commodities-api.js'
import { axeAudit } from '../utils/axe.js'

// See commodity-bulk-and-filter.spec.ts for why overlays are included
// and the sidebar is not.
function auditList(page: Page): Promise<void> {
  return axeAudit(page, {
    include: [
      '[data-testid="page-commodities"]',
      '[role="dialog"]',
      '[role="menu"][data-state="open"]',
    ],
  })
}

async function gotoScoped(page: Page, group: ResolvedGroup, q: string): Promise<void> {
  await page.goto(`/g/${encodeURIComponent(group.slug)}/commodities?q=${encodeURIComponent(q)}`)
  await expect(page.getByTestId('page-commodities')).toBeVisible()
}

function waitForBulk(page: Page, action: string) {
  return page.waitForResponse(
    (resp) => resp.url().includes(`/commodities/${action}`) && resp.request().method() === 'POST',
    { timeout: 30000 },
  )
}

async function openEditItem(page: Page, bar: Locator, testId: string): Promise<void> {
  await bar.getByTestId('commodities-bulk-edit').click()
  await page.getByTestId(testId).click()
}

function newSuffix(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

async function cleanupItems(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  ids: string[],
  suffix: string,
): Promise<void> {
  for (const id of ids) {
    await deleteCommodityViaAPI(request, auth, slug, id).catch(() => {})
    await purgeTrashedCommodityViaAPI(request, auth, slug, id).catch(() => {})
  }
  await deleteCommodityTagsViaAPI(request, auth, slug, suffix).catch(() => {})
}

test.describe('Commodities — bulk edit', () => {
  test('adds and removes tags, sets the type and changes the status of the selection', async ({
    page,
    request,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
    const suffix = newSuffix()
    const keep = `keep-${suffix}`
    const drop = `drop-${suffix}`
    const ids: string[] = []

    try {
      for (const name of ['A', 'B']) {
        const item = await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Bulk Edit ${name} ${suffix}`, areaId, tags: [keep, drop] },
          group.groupCurrency,
        )
        ids.push(item.id)
      }

      await gotoScoped(page, group, suffix)
      for (const id of ids) {
        const cb = page.locator(`[data-commodity-id="${id}"] [data-testid="commodity-select"]`)
        await expect(cb).toBeVisible({ timeout: 15000 })
        await cb.click()
      }
      const bar = page.getByTestId('commodities-bulk-bar')
      await expect(bar.getByTestId('commodities-bulk-count')).toHaveText('2 items selected')

      // Tags: add a new one, remove one, leave the third alone.
      await openEditItem(page, bar, 'commodities-bulk-edit-tags')
      const tagsDialog = page.getByTestId('bulk-tags-dialog')
      await expect(tagsDialog).toBeVisible()
      await tagsDialog.getByTestId('bulk-tags-add-input').fill(`Bulk ${suffix}`)
      await tagsDialog.getByTestId('bulk-tags-add-input').press('Enter')
      await tagsDialog.getByTestId(`bulk-tags-remove-option-${drop}`).click()
      await auditList(page)
      let response = waitForBulk(page, 'bulk-update')
      await tagsDialog.getByTestId('bulk-tags-confirm').click()
      expect((await response).status()).toBe(200)
      await expect(tagsDialog).toBeHidden()
      for (const id of ids) {
        const item = await getCommodityViaAPI(request, auth, group.slug, id)
        expect(item.tags).toEqual([keep, `bulk-${suffix}`])
      }

      // The selection was cleared; select both again for the type.
      for (const id of ids) {
        await page.locator(`[data-commodity-id="${id}"] [data-testid="commodity-select"]`).click()
      }
      await openEditItem(page, bar, 'commodities-bulk-edit-type')
      const typeDialog = page.getByTestId('bulk-type-dialog')
      await typeDialog.getByTestId('bulk-type-select').click()
      const listbox = page.getByRole('listbox')
      await listbox.getByRole('option', { name: /Electronics/i }).click()
      await listbox.waitFor({ state: 'detached', timeout: 5000 })
      response = waitForBulk(page, 'bulk-update')
      await typeDialog.getByTestId('bulk-type-confirm').click()
      expect((await response).status()).toBe(200)
      for (const id of ids) {
        expect((await getCommodityViaAPI(request, auth, group.slug, id)).type).toBe('electronics')
      }

      // Status: the same date and note dialog as a single change.
      for (const id of ids) {
        await page.locator(`[data-commodity-id="${id}"] [data-testid="commodity-select"]`).click()
      }
      await openEditItem(page, bar, 'commodities-bulk-status-lost')
      const statusDialog = page.getByTestId('status-transition-dialog')
      await expect(statusDialog).toContainText('Mark 2 items as Lost')
      await statusDialog.getByTestId('status-transition-date').fill('2026-11-20')
      await statusDialog.getByTestId('status-transition-note').fill(`Lost ${suffix}`)
      await auditList(page)
      response = waitForBulk(page, 'bulk-update')
      await statusDialog.getByTestId('status-transition-confirm').click()
      expect((await response).status()).toBe(200)
      for (const id of ids) {
        const item = await getCommodityViaAPI(request, auth, group.slug, id)
        expect(item.status).toBe('lost')
        expect(item.status_date).toBe('2026-11-20')
        expect(item.status_note).toBe(`Lost ${suffix}`)
      }
      // Lost items are inactive, so the default list no longer shows them.
      await expect(page.locator(`[data-commodity-id="${ids[0]}"]`)).toHaveCount(0)
      await auditList(page)
    } finally {
      await cleanupItems(request, auth, group.slug, ids, suffix)
    }
  })

  test('selects every matching item across pages and acts on all of them', async ({
    page,
    request,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
    const suffix = newSuffix()
    const ids: string[] = []
    // One more than the list's default page size (24), so the query spans
    // two pages and the bar offers "Select all N matching items".
    const total = 25

    try {
      for (let i = 0; i < total; i++) {
        const item = await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Bulk All ${String(i).padStart(2, '0')} ${suffix}`, areaId },
          group.groupCurrency,
        )
        ids.push(item.id)
      }

      await gotoScoped(page, group, suffix)
      const first = page.locator(`[data-commodity-id="${ids[0]}"] [data-testid="commodity-select"]`)
      await expect(first).toBeVisible({ timeout: 15000 })
      await first.click()
      const bar = page.getByTestId('commodities-bulk-bar')
      await bar.getByTestId('commodities-select-all').click()
      await bar.getByTestId('commodities-select-all-matching').click()
      await expect(bar.getByTestId('commodities-bulk-count')).toHaveText(
        `All ${total} matching items selected`,
      )
      await auditList(page)

      await openEditItem(page, bar, 'commodities-bulk-edit-tags')
      const tagsDialog = page.getByTestId('bulk-tags-dialog')
      await expect(tagsDialog).toContainText(`Edit the tags of ${total} items`)
      await tagsDialog.getByTestId('bulk-tags-add-input').fill(`All ${suffix}`)
      await tagsDialog.getByTestId('bulk-tags-add-input').press('Enter')
      const response = waitForBulk(page, 'bulk-update')
      await tagsDialog.getByTestId('bulk-tags-confirm').click()
      const updated = await response
      expect(updated.status()).toBe(200)
      // The request named the query, and the server resolved it to every
      // matching item, the one on the second page included.
      const sent = updated.request().postDataJSON() as {
        data: {
          attributes: { ids?: string[]; query?: string; expected_count?: number; expected_digest?: string }
        }
      }
      expect(sent.data.attributes.ids).toBeUndefined()
      expect(sent.data.attributes.expected_count).toBe(total)
      // The digest of the matched ids the list gave, so a swapped item is
      // refused as a changed count is.
      expect(sent.data.attributes.expected_digest).toMatch(/^[0-9a-f]{64}$/)
      const body = (await updated.json()) as { data: { attributes: { succeeded: string[] } } }
      expect(body.data.attributes.succeeded).toHaveLength(total)
      const last = await getCommodityViaAPI(request, auth, group.slug, ids[total - 1])
      expect(last.tags).toEqual([`all-${suffix}`])

      // Delete them all the same way; they go to Trash.
      await first.click()
      await bar.getByTestId('commodities-select-all').click()
      await bar.getByTestId('commodities-select-all-matching').click()
      const deleted = waitForBulk(page, 'bulk-delete')
      await bar.getByTestId('commodities-bulk-delete').click()
      await expect(page.getByTestId('confirm-dialog')).toContainText(`Delete ${total} items?`)
      await page.getByTestId('confirm-accept').click()
      expect((await deleted).status()).toBe(200)
      await expect(page.getByTestId('commodities-empty-filtered')).toBeVisible({ timeout: 15000 })
    } finally {
      await cleanupItems(request, auth, group.slug, ids, suffix)
    }
  })
})
