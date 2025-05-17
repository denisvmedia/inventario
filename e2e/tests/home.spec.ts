import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

test.describe('Home Page', () => {
  test('should display total inventory value', async ({ page }) => {
    // Navigate to home page
    await page.goto('/');
    
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
    // Navigate to home page
    await page.goto('/');
    
    // Click on the Locations card
    await page.locator('.navigation-cards .card', { hasText: 'Locations' }).click();
    
    // Verify we're on the locations page
    await expect(page).toHaveURL(/\/locations/);
    await expect(page.locator('h1')).toContainText('Locations');
  });

  test('should navigate to commodities when clicking the commodities card', async ({ page }) => {
    // Navigate to home page
    await page.goto('/');
    
    // Click on the Commodities card
    await page.locator('.navigation-cards .card', { hasText: 'Commodities' }).click();
    
    // Verify we're on the commodities page
    await expect(page).toHaveURL(/\/commodities/);
    await expect(page.locator('h1')).toContainText('Commodities');
  });

  test('should navigate to settings when clicking the settings card', async ({ page }) => {
    // Navigate to home page
    await page.goto('/');
    
    // Click on the Settings card
    await page.locator('.navigation-cards .card', { hasText: 'Settings' }).click();
    
    // Verify we're on the settings page
    await expect(page).toHaveURL(/\/settings/);
    await expect(page.locator('h1')).toContainText('Settings');
  });
});
