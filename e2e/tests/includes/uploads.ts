import * as fs from 'node:fs'
import * as path from 'node:path'
import { expect, type Page } from '@playwright/test'

import { TestRecorder } from '../../utils/test-recorder.js'

// React-era unified-attach helpers (post-cutover #1423, post-#1448
// quick-attach). The legacy Vue surface had three category-specific
// upload sections per entity — `.commodity-images`, `.commodity-manuals`,
// `.commodity-invoices`, plus location twins. The React app replaces
// those with a single `EntityFilesPanel` per entity (read-only grid)
// and a unified `UploadFilesDialog` triggered via the panel's attach
// button. Category is set per-file inside the dialog's metadata step.

export type FileCategory = 'photos' | 'invoices' | 'documents' | 'other'

export interface UploadItem {
  /** Filesystem path under e2e/fixtures/ — read directly with fs. */
  fixturePath: string
  /** Optional override for the upload's filename (e.g. unique per run). */
  uploadName?: string
  /** Category to set in the dialog's metadata step. Defaults to 'other'. */
  category?: FileCategory
  /** Optional title to set in metadata; falls back to BE-derived default. */
  title?: string
}

/**
 * Open the unified upload dialog from an attach affordance on the
 * caller side (e.g. clicking `entity-files-panel-attach`), then walk
 * select → metadata → progress → close. Asserts each step lands on
 * its testid and the upload network call returns 201 per file.
 *
 * Caller is expected to have already opened the dialog by clicking
 * the trigger; this helper drives the dialog from the SelectStep.
 */
export async function uploadViaDialog(
  page: Page,
  recorder: TestRecorder,
  items: UploadItem[],
  screenshotPrefix = 'upload',
): Promise<void> {
  if (items.length === 0) {
    throw new Error('uploadViaDialog: items[] must contain at least one file')
  }

  const dialog = page.getByTestId('files-upload-dialog')
  await dialog.waitFor({ state: 'visible', timeout: 10_000 })

  // --- Step 1 (select): seed the file input directly. The dropzone
  // path requires a real DataTransfer which Playwright can't fabricate
  // outside a real drag-drop; setInputFiles on the hidden <input> is
  // the canonical Playwright pattern and exercises the same code path
  // (addFiles() runs on both onChange and onDrop). -----------------
  const inputs = items.map((it) => ({
    name: it.uploadName ?? path.basename(it.fixturePath),
    mimeType: mimeForExt(path.extname(it.fixturePath)),
    buffer: fs.readFileSync(path.join('fixtures', it.fixturePath)),
  }))
  await page.getByTestId('files-upload-input').setInputFiles(inputs)
  // The list testid renders only when items.length > 0.
  await expect(page.getByTestId('files-upload-list')).toBeVisible()
  await recorder.takeScreenshot(`${screenshotPrefix}-01-selected`)
  await page.getByTestId('files-upload-next').click()

  // --- Step 2 (metadata): set category per item via the per-row
  // <select>. Title field is left as default unless the caller wants
  // to override it. The list-item testid embeds the dialog's internal
  // id, not the file id (the BE id only exists post-upload), so we
  // walk the metadata items by index. ---------------------------
  await expect(page.getByTestId('files-upload-metadata-list')).toBeVisible()
  const metaItems = page.locator('[data-testid^="files-upload-metadata-item-"]')
  const renderedCount = await metaItems.count()
  if (renderedCount !== items.length) {
    throw new Error(
      `uploadViaDialog: dialog rendered ${renderedCount} metadata rows, expected ${items.length}`,
    )
  }
  for (let i = 0; i < items.length; i += 1) {
    const it = items[i]
    const row = metaItems.nth(i)
    const categorySelect = row.locator('[data-testid^="files-upload-meta-category-"]')
    await categorySelect.selectOption(it.category ?? 'other')
    if (it.title) {
      const titleInput = row.locator('[data-testid^="files-upload-meta-title-"]')
      await titleInput.fill(it.title)
    }
  }
  await recorder.takeScreenshot(`${screenshotPrefix}-02-metadata`)

  // --- Step 3 (progress): click Start, wait for one POST per file
  // (the upload mutation issues them sequentially). ----------------
  const responses = items.map(() =>
    page.waitForResponse(
      (resp) => resp.url().includes('/uploads/file') && resp.request().method() === 'POST',
      { timeout: 30_000 },
    ),
  )
  await page.getByTestId('files-upload-start').click()
  const settled = await Promise.all(responses)
  for (const resp of settled) {
    expect(resp.status(), `upload POST status for ${resp.url()}`).toBe(201)
  }

  // --- Close: the close button is gated on every item reaching a
  // terminal state (`done` or `failed`); waitFor enabled, click. ---
  const closeBtn = page.getByTestId('files-upload-close')
  await expect(closeBtn).toBeEnabled({ timeout: 30_000 })
  await closeBtn.click()
  await dialog.waitFor({ state: 'hidden', timeout: 10_000 })
  await recorder.takeScreenshot(`${screenshotPrefix}-03-closed`)
}

/**
 * Wait for the entity-files panel to render the expected number of
 * file cards. The panel renders three skeleton cards while loading,
 * an empty paragraph when there are no files, and the grid only
 * once `useFiles` resolves with rows — so we explicitly wait for the
 * grid testid before counting.
 */
export async function expectEntityFilesPanelCount(page: Page, expected: number): Promise<void> {
  if (expected === 0) {
    await expect(page.getByTestId('entity-files-panel-empty')).toBeVisible({ timeout: 15_000 })
    return
  }
  await expect(page.getByTestId('entity-files-panel-grid')).toBeVisible({ timeout: 15_000 })
  await expect(
    page
      .getByTestId('entity-files-panel-grid')
      .locator('[data-testid^="file-card-"]'),
  ).toHaveCount(expected, { timeout: 15_000 })
}

/**
 * Click the first file card in the entity-files panel. The card's
 * onClick navigates to `/g/<slug>/files/<id>` which mounts the
 * FileDetailSheet on the global Files page. Returns the navigated-to
 * file id so callers can assert against it later.
 */
export async function openFirstFileFromEntityPanel(page: Page): Promise<string> {
  const grid = page.getByTestId('entity-files-panel-grid')
  await grid.waitFor({ state: 'visible', timeout: 15_000 })
  const firstOpenButton = grid.locator('[data-testid^="file-card-open-"]').first()
  // The testid pattern is `file-card-open-<id>`; pull the id straight
  // out of the attribute so we don't need to wait on URL parsing
  // before clicking.
  const testId = await firstOpenButton.getAttribute('data-testid')
  if (!testId) {
    throw new Error('openFirstFileFromEntityPanel: first card has no data-testid')
  }
  const id = testId.replace(/^file-card-open-/, '')
  await firstOpenButton.click()
  await page.getByTestId('file-detail-sheet').waitFor({ state: 'visible', timeout: 15_000 })
  await expect(page).toHaveURL(new RegExp(`/files/${id}(?:[/?#]|$)`))
  return id
}

/**
 * Assert the FileDetailSheet shows a usable signed download link
 * (href is set on the anchor). Fetching the URL is OK as a smoke
 * check — the BE returns 200 (or 206 with a Range header).
 */
export async function assertSheetDownloadable(
  page: Page,
  recorder: TestRecorder,
  fileId: string,
): Promise<void> {
  const sheet = page.getByTestId('file-detail-sheet')
  await expect(sheet).toBeVisible()
  const downloadLink = sheet.getByTestId('file-detail-download')
  await expect(downloadLink).toBeVisible()
  const href = await downloadLink.getAttribute('href')
  if (!href || href === '#') {
    throw new Error(`assertSheetDownloadable: file-detail-download href missing for ${fileId}`)
  }
  // Smoke-check the signed URL with a Range header so we don't pull
  // the whole file. Both 200 (full content) and 206 (partial) are OK.
  const resp = await page.request.get(href, { headers: { Range: 'bytes=0-0' } })
  expect([200, 206]).toContain(resp.status())
  await recorder.takeScreenshot(`download-verified-${fileId}`)
}

/**
 * Delete the currently-open file via the FileDetailSheet's Delete
 * action and confirm via the shared useConfirm dialog. Closes the
 * sheet on success (the React mutation triggers `onOpenChange(false)`,
 * which clears the route param and the sheet unmounts).
 */
export async function deleteFileFromSheet(
  page: Page,
  recorder: TestRecorder,
  screenshotPrefix = 'delete',
): Promise<void> {
  const sheet = page.getByTestId('file-detail-sheet')
  await expect(sheet).toBeVisible()
  await sheet.getByTestId('file-detail-delete').click()
  await page.getByTestId('confirm-dialog').waitFor({ state: 'visible', timeout: 5_000 })
  await recorder.takeScreenshot(`${screenshotPrefix}-confirm`)
  await page.getByTestId('confirm-accept').click()
  await page.getByTestId('confirm-dialog').waitFor({ state: 'hidden', timeout: 15_000 })
  // The mutation onSuccess closes the sheet by calling onOpenChange(false)
  // which navigates to /files and unmounts the sheet.
  await sheet.waitFor({ state: 'hidden', timeout: 15_000 })
  await recorder.takeScreenshot(`${screenshotPrefix}-done`)
}

function mimeForExt(ext: string): string {
  const lower = ext.toLowerCase()
  switch (lower) {
    case '.jpg':
    case '.jpeg':
      return 'image/jpeg'
    case '.png':
      return 'image/png'
    case '.gif':
      return 'image/gif'
    case '.pdf':
      return 'application/pdf'
    default:
      return 'application/octet-stream'
  }
}
