import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import { TestRecorder } from '../utils/test-recorder.js';

test.describe('Test Recorder Example', () => {
  test('should record test execution with screenshots', async ({ page }) => {
    // Create a recorder for this test
    const recorder = new TestRecorder(page, 'Navigation Flow');
    
    // Navigate to home page
    await page.goto('/');
    await recorder.takeScreenshot('home-page');
    
    // Click on the Locations card
    await page.locator('.navigation-cards .card', { hasText: 'Locations' }).click();
    
    // Verify we're on the locations page
    await expect(page).toHaveURL(/\/locations/);
    await recorder.takeScreenshot('locations-page');
    
    // Take a screenshot of the header element
    await recorder.takeElementScreenshot('h1', 'locations-header');
    
    // Click on the Commodities link in the navigation
    await page.click('nav >> text=Commodities');
    
    // Verify we're on the commodities page
    await expect(page).toHaveURL(/\/commodities/);
    await recorder.takeScreenshot('commodities-page');
    
    // Take a screenshot of the commodities list if it exists
    const commoditiesList = page.locator('.commodities-list');
    if (await commoditiesList.isVisible()) {
      await recorder.takeElementScreenshot('.commodities-list', 'commodities-list');
    }
    
    // Navigate to settings
    await page.click('nav >> text=Settings');
    await expect(page).toHaveURL(/\/settings/);
    await recorder.takeScreenshot('settings-page');
  });
});
