import {expect} from '@playwright/test';
import {test} from '../fixtures/app-fixture.js';
import {navigateTo, TO_HOME} from "./includes/navigate.js";

test.describe('Home Page', () => {
  test.beforeEach(async ({ page, recorder }) => {
    await navigateTo(page, recorder, TO_HOME);
  });


  test('should display total inventory value', async ({ page }) => {
    // Wait for the value to load (it's fetched asynchronously)
    await page.waitForSelector('.value-summary', { state: 'visible' });
    
    // Verify the value summary section is visible
    await expect(page.locator('.value-summary')).toBeVisible();
    
    // The value should either be loading, showing a value, or showing "No valued items"
    const hasValue = await page.locator('.value-amount').isVisible();
    const isLoading = await page.locator('.value-loading').isVisible();
    const isEmpty = await page.locator('.value-empty').isVisible();
    
    // One of these states should be true
    expect(hasValue || isLoading || isEmpty).toBeTruthy();
  });

  test('should navigate to locations when clicking the locations card', async ({ page }) => {
    // Click on the Locations card
    await page.locator('.navigation-cards .card', { hasText: 'Locations' }).click();
    
    // Verify we're on the locations page
    await expect(page).toHaveURL(/\/locations/);
    await expect(page.locator('h1')).toContainText('Locations');
  });

  test('should navigate to commodities when clicking the commodities card', async ({ page }) => {
    // Click on the Commodities card
    await page.locator('.navigation-cards .card', { hasText: 'Commodities' }).click();
    
    // Verify we're on the commodities page
    await expect(page).toHaveURL(/\/commodities/);
    await expect(page.locator('h1')).toContainText('Commodities');
  });

  test('should navigate to system when clicking the system card', async ({ page }) => {
    // Click on the System card
    await page.locator('.navigation-cards .card', { hasText: 'System' }).click();

    // Verify we're on the system page
    await expect(page).toHaveURL(/\/system/);
    await expect(page.locator('h1')).toContainText('System');
  });
});
