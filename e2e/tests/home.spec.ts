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

  test('renders the four hero stat cards (#1544 item 2)', async ({ page }) => {
    // The mock-aligned hero grid: Total Items, Active Warranties,
    // Expired Warranties, Est. Total Value. Locations / Areas / Files /
    // Avg Value cards were dropped in favour of warranty framing — see
    // Dashboard.tsx layout comment.
    await expect(page.locator('[data-testid="dashboard-commodities-count"]')).toBeVisible();
    await expect(page.locator('[data-testid="dashboard-active-warranties"]')).toBeVisible();
    await expect(page.locator('[data-testid="dashboard-expired-warranties"]')).toBeVisible();
    await expect(page.locator('[data-testid="dashboard-total-value"]')).toBeVisible();
  });

  test('hero stat cards drill into the matching list pages (#1390)', async ({ page }) => {
    // Click each hero card and assert the resulting URL pathname +
    // query — the cards must land the user on the surface the issue
    // promises (items → /commodities, warranties → /warranties with
    // the right tab pre-selected, value → /commodities). The list
    // pages own their own e2e specs; this test only pins the routing
    // contract.
    const startUrl = page.url();
    const groupPath = new URL(startUrl).pathname.replace(/\/?$/, "");

    await page.locator('[data-testid="dashboard-commodities-count"]').click();
    await expect(page).toHaveURL(`${groupPath}/commodities`);

    await page.goto(startUrl);
    await page.locator('[data-testid="dashboard-active-warranties"]').click();
    await expect(page).toHaveURL(`${groupPath}/warranties?tab=active`);

    await page.goto(startUrl);
    await page.locator('[data-testid="dashboard-expired-warranties"]').click();
    await expect(page).toHaveURL(`${groupPath}/warranties?tab=expired`);

    await page.goto(startUrl);
    await page.locator('[data-testid="dashboard-total-value"]').click();
    await expect(page).toHaveURL(`${groupPath}/commodities`);
  });

});
