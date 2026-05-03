import { expect, type APIRequestContext, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { navigateWithAuth } from './includes/auth.js'
import {
  createCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
} from './includes/commodities-api.js'

/**
 * E2E coverage for the Tags page (#1412).
 *
 * Covers the issue's E2E acceptance line:
 *   create → rename → assign to an item → delete with force.
 *
 * The "assign to an item" step is done via the BE (PUT commodity)
 * because the Tags page itself does not own the assignment surface —
 * tags are attached on the commodity / file detail pages. The point
 * here is that the BE rewrite (slug change) propagates to existing
 * commodities, and that the Tags page surfaces a non-zero usage count
 * which then forces the delete flow through the in-use confirm dialog.
 */

async function gotoTags(page: Page): Promise<void> {
  await navigateWithAuth(page, '/tags')
  await expect(page.getByTestId('page-tags')).toBeVisible()
}

async function attachTagToCommodity(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  commodityId: string,
  tagSlug: string,
): Promise<void> {
  const headers = {
    'Content-Type': 'application/vnd.api+json',
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${auth.accessToken}`,
    'X-CSRF-Token': auth.csrfToken,
  }
  // BE PUT replaces the full entity, so re-fetch attributes first and
  // splice in the new tag slug. A partial body (`{ tags: [...] }` only)
  // returns 422 ("cannot be blank" for every other field).
  const getResp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityId)}`,
    { headers },
  )
  if (!getResp.ok()) {
    throw new Error(
      `attachTagToCommodity: GET /commodities/${commodityId} → ${getResp.status()} ${await getResp.text()}`,
    )
  }
  const body = (await getResp.json()) as {
    data: { id: string; type: string; attributes: Record<string, unknown> }
  }
  const existingTags = (body.data.attributes.tags as string[] | undefined) ?? []
  const merged = Array.from(new Set([...existingTags, tagSlug]))

  const resp = await request.put(
    `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityId)}`,
    {
      headers,
      data: {
        data: {
          id: commodityId,
          type: 'commodities',
          attributes: { ...body.data.attributes, tags: merged },
        },
      },
    },
  )
  if (!resp.ok()) {
    throw new Error(
      `attachTagToCommodity: PUT /commodities/${commodityId} → ${resp.status()} ${await resp.text()}`,
    )
  }
}

test.describe('Tags page', () => {
  test('renders the page shell with stats + create CTA', async ({ page }) => {
    await gotoTags(page)
    await expect(page.getByTestId('tags-stats-tags-total')).toBeVisible()
    await expect(page.getByTestId('tags-stats-items-tagged')).toBeVisible()
    await expect(page.getByTestId('tags-stats-items-untagged')).toBeVisible()
    await expect(page.getByTestId('tags-create-button')).toBeVisible()
  })

  test('create → rename → attach → force-delete full flow', async ({ page, request }) => {
    // Unique label so parallel runs / re-runs don't collide.
    const initialLabel = `E2E Initial ${Date.now()}`
    const renamedSlug = `e2e-renamed-${Date.now()}`

    await gotoTags(page)

    // --- Create ---
    await page.getByTestId('tags-create-button').click()
    await expect(page.getByTestId('tag-form-dialog')).toBeVisible()
    await page.getByTestId('tag-form-label').fill(initialLabel)
    // Slug auto-derives from label; verify before submitting.
    const derivedSlug = await page.getByTestId('tag-form-slug').inputValue()
    expect(derivedSlug.length).toBeGreaterThan(0)
    await page.getByTestId('tag-form-color-amber').click()
    await page.getByTestId('tag-form-submit').click()
    await expect(page.getByTestId('tag-form-dialog')).not.toBeVisible()
    const initialRow = page.getByTestId(`tag-row-${derivedSlug}`)
    await expect(initialRow).toBeVisible()

    // --- Rename --- (the slug change is what we care about — it
    // proves the rename rewrite path; the BE rewrites JSONB refs in
    // commodities + files, which we exercise on the next step).
    await page.getByTestId(`tag-row-${derivedSlug}-edit`).click()
    await expect(page.getByTestId('tag-form-dialog')).toBeVisible()
    await page.getByTestId('tag-form-slug').fill(renamedSlug)
    await page.getByTestId('tag-form-color-blue').click()
    await page.getByTestId('tag-form-submit').click()
    await expect(page.getByTestId('tag-form-dialog')).not.toBeVisible()
    const renamedRow = page.getByTestId(`tag-row-${renamedSlug}`)
    await expect(renamedRow).toBeVisible()

    // --- Attach to a commodity via BE ---
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)
    const commodity = await createCommodityViaAPI(
      request,
      auth,
      group.slug,
      { name: `tag-target-${Date.now()}`, areaId },
      group.mainCurrency,
    )
    await attachTagToCommodity(request, auth, group.slug, commodity.id, renamedSlug)

    // Reload so the page picks up the fresh usage count.
    await page.reload()
    await expect(page.getByTestId(`tag-row-${renamedSlug}`)).toBeVisible()
    await expect(page.getByTestId(`tag-row-${renamedSlug}-usage`)).toContainText('1 item')

    // --- Force-delete --- (in-use → confirm dialog uses the
    // "Force delete" button; useConfirm renders its accept button as
    // `confirm-accept` regardless of label, so we click that.)
    await page.getByTestId(`tag-row-${renamedSlug}-delete`).click()
    await expect(page.getByTestId('confirm-dialog')).toBeVisible()
    await expect(page.getByTestId('confirm-dialog')).toContainText('in use')
    await page.getByTestId('confirm-accept').click()
    await expect(page.getByTestId(`tag-row-${renamedSlug}`)).not.toBeVisible()
  })
})
