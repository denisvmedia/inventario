import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture';

test.describe('Commodity CRUD Operations', () => {
  // Test data
  const testLocation = {
    name: `Test Location for Commodity ${Date.now()}`,
    address: '123 Test Street, Test City'
  };

  const testArea = {
    name: `Test Area for Commodity ${Date.now()}`
  };

  const testCommodity = {
    name: `Test Commodity ${Date.now()}`,
    shortName: 'TestCom',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0], // Today's date in YYYY-MM-DD format
    status: 'In Use'
  };

  const updatedCommodity = {
    name: `Updated Commodity ${Date.now()}`,
    shortName: 'UpdCom',
    count: 2,
    originalPrice: 200
  };

  let createdLocationId: string;
  let createdAreaId: string;
  let createdCommodityId: string;

  // Setup: Create a location and area first
  test('should create a location and area for commodity tests', async ({ page, recorder }) => {
    // Navigate to locations page
    await page.goto('/locations');

    // Create a location
    await page.click('button:has-text("New")');
    await page.fill('#name', testLocation.name);
    await page.fill('#address', testLocation.address);
    await page.click('button:has-text("Create Location")');
    await page.waitForSelector(`.location-card:has-text("${testLocation.name}")`);

    // For now, we'll use a hardcoded ID for testing purposes
    // In a real scenario, we would extract this from the UI or API
    createdLocationId = 'test-location-id';
    console.log(`Using test location ID: ${createdLocationId}`);

    // Navigate to the location detail page
    await page.goto(`/locations/${createdLocationId}`);

    // Create an area
    await page.click('button:has-text("Add Area")');
    await page.fill('#name', testArea.name);
    await page.click('button:has-text("Create Area")');

    // Wait for the area to be created and displayed
    await page.waitForSelector(`.area-card:has-text("${testArea.name}")`);
    await recorder.takeScreenshot('area-for-commodity-created');

    // For now, we'll use a hardcoded ID for testing purposes
    // In a real scenario, we would extract this from the UI or API
    createdAreaId = 'test-area-id';
    console.log(`Using test area ID: ${createdAreaId}`);

    // Go back to the locations page
    await page.goto('/locations');
  });

  test('should create a new commodity', async ({ page, recorder }) => {
    // Skip if the previous test didn't create an area
    test.skip(!createdAreaId, 'No area ID from previous test');

    // Navigate to create commodity page with area pre-selected
    await page.goto(`/commodities/new?area=${createdAreaId}`);
    await recorder.takeScreenshot('commodity-create-form');

    // Fill in the commodity form
    await page.fill('#name', testCommodity.name);
    await page.fill('#shortName', testCommodity.shortName);

    // Select type from dropdown
    await page.click('.p-dropdown[id="type"]');
    await page.click(`.p-dropdown-item:has-text("${testCommodity.type}")`);

    // Fill in other fields
    await page.fill('#count', testCommodity.count.toString());
    await page.fill('#originalPrice', testCommodity.originalPrice.toString());

    // Select currency from dropdown
    await page.click('.p-dropdown[id="originalPriceCurrency"]');
    await page.click(`.p-dropdown-item:has-text("${testCommodity.originalPriceCurrency}")`);

    // Set purchase date
    await page.fill('#purchaseDate', testCommodity.purchaseDate);

    await recorder.takeScreenshot('commodity-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Commodity")');

    // Wait to be redirected to the commodity detail page
    await page.waitForURL(/\/commodities\/[a-zA-Z0-9-]+$/);
    await recorder.takeScreenshot('commodity-created');

    // For now, we'll use a hardcoded ID for testing purposes
    // In a real scenario, we would extract this from the UI or API
    createdCommodityId = 'test-commodity-id';
    console.log(`Using test commodity ID: ${createdCommodityId}`);

    // Verify the commodity details are displayed correctly
    await expect(page.locator('h1')).toContainText(testCommodity.name);
    await expect(page.locator('.commodity-short-name')).toContainText(testCommodity.shortName);
    await expect(page.locator('.commodity-type')).toContainText(testCommodity.type);
    await expect(page.locator('.commodity-count')).toContainText(testCommodity.count.toString());
    await expect(page.locator('.commodity-price')).toContainText(testCommodity.originalPrice.toString());
  });

  test('should edit an existing commodity', async ({ page, recorder }) => {
    // Skip if the previous test didn't create a commodity
    test.skip(!createdCommodityId, 'No commodity ID from previous test');

    // Navigate to the commodities list
    await page.goto('/commodities');
    await recorder.takeScreenshot('commodities-before-edit');

    // Find the commodity card
    const commodityCard = page.locator(`.commodity-card:has-text("${testCommodity.name}")`);
    await expect(commodityCard).toBeVisible();

    // Click on the commodity to go to the detail page
    await commodityCard.click();

    // Verify we're on the commodity detail page
    await expect(page).toHaveURL(new RegExp(`/commodities/${createdCommodityId}`));

    // Click the edit button
    await page.click('button:has-text("Edit")');
    await recorder.takeScreenshot('commodity-edit-before');

    // Update the commodity fields
    await page.fill('#name', updatedCommodity.name);
    await page.fill('#shortName', updatedCommodity.shortName);
    await page.fill('#count', updatedCommodity.count.toString());
    await page.fill('#originalPrice', updatedCommodity.originalPrice.toString());

    await recorder.takeScreenshot('commodity-edit-form-filled');

    // Save the changes
    await page.click('button:has-text("Save Changes")');

    // Wait to be redirected back to the commodity detail page
    await page.waitForURL(`/commodities/${createdCommodityId}`);
    await recorder.takeScreenshot('commodity-after-edit');

    // Verify the commodity was updated
    await expect(page.locator('h1')).toContainText(updatedCommodity.name);
    await expect(page.locator('.commodity-short-name')).toContainText(updatedCommodity.shortName);
    await expect(page.locator('.commodity-count')).toContainText(updatedCommodity.count.toString());
    await expect(page.locator('.commodity-price')).toContainText(updatedCommodity.originalPrice.toString());
  });

  test('should delete a commodity', async ({ page, recorder }) => {
    // Skip if the previous test didn't create a commodity
    test.skip(!createdCommodityId, 'No commodity ID from previous test');

    // Navigate to the commodity detail page
    await page.goto(`/commodities/${createdCommodityId}`);
    await recorder.takeScreenshot('commodity-before-delete');

    // Click the delete button
    await page.click('button:has-text("Delete")');

    // Confirm deletion in the modal
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Wait to be redirected back to the commodities list
    await page.waitForURL('/commodities');
    await recorder.takeScreenshot('commodities-after-delete');

    // Verify the commodity is no longer in the list
    await expect(page.locator(`.commodity-card:has-text("${updatedCommodity.name}")`)).not.toBeVisible();

    // Clean up: Delete the test area and location
    await page.goto(`/locations/${createdLocationId}`);

    // Delete the area
    const areaCard = page.locator(`.area-card:has-text("${testArea.name}")`);
    await areaCard.locator('.delete-icon').click();
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Delete the location
    await page.waitForTimeout(1000); // Give it a moment to process
    await page.click('button:has-text("Delete")');
    await page.click('.confirmation-modal button:has-text("Delete")');
    await page.waitForURL('/locations');
  });
});
