import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

test.describe('Commodity Simple CRUD Operations', () => {
  // Test data with timestamps to ensure uniqueness
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location for Commodity ${timestamp}`,
    address: '123 Test Street, Test City'
  };

  const testArea = {
    name: `Test Area for Commodity ${timestamp}`
  };

  const testCommodity = {
    name: `Test Commodity ${timestamp}`,
    shortName: 'TestCom',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0], // Today's date in YYYY-MM-DD format
    status: 'In Use'
  };

  const updatedCommodity = {
    name: `Updated Commodity ${timestamp}`,
    shortName: 'UpdCom',
    count: 2,
    originalPrice: 200
  };

  test('should perform full CRUD operations on a commodity', async ({ page, recorder }) => {
    // STEP 1: CREATE LOCATION - First create a location
    console.log('Step 1: Creating a new location');
    await page.goto('/locations');
    await recorder.takeScreenshot('01-locations-before-create');

    // Click the New button to show the location form
    await page.click('button:has-text("New")');

    // Fill in the location form
    await page.fill('#name', testLocation.name);
    await page.fill('#address', testLocation.address);
    await recorder.takeScreenshot('02-location-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Location")');

    // Wait for the location to be created and displayed
    await page.waitForSelector(`.location-card:has-text("${testLocation.name}")`);
    await recorder.takeScreenshot('03-location-created');

    // Click on the location card to expand it
    // await page.click(`.location-card:has-text("${testLocation.name}")`);

    // STEP 2: CREATE AREA - Create a new area in-place in the location list view
    console.log('Step 2: Creating a new area');

    // Click the Add Area button within the expanded location
    await page.click('.areas-header button:has-text("Add Area")');

    // Fill in the area form
    await page.fill('#name', testArea.name);
    await recorder.takeScreenshot('05-area-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Area")');

    // Wait for the area to be created and displayed in-place
    await page.waitForSelector(`.area-card:has-text("${testArea.name}")`);
    await recorder.takeScreenshot('06-area-created');

    // STEP 3: CREATE COMMODITY - Create a new commodity
    console.log('Step 3: Creating a new commodity');
    // Navigate to commodities page
    await page.click(`.area-card:has-text("${testArea.name}")`);

    // Verify we're on the area detail page with no commodities
    await page.waitForSelector('.no-commodities p:has-text("No commodities found in this area.")');
    await recorder.takeScreenshot('07-commodities-before-create');

    // Click the New button to show the commodity form
    await page.click('a:has-text("Add Commodity")');

    // Fill in the commodity form
    await page.waitForTimeout(1000);
    await page.fill('#name', testCommodity.name);
    await page.fill('#shortName', testCommodity.shortName);

    // Select type from dropdown
    await page.click('.p-select[id="type"]');
    await page.click(`.p-select-option-label:has-text("${testCommodity.type}")`);

    // Fill in other fields
    await page.fill('#count', testCommodity.count.toString());
    await page.fill('#originalPrice', testCommodity.originalPrice.toString());

    // Select currency from dropdown
    await page.click('.p-select[id="originalPriceCurrency"]');
    await page.click(`.p-select-option-label:has-text("${testCommodity.originalPriceCurrency}")`);

    // Set purchase date
    await page.fill('#purchaseDate', testCommodity.purchaseDate);

    await recorder.takeScreenshot('08-commodity-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Commodity")');

    // Wait to be redirected to the commodity detail page
    await page.waitForURL(/\/commodities\/[a-zA-Z0-9-]+\?/);
    await recorder.takeScreenshot('09-commodity-created');

    // STEP 4: READ - Verify the commodity details
    console.log('Step 4: Verifying the commodity details');
    // Verify the commodity details are displayed correctly
    await expect(page.locator('h1')).toContainText(testCommodity.name);
    await expect(page.locator('.commodity-short-name')).toContainText(testCommodity.shortName);
    await expect(page.locator('.commodity-type')).toContainText(testCommodity.type);
    await expect(page.locator('.commodity-count')).toContainText(testCommodity.count.toString());
    await expect(page.locator('.commodity-original-price')).toContainText(testCommodity.originalPrice.toString());

    // STEP 5: UPDATE - Edit the commodity
    console.log('Step 5: Editing the commodity');
    // Click the Edit button
    await page.click('button:has-text("Edit")');

    // Verify we're on the edit page
    await expect(page).toHaveURL(/\/commodities\/[a-zA-Z0-9-]+\/edit\?/);
    await recorder.takeScreenshot('10-commodity-edit-form');

    // Update the commodity fields
    await page.fill('#name', updatedCommodity.name);
    await page.fill('#shortName', updatedCommodity.shortName);
    await page.fill('#count', updatedCommodity.count.toString());
    await page.fill('#originalPrice', updatedCommodity.originalPrice.toString());

    await recorder.takeScreenshot('11-commodity-edit-form-filled');

    // Save the changes
    await page.click('button:has-text("Save Commodity")');

    // Wait to be redirected back to the commodity detail page
    await expect(page).toHaveURL(/\/commodities\/[a-zA-Z0-9-]+\?/);
    await recorder.takeScreenshot('12-commodity-after-edit');

    // Verify the commodity was updated
    await expect(page.locator('h1')).toContainText(updatedCommodity.name);
    await expect(page.locator('.commodity-short-name')).toContainText(updatedCommodity.shortName);
    await expect(page.locator('.commodity-count')).toContainText(updatedCommodity.count.toString());
    await expect(page.locator('.commodity-original-price')).toContainText(updatedCommodity.originalPrice.toString());

    // STEP 6: DELETE - Delete the commodity
    console.log('Step 6: Deleting the commodity');
    // Click the Delete button
    await page.click('button:has-text("Delete")');

    // Confirm deletion in the modal
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Verify we're redirected back to the commodities list
    await expect(page).toHaveURL(/\/areas\/[a-zA-Z0-9-]+/);
    await recorder.takeScreenshot('13-commodities-after-delete');

    // Verify the commodity is no longer in the list
    await expect(page.locator(`.commodity-card:has-text("${updatedCommodity.name}")`)).not.toBeVisible();

    // STEP 7: CLEANUP - Delete the area and location
    console.log('Step 7: Cleaning up - deleting the area and location');
    // Navigate back to the location detail page
    await page.click(`.breadcrumb-link:has-text("Back to Locations")`);

    // Wait for the areas section to be visible after location expansion
    await page.waitForSelector('.areas-header');
    await recorder.takeScreenshot('14-location-expanded');

    // Delete the area
    const areaCard = page.locator(`.area-card:has-text("${testArea.name}")`);
    await areaCard.locator('.area-actions button[title="Delete"]').click();
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Delete the location
    await page.click(`.location-card:has-text("${testLocation.name}") button[title="Delete"]`);
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Verify we're redirected back to the locations list
    await expect(page).toHaveURL(/\/locations/);
    await recorder.takeScreenshot('15-locations-after-cleanup');

    // Verify the location is no longer in the list
    await expect(page.locator(`.location-card:has-text("${testLocation.name}")`)).not.toBeVisible();
  });
});
