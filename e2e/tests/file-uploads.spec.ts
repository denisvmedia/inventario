import path from 'node:path'

import { expect } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { createArea, deleteArea, verifyAreaHasCommodities } from './includes/areas.js'
import {
  BACK_TO_COMMODITIES,
  createCommodity,
  deleteCommodity,
  verifyCommodityDetails,
} from './includes/commodities.js'
import { extractApiAuth } from './includes/commodities-api.js'
import { groupApiBase } from './includes/group-url.js'
import { createLocation, deleteLocation } from './includes/locations.js'
import {
  FROM_LOCATIONS_AREA,
  TO_AREA_COMMODITIES,
  TO_LOCATIONS,
  navigateTo,
} from './includes/navigate.js'
import {
  deleteFromCommodityRow,
  expectCommodityFilesCount,
  getCommodityFileIds,
  openFirstCommodityPdf,
  uploadViaDialog,
} from './includes/uploads.js'

// Post-cutover (#1423) + post-#1448 quick-attach + post-#1530 commodity
// Files tab redesign: the Vue commodity detail had three
// category-specific upload sections; the React commodity detail Files
// tab now renders the chip-bar + photo grid + non-photo list contract
// from `CommodityFilesTab` with a contextual upload zone that opens
// the shared `UploadFilesDialog`. Category is set per-file inside the
// dialog's metadata step.
//
// Detail/preview/delete now happen in-place via the inline
// `FilePreviewDialog` (image fullscreen viewer for images, PDF canvas
// for PDFs, small metadata + Download dialog for everything else),
// mock parity with `design-mocks/src/components/FilePreviewDialog.tsx`.
// The global `FileDetailSheet` is exercised separately by
// `files.spec.ts`, so this spec stays focused on the commodity flow:
// upload → preview-open → delete via the row affordance → count drops.
//
// Re-enables the spec retired by #1474; the legacy `.commodity-images`
// / `.commodity-manuals` / `.commodity-invoices` flow it tested no
// longer exists.
test.describe('Commodity quick-attach (Files tab)', () => {
  const timestamp = Date.now()

  const testLocation = {
    name: `Test Location for Files ${timestamp}`,
    address: '123 File Test Street, Test City',
  }
  const testArea = { name: `Test Area for Files ${timestamp}` }
  const testCommodity = {
    name: `Test Commodity for Files ${timestamp}`,
    shortName: 'TestFiles',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0],
    status: 'In Use',
    // The React form requires `area_id` on step 1 (Basics) and the
    // dropdown defaults to empty — without an explicit areaName the
    // step 1 → 2 transition validates-fails silently and we hang on
    // step 2's #commodity-purchase-date forever. Pin the parent
    // location too: seeded data adds Home/Office/Storage Unit
    // alphabetically before any test fixture, so picking the "first"
    // location alone would mis-target Home.
    areaName: testArea.name,
    locationName: testLocation.name,
  }

  const imageFixture = path.join('files', 'image.jpg')
  const manualFixture = path.join('files', 'manual.pdf')
  const invoiceFixture = path.join('files', 'invoice.pdf')

  test('attach files of three categories, browse, download, delete one', async ({
    page,
    recorder,
  }) => {
    let step = 1
    recorder.log(`Step ${step++}: creating location`)
    await navigateTo(page, recorder, TO_LOCATIONS)
    await createLocation(page, recorder, testLocation)

    recorder.log(`Step ${step++}: creating area`)
    await createArea(page, recorder, testArea, testLocation.name)

    recorder.log(`Step ${step++}: creating commodity`)
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name)
    await verifyAreaHasCommodities(page, recorder)
    const commodityUrl = await createCommodity(page, recorder, testCommodity)
    await verifyCommodityDetails(page, testCommodity)

    // Open the Files tab — that's where #1530's CommodityFilesTab
    // hosts the chip-bar + upload zone on commodity detail.
    recorder.log(`Step ${step++}: opening Files tab`)
    await page.getByTestId('commodity-detail-tab-files').click()
    await expect(page.getByTestId('commodity-detail-files')).toBeVisible()
    await expect(page.getByTestId('commodity-files-empty')).toBeVisible()

    // Attach three files in one go — the dialog accepts multi-file
    // selection and lets us set per-file category in step 2. This is
    // the same flow the user runs from the Files page upload CTA, so
    // covering it here doubles as smoke for the quick-attach link to
    // the unified dialog.
    recorder.log(`Step ${step++}: opening attach dialog + uploading 3 files`)
    await page.getByTestId('commodity-files-upload-zone').click()
    await uploadViaDialog(
      page,
      recorder,
      [
        {
          fixturePath: imageFixture,
          uploadName: `e2e-files-photo-${timestamp}.jpg`,
          category: 'images',
        },
        {
          fixturePath: manualFixture,
          uploadName: `e2e-files-manual-${timestamp}.pdf`,
          category: 'documents',
        },
        {
          // #1622: the upload UI now offers only images/documents/other;
          // the legacy `invoices` selection was dropped. The fixture
          // is still an invoice — it lands in `documents` and the
          // commodity/invoices linked-entity bucket auto-tags `invoice`
          // on the BE.
          fixturePath: invoiceFixture,
          uploadName: `e2e-files-invoice-${timestamp}.pdf`,
          category: 'documents',
        },
      ],
      'commodity-attach',
    )

    // Tab re-fetches via TanStack Query invalidation after upload.
    recorder.log(`Step ${step++}: verifying tab shows 3 attachments`)
    await expectCommodityFilesCount(page, 3)

    // Capture the BE file ids straight off the rendered DOM so we can
    // assert against /files/<id> via API later. CommodityFilesTab
    // splits photos into the grid and the rest into the non-photo
    // list, so the helper merges both buckets.
    recorder.log(`Step ${step++}: capturing file ids`)
    const fileIds = await getCommodityFileIds(page)
    expect(fileIds.length, 'three uploaded ids should be visible').toBe(3)

    // Sanity-check each row resolves to a 200 with a usable signed
    // URL on the API. The upload flow already asserts the dialog's
    // own progress meter (no failed items), so this is the
    // "everything reachable" smoke check. NB: `GET /files/{id}` is
    // the singular detail endpoint — `jsonapi.FileResponse` renders
    // FLAT at the top level (no `data` wrapper). See
    // `frontend/src/features/files/api.ts::getFile` and the comment
    // on `FileDetailEnvelope` for the contract.
    recorder.log(`Step ${step++}: verifying API metadata + signed download for each file`)
    const auth = await extractApiAuth(page)
    const apiBase = await groupApiBase(page)
    const headers = {
      Authorization: `Bearer ${auth.accessToken}`,
      Accept: 'application/vnd.api+json',
    }
    // State-changing requests below must include the CSRF token; the
    // middleware in go/apiserver/auth.go rejects PUT/PATCH/POST/DELETE
    // without `X-CSRF-Token`. GET stays plain. See
    // e2e/tests/includes/commodities-api.ts:48 for the canonical shape.
    const writeHeaders = { ...headers, 'X-CSRF-Token': auth.csrfToken }
    // #1622 acceptance: locate the invoice fixture by upload-name pattern,
    // then exercise the tag write + tag filter end-to-end. We don't rely
    // on the BE auto-tagging from `linked_entity_meta` because the
    // unified upload dialog (#1411) doesn't pass meta — the invoice
    // semantic flows through the tag-input on step 2 of the dialog or a
    // follow-up PATCH /files/{id}, which is what users actually do.
    let invoiceFileId: string | undefined
    for (const fileId of fileIds) {
      const resp = await page.request.get(`${apiBase}/files/${fileId}`, { headers })
      expect(resp.status(), `GET /files/${fileId}`).toBe(200)
      const body = await resp.json()
      const attrs = body?.attributes ?? {}
      const signed = body?.meta?.signed_urls?.[fileId] ?? null
      const signedUrl: string | undefined =
        typeof signed === 'string' ? signed : (signed?.url ?? signed?.URL)
      const titleStr: string =
        (typeof attrs.title === 'string' && attrs.title) ||
        (typeof attrs.path === 'string' && attrs.path) ||
        (typeof attrs.original_path === 'string' && attrs.original_path) ||
        ''
      expect(titleStr, `metadata for ${fileId}`).toBeTruthy()
      if (titleStr.includes('invoice')) {
        invoiceFileId = fileId
        // Post-#1622 the invoice fixture lands in `documents` (the
        // category dropdown in the dialog was trimmed to three values
        // and the test fixture sets category='documents' explicitly).
        expect(attrs.category, `invoice file ${fileId} category (#1622)`).toBe('documents')
      }
      if (signedUrl) {
        const head = await page.request.get(signedUrl, { headers: { Range: 'bytes=0-0' } })
        expect([200, 206], `signed URL probe for ${fileId}`).toContain(head.status())
      }
    }
    expect(invoiceFileId, 'invoice file should be present in the uploaded set').toBeTruthy()

    // Apply the conventional `invoice` tag via PATCH — same call the FE
    // detail-edit form makes when a user tags a file. Keeps the BE +
    // tag-filter assertions deterministic regardless of which client
    // attached the file.
    // The files API exposes PUT /files/{id} (not PATCH) — see
    // go/apiserver/files.go:646. The request also needs the CSRF
    // header; without it the auth middleware returns 403 before chi
    // matches the route.
    recorder.log(`Step ${step++}: PUT invoice file with tag=invoice`)
    const patchResp = await page.request.put(`${apiBase}/files/${invoiceFileId}`, {
      headers: { ...writeHeaders, 'Content-Type': 'application/vnd.api+json' },
      data: {
        data: {
          type: 'files',
          id: invoiceFileId,
          attributes: {
            tags: ['invoice'],
          },
        },
      },
    })
    expect(patchResp.status(), `PUT /files/${invoiceFileId} (#1622)`).toBeLessThan(400)
    const patchedBody = await patchResp.json()
    const patchedTags = Array.isArray(patchedBody?.attributes?.tags)
      ? (patchedBody.attributes.tags as string[])
      : Array.isArray(patchedBody?.data?.attributes?.tags)
        ? (patchedBody.data.attributes.tags as string[])
        : []
    expect(patchedTags, `invoice file tags after PATCH (#1622)`).toContain('invoice')

    // #1622 acceptance: filtering the global Files list by ?tag=invoice
    // must surface the invoice-tagged file. We hit the BE filter
    // directly (the FE toolbar pill backs the same query) — keeps the
    // assertion deterministic without depending on FilesListPage's
    // load timing.
    recorder.log(`Step ${step++}: verifying ?tag=invoice filter finds the row`)
    const filteredResp = await page.request.get(`${apiBase}/files?tag=invoice`, { headers })
    expect(filteredResp.status(), 'GET /files?tag=invoice').toBe(200)
    const filteredBody = await filteredResp.json()
    const filteredIds = Array.isArray(filteredBody?.data)
      ? (filteredBody.data as Array<{ id?: string }>).map((row) => row.id ?? '').filter(Boolean)
      : []
    expect(filteredIds, 'invoice-tag-filtered list contains the invoice file').toContain(
      invoiceFileId,
    )

    // Open one of the PDFs — clicking a card opens the shared right-side
    // FileDetailSheet *in place* (#1966), exactly like the Files page and
    // the location/area panel (no fullscreen overlay, no navigation away).
    // The PDF preview renders inside the sheet; assert the sheet + its
    // metadata mount, then close it with Escape.
    recorder.log(`Step ${step++}: opening the file detail sheet`)
    const pdfId = await openFirstCommodityPdf(page)
    const previewSheet = page.getByTestId('file-detail-sheet')
    await expect(previewSheet).toBeVisible()
    await expect(previewSheet.getByTestId('file-detail-filename')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(previewSheet).toBeHidden()
    await recorder.takeScreenshot('commodity-preview-closed')

    // Delete one row via the per-row affordance + useConfirm dialog;
    // the row should unmount and the rendered count drops to 2.
    recorder.log(`Step ${step++}: deleting one file`)
    await deleteFromCommodityRow(page, recorder, pdfId, 'commodity-attach-delete')

    recorder.log(`Step ${step++}: verifying tab count dropped to 2`)
    await expectCommodityFilesCount(page, 2)

    // Re-open from the commodity detail URL — the page navigation
    // refetch path is the one the user follows after closing the
    // sheet, so we exercise it explicitly here.
    recorder.log(`Step ${step++}: re-loading commodity URL + Files tab`)
    await page.goto(commodityUrl)
    await page.getByTestId('commodity-detail-tab-files').click()
    await expectCommodityFilesCount(page, 2)

    // Cleanup — the e2e DB is shared, so leaving the test commodity
    // (with two attached files) + the area + the location around
    // would inflate later specs' query results and slow pagination
    // probes. The commodity delete cascades to its linked files via
    // EntityService.DeleteCommodityRecursive, so we don't have to
    // touch the file rows directly.
    recorder.log(`Step ${step++}: cleanup — commodity (cascades 2 remaining files)`)
    await deleteCommodity(page, recorder, testCommodity.name, BACK_TO_COMMODITIES)
    recorder.log(`Step ${step++}: cleanup — area + location`)
    await navigateTo(page, recorder, TO_LOCATIONS)
    await deleteArea(page, recorder, testArea.name, testLocation.name)
    await deleteLocation(page, recorder, testLocation.name)
  })
})
