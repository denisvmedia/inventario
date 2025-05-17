import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

test.describe('Application Navigation', () => {
  test('should load the home page', async ({ page }) => {
    // Verify the home page loaded correctly
    await expect(page.locator('h1')).toContainText('Welcome to Inventario');
    
    // Verify navigation elements are present
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.locator('nav')).toContainText('Home');
    await expect(page.locator('nav')).toContainText('Locations');
    await expect(page.locator('nav')).toContainText('Commodities');
    await expect(page.locator('nav')).toContainText('Settings');
    
    // Verify navigation cards are present
    await expect(page.locator('.navigation-cards')).toBeVisible();
    await expect(page.locator('.navigation-cards .card')).toHaveCount(3);
  });

  test('should navigate to locations page', async ({ page }) => {
    // Click on the Locations link in the navigation
    await page.click('nav >> text=Locations');
    
    // Verify we're on the locations page
    await expect(page).toHaveURL(/\/locations/);
    await expect(page.locator('h1')).toContainText('Locations');
  });

  test('should navigate to commodities page', async ({ page }) => {
    // Click on the Commodities link in the navigation
    await page.click('nav >> text=Commodities');
    
    // Verify we're on the commodities page
    await expect(page).toHaveURL(/\/commodities/);
    await expect(page.locator('h1')).toContainText('Commodities');
  });

  test('should navigate to settings page', async ({ page }) => {
    // Click on the Settings link in the navigation
    await page.click('nav >> text=Settings');
    
    // Verify we're on the settings page
    await expect(page).toHaveURL(/\/settings/);
    await expect(page.locator('h1')).toContainText('Settings');
  });
});
