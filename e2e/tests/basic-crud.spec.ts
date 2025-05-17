import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture';

test.describe('Basic CRUD Operations', () => {
  // Test data with timestamps to ensure uniqueness
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location ${timestamp}`,
    address: '123 Test Street, Test City'
  };
  
  const testArea = {
    name: `Test Area ${timestamp}`
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

  test('should create a new location', async ({ page, recorder }) => {
    // Navigate to locations page
    await page.goto('/locations');
    await recorder.takeScreenshot('locations-before-create');
    
    // Click the New button to show the location form
    await page.click('button:has-text("New")');
    
    // Fill in the location form
    await page.fill('#name', testLocation.name);
    await page.fill('#address', testLocation.address);
    await recorder.takeScreenshot('location-form-filled');
    
    // Submit the form
    await page.click('button:has-text("Create Location")');
    
    // Wait for the location to be created and displayed
    await page.waitForSelector(`.location-card:has-text("${testLocation.name}")`);
    await recorder.takeScreenshot('location-created');
    
    // Verify the new location is displayed
    const locationCard = page.locator(`.location-card:has-text("${testLocation.name}")`);
    await expect(locationCard).toBeVisible();
    await expect(locationCard).toContainText(testLocation.address);
  });

  test('should create a new area in a location', async ({ page, recorder }) => {
    // Navigate to locations page
    await page.goto('/locations');
    
    // Find the test location we created
    const locationCard = page.locator(`.location-card:has-text("${testLocation.name}")`);
    await expect(locationCard).toBeVisible();
    
    // Click on the location to go to its detail page
    await locationCard.click();
    
    // Verify we're on the location detail page
    await expect(page.locator('h1')).toContainText(testLocation.name);
    await recorder.takeScreenshot('location-detail-before-area');
    
    // Click the "Add Area" button
    await page.click('button:has-text("Add Area")');
    
    // Fill in the area form
    await page.fill('#name', testArea.name);
    await recorder.takeScreenshot('area-form-filled');
    
    // Submit the form
    await page.click('button:has-text("Create Area")');
    
    // Wait for the area to be created and displayed
    await page.waitForSelector(`.area-card:has-text("${testArea.name}")`);
    await recorder.takeScreenshot('area-created');
    
    // Verify the new area is displayed
    const areaCard = page.locator(`.area-card:has-text("${testArea.name}")`);
    await expect(areaCard).toBeVisible();
  });

  test('should create a new commodity', async ({ page, recorder }) => {
    // Navigate to commodities page
    await page.goto('/commodities');
    await recorder.takeScreenshot('commodities-before-create');
    
    // Click the New button to show the commodity form
    await page.click('button:has-text("New")');
    
    // Fill in the commodity form
    await page.fill('#name', testCommodity.name);
    await page.fill('#shortName', testCommodity.shortName);
    
    // Select type from dropdown
    await page.click('.p-dropdown[id="type"]');
    await page.click(`.p-dropdown-item:has-text("${testCommodity.type}")`);
    
    // Select area
    await page.click('.p-dropdown[id="areaId"]');
    // Find and select our test area (which should be under our test location)
    const areaOption = page.locator(`.p-dropdown-item:has-text("${testArea.name}")`);
    if (await areaOption.isVisible()) {
      await areaOption.click();
    } else {
      // If not visible, just select the first area
      await page.click('.p-dropdown-item:first-child');
    }
    
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
    
    // Verify the commodity details are displayed correctly
    await expect(page.locator('h1')).toContainText(testCommodity.name);
    await expect(page.locator('.commodity-short-name')).toContainText(testCommodity.shortName);
    await expect(page.locator('.commodity-type')).toContainText(testCommodity.type);
    await expect(page.locator('.commodity-count')).toContainText(testCommodity.count.toString());
    await expect(page.locator('.commodity-price')).toContainText(testCommodity.originalPrice.toString());
  });
});
