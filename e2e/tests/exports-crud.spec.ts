import {test} from '../fixtures/app-fixture.js';
import {createLocation, deleteLocation} from "./includes/locations.js";
import {createArea, deleteArea, verifyAreaHasCommodities} from "./includes/areas.js";
import {createCommodity, deleteCommodity, BACK_TO_COMMODITIES} from "./includes/commodities.js";
import {createExport, deleteExport, verifyExportDetails, verifySelectedItems} from "./includes/exports.js";
import {
  navigateTo,
  TO_LOCATIONS,
  TO_EXPORTS,
  TO_AREA_COMMODITIES,
  FROM_LOCATIONS_AREA,
  TO_COMMODITIES
} from "./includes/navigate.js";
import {expect} from "@playwright/test";

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
    console.log('Creating a new location');
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    console.log('Creating a new area');
    await createArea(page, recorder, testArea);

    console.log('Creating a new commodity');
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
    await verifyExportInformation(page, recorder, fullDatabaseExport);

    // Verify it shows as full database type
    await page.waitForSelector('text=Full Database');
    await recorder.takeScreenshot('exports-full-03-details');

    // Delete the export
    await deleteExport(page, recorder, fullDatabaseExport.description);
  });

  test('should handle export errors gracefully', async ({ page, recorder }) => {
    const errorExport = {
      description: `Error Test Export ${timestamp}`,
      type: 'selected_items',
      includeFileData: false
    };

    // Navigate to exports
    await navigateTo(page, recorder, TO_EXPORTS);

    // Try to create an export with no selected items (should cause validation error)
    await page.click('a:has-text("New")');
    await page.waitForSelector('h1:has-text("Create Export")');
    
    await page.fill('#description', errorExport.description);
    
    // Select type
    await page.click('.p-select[id="type"]');
    await page.click('.p-select-option-label:has-text("Selected Items")');
    
    // Try to submit without selecting any items
    await page.click('button:has-text("Create Export")');
    
    // Should show validation error
    await page.waitForSelector('text=You must select at least one item');
    await recorder.takeScreenshot('exports-error-01-validation');
  });
});
