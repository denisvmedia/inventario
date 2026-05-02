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
    await expect(page.locator('h1')).toContainText('Overview');

    // Verify navigation labels are visible in the shell. The React port
    // uses shadcn's sidebar primitive (no <nav> / role=navigation wrapper),
    // so we scope to the sidebar surface (`data-slot="sidebar"`) to avoid
    // strict-mode dupes against repeated links inside the dashboard
    // StatCards body. Same labels otherwise (Dashboard ⇆ Home etc.).
    const sidebar = page.locator('[data-slot="sidebar"]').first();
    await expect(sidebar).toBeVisible();
    await expect(sidebar.getByRole('link', { name: /^Dashboard$/ })).toBeVisible();
    await expect(sidebar.getByRole('link', { name: /^Locations$/ })).toBeVisible();
    await expect(sidebar.getByRole('link', { name: /^All Items$/ })).toBeVisible();
    await expect(sidebar.getByRole('link', { name: /^Settings$/ })).toBeVisible();

    // Phase 5 rewrote the home dashboard: the old `.navigation-cards`
    // grid was replaced with read-only StatCards. Verify one of the
    // stable hooks renders so the home view is fully painted.
    await expect(page.locator('[data-testid="dashboard-total-value"]')).toBeVisible();
  });

  test('should navigate to locations page', async ({ page }) => {
    // Navigate to locations page with authentication
    await navigateWithAuth(page, '/locations');

    // Verify we're on the locations page
    await expect(page).toHaveURL(/\/locations/);
    await expect(page.locator('h1')).toContainText('Locations');
  });

  test('should navigate to commodities page', async ({ page }) => {
    // Navigate to commodities page with authentication. The React port
    // landed the items list under "Items" copy (`commodities:list.heading`)
    // — same surface, terser label.
    await navigateWithAuth(page, '/commodities');

    // Verify we're on the commodities page
    await expect(page).toHaveURL(/\/commodities/);
    await expect(page.locator('h1')).toContainText('Items');
  });

  test('should navigate to system page', async ({ page }) => {
    // Navigate to system page with authentication
    await navigateWithAuth(page, '/system');

    // Verify we're on the system page
    await expect(page).toHaveURL(/\/system/);
    await expect(page.locator('h1')).toContainText('System');
  });
});
