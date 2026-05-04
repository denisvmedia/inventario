import path from 'node:path'

import { expect } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { createLocation, deleteLocation } from './includes/locations.js'
import { TO_LOCATIONS, navigateTo } from './includes/navigate.js'
import {
  assertSheetDownloadable,
  deleteFileFromSheet,
  expectEntityFilesPanelCount,
  openFirstFileFromEntityPanel,
  uploadViaDialog,
} from './includes/uploads.js'

// Post-cutover (#1423) + post-#1448 quick-attach: the Vue location
// detail had two upload sections (`.location-images` + `.location-files`);
// the React LocationDetailPage hosts a single `EntityFilesPanel`
// (read-only grid) with an attach button that opens the shared
// upload dialog. Category is per-file. Detail / download / delete go
// through the global FileDetailSheet via `/g/<slug>/files/<id>`.
test.describe('Location quick-attach', () => {
  const timestamp = Date.now()
  const testLocation = {
    name: `Test Location for File Uploads ${timestamp}`,
    address: '42 Upload Test Street, Test City',
  }

  const imageFixture = path.join('files', 'image.jpg')
  const manualFixture = path.join('files', 'manual.pdf')

  test('attach image + document, browse, download, delete one', async ({ page, recorder }) => {
    let step = 1
    recorder.log(`Step ${step++}: creating location`)
    await navigateTo(page, recorder, TO_LOCATIONS)
    const locationId = await createLocation(page, recorder, testLocation)

    // Click the title link on the just-created card to land on the
    // detail page. The card hosts no dedicated `view` button — the
    // title <Link> inside the CardHeader is the canonical entry, and
    // it's the same one a user clicks.
    recorder.log(`Step ${step++}: navigating to location detail`)
    const card = page.locator(`[data-testid="location-card"][data-location-id="${locationId}"]`)
    await card.locator('a').first().click()
    await expect(page.getByTestId('page-location-detail')).toBeVisible()
    await expect(page.getByTestId('entity-files-panel')).toBeVisible()
    await expect(page.getByTestId('entity-files-panel-empty')).toBeVisible()

    recorder.log(`Step ${step++}: opening attach dialog + uploading 2 files`)
    await page.getByTestId('entity-files-panel-attach').click()
    await uploadViaDialog(
      page,
      recorder,
      [
        {
          fixturePath: imageFixture,
          uploadName: `e2e-loc-photo-${timestamp}.jpg`,
          category: 'photos',
        },
        {
          fixturePath: manualFixture,
          uploadName: `e2e-loc-doc-${timestamp}.pdf`,
          category: 'documents',
        },
      ],
      'location-attach',
    )

    recorder.log(`Step ${step++}: verifying entity panel shows 2 cards`)
    await expectEntityFilesPanelCount(page, 2)

    recorder.log(`Step ${step++}: opening file detail sheet`)
    const fileId = await openFirstFileFromEntityPanel(page)
    const sheet = page.getByTestId('file-detail-sheet')
    await expect(sheet.getByTestId('file-detail-filename')).toBeVisible()
    await expect(sheet.getByTestId('file-detail-category')).toBeVisible()

    recorder.log(`Step ${step++}: verifying signed download for ${fileId}`)
    await assertSheetDownloadable(page, recorder, fileId)

    recorder.log(`Step ${step++}: deleting one file`)
    await deleteFileFromSheet(page, recorder, 'location-attach-delete')

    // Return to the location detail and confirm the panel count
    // dropped to 1. Uses the same in-app navigation as the user.
    recorder.log(`Step ${step++}: verifying panel count dropped to 1`)
    await navigateTo(page, recorder, TO_LOCATIONS)
    await page
      .locator(`[data-testid="location-card"][data-location-id="${locationId}"]`)
      .locator('a')
      .first()
      .click()
    await expect(page.getByTestId('page-location-detail')).toBeVisible()
    await expectEntityFilesPanelCount(page, 1)

    recorder.log(`Step ${step++}: cleanup — deleting location (cascades file removal)`)
    await navigateTo(page, recorder, TO_LOCATIONS)
    await deleteLocation(page, recorder, testLocation.name, locationId)
  })
})
