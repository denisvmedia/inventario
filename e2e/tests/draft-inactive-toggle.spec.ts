import {expect} from '@playwright/test';
import {test} from '../fixtures/app-fixture.js';
import {navigateTo, TO_COMMODITIES, TO_LOCATIONS} from "./includes/navigate.js";

// React port (#1423) carries `data-testid="commodity-card"` (vs Vue's
// `.commodity-card` class) and exposes the show-inactive control as a
// regular Button (`data-testid="commodities-toggle-inactive"`) rather
// than a `[role="switch"]`. The behaviour the spec guards is the same:
// the active-only filter is on by default and clicking the control
// reveals the rest of the seeded items.
const CARD = '[data-testid="commodity-card"]';
const TOGGLE = '[data-testid="commodities-toggle-inactive"]';

test.describe('Draft and Inactive Items Toggle Functionality', () => {
  test('should toggle visibility of draft and inactive items in Commodity List view', async ({ page, recorder }) => {
    await navigateTo(page, recorder, TO_COMMODITIES);
    await recorder.takeScreenshot('01-commodities-page-initial');

    await page.waitForSelector(CARD);

    const beforeCount = await page.locator(CARD).count();

    await page.locator(TOGGLE).click();
    await recorder.takeScreenshot('02-commodities-with-inactive-shown');
    await page.waitForTimeout(500);

    const afterCount = await page.locator(CARD).count();
    recorder.log(`Commodities count before toggle: ${beforeCount}, after toggle: ${afterCount}`);

    // Seed data has Sold / Lost / Disposed / Written Off / Draft items
    // hidden behind the toggle, so revealing them must grow the list.
    expect(afterCount).toBeGreaterThan(beforeCount);

    await page.locator(TOGGLE).click();
    await recorder.takeScreenshot('03-commodities-with-inactive-hidden');
    await page.waitForTimeout(500);

    const finalCount = await page.locator(CARD).count();
    recorder.log(`Commodities count after hiding inactive: ${finalCount}`);
    expect(finalCount).toEqual(beforeCount);
  });

  test('Area Detail view does not yet host the commodity list (#1410 carryover)', async ({ page, recorder }) => {
    // The React port (#1423) ships a placeholder Area Detail page until
    // the items list lands as part of #1410; the Vue-era version of this
    // test counted `.commodity-card` rows under the area, which doesn't
    // apply yet. We assert the placeholder is what renders, so this stays
    // a meaningful guard: when #1410 reintroduces the embedded items list
    // here, the assertion below will start failing and the test must be
    // updated back to the toggle-and-count flow above.
    await navigateTo(page, recorder, TO_LOCATIONS);

    await page.locator('[data-testid="location-card"]').first().waitFor();
    // The area testid is on the <li> wrapper; the actual nav link is the
    // anchor inside it. Click that to land on the area-detail route.
    const firstAreaLink = page
      .locator('[data-testid="location-card"]')
      .first()
      .locator('[data-testid="location-card-area"] a')
      .first();
    await firstAreaLink.click();

    await expect(page.locator('[data-testid="page-area-detail"]')).toBeVisible();
    await expect(page.locator('[data-testid="area-detail-items-soon"]')).toBeVisible();
    // The list-page commodity cards must NOT be present here while the
    // page is in placeholder mode.
    await expect(page.locator(CARD)).toHaveCount(0);
    await recorder.takeScreenshot('area-detail-placeholder');
  });
});
