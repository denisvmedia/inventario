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
import { createLocation, deleteLocation } from './includes/locations.js'
import {
  FROM_LOCATIONS_AREA,
  TO_AREA_COMMODITIES,
  TO_LOCATIONS,
  navigateTo,
} from './includes/navigate.js'
import {
  assertSheetDownloadable,
  deleteFileFromSheet,
  expectCommodityFilesCount,
  openFirstCommodityFile,
  uploadViaDialog,
} from './includes/uploads.js'

// Post-cutover (#1423) + post-#1448 quick-attach + post-#1530 commodity
// Files tab redesign: the Vue commodity detail had three
// category-specific upload sections; the React commodity detail Files
// tab now renders the chip-bar + photo grid + non-photo list contract
// from `CommodityFilesTab` with a contextual upload zone that opens
// the shared `UploadFilesDialog`. Category is set per-file inside the
// dialog's metadata step. Detail/download/delete reuse the global
// `FileDetailSheet` via the `/g/<slug>/files/<id>` deep-link.
//
// Re-enables the spec retired by #1474; the legacy `.commodity-images`
// / `.commodity-manuals` / `.commodity-invoices` flow it tested no
// longer exists. PDF and image-viewer interaction tests from the Vue
// era are intentionally dropped — the React inline preview is
// covered separately by component-level vitest, and the FileDetailSheet
// path is exercised by `files.spec.ts`.
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
    // step 2's #commodity-purchase-date forever.
    areaName: testArea.name,
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
    await createArea(page, recorder, testArea)

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
          category: 'photos',
        },
        {
          fixturePath: manualFixture,
          uploadName: `e2e-files-manual-${timestamp}.pdf`,
          category: 'documents',
        },
        {
          fixturePath: invoiceFixture,
          uploadName: `e2e-files-invoice-${timestamp}.pdf`,
          category: 'invoices',
        },
      ],
      'commodity-attach',
    )

    // Tab re-fetches via TanStack Query invalidation after upload.
    recorder.log(`Step ${step++}: verifying tab shows 3 attachments`)
    await expectCommodityFilesCount(page, 3)

    // Open detail sheet for one file; verify metadata block + signed
    // download URL. The photo click navigates to /g/<slug>/files/<id>
    // and mounts the global FileDetailSheet — same surface the unified
    // /files page uses.
    recorder.log(`Step ${step++}: opening file detail sheet`)
    const fileId = await openFirstCommodityFile(page)
    const sheet = page.getByTestId('file-detail-sheet')
    await expect(sheet.getByTestId('file-detail-filename')).toBeVisible()
    await expect(sheet.getByTestId('file-detail-category')).toBeVisible()

    recorder.log(`Step ${step++}: verifying signed download for ${fileId}`)
    await assertSheetDownloadable(page, recorder, fileId)

    // Delete the file from the sheet; sheet closes on success and
    // navigates back to /files. We then return to the commodity
    // detail and re-open the Files tab to verify the panel count
    // dropped to 2.
    recorder.log(`Step ${step++}: deleting one file`)
    await deleteFileFromSheet(page, recorder, 'commodity-attach-delete')

    recorder.log(`Step ${step++}: verifying tab count dropped to 2`)
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
