import * as fs from 'node:fs'
import * as path from 'node:path'

import { expect, type APIRequestContext } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { navigateWithAuth } from './includes/auth.js'
import { extractApiAuth, resolveActiveGroup, type ApiAuth } from './includes/commodities-api.js'

/**
 * Reviewing files one after another from a filtered Files list (#2814).
 *
 * Three files share a tag unique to this run, so the list filtered by
 * that tag holds exactly them. The user opens the first one for editing,
 * saves it with "Save and next", removes the tag from the second one (the
 * edit takes it out of the filter) and saves again, and must land on the
 * third file. "Back to files" then returns to the same filtered list with
 * the last file's panel open.
 */

function jsonHeaders(auth: ApiAuth): Record<string, string> {
  return {
    'Content-Type': 'application/vnd.api+json',
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${auth.accessToken}`,
    'X-CSRF-Token': auth.csrfToken,
  }
}

interface SeededFile {
  id: string
  path: string
}

async function uploadPdf(
  request: APIRequestContext,
  auth: ApiAuth,
  groupSlug: string,
  name: string,
): Promise<SeededFile> {
  const resp = await request.post(`/api/v1/g/${encodeURIComponent(groupSlug)}/uploads/file`, {
    headers: {
      Accept: 'application/vnd.api+json',
      Authorization: `Bearer ${auth.accessToken}`,
      'X-CSRF-Token': auth.csrfToken,
    },
    multipart: {
      file: {
        name,
        mimeType: 'application/pdf',
        buffer: fs.readFileSync(path.join('fixtures', 'files', 'manual.pdf')),
      },
    },
  })
  expect(resp.status(), `upload ${name} → ${resp.status()}`).toBe(201)
  const body = (await resp.json()) as { id: string; attributes: { path: string } }
  return { id: body.id, path: body.attributes.path }
}

async function updateFile(
  request: APIRequestContext,
  auth: ApiAuth,
  groupSlug: string,
  id: string,
  attributes: Record<string, unknown>,
): Promise<void> {
  const resp = await request.put(
    `/api/v1/g/${encodeURIComponent(groupSlug)}/files/${encodeURIComponent(id)}`,
    { headers: jsonHeaders(auth), data: { data: { id, type: 'files', attributes } } },
  )
  expect(resp.ok(), `PUT /files/${id} → ${resp.status()}`).toBe(true)
}

async function fileAttrs(
  request: APIRequestContext,
  auth: ApiAuth,
  groupSlug: string,
  id: string,
): Promise<{ title: string; tags: string[] }> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(groupSlug)}/files/${encodeURIComponent(id)}`,
    { headers: jsonHeaders(auth) },
  )
  expect(resp.ok(), `GET /files/${id} → ${resp.status()}`).toBe(true)
  const body = (await resp.json()) as { attributes: { title: string; tags?: string[] } }
  return { title: body.attributes.title, tags: body.attributes.tags ?? [] }
}

async function listedIds(
  request: APIRequestContext,
  auth: ApiAuth,
  groupSlug: string,
  tag: string,
): Promise<string[]> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(groupSlug)}/files?page=1&limit=24&tags=${encodeURIComponent(tag)}`,
    { headers: jsonHeaders(auth) },
  )
  expect(resp.ok(), `GET /files?tags=${tag} → ${resp.status()}`).toBe(true)
  const body = (await resp.json()) as { data?: Array<{ id: string }> }
  return (body.data ?? []).map((row) => row.id)
}

async function cleanup(
  request: APIRequestContext,
  auth: ApiAuth,
  groupSlug: string,
  ids: string[],
  tagSlug: string,
): Promise<void> {
  const base = `/api/v1/g/${encodeURIComponent(groupSlug)}`
  for (const id of ids) {
    await request.delete(`${base}/files/${encodeURIComponent(id)}`, { headers: jsonHeaders(auth) })
  }
  const resp = await request.get(`${base}/tags?kind=file&per_page=100`, { headers: jsonHeaders(auth) })
  if (!resp.ok()) return
  const body = (await resp.json()) as { data?: Array<{ id: string; slug: string }> }
  for (const tag of body.data ?? []) {
    if (tag.slug !== tagSlug) continue
    await request.delete(`${base}/tags/${encodeURIComponent(tag.id)}?force=true`, {
      headers: jsonHeaders(auth),
    })
  }
}

test.describe('File review', () => {
  test('Save and next walks a filtered list and Back returns to it', async ({ page, request }) => {
    await navigateWithAuth(page, '/files')
    await expect(page.getByTestId('page-files')).toBeVisible()
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)

    const suffix = `${Date.now()}`
    const tag = `review-${suffix}`
    const seeded: SeededFile[] = []
    try {
      for (const n of [1, 2, 3]) {
        const file = await uploadPdf(request, auth, group.slug, `review-${n}-${suffix}.pdf`)
        seeded.push(file)
        await updateFile(request, auth, group.slug, file.id, {
          title: `Review ${n} ${suffix}`,
          path: file.path,
          category: 'documents',
          tags: [tag],
        })
      }
      // The list order is the server's (newest first); read it rather
      // than assume it.
      const order = await listedIds(request, auth, group.slug, tag)
      expect(order).toHaveLength(3)
      const [first, second, third] = order
      const titleOf = async (id: string) => (await fileAttrs(request, auth, group.slug, id)).title

      const listUrl = `/g/${encodeURIComponent(group.slug)}/files?tags=${encodeURIComponent(tag)}`
      await page.goto(listUrl)
      await page.getByTestId(`file-row-${first}`).getByRole('button').first().click()
      await expect(page.getByTestId('file-detail-sheet')).toBeVisible()
      await page.getByTestId('file-detail-edit').click()

      // The review page knows the list it came from.
      await expect(page).toHaveURL(new RegExp(`/files/${first}/edit\\?from=`))
      await expect(page.getByTestId('file-review-position')).toHaveText('1 of 3')
      await expect(page.getByTestId('file-review-preview')).toBeVisible()

      const title = page.getByTestId('file-edit-title')
      await expect(title).toHaveValue(await titleOf(first))
      await title.fill(`Reviewed ${suffix}`)
      await page.getByTestId('file-review-save-next').click()

      await expect(page).toHaveURL(new RegExp(`/files/${second}/edit\\?from=`))
      await expect(page.getByTestId('file-review-position')).toHaveText('2 of 3')
      await expect(title).toHaveValue(await titleOf(second))
      expect(await titleOf(first)).toBe(`Reviewed ${suffix}`)

      // Removing the tag takes this file out of the filtered list. The
      // walk still continues with the file that followed it.
      await page
        .getByTestId('file-edit-tags')
        .getByRole('button', { name: `remove ${tag}` })
        .click()
      await page.getByTestId('file-review-save-next').click()

      await expect(page).toHaveURL(new RegExp(`/files/${third}/edit\\?from=`))
      await expect(title).toHaveValue(await titleOf(third))
      await expect(page.getByTestId('file-review-position')).toHaveText('2 of 2')
      expect((await fileAttrs(request, auth, group.slug, second)).tags).not.toContain(tag)
      await expect(page.getByTestId('file-review-save-next')).toBeDisabled()

      // Back lands on the same filtered list with this file's panel open.
      await page.getByTestId('file-review-back').click()
      await expect(page).toHaveURL(new RegExp(`/g/[^/]+/files/${third}\\?tags=${tag}$`))
      await expect(page.getByTestId('file-detail-sheet')).toBeVisible()
      await page.keyboard.press('Escape')
      await expect(page.getByTestId('file-detail-sheet')).toBeHidden()
      await expect(page.getByTestId(`file-row-${first}`)).toBeVisible()
      await expect(page.getByTestId(`file-row-${third}`)).toBeVisible()
      await expect(page.getByTestId(`file-row-${second}`)).toHaveCount(0)
    } finally {
      await cleanup(
        request,
        auth,
        group.slug,
        seeded.map((f) => f.id),
        tag,
      )
    }
  })

  test('unsaved edits ask before leaving for the next file', async ({ page, request }) => {
    await navigateWithAuth(page, '/files')
    await expect(page.getByTestId('page-files')).toBeVisible()
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)

    const suffix = `${Date.now()}`
    const tag = `review-guard-${suffix}`
    const seeded: SeededFile[] = []
    try {
      for (const n of [1, 2]) {
        const file = await uploadPdf(request, auth, group.slug, `guard-${n}-${suffix}.pdf`)
        seeded.push(file)
        await updateFile(request, auth, group.slug, file.id, {
          title: `Guard ${n} ${suffix}`,
          path: file.path,
          category: 'documents',
          tags: [tag],
        })
      }
      const [first, second] = await listedIds(request, auth, group.slug, tag)
      const from = `/g/${group.slug}/files?tags=${tag}`
      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/files/${first}/edit?from=${encodeURIComponent(from)}`,
      )
      await expect(page.getByTestId('file-review-position')).toHaveText('1 of 2')

      await page.getByTestId('file-edit-title').fill(`Unsaved ${suffix}`)
      await page.getByTestId('file-review-next').click()
      const dialog = page.getByTestId('unsaved-changes-dialog')
      await expect(dialog).toBeVisible()
      await dialog.getByTestId('unsaved-changes-keep-editing').click()
      await expect(page).toHaveURL(new RegExp(`/files/${first}/edit`))
      await expect(page.getByTestId('file-edit-title')).toHaveValue(`Unsaved ${suffix}`)

      await page.getByTestId('file-review-next').click()
      await page.getByTestId('unsaved-changes-save').click()
      await expect(page).toHaveURL(new RegExp(`/files/${second}/edit`))
      expect((await fileAttrs(request, auth, group.slug, first)).title).toBe(`Unsaved ${suffix}`)
    } finally {
      await cleanup(
        request,
        auth,
        group.slug,
        seeded.map((f) => f.id),
        tag,
      )
    }
  })
})
