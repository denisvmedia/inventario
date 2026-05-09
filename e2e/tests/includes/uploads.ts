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

export type FileCategory = 'images' | 'invoices' | 'documents' | 'other'

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

  // --- Step 3 (progress): click Start, then gate on the dialog's
  // own per-item terminal status. The earlier approach of arming N
  // identical `page.waitForResponse` listeners didn't work — they
  // share a predicate, so Promise.all would resolve all N with the
  // *same* first matching response, leaving the remaining uploads
  // unvalidated. The close button only enables once *every* item is
  // `done` or `failed` (UploadFilesDialog's `allDone` gate), so
  // waiting for it gives us "all uploads settled" deterministically.
  // After that, asserting that no progress row ended up
  // `data-status="failed"` covers the per-file success check that
  // the duplicate-listener `expect(status).toBe(201)` was intended
  // to provide.
  await page.getByTestId('files-upload-start').click()
  const closeBtn = page.getByTestId('files-upload-close')
  await expect(closeBtn).toBeEnabled({ timeout: 30_000 })
  const failedItems = page.locator(
    '[data-testid^="files-upload-progress-item-"][data-status="failed"]',
  )
  await expect(failedItems, 'no upload should fail').toHaveCount(0)
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
  // FileCard renders three nested testids per card: the outer Card
  // (`file-card-<id>`), the inner open button (`file-card-open-<id>`),
  // and the optional bulk checkbox (`file-card-checkbox-<id>`). Match
  // only the outer Card via the unique `data-category` attribute it
  // alone carries — counting all three would overcount by 2-3×.
  await expect(
    page
      .getByTestId('entity-files-panel-grid')
      .locator('[data-testid^="file-card-"][data-category]'),
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

// ── Commodity Files tab (#1530 item 3) ──────────────────────────────
// CommodityFilesTab replaces EntityFilesPanel on commodity detail. Its
// surface is structurally different (chip-bar + photo grid + non-photo
// list), so the e2e helpers below mirror the panel-era helpers above
// against the new testid contract. Location detail still uses
// EntityFilesPanel and keeps the originals.

/**
 * Wait for the commodity Files tab to render the expected number of
 * attached files. The new tab splits the loaded set into a photo grid
 * (`commodity-files-photo-grid`) and a non-photo list
 * (`commodity-files-list`); we sum the rendered <li>s in each so the
 * count reflects the active chip's view.
 */
export async function expectCommodityFilesCount(page: Page, expected: number): Promise<void> {
  if (expected === 0) {
    await expect(page.getByTestId('commodity-files-empty')).toBeVisible({ timeout: 15_000 })
    return
  }
  const grid = page.getByTestId('commodity-files-photo-grid')
  const list = page.getByTestId('commodity-files-list')
  await expect(async () => {
    const photos = (await grid.count()) ? await grid.locator('> li').count() : 0
    const rows = (await list.count()) ? await list.locator('> li').count() : 0
    expect(photos + rows, 'commodity files total').toBe(expected)
  }).toPass({ timeout: 15_000, intervals: [100, 250, 500, 1000] })
}

/**
 * Collect every BE file id rendered inside the commodity Files tab —
 * photo-grid and non-photo list combined. Used by cascade tests that
 * need to assert each id 404s post-delete.
 */
export async function getCommodityFileIds(page: Page): Promise<string[]> {
  const photoIds = await page
    .getByTestId('commodity-files-photo-grid')
    .locator(
      '[data-testid^="commodity-files-photo-"]:not([data-testid^="commodity-files-photo-cover-"]):not([data-testid^="commodity-files-photo-delete-"])',
    )
    .evaluateAll((els) =>
      els.map((e) => (e.getAttribute('data-testid') ?? '').replace(/^commodity-files-photo-/, '')),
    )
  const rowIds = await page
    .getByTestId('commodity-files-list')
    .locator('li[data-testid^="commodity-files-row-"]')
    .evaluateAll((els) =>
      els.map((e) => (e.getAttribute('data-testid') ?? '').replace(/^commodity-files-row-/, '')),
    )
  return [...photoIds, ...rowIds].filter(Boolean)
}

/**
 * Open the first PDF attachment in the commodity Files tab. Clicking
 * a row's "Open" CTA mounts the inline `FilePreviewDialog`'s PDF
 * variant (`file-preview-dialog-pdf`). Returns the file id and leaves
 * the dialog open for the caller to assert + close.
 *
 * The image preview branch routes to `ImageViewer`
 * (`file-image-viewer`) and the catch-all branch renders
 * `file-preview-dialog-other` — both are reachable via similar
 * helpers if a future spec needs them.
 */
export async function openFirstCommodityPdf(page: Page): Promise<string> {
  const openBtn = page
    .getByTestId('commodity-files-list')
    .locator('button[data-testid^="commodity-files-row-open-"]')
    .first()
  const tid = await openBtn.getAttribute('data-testid')
  if (!tid) {
    throw new Error('openFirstCommodityPdf: no openable row CTA found')
  }
  const id = tid.replace(/^commodity-files-row-open-/, '')
  await openBtn.click()
  await page.getByTestId('file-preview-dialog-pdf').waitFor({ state: 'visible', timeout: 15_000 })
  return id
}

/**
 * Click the per-row delete affordance on a commodity Files tab entry,
 * accept the destructive useConfirm dialog, and wait for the row to
 * unmount. Works for both photo-grid rows (`commodity-files-photo-…`)
 * and non-photo list rows (`commodity-files-row-…`).
 */
export async function deleteFromCommodityRow(
  page: Page,
  recorder: TestRecorder,
  fileId: string,
  screenshotPrefix = 'delete',
): Promise<void> {
  const photoDelete = page.getByTestId(`commodity-files-photo-delete-${fileId}`)
  const rowDelete = page.getByTestId(`commodity-files-row-delete-${fileId}`)
  const target = (await photoDelete.count()) ? photoDelete : rowDelete
  // The photo-grid delete button is hover-only (`opacity-0
  // group-hover:opacity-100`) so we use force:true; the click target
  // is still wired regardless of opacity.
  await target.click({ force: true })
  await page.getByTestId('confirm-dialog').waitFor({ state: 'visible', timeout: 5_000 })
  await recorder.takeScreenshot(`${screenshotPrefix}-confirm`)
  await page.getByTestId('confirm-accept').click()
  await page.getByTestId('confirm-dialog').waitFor({ state: 'hidden', timeout: 15_000 })
  // Wait for the deleted row's testid to disappear so subsequent
  // count assertions don't race the cache invalidation.
  await expect(photoDelete).toHaveCount(0, { timeout: 15_000 })
  await expect(rowDelete).toHaveCount(0, { timeout: 15_000 })
  await recorder.takeScreenshot(`${screenshotPrefix}-done`)
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
