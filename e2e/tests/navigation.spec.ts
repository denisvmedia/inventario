import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import { checkSettingsRequired } from './includes/settings-check.js';
import { navigateWithAuth } from './includes/auth.js';

test.describe('Application Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await checkSettingsRequired(page);
  });

  test('should load the home page', async ({ page }) => {
    // Navigate to home page with authentication (required since Phase 4)
    await navigateWithAuth(page, '/');

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
    // Navigate to locations page with authentication
    await navigateWithAuth(page, '/locations');

    // Verify we're on the locations page
    await expect(page).toHaveURL(/\/locations/);
    await expect(page.locator('h1')).toContainText('Locations');
  });

  test('should navigate to commodities page', async ({ page }) => {
    // Navigate to commodities page with authentication
    await navigateWithAuth(page, '/commodities');

    // Verify we're on the commodities page
    await expect(page).toHaveURL(/\/commodities/);
    await expect(page.locator('h1')).toContainText('Commodities');
  });

  test('should navigate to system page', async ({ page }) => {
    // Navigate to system page with authentication
    await navigateWithAuth(page, '/system');

    // Verify we're on the system page
    await expect(page).toHaveURL(/\/system/);
    await expect(page.locator('h1')).toContainText('System');
  });
});
