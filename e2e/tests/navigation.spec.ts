import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import { checkSettingsRequired } from './includes/settings-check';

test.describe('Application Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await checkSettingsRequired(page);
  });

  test('should load the home page', async ({ page }) => {
    // Verify the home page loaded correctly
    await expect(page.locator('h1')).toContainText('Welcome to Inventario');
    
    // Verify navigation elements are present
    await expect(page.locator('nav')).toBeVisible();
    await expect(page.locator('nav')).toContainText('Home');
    await expect(page.locator('nav')).toContainText('Locations');
    await expect(page.locator('nav')).toContainText('Commodities');
    await expect(page.locator('nav')).toContainText('System');
    
    // Verify navigation cards are present
    await expect(page.locator('.navigation-cards')).toBeVisible();
    await expect(page.locator('.navigation-cards .card')).toHaveCount(4);
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

  test('should navigate to system page', async ({ page }) => {
    // Click on the System link in the navigation
    await page.click('nav >> text=System');

    // Verify we're on the system page
    await expect(page).toHaveURL(/\/system/);
    await expect(page.locator('h1')).toContainText('System');
  });
});
