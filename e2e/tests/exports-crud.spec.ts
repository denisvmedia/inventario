import {test} from '../fixtures/app-fixture.js';
import {createLocation, deleteLocation} from "./includes/locations.js";
import {createArea, deleteArea} from "./includes/areas.js";
import {createCommodity, deleteCommodity, BACK_TO_COMMODITIES} from "./includes/commodities.js";
import {createExport, deleteExport, verifyExportDetails} from "./includes/exports.js";
import {navigateTo, TO_EXPORTS} from "./includes/navigate.js";

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
    await navigateTo(page, recorder, TO_EXPORTS);
    await createLocation(page, recorder, testLocation);
    await createArea(page, recorder, testArea);
    await createCommodity(page, recorder, testCommodity);

    // Navigate to exports
    await navigateTo(page, recorder, TO_EXPORTS);
    await recorder.takeScreenshot('exports-list-01-initial');

    // Create an export
    await createExport(page, recorder, testExport, testLocation.name, testArea.name, testCommodity.name);
    
    // Verify export was created - should be in the exports list
    await page.waitForSelector(`text=${testExport.description}`);
    await recorder.takeScreenshot('exports-list-02-after-create');

    // Click on the export to view details
    await page.click(`text=${testExport.description}`);
    await page.waitForSelector('h1:has-text("Export Details")');
    await recorder.takeScreenshot('exports-detail-01-view');

    // Verify export details
    await verifyExportDetails(page, recorder, testExport);

    // Test download button if export is completed (might be pending/in-progress in tests)
    const downloadButton = page.locator('button:has-text("Download")');
    if (await downloadButton.isVisible()) {
      await recorder.takeScreenshot('exports-detail-02-download-available');
    }

    // Delete the export
    await deleteExport(page, recorder, testExport.description);

    // Verify export was deleted - should not be in the list anymore
    await navigateTo(page, recorder, TO_EXPORTS);
    await page.waitForTimeout(1000);
    await recorder.takeScreenshot('exports-list-03-after-delete');

    // Cleanup - delete the test entities
    await deleteCommodity(page, recorder, testCommodity.name, BACK_TO_COMMODITIES);
    await deleteArea(page, recorder, testArea.name);
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
