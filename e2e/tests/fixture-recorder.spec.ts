import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

test.describe('Fixture Recorder Example', () => {
  test('should use the recorder fixture', async ({ page, recorder }) => {
    // Navigate to home page
    await page.goto('/');
    
    // Take a screenshot using the recorder fixture
    await recorder.takeScreenshot('home-page');
    
    // Navigate to locations
    await page.locator('.navigation-cards .card', { hasText: 'Locations' }).click();
    await expect(page).toHaveURL(/\/locations/);
    
    // Take another screenshot
    await recorder.takeScreenshot('locations-page');
    
    // Navigate to commodities
    await page.click('nav >> text=Commodities');
    await expect(page).toHaveURL(/\/commodities/);
    
    // Take a screenshot of the commodities page
    await recorder.takeScreenshot('commodities-page');
    
    // Take a screenshot of the header
    await recorder.takeElementScreenshot('h1', 'commodities-header');
  });
});
