/**
 * E2E coverage for hiding a section in a group (#1607): an admin hides
 * Warranties from the group settings, the sidebar, the dashboard and the
 * Add item dialog lose it, its page shows a notice, and switching it back
 * on brings everything back.
 *
 * The spec works in a throwaway group so a parallel spec that reads the
 * shared group's warranties never sees them hidden.
 */
import { expect, type APIRequestContext, type Page } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { ensureLocationAndArea, extractApiAuth, type ApiAuth } from './includes/commodities-api.js'

const ADMIN_PASSWORD = 'TestPassword123'

interface Group {
  id: string
  name: string
  slug: string
}

function headers(auth: ApiAuth): Record<string, string> {
  return {
    'Content-Type': 'application/vnd.api+json',
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${auth.accessToken}`,
    'X-CSRF-Token': auth.csrfToken,
  }
}

async function createGroup(request: APIRequestContext, auth: ApiAuth): Promise<Group> {
  const name = `Hidden Sections ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  const resp = await request.post('/api/v1/groups', {
    headers: headers(auth),
    data: { data: { type: 'groups', attributes: { name, icon: '🧪' } } },
  })
  expect(resp.status(), await resp.text()).toBe(201)
  const body = await resp.json()
  return { id: body.data.id as string, name, slug: body.data.attributes.slug as string }
}

async function deleteGroup(request: APIRequestContext, auth: ApiAuth, group: Group) {
  await request.delete(`/api/v1/groups/${group.id}`, {
    headers: headers(auth),
    data: { confirm_word: group.name, password: ADMIN_PASSWORD },
  })
}

// toggleWarranties flips the Warranties switch on the Sections card and
// waits for the save, returning the hidden sections the server stored.
async function toggleWarranties(page: Page, group: Group): Promise<string[]> {
  await page.goto(`/groups/${group.id}/settings?section=sections`)
  const toggle = page.getByTestId('sections-toggle-warranties')
  await expect(toggle).toBeEnabled()
  const saved = page.waitForResponse(
    (r) => r.url().endsWith(`/api/v1/groups/${group.id}/hidden-sections`) && r.request().method() === 'PUT',
  )
  await toggle.click()
  const resp = await saved
  expect(resp.status()).toBe(200)
  const body = await resp.json()
  return body.data.attributes.hidden_sections as string[]
}

function inventoryNav(page: Page) {
  return page.getByTestId('sidebar-inventory-group')
}

async function openAddItemStepper(page: Page, group: Group) {
  await page.goto(`/g/${encodeURIComponent(group.slug)}/commodities`)
  await page.getByTestId('commodities-add-button').click()
  await page.getByTestId('commodity-form-ai-step').waitFor({ state: 'visible' })
  await page.getByTestId('commodity-form-ai-fill-manually').click()
  await page.getByTestId('commodity-form-basics-step').waitFor({ state: 'visible' })
  return page.getByRole('list', { name: /form steps/i })
}

test.describe('Hidden sections (#1607)', () => {
  test('an admin hides Warranties for the group and shows them again', async ({ page, request }) => {
    const auth = await extractApiAuth(page)
    const group = await createGroup(request, auth)

    try {
      await ensureLocationAndArea(request, auth, group.slug)

      // Shown by default.
      await page.goto(`/g/${encodeURIComponent(group.slug)}`)
      await expect(inventoryNav(page).getByRole('link', { name: 'Warranties' })).toBeVisible()
      await expect(page.getByTestId('dashboard-warranty-health')).toBeVisible()

      // Hide.
      expect(await toggleWarranties(page, group)).toEqual(['warranties'])
      await expect(page.getByTestId('sections-toggle-warranties')).not.toBeChecked()

      await page.goto(`/g/${encodeURIComponent(group.slug)}`)
      await expect(page.getByTestId('dashboard-commodities-count')).toBeVisible()
      await expect(inventoryNav(page).getByRole('link', { name: 'Warranties' })).toHaveCount(0)
      await expect(page.getByTestId('dashboard-warranty-health')).toHaveCount(0)
      await expect(page.getByTestId('dashboard-expiring-warranties')).toHaveCount(0)
      await expect(page.getByTestId('dashboard-active-warranties')).toHaveCount(0)

      let stepper = await openAddItemStepper(page, group)
      await expect(stepper.getByRole('button', { name: 'Purchase' })).toBeVisible()
      await expect(stepper.getByRole('button', { name: 'Warranty' })).toHaveCount(0)

      await page.goto(`/g/${encodeURIComponent(group.slug)}/warranties`)
      const notice = page.getByTestId('hidden-section-notice')
      await expect(notice).toBeVisible()
      await expect(notice).toHaveAttribute('data-section', 'warranties')

      // Show again.
      await notice.getByTestId('hidden-section-settings').click()
      await expect(page.getByTestId('sections-card')).toBeVisible()
      expect(await toggleWarranties(page, group)).toEqual([])

      await page.goto(`/g/${encodeURIComponent(group.slug)}`)
      await expect(inventoryNav(page).getByRole('link', { name: 'Warranties' })).toBeVisible()
      await expect(page.getByTestId('dashboard-warranty-health')).toBeVisible()

      stepper = await openAddItemStepper(page, group)
      await expect(stepper.getByRole('button', { name: 'Warranty' })).toBeVisible()
    } finally {
      await deleteGroup(request, auth, group)
    }
  })
})
