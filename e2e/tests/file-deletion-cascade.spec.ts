// filepath: e2e/tests/file-deletion-cascade.spec.ts
import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import path from 'path';
import { createLocation, deleteLocation } from "./includes/locations.js";
import { createArea, deleteArea, verifyAreaHasCommodities } from "./includes/areas.js";
import { createCommodity, deleteCommodity, BACK_TO_AREAS } from "./includes/commodities.js";
import { createExport, deleteExport } from "./includes/exports.js";
import { FROM_LOCATIONS_AREA, navigateTo, TO_AREA_COMMODITIES, TO_LOCATIONS, TO_EXPORTS } from "./includes/navigate.js";
import { uploadFile } from "./includes/uploads.js";

test.describe('File Deletion Cascade Tests', () => {
  // Test data with timestamps to ensure uniqueness
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location for File Deletion ${timestamp}`,
    address: '123 File Deletion Test Street, Test City'
  };

  const testArea = {
    name: `Test Area for File Deletion ${timestamp}`
  };

  const testCommodity = {
    name: `Test Commodity for File Deletion ${timestamp}`,
    shortName: 'TestFileDel',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0], // Today's date in YYYY-MM-DD format
    status: 'In Use'
  };

  const testExport = {
    description: `Test Export for File Deletion ${timestamp}`,
    type: 'selected_items',
    includeFileData: false
  };

  // File paths for test uploads
  const testImagePath = path.join('fixtures', 'files', 'image.jpg');
  const testManualPath = path.join('fixtures', 'files', 'manual.pdf');
  const testInvoicePath = path.join('fixtures', 'files', 'invoice.pdf');

  test('should delete commodity files when commodity is deleted', async ({ page, recorder }) => {
    let step = 1;

    // STEP 1: CREATE LOCATION
    console.log(`Step ${step++}: Creating a new location`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA
    console.log(`Step ${step++}: Creating a new area`);
    await createArea(page, recorder, testArea);

    // STEP 3: CREATE COMMODITY
    console.log(`Step ${step++}: Creating a new commodity`);
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    const commodityUrl = await createCommodity(page, recorder, testCommodity);

    // STEP 4: UPLOAD FILES TO COMMODITY
    console.log(`Step ${step++}: Uploading files to commodity`);
    
    // Upload image
    await uploadFile(page, recorder, '.commodity-images', testImagePath);
    
    // Upload manual
    await uploadFile(page, recorder, '.commodity-manuals', testManualPath);
    
    // Upload invoice
    await uploadFile(page, recorder, '.commodity-invoices', testInvoicePath);

    // STEP 5: COLLECT FILE URLS AND IDS BEFORE DELETION
    console.log(`Step ${step++}: Collecting file URLs and IDs before deletion`);

    // Get file URLs and IDs by clicking on file items and capturing the data
    const fileUrls: string[] = [];
    const fileIds: string[] = [];

    // Collect image file URL and ID
    const imageFileItem = page.locator('.commodity-images .file-item').first();
    await expect(imageFileItem).toBeVisible();
    const imageFileId = await imageFileItem.getAttribute('data-file-id');
    if (imageFileId) {
      fileIds.push(imageFileId);
      console.log(`Image file ID: ${imageFileId}`);
    }
    await imageFileItem.click();
    await page.waitForSelector('.file-modal');
    const imageUrl = page.url();
    fileUrls.push(imageUrl);
    console.log(`Image file URL: ${imageUrl}`);
    await page.click('.file-modal .btn-secondary'); // Close modal
    await expect(page.locator('.file-modal')).toBeHidden();

    // Collect manual file URL and ID
    const manualFileItem = page.locator('.commodity-manuals .file-item').first();
    await expect(manualFileItem).toBeVisible();
    const manualFileId = await manualFileItem.getAttribute('data-file-id');
    if (manualFileId) {
      fileIds.push(manualFileId);
      console.log(`Manual file ID: ${manualFileId}`);
    }
    await manualFileItem.click();
    await page.waitForSelector('.file-modal');
    const manualUrl = page.url();
    fileUrls.push(manualUrl);
    console.log(`Manual file URL: ${manualUrl}`);
    await page.click('.file-modal .btn-secondary'); // Close modal
    await expect(page.locator('.file-modal')).toBeHidden();

    // Collect invoice file URL and ID
    const invoiceFileItem = page.locator('.commodity-invoices .file-item').first();
    await expect(invoiceFileItem).toBeVisible();
    const invoiceFileId = await invoiceFileItem.getAttribute('data-file-id');
    if (invoiceFileId) {
      fileIds.push(invoiceFileId);
      console.log(`Invoice file ID: ${invoiceFileId}`);
    }
    await invoiceFileItem.click();
    await page.waitForSelector('.file-modal');
    const invoiceUrl = page.url();
    fileUrls.push(invoiceUrl);
    console.log(`Invoice file URL: ${invoiceUrl}`);
    await page.click('.file-modal .btn-secondary'); // Close modal
    await expect(page.locator('.file-modal')).toBeHidden();

    // STEP 5.5: VERIFY FILE ENTITIES EXIST BEFORE DELETION
    console.log(`Step ${step++}: Verifying file entities exist before deletion`);

    // Get authentication token from localStorage for API requests
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token'));

    for (let i = 0; i < fileIds.length; i++) {
      const fileId = fileIds[i];
      console.log(`Verifying file entity ${i + 1} exists: ${fileId}`);

      // Check via API with authentication
      const apiResponse = await page.request.get(`/api/v1/files/${fileId}`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Accept': 'application/vnd.api+json'
        }
      });
      expect(apiResponse.status()).toBe(200);

      // Also check via UI - navigate to file detail page
      await page.goto(`/files/${fileId}`);
      await expect(page.locator('.breadcrumb-link')).toContainText('Back to Files');
      console.log(`File entity ${i + 1} confirmed to exist`);
    }

    await recorder.takeScreenshot('commodity-files-before-deletion');

    // STEP 6: DELETE COMMODITY
    console.log(`Step ${step++}: Deleting commodity`);
    await page.goto(commodityUrl);
    await deleteCommodity(page, recorder, testCommodity.name, BACK_TO_AREAS);

    // STEP 7: VERIFY FILES ARE NO LONGER ACCESSIBLE
    console.log(`Step ${step++}: Verifying files are no longer accessible`);

    for (let i = 0; i < fileUrls.length; i++) {
      const fileUrl = fileUrls[i];
      console.log(`Testing file URL ${i + 1}: ${fileUrl}`);

      // Navigate to the file URL
      await page.goto(fileUrl);

      // Verify that the file is no longer accessible (should show 404)
      await page.waitForSelector('.resource-not-found')
      console.log(`File ${i + 1} is no longer accessible`);
    }

    // STEP 8: VERIFY FILE ENTITIES ARE DELETED FROM DATABASE
    console.log(`Step ${step++}: Verifying file entities are deleted from database`);

    for (let i = 0; i < fileIds.length; i++) {
      const fileId = fileIds[i];
      console.log(`Testing file entity ${i + 1} deletion: ${fileId}`);

      // Check via API - should return 404
      const apiResponse = await page.request.get(`/api/v1/files/${fileId}`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Accept': 'application/vnd.api+json'
        }
      });
      expect(apiResponse.status()).toBe(404);
      console.log(`File entity ${i + 1} API returns 404: ${apiResponse.status()}`);

      // Check via UI - should show error or redirect
      await page.goto(`/files/${fileId}`);

      // Verify that the file is no longer accessible (should show 404)
      await page.waitForSelector('.resource-not-found')
      console.log(`File entity ${i + 1} is no longer accessible via UI`);
    }

    await recorder.takeScreenshot('commodity-files-after-deletion-verified');

    // STEP 9: CLEANUP
    console.log(`Step ${step++}: Cleaning up - deleting area and location`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await deleteArea(page, recorder, testArea.name, testLocation.name);
    await deleteLocation(page, recorder, testLocation.name);
  });

  test('should delete export files when export is deleted', async ({ page, recorder }) => {
    let step = 1;

    // Get authentication token for API requests
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token'));

    // STEP 1: CREATE LOCATION
    console.log(`Step ${step++}: Creating a new location`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA
    console.log(`Step ${step++}: Creating a new area`);
    await createArea(page, recorder, testArea);

    // STEP 3: CREATE COMMODITY
    console.log(`Step ${step++}: Creating a new commodity`);
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    const commodityUrl = await createCommodity(page, recorder, testCommodity);

    // STEP 1: CREATE EXPORT
    console.log(`Step ${step++}: Creating a new export`);
    await navigateTo(page, recorder, TO_EXPORTS);
    await createExport(page, recorder, testExport, testLocation.name, testArea.name, testCommodity.name);

    // STEP 2: WAIT FOR EXPORT TO COMPLETE AND GET FILE INFO
    console.log(`Step ${step++}: Waiting for export to complete and getting file info`);

    // Wait for export to be completed
    await page.waitForSelector('.status-badge.export-status--completed', { timeout: 30000 });
    await recorder.takeScreenshot('export-completed');

    // Click on the export to go to detail view
    await page.click(`text=${testExport.description}`);
    await page.waitForSelector('h1:has-text("Export Details")');

    // Get the export file URL by checking the download link
    const downloadButton = page.locator('button:has-text("Download")').first();
    await expect(downloadButton).toBeVisible();

    // Get the export ID from the URL or page content
    const currentUrl = page.url();
    const exportId = currentUrl.split('/').pop();
    console.log(`Export ID: ${exportId}`);

    // Construct the expected file URL (this may vary based on implementation)
    const exportFileUrl = `/api/v1/exports/${exportId}/download`;
    console.log(`Export file URL: ${exportFileUrl}`);

    // STEP 2.5: GET EXPORT FILE ENTITY ID
    console.log(`Step ${step++}: Getting export file entity ID`);

    // Get export details via API to find the file_id
    const exportResponse = await page.request.get(`/api/v1/exports/${exportId}`, {
      headers: {
        'Authorization': `Bearer ${authToken}`,
        'Accept': 'application/vnd.api+json'
      }
    });
    expect(exportResponse.status()).toBe(200);
    const exportData = await exportResponse.json();
    const fileId = exportData.data.attributes.file_id;
    console.log(`Export file entity ID: ${fileId}`);

    // Verify the file entity exists before deletion
    if (fileId) {
      const fileResponse = await page.request.get(`/api/v1/files/${fileId}`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Accept': 'application/vnd.api+json'
        }
      });
      expect(fileResponse.status()).toBe(200);
      console.log(`Export file entity confirmed to exist: ${fileId}`);

      // Also check via UI
      await page.goto(`/files/${fileId}`);
      await expect(page.locator('h1')).toContainText('Export: Test Export for File Deletion');
      console.log(`Export file entity accessible via UI: ${fileId}`);
    }

    await recorder.takeScreenshot('export-before-deletion');

    // STEP 4: DELETE EXPORT
    console.log(`Step ${step++}: Deleting export`);
    await page.goto(`/exports/${exportId}`);
    await deleteExport(page, recorder, testExport.description);

    // STEP 5: VERIFY EXPORT FILE IS NO LONGER ACCESSIBLE
    console.log(`Step ${step++}: Verifying export file is no longer accessible`);

    // Try to access the export file URL directly
    const response = await page.request.get(exportFileUrl);

    // The file should not be accessible (404 or other error status)
    expect(response.status()).not.toBe(200);
    console.log(`Export file is no longer accessible. Status: ${response.status()}`);

    // STEP 6: VERIFY EXPORT FILE ENTITY IS DELETED FROM DATABASE
    console.log(`Step ${step++}: Verifying export file entity is deleted from database`);

    if (fileId) {
      console.log(`Testing export file entity deletion: ${fileId}`);

      // Check via API - should return 404
      const fileApiResponse = await page.request.get(`/api/v1/files/${fileId}`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Accept': 'application/vnd.api+json'
        }
      });
      expect(fileApiResponse.status()).toBe(404);
      console.log(`Export file entity API returns 404: ${fileApiResponse.status()}`);

      // Check via UI - should show error or redirect
      await page.goto(`/files/${fileId}`);

      // Verify that the file is no longer accessible (should show 404)
      await page.waitForSelector('.resource-not-found')
      console.log(`Export file entity is no longer accessible via UI`);
    } else {
      console.log('No file entity ID found, skipping file entity deletion check');
    }

    await recorder.takeScreenshot('export-file-after-deletion-verified');


    // STEP 9: CLEANUP
    console.log(`Step ${step++}: Cleaning up - deleting commodity, area, and location`);
    await page.goto(commodityUrl);
    await deleteCommodity(page, recorder, testCommodity.name, BACK_TO_AREAS);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await deleteArea(page, recorder, testArea.name, testLocation.name);
    await deleteLocation(page, recorder, testLocation.name);
  });

  test('should delete multiple commodity files when commodity with many files is deleted', async ({ page, recorder }) => {
    let step = 1;

    // Get authentication token for API requests
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token'));

    // STEP 1: CREATE LOCATION
    console.log(`Step ${step++}: Creating a new location`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA
    console.log(`Step ${step++}: Creating a new area`);
    await createArea(page, recorder, testArea);

    // STEP 3: CREATE COMMODITY
    console.log(`Step ${step++}: Creating a new commodity`);
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    const commodityUrl = await createCommodity(page, recorder, testCommodity);

    // STEP 4: UPLOAD MULTIPLE FILES OF EACH TYPE
    console.log(`Step ${step++}: Uploading multiple files to commodity`);

    // Upload multiple images
    await uploadFile(page, recorder, '.commodity-images', testImagePath);
    await uploadFile(page, recorder, '.commodity-images', testImagePath);

    // Upload multiple manuals
    await uploadFile(page, recorder, '.commodity-manuals', testManualPath);
    await uploadFile(page, recorder, '.commodity-manuals', testManualPath);

    // Upload multiple invoices
    await uploadFile(page, recorder, '.commodity-invoices', testInvoicePath);
    await uploadFile(page, recorder, '.commodity-invoices', testInvoicePath);

    // STEP 5: VERIFY ALL FILES ARE PRESENT
    console.log(`Step ${step++}: Verifying all files are present`);

    // Count files in each section
    const imageCount = await page.locator('.commodity-images .file-item').count();
    const manualCount = await page.locator('.commodity-manuals .file-item').count();
    const invoiceCount = await page.locator('.commodity-invoices .file-item').count();

    expect(imageCount).toBe(2);
    expect(manualCount).toBe(2);
    expect(invoiceCount).toBe(2);

    console.log(`Found ${imageCount} images, ${manualCount} manuals, ${invoiceCount} invoices`);

    // STEP 6: COLLECT ALL FILE URLS AND IDS
    console.log(`Step ${step++}: Collecting all file URLs and IDs`);

    const allFileUrls: string[] = [];
    const allFileIds: string[] = [];

    // Collect all image file URLs and IDs
    for (let i = 0; i < imageCount; i++) {
      const fileItem = page.locator('.commodity-images .file-item').nth(i);
      const fileId = await fileItem.getAttribute('data-file-id');
      if (fileId) {
        allFileIds.push(fileId);
        console.log(`Image file ${i + 1} ID: ${fileId}`);
      }
      await fileItem.click();
      await page.waitForSelector('.file-modal');
      const fileUrl = page.url();
      allFileUrls.push(fileUrl);
      console.log(`Image file ${i + 1} URL: ${fileUrl}`);
      await page.click('.file-modal .btn-secondary');
      await expect(page.locator('.file-modal')).toBeHidden();
    }

    // Collect all manual file URLs and IDs
    for (let i = 0; i < manualCount; i++) {
      const fileItem = page.locator('.commodity-manuals .file-item').nth(i);
      const fileId = await fileItem.getAttribute('data-file-id');
      if (fileId) {
        allFileIds.push(fileId);
        console.log(`Manual file ${i + 1} ID: ${fileId}`);
      }
      await fileItem.click();
      await page.waitForSelector('.file-modal');
      const fileUrl = page.url();
      allFileUrls.push(fileUrl);
      console.log(`Manual file ${i + 1} URL: ${fileUrl}`);
      await page.click('.file-modal .btn-secondary');
      await expect(page.locator('.file-modal')).toBeHidden();
    }

    // Collect all invoice file URLs and IDs
    for (let i = 0; i < invoiceCount; i++) {
      const fileItem = page.locator('.commodity-invoices .file-item').nth(i);
      const fileId = await fileItem.getAttribute('data-file-id');
      if (fileId) {
        allFileIds.push(fileId);
        console.log(`Invoice file ${i + 1} ID: ${fileId}`);
      }
      await fileItem.click();
      await page.waitForSelector('.file-modal');
      const fileUrl = page.url();
      allFileUrls.push(fileUrl);
      console.log(`Invoice file ${i + 1} URL: ${fileUrl}`);
      await page.click('.file-modal .btn-secondary');
      await expect(page.locator('.file-modal')).toBeHidden();
    }

    console.log(`Collected ${allFileUrls.length} file URLs and ${allFileIds.length} file IDs total`);

    // STEP 6.5: VERIFY ALL FILE ENTITIES EXIST BEFORE DELETION
    console.log(`Step ${step++}: Verifying all file entities exist before deletion`);

    for (let i = 0; i < allFileIds.length; i++) {
      const fileId = allFileIds[i];
      console.log(`Verifying file entity ${i + 1}/${allFileIds.length} exists: ${fileId}`);

      // Check via API with authentication
      const apiResponse = await page.request.get(`/api/v1/files/${fileId}`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Accept': 'application/vnd.api+json'
        }
      });
      expect(apiResponse.status()).toBe(200);
      console.log(`File entity ${i + 1} confirmed to exist`);
    }

    await recorder.takeScreenshot('commodity-multiple-files-before-deletion');

    // STEP 7: DELETE COMMODITY
    console.log(`Step ${step++}: Deleting commodity with multiple files`);
    await page.goto(commodityUrl);
    await deleteCommodity(page, recorder, testCommodity.name, BACK_TO_AREAS);

    // STEP 8: VERIFY ALL FILES ARE NO LONGER ACCESSIBLE
    console.log(`Step ${step++}: Verifying all files are no longer accessible`);

    for (let i = 0; i < allFileUrls.length; i++) {
      const fileUrl = allFileUrls[i];
      console.log(`Testing file URL ${i + 1}/${allFileUrls.length}: ${fileUrl}`);

      await page.goto(fileUrl);

      // Verify that the file is no longer accessible (should show 404)
      await page.waitForSelector('.resource-not-found')
      console.log(`File ${i + 1} is no longer accessible`);
    }

    // STEP 9: VERIFY ALL FILE ENTITIES ARE DELETED FROM DATABASE
    console.log(`Step ${step++}: Verifying all file entities are deleted from database`);

    for (let i = 0; i < allFileIds.length; i++) {
      const fileId = allFileIds[i];
      console.log(`Testing file entity ${i + 1}/${allFileIds.length} deletion: ${fileId}`);

      // Check via API - should return 404
      const apiResponse = await page.request.get(`/api/v1/files/${fileId}`, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Accept': 'application/vnd.api+json'
        }
      });
      expect(apiResponse.status()).toBe(404);
      console.log(`File entity ${i + 1} API returns 404: ${apiResponse.status()}`);

      // Check via UI - should show error or redirect
      await page.goto(`/files/${fileId}`);

      // Verify that the file is no longer accessible (should show 404)
      await page.waitForSelector('.resource-not-found')
      console.log(`File entity ${i + 1} is no longer accessible via UI`);
    }

    await recorder.takeScreenshot('commodity-multiple-files-after-deletion-verified');

    // STEP 10: CLEANUP
    console.log(`Step ${step++}: Cleaning up - deleting area and location`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await deleteArea(page, recorder, testArea.name, testLocation.name);
    await deleteLocation(page, recorder, testLocation.name);
  });

  test('should handle file deletion when commodity has no files', async ({ page, recorder }) => {
    let step = 1;

    // STEP 1: CREATE LOCATION
    console.log(`Step ${step++}: Creating a new location`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA
    console.log(`Step ${step++}: Creating a new area`);
    await createArea(page, recorder, testArea);

    // STEP 3: CREATE COMMODITY WITHOUT FILES
    console.log(`Step ${step++}: Creating a new commodity without files`);
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    const commodityUrl = await createCommodity(page, recorder, testCommodity);

    // STEP 4: VERIFY NO FILES ARE PRESENT
    console.log(`Step ${step++}: Verifying no files are present`);

    const imageCount = await page.locator('.commodity-images .file-item').count();
    const manualCount = await page.locator('.commodity-manuals .file-item').count();
    const invoiceCount = await page.locator('.commodity-invoices .file-item').count();

    expect(imageCount).toBe(0);
    expect(manualCount).toBe(0);
    expect(invoiceCount).toBe(0);

    console.log(`Confirmed no files present: ${imageCount} images, ${manualCount} manuals, ${invoiceCount} invoices`);
    await recorder.takeScreenshot('commodity-no-files-before-deletion');

    // STEP 5: DELETE COMMODITY (SHOULD WORK WITHOUT ERRORS)
    console.log(`Step ${step++}: Deleting commodity with no files`);
    await page.goto(commodityUrl);
    await deleteCommodity(page, recorder, testCommodity.name, BACK_TO_AREAS);

    await recorder.takeScreenshot('commodity-no-files-after-deletion');

    // STEP 6: CLEANUP
    console.log(`Step ${step++}: Cleaning up - deleting area and location`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await deleteArea(page, recorder, testArea.name, testLocation.name);
    await deleteLocation(page, recorder, testLocation.name);
  });
});
