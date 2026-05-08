import path from 'node:path'
import fs from 'node:fs'

import { expect } from '@playwright/test'

import { test } from '../fixtures/app-fixture.js'
import { createArea, verifyAreaHasCommodities } from './includes/areas.js'
import { createCommodity } from './includes/commodities.js'
import { extractApiAuth } from './includes/commodities-api.js'
import { groupApiBase } from './includes/group-url.js'
import { createLocation } from './includes/locations.js'
import {
  FROM_LOCATIONS_AREA,
  TO_AREA_COMMODITIES,
  TO_LOCATIONS,
  navigateTo,
} from './includes/navigate.js'
import { uploadViaDialog } from './includes/uploads.js'

// Storage usage endpoint smoke test (#1388). Drives the upload UX once
// to give the group some files, then asserts the API returns the byte
// total back via the freshly-mounted /storage-usage endpoint.
test.describe('Storage usage', () => {
  const timestamp = Date.now()
  const testLocation = {
    name: `Test Location for Storage ${timestamp}`,
    address: '123 Storage Test Street, Test City',
  }
  const testArea = { name: `Test Area for Storage ${timestamp}` }
  const testCommodity = {
    name: `Test Commodity for Storage ${timestamp}`,
    shortName: 'TestStorage',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0],
    status: 'In Use',
    areaName: testArea.name,
  }
  const imageFixture = path.join('files', 'image.jpg')
  const imageFixtureAbsPath = path.join(
    new URL('../fixtures', import.meta.url).pathname,
    imageFixture,
  )

  test('upload increments used_bytes returned by /storage-usage', async ({ page, recorder }) => {
    let step = 1

    // Establish baseline storage usage. The shared fixture (other specs
    // in the same group) may have left rows behind, so we compare deltas
    // rather than absolute totals.
    recorder.log(`Step ${step++}: navigate, capture baseline /storage-usage`)
    await navigateTo(page, recorder, TO_LOCATIONS)
    const auth = await extractApiAuth(page)
    const apiBase = await groupApiBase(page)
    const headers = {
      Authorization: `Bearer ${auth.accessToken}`,
      Accept: 'application/json',
    }

    const baselineResp = await page.request.get(`${apiBase}/storage-usage`, { headers })
    expect(baselineResp.status(), 'baseline GET /storage-usage').toBe(200)
    const baseline = await baselineResp.json()
    expect(baseline).toMatchObject({
      breakdown: expect.objectContaining({
        photos: expect.any(Number),
        invoices: expect.any(Number),
        documents: expect.any(Number),
        other: expect.any(Number),
        exports: expect.any(Number),
      }),
      used_bytes: expect.any(Number),
    })
    expect(typeof baseline.quota_bytes === 'number' || baseline.quota_bytes === null).toBe(true)

    recorder.log(`Step ${step++}: create location/area/commodity`)
    await createLocation(page, recorder, testLocation)
    await createArea(page, recorder, testArea)
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name)
    await verifyAreaHasCommodities(page, recorder)
    await createCommodity(page, recorder, testCommodity)

    recorder.log(`Step ${step++}: open Files tab + upload one image`)
    await page.getByTestId('commodity-detail-tab-files').click()
    await page.getByTestId('entity-files-panel-attach').click()
    const uploadName = `e2e-storage-photo-${timestamp}.jpg`
    await uploadViaDialog(
      page,
      recorder,
      [
        {
          fixturePath: imageFixture,
          uploadName,
          category: 'photos',
        },
      ],
      'storage-attach',
    )

    // Read the fixture's actual byte size — the BE captures bytes
    // written via io.Copy on the upload pipeline, which equals the
    // file's on-disk size (the upload is unmodified before storage).
    const expectedSize = fs.statSync(imageFixtureAbsPath).size
    expect(expectedSize, 'fixture image.jpg should be non-empty').toBeGreaterThan(0)

    recorder.log(`Step ${step++}: poll /storage-usage for the increment`)
    // The upload completes synchronously so a single fetch is normally
    // enough; allow a brief retry budget for slow CI bookkeeping.
    let after: { used_bytes: number; breakdown: { photos: number } } | undefined
    for (let attempt = 0; attempt < 10; attempt += 1) {
      const resp = await page.request.get(`${apiBase}/storage-usage`, { headers })
      expect(resp.status(), `GET /storage-usage attempt ${attempt}`).toBe(200)
      after = await resp.json()
      if (after && after.used_bytes >= baseline.used_bytes + expectedSize) break
      await page.waitForTimeout(250)
    }
    expect(after, '/storage-usage response after upload').toBeTruthy()
    if (!after) return

    // The new image lands in the photos bucket — both the headline
    // total and the per-bucket counter must move by the upload size.
    expect(after.used_bytes - baseline.used_bytes).toBeGreaterThanOrEqual(expectedSize)
    expect(after.breakdown.photos - baseline.breakdown.photos).toBeGreaterThanOrEqual(expectedSize)
  })
})
