// filepath: d:\Work\coding\projects\buster\inventario\e2e\tests\file-uploads.spec.ts
import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import path from 'path';
import {createLocation} from "./includes/locations.js";
import {createArea, verifyAreaHasCommodities} from "./includes/areas.js";
import {createCommodity, verifyCommodityDetails} from "./includes/commodities.js";
import {FROM_LOCATIONS_AREA, navitateTo, TO_AREA_COMMODITIES, TO_LOCATIONS} from "./includes/navigate.js";
import {deleteFile, downloadFile, uploadFile} from "./includes/uploads.js";

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
    let step = 1;

    // STEP 1: CREATE LOCATION - First create a location
    console.log(`Step ${step++}: Creating a new location`);
    await navitateTo(page, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA - Create a new area
    console.log(`Step ${step++}: Creating a new area`);
    await createArea(page, recorder, testArea)

    // STEP 3: CREATE COMMODITY - Create a new commodity
    console.log(`Step ${step++}: Creating a new commodity`);
    await navitateTo(page, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    await createCommodity(page, recorder, testCommodity);

    // STEP 4: READ - Verify the commodity details
    console.log(`Step ${step++}: Verifying the commodity details`);
    await verifyCommodityDetails(page, testCommodity);

    // STEP 5: UPLOAD IMAGE - Upload an image to the commodity
    console.log(`Step ${step++}: Uploading an image`);
    await uploadFile(page, recorder, '.commodity-images', testImagePath);

    // STEP 6: UPLOAD MANUAL - Upload a manual to the commodity
    console.log(`Step ${step++}: Uploading a manual`);
    await uploadFile(page, recorder, '.commodity-manuals', testManualPath);

    // STEP 7: UPLOAD INVOICE - Upload an invoice to the commodity
    console.log(`Step ${step++}: Uploading an invoice`);
    await uploadFile(page, recorder, '.commodity-invoices', testInvoicePath);

    // STEP 8: Check file properties by looking at displayed information
    console.log(`Step ${step++}: Testing file properties dialog`);

    // For each file type, test file details view
    for (const { selector, fileType } of [
      { selector: '.commodity-images', fileType: 'image' },
      { selector: '.commodity-manuals', fileType: 'manual' },
      { selector: '.commodity-invoices', fileType: 'invoice' }
    ]) {
      console.log(`Testing file details for ${fileType}`);

      // Get the file item
      const fileItem = page.locator(`${selector} .file-item`).first();
      await expect(fileItem).toBeVisible();

      // Click the details/info button
      await fileItem.locator('.file-actions button.btn-info').click();
      await recorder.takeScreenshot(`${fileType}-details-dialog`);

      // Verify the dialog is displayed
      const detailsDialog = page.locator('.file-details-modal');
      await expect(detailsDialog).toBeVisible();

      // Verify file name and original name are displayed
      await expect(detailsDialog.locator('.file-name')).toBeVisible();
      await expect(detailsDialog.locator('.file-original-name')).toBeVisible();

      // Verify appropriate preview is shown based on file type
      if (fileType === 'image') {
        // For images, verify an actual image is displayed
        await expect(detailsDialog.locator('.image-preview img')).toBeVisible();
      } else {
        // For PDFs, verify the PDF icon is displayed
        await expect(detailsDialog.locator('.file-icon-preview .fa-file-pdf')).toBeVisible();
      }

      // Close the dialog
      await detailsDialog.locator('button.action-close').click();
      await expect(detailsDialog).not.toBeVisible();

      await recorder.takeScreenshot(`${fileType}-details-closed`);
    }

    // Wait to ensure all uploads are processed and displayed
    await page.waitForTimeout(1000);

    // Verify files are visible in the UI
    for (const selector of ['.commodity-images', '.commodity-manuals', '.commodity-invoices']) {
      await expect(page.locator(`${selector} .file-item`)).toBeVisible();
    }

    // STEP 9: TEST FILE DOWNLOAD - Verify that files can be downloaded
    console.log(`Step ${step++}: Testing file downloads`);

  // For each file type, test downloads
    for (const { selector, fileType } of [
      { selector: '.commodity-images', fileType: 'image' },
      { selector: '.commodity-manuals', fileType: 'manual' },
      { selector: '.commodity-invoices', fileType: 'invoice' }
    ]) {
      console.log(`Testing download for ${fileType}`);
      await downloadFile(page, recorder, selector, fileType);
    }

    // STEP 10: TEST PDF VIEWER - Verify that PDFs can be viewed
    console.log(`Step ${step++}: Testing PDF viewer`);

    // STEP 11: TEST Image viewer - Verify that images can be viewed
    console.log(`Step ${step++}: Testing image viewer`);

    // STEP 12: CLEANUP - Delete the test image, manual, and invoice
    console.log(`Step ${step++}: Cleaning up - deleting the test files`);
    // For each file type, delete files
    for (const { selector, fileType } of [
      { selector: '.commodity-images', fileType: 'image' },
      { selector: '.commodity-manuals', fileType: 'manual' },
      { selector: '.commodity-invoices', fileType: 'invoice' }
    ]) {
      console.log(`Deleting ${fileType}`);
      await deleteFile(page, recorder, selector, fileType);
    }
  });
});
