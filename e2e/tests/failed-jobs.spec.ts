/**
 * E2E coverage for the group view of failed background jobs (#1307): a
 * group admin opens Group settings → Data & storage and sees the Failed
 * background jobs card, backed by GET /g/{slug}/failed-jobs.
 *
 * The spec works in a throwaway group, so the list is empty: nothing in
 * the e2e stack makes a job fail for good on demand (an export fails only
 * on a storage error, a thumbnail only after its last attempt), and the
 * list, retry and refusal paths with failed rows are covered by the
 * registry contract tests, the API tests and the Vitest suite.
 */
import { expect, type APIRequestContext } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { extractApiAuth, type ApiAuth } from './includes/commodities-api.js'

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
  const name = `Failed Jobs ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
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

test.describe('Failed background jobs (#1307)', () => {
  test('a group admin sees the failed jobs card in group settings', async ({ page, request }) => {
    const auth = await extractApiAuth(page)
    const group = await createGroup(request, auth)

    try {
      const listed = page.waitForResponse(
        (r) =>
          r.url().includes(`/api/v1/g/${encodeURIComponent(group.slug)}/failed-jobs`) &&
          r.request().method() === 'GET',
      )
      await page.goto(`/groups/${group.id}/settings?section=data`)

      const resp = await listed
      expect(resp.status()).toBe(200)
      const body = await resp.json()
      expect(body.data).toEqual([])
      expect(body.meta.total).toBe(0)
      expect(body.meta.job_types.map((t: { type: string }) => t.type)).toEqual([
        'export',
        'import',
        'restore',
        'thumbnail',
        'intake',
        'currency-migration',
      ])

      const card = page.getByTestId('failed-jobs-card')
      await expect(card).toBeVisible()
      await expect(card.getByTestId('failed-jobs-empty')).toHaveText('No failed jobs in this group.')

      // A retry of a job that is not failed is refused with a coded 404.
      const retry = await request.post(
        `/api/v1/g/${encodeURIComponent(group.slug)}/failed-jobs/export/00000000-0000-0000-0000-000000000000/retry`,
        { headers: headers(auth) },
      )
      expect(retry.status()).toBe(404)
      expect((await retry.json()).errors[0].code).toBe('failed_job.not_found')
    } finally {
      await deleteGroup(request, auth, group)
    }
  })
})
