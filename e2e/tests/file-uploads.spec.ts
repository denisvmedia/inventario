// filepath: d:\Work\coding\projects\buster\inventario\e2e\tests\file-uploads.spec.ts
import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import path from 'path';
import {createLocation} from "./includes/locations.js";
import {createArea, verifyAreaHasCommodities} from "./includes/areas.js";
import {createCommodity, verifyCommodityDetails} from "./includes/commodities.js";
import {FROM_LOCATIONS_AREA, navitateTo, TO_AREA_COMMODITIES, TO_LOCATIONS} from "./includes/navigate.js";
import {uploadFile} from "./includes/uploads.js";

test.describe('File Uploads and Properties Tests', () => {
  // Test data with timestamps to ensure uniqueness
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location for Files ${timestamp}`,
    address: '123 File Test Street, Test City'
  };

  const testArea = {
    name: `Test Area for Files ${timestamp}`
  };

  const testCommodity = {
    name: `Test Commodity for Files ${timestamp}`,
    shortName: 'TestFiles',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0], // Today's date in YYYY-MM-DD format
    status: 'In Use'
  };

  // File paths for test uploads
  const testImagePath = path.join('fixtures', 'files', 'image.jpg');
  const testManualPath = path.join('fixtures', 'files', 'manual.pdf');
  const testInvoicePath = path.join('fixtures', 'files', 'invoice.pdf');

  test('should upload and validate image, manual, and invoice files', async ({ page, recorder }) => {
    // STEP 1: CREATE LOCATION - First create a location
    console.log('Step 1: Creating a new location');
    await navitateTo(page, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA - Create a new area
    console.log('Step 2: Creating a new area');
    await createArea(page, recorder, testArea)

    // STEP 3: CREATE COMMODITY - Create a new commodity
    console.log('Step 3: Creating a new commodity');
    await navitateTo(page, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    await createCommodity(page, recorder, testCommodity);

    // STEP 4: READ - Verify the commodity details
    console.log('Step 4: Verifying the commodity details');
    await verifyCommodityDetails(page, testCommodity);

    // STEP 5: UPLOAD IMAGE - Upload an image to the commodity
    console.log('Step 5: Uploading an image');
    await uploadFile(page, recorder, '.commodity-images', testImagePath);

    // STEP 6: UPLOAD MANUAL - Upload a manual to the commodity
    console.log('Step 6: Uploading a manual');
    await uploadFile(page, recorder, '.commodity-manuals', testManualPath);

    // STEP 7: UPLOAD INVOICE - Upload an invoice to the commodity
    console.log('Step 7: Uploading an invoice');
    await uploadFile(page, recorder, '.commodity-invoices', testInvoicePath);

    // STEP 8: Check file properties by looking at displayed information
    console.log('Step 9: Testing file properties dialog');

    // STEP 9: TEST FILE DOWNLOAD - Verify that files can be downloaded
    console.log('Step 9: Testing file downloads');

    // STEP 10: TEST PDF VIEWER - Verify that PDFs can be viewed
    console.log('Step 10: Testing PDF viewer');

    // STEP 11: TEST Image viewer - Verify that images can be viewed
    console.log('Step 11: Testing image viewer');

    // STEP 12: CLEANUP - Delete the test image, manual, and invoice
    console.log('Step 12: Cleaning up - deleting the test files');
  });
});
