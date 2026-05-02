import {expect} from '@playwright/test';
import {test} from '../fixtures/app-fixture.js';
import {navigateTo, TO_HOME} from "./includes/navigate.js";

test.describe('Home Page', () => {
  test.beforeEach(async ({ page, recorder }) => {
    await navigateTo(page, recorder, TO_HOME);
  });

  test('renders the dashboard header and total-value StatCard', async ({ page }) => {
    // Phase 5 rewrote the home dashboard. The legacy `.value-summary` /
    // `.navigation-cards` markup is gone; the new patterns expose
    // stable data-testid hooks instead.
    await expect(page.locator('h1')).toContainText('Overview');

    const totalValue = page.locator('[data-testid="dashboard-total-value"]');
    await expect(totalValue).toBeVisible();

    // The card prints either a numeric value or a skeleton while loading.
    const valueText = await totalValue.textContent();
    expect(valueText?.length).toBeGreaterThan(0);
  });

  test('renders the per-entity StatCards', async ({ page }) => {
    await expect(page.locator('[data-testid="dashboard-locations-count"]')).toBeVisible();
    await expect(page.locator('[data-testid="dashboard-areas-count"]')).toBeVisible();
    await expect(page.locator('[data-testid="dashboard-commodities-count"]')).toBeVisible();
    await expect(page.locator('[data-testid="dashboard-files-count"]')).toBeVisible();
    await expect(page.locator('[data-testid="dashboard-avg-value"]')).toBeVisible();
  });

  test('renders the Value-by-Location and Value-by-Area cards', async ({ page }) => {
    await expect(page.locator('[data-testid="dashboard-value-by-location"]')).toBeVisible();
    await expect(page.locator('[data-testid="dashboard-value-by-area"]')).toBeVisible();
  });
});
