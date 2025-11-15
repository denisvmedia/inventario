import {test} from '../fixtures/app-fixture.js';
import {expect} from '@playwright/test';
import {createLocation, deleteLocation} from "./includes/locations.js";
import {createArea, deleteArea, verifyAreaHasCommodities} from "./includes/areas.js";
import {createCommodity, deleteCommodity, BACK_TO_COMMODITIES} from "./includes/commodities.js";
import {createExport, deleteExport, verifyExportDetails, verifySelectedItems, downloadExport, downloadExportFromList} from "./includes/exports.js";
import {
  navigateTo,
  TO_LOCATIONS,
  TO_EXPORTS,
  TO_AREA_COMMODITIES,
  FROM_LOCATIONS_AREA,
  TO_COMMODITIES
} from "./includes/navigate.js";

test.describe('Export CRUD Operations', () => {
  // Test data with timestamps to ensure uniqueness
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location for Export ${timestamp}`,
    address: '123 Test Street, Test City'
  };

  const testArea = {
    name: `Test Area for Export ${timestamp}`
  };

  const testCommodity = {
    name: `Test Commodity for Export ${timestamp}`,
    shortName: 'TestExp',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0], // Today's date in YYYY-MM-DD format
    status: 'In Use',
    serialNumber: `SN-${timestamp}`,
    tags: ['test', 'export']
  };

  const testExport = {
    description: `Test Export ${timestamp}`,
    type: 'selected_items',
    includeFileData: false
  };

  test('should create, view, and delete an export', async ({ page, recorder }) => {
    // Create prerequisite entities
    recorder.log('Creating a new location');
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    recorder.log('Creating a new area');
    await createArea(page, recorder, testArea);

    recorder.log('Creating a new commodity');
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    await createCommodity(page, recorder, testCommodity);

    // Navigate to exports
    await navigateTo(page, recorder, TO_EXPORTS);
    await recorder.takeScreenshot('exports-list-01-initial');

    // Create an export
    await createExport(page, recorder, testExport, testLocation.name, testArea.name, testCommodity.name);
    
    // Verify export was created - should be in the export details
    await page.waitForSelector(`text=${testExport.description}`);
    await recorder.takeScreenshot('exports-list-02-after-create');

    // Verify export details
    await verifyExportDetails(page, recorder, testExport);

    // Verify selected items are displayed correctly
    await verifySelectedItems(page, recorder, {
      locationName: testLocation.name,
      areaName: testArea.name,
      commodityName: testCommodity.name
    });

    // Test download functionality from detail view
    recorder.log('Testing export download from detail view');
    const downloadResult = await downloadExport(page, recorder, testExport.description, true);
    recorder.log(`Downloaded file: ${downloadResult.filename} to ${downloadResult.path}`);

    // Test download functionality from list view
    recorder.log('Testing export download from list view');
    const listDownloadResult = await downloadExportFromList(page, recorder, testExport.description);
    recorder.log(`Downloaded file from list: ${listDownloadResult.filename} to ${listDownloadResult.path}`);

    // Navigate back to detail view for cleanup
    await page.click(`text=${testExport.description}`);
    await page.waitForSelector('h1:has-text("Export Details")');

    // Delete the export
    await deleteExport(page, recorder, testExport.description);

    // Cleanup - delete the test entities
    await navigateTo(page, recorder, TO_COMMODITIES);
    // Navigate to commodity detail page
    await page.click(`text=${testCommodity.name}`);
    await deleteCommodity(page, recorder, testCommodity.name, BACK_TO_COMMODITIES);

    // Delete area and location
    await navigateTo(page, recorder, TO_LOCATIONS);
    await deleteArea(page, recorder, testArea.name, testLocation.name);
    await deleteLocation(page, recorder, testLocation.name);
  });

  test('should create a full database export', async ({ page, recorder }) => {
    const fullDatabaseExport = {
      description: `Full Database Export ${timestamp}`,
      type: 'full_database',
      includeFileData: false
    };

    // Navigate to exports
    await navigateTo(page, recorder, TO_EXPORTS);
    await recorder.takeScreenshot('exports-full-01-initial');

    // Create a full database export
    await createExport(page, recorder, fullDatabaseExport);
    
    // Verify export was created
    await page.waitForSelector(`text=${fullDatabaseExport.description}`);
    await recorder.takeScreenshot('exports-full-02-after-create');

    // View export details
    await page.click(`text=${fullDatabaseExport.description}`);
    await page.waitForSelector('h1:has-text("Export Details")');

    // Verify export details and information
    await verifyExportDetails(page, recorder, fullDatabaseExport);

    // Verify it shows as full database type
    await page.waitForSelector('text=Full Database');
    await recorder.takeScreenshot('exports-full-03-details');

    // Test download functionality for full database export
    recorder.log('Testing full database export download from detail view');
    const fullDbDownloadResult = await downloadExport(page, recorder, fullDatabaseExport.description, true);
    recorder.log(`Downloaded full database export: ${fullDbDownloadResult.filename} to ${fullDbDownloadResult.path}`);

    // Test download from list view as well
    recorder.log('Testing full database export download from list view');
    const fullDbListDownloadResult = await downloadExportFromList(page, recorder, fullDatabaseExport.description);
    recorder.log(`Downloaded full database export from list: ${fullDbListDownloadResult.filename} to ${fullDbListDownloadResult.path}`);

    // Navigate back to detail view for cleanup
    await page.click(`text=${fullDatabaseExport.description}`);
    await page.waitForSelector('h1:has-text("Export Details")');

    // Delete the export
    await deleteExport(page, recorder, fullDatabaseExport.description);
  });

  test('should test export download functionality thoroughly', async ({ page, recorder }) => {
    const downloadTestExport = {
      description: `Download Test Export ${timestamp}`,
      type: 'full_database',
      includeFileData: true // Test with file data included
    };

    // Navigate to exports
    await navigateTo(page, recorder, TO_EXPORTS);
    await recorder.takeScreenshot('exports-download-test-01-initial');

    // Create an export specifically for download testing
    await createExport(page, recorder, downloadTestExport);

    // Verify export was created and is completed
    await page.waitForSelector(`text=${downloadTestExport.description}`);
    await page.waitForSelector('.status-badge.export-status--completed');
    await recorder.takeScreenshot('exports-download-test-02-export-ready');

    // Test 1: Download from detail view
    recorder.log('Test 1: Download from export detail view');
    const detailDownload = await downloadExport(page, recorder, downloadTestExport.description, true);

    // Verify the downloaded file has expected characteristics
    expect(detailDownload.filename).toMatch(/\.xml$/);
    expect(detailDownload.path).toBeTruthy();
    recorder.log(`Detail view download successful: ${detailDownload.filename}`);

    // Test 2: Download from list view
    recorder.log('Test 2: Download from export list view');
    const listDownload = await downloadExportFromList(page, recorder, downloadTestExport.description);

    // Verify the downloaded file has expected characteristics
    expect(listDownload.filename).toMatch(/\.xml$/);
    expect(listDownload.path).toBeTruthy();
    recorder.log(`List view download successful: ${listDownload.filename}`);

    // Test 3: Verify download button states
    recorder.log('Test 3: Verify download button availability');

    // Navigate back to detail view
    await page.click(`text=${downloadTestExport.description}`);
    await page.waitForSelector('h1:has-text("Export Details")');

    // Verify both download buttons are present and enabled
    await page.waitForSelector('button:has-text("Download")');
    await page.waitForSelector('button:has-text("Download Export")');

    await recorder.takeScreenshot('exports-download-test-03-buttons-verified');

    // Cleanup - delete the test export
    await deleteExport(page, recorder, downloadTestExport.description);
  });
});
