import path from 'node:path'

import { expect } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { createArea, deleteArea, verifyAreaHasCommodities } from './includes/areas.js'
import {
  BACK_TO_AREAS,
  createCommodity,
  deleteCommodity,
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
  expectCommodityFilesCount,
  getCommodityFileIds,
  uploadViaDialog,
} from './includes/uploads.js'

// Post-cutover (#1423) + post-#1448 quick-attach: cascade tests for the
// unified file model. Two scenarios still worth covering at the e2e
// level:
//   1. Commodity delete cascades to every linked file row.
//   2. Export delete cascades to its generated XML file row.
// The Vue-era "multiple files" subtest is folded into #1 (we already
// upload two files); the "no-files commodity delete" subtest is
// dropped — it duplicates `commodity-simple-crud.spec.ts`.
//
// Replaces the legacy spec retired by #1474.
test.describe('File deletion cascade', () => {
  const timestamp = Date.now()
  const testLocation = {
    name: `Test Location for File Deletion ${timestamp}`,
    address: '123 File Deletion Test Street, Test City',
  }
  const testArea = { name: `Test Area for File Deletion ${timestamp}` }
  const testCommodity = {
    name: `Test Commodity for File Deletion ${timestamp}`,
    shortName: 'TestFileDel',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0],
    status: 'In Use',
    // Required on step 1 (Basics) — the form's area_id default is
    // empty and step transition silently fails validation otherwise.
    areaName: testArea.name,
  }

  const imageFixture = path.join('files', 'image.jpg')
  const manualFixture = path.join('files', 'manual.pdf')

  test('commodity delete removes its linked files (DB + UI)', async ({ page, recorder }) => {
    let step = 1
    recorder.log(`Step ${step++}: creating location/area/commodity`)
    await navigateTo(page, recorder, TO_LOCATIONS)
    await createLocation(page, recorder, testLocation)
    await createArea(page, recorder, testArea)
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name)
    await verifyAreaHasCommodities(page, recorder)
    const commodityUrl = await createCommodity(page, recorder, testCommodity)

    recorder.log(`Step ${step++}: opening Files tab + uploading 2 files`)
    await page.getByTestId('commodity-detail-tab-files').click()
    await page.getByTestId('commodity-files-upload-zone').click()
    await uploadViaDialog(
      page,
      recorder,
      [
        {
          fixturePath: imageFixture,
          uploadName: `e2e-cascade-photo-${timestamp}.jpg`,
          category: 'images',
        },
        {
          fixturePath: manualFixture,
          uploadName: `e2e-cascade-doc-${timestamp}.pdf`,
          category: 'documents',
        },
      ],
      'cascade-attach',
    )
    await expectCommodityFilesCount(page, 2)

    // Capture the file ids straight off the rendered set. We need the
    // BE ids to assert on /files/<id> after the cascade.
    // CommodityFilesTab splits photos into the grid and the rest into
    // the non-photo list, so we collect both.
    recorder.log(`Step ${step++}: capturing file ids from commodity Files tab`)
    const fileIds = await getCommodityFileIds(page)
    expect(fileIds.length).toBe(2)

    // Sanity: each file is reachable via API before the delete. This
    // separates "row gone" failures from "we never had the row in
    // the first place" — the assertion message is part of the spec's
    // diagnostic value.
    recorder.log(`Step ${step++}: verifying files exist via API before delete`)
    const auth = await extractApiAuth(page)
    const apiBase = await groupApiBase(page)
    const headers = {
      Authorization: `Bearer ${auth.accessToken}`,
      Accept: 'application/vnd.api+json',
    }
    for (const fileId of fileIds) {
      const resp = await page.request.get(`${apiBase}/files/${fileId}`, { headers })
      expect(resp.status(), `pre-delete GET /files/${fileId}`).toBe(200)
    }

    recorder.log(`Step ${step++}: deleting the commodity`)
    await page.goto(commodityUrl)
    await deleteCommodity(page, recorder, testCommodity.name, BACK_TO_AREAS)

    // After the cascade settles each file row is 404. Loop with a
    // small retry so a slow BE cascade doesn't false-positive — but
    // the BE should handle this synchronously, so a 5s budget per
    // file is plenty.
    recorder.log(`Step ${step++}: verifying files cascaded to 404`)
    for (const fileId of fileIds) {
      let lastStatus = 200
      for (let attempt = 0; attempt < 10; attempt += 1) {
        const resp = await page.request.get(`${apiBase}/files/${fileId}`, { headers })
        lastStatus = resp.status()
        if (lastStatus === 404) break
        await page.waitForTimeout(500)
      }
      expect(lastStatus, `post-delete GET /files/${fileId} should be 404`).toBe(404)
    }

    recorder.log(`Step ${step++}: cleanup — area + location`)
    await navigateTo(page, recorder, TO_LOCATIONS)
    await deleteArea(page, recorder, testArea.name, testLocation.name)
    await deleteLocation(page, recorder, testLocation.name)
  })

  test('export delete removes its generated file (DB)', async ({ page, recorder }) => {
    // The export wizard UI is exercised by `exports-react.spec.ts`;
    // here we only care about the cascade, so we drive the BE
    // directly via the API. This keeps the spec focused on the
    // /files row that gets reaped when the export goes away.
    let step = 1
    recorder.log(`Step ${step++}: navigating to land an authed page (for token + slug)`)
    await navigateTo(page, recorder, TO_LOCATIONS)

    const auth = await extractApiAuth(page)
    const apiBase = await groupApiBase(page)
    const writeHeaders = {
      'Content-Type': 'application/vnd.api+json',
      Accept: 'application/vnd.api+json',
      Authorization: `Bearer ${auth.accessToken}`,
      'X-CSRF-Token': auth.csrfToken,
    }
    const readHeaders = {
      Authorization: `Bearer ${auth.accessToken}`,
      Accept: 'application/vnd.api+json',
    }

    recorder.log(`Step ${step++}: POST /exports (full_database, include_file_data=false)`)
    const createResp = await page.request.post(`${apiBase}/exports`, {
      headers: writeHeaders,
      data: {
        data: {
          type: 'exports',
          attributes: {
            type: 'full_database',
            description: `Cascade Test Export ${timestamp}`,
            include_file_data: false,
          },
        },
      },
    })
    expect(createResp.status(), 'POST /exports').toBe(201)
    const createBody = await createResp.json()
    const exportId = createBody?.data?.id as string | undefined
    expect(exportId, 'POST /exports response missing data.id').toBeTruthy()

    // Poll until status flips to completed and a file_id has been
    // attached. The export worker writes ~immediately for an empty /
    // minimal dataset; we cap the wait at 30s under CI load.
    recorder.log(`Step ${step++}: waiting for export ${exportId} → completed`)
    let fileId: string | undefined
    for (let attempt = 0; attempt < 30; attempt += 1) {
      const resp = await page.request.get(`${apiBase}/exports/${exportId}`, {
        headers: readHeaders,
      })
      expect(resp.status(), `GET /exports/${exportId}`).toBe(200)
      const body = await resp.json()
      const status = body?.data?.attributes?.status as string | undefined
      const candidateFileId = body?.data?.attributes?.file_id as string | undefined
      if (status === 'completed' && candidateFileId) {
        fileId = candidateFileId
        break
      }
      if (status === 'failed') {
        throw new Error(`Export ${exportId} failed before producing a file_id`)
      }
      await page.waitForTimeout(1000)
    }
    expect(fileId, 'export never reached completed state').toBeTruthy()

    recorder.log(`Step ${step++}: verifying export file ${fileId} exists pre-delete`)
    const preResp = await page.request.get(`${apiBase}/files/${fileId}`, { headers: readHeaders })
    expect(preResp.status()).toBe(200)

    recorder.log(`Step ${step++}: DELETE /exports/${exportId}`)
    const delResp = await page.request.delete(`${apiBase}/exports/${exportId}`, {
      headers: writeHeaders,
    })
    expect([200, 204]).toContain(delResp.status())

    recorder.log(`Step ${step++}: verifying export file ${fileId} cascaded to 404`)
    let lastStatus = 200
    for (let attempt = 0; attempt < 10; attempt += 1) {
      const resp = await page.request.get(`${apiBase}/files/${fileId}`, { headers: readHeaders })
      lastStatus = resp.status()
      if (lastStatus === 404) break
      await page.waitForTimeout(500)
    }
    expect(lastStatus, `post-delete GET /files/${fileId}`).toBe(404)
  })
})
