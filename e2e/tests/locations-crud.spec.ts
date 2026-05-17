import { expect } from '@playwright/test';

import { test } from '../fixtures/app-fixture.js';
import { createLocation, deleteLocation, editLocationViaDetail } from './includes/locations.js';
import { TO_LOCATIONS, navigateTo } from './includes/navigate.js';

// #1662 regression: the PUT envelope used to omit `data.id`, so the
// BE id-match check at apiserver/locations.go:229 rejected every edit
// with a bare 422 (no body the FE could surface). The dialog also
// silently wiped any inline error when the post-mutation refetch
// landed a fresh `location` prop reference, so the user saw nothing
// on the first failed submit. This spec exercises a full
// create-edit-delete loop and asserts the edit reaches the server
// with a matching id and updates the rendered heading.
test.describe('Locations CRUD', () => {
  test('create → edit → delete a location end-to-end (#1662)', async ({ page, recorder }) => {
    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const initial = {
      name: `E2E Location ${suffix}`,
      address: `1 Original Way, Test City`,
    };
    const renamed = {
      name: `E2E Location ${suffix} (renamed)`,
      address: `2 Updated Way, Test City`,
    };

    let step = 1;
    recorder.log(`Step ${step++}: navigating to /locations`);
    await navigateTo(page, recorder, TO_LOCATIONS);

    recorder.log(`Step ${step++}: creating "${initial.name}"`);
    const locationId = await createLocation(page, recorder, initial);

    recorder.log(`Step ${step++}: editing via detail header`);
    await editLocationViaDetail(page, recorder, locationId, renamed);

    // After save we're on /locations/:id; navigate back so deleteLocation's
    // list-page path applies (the helper auto-detects detail, but the
    // post-edit detail re-render is the spot where a stale `location`
    // prop reference used to clobber inline errors — re-asserting the
    // heading after the navigate guards that the refetch didn't blank the
    // page either).
    recorder.log(`Step ${step++}: navigating back to /locations`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await expect(
      page.locator(`[data-testid="location-card"][data-location-id="${locationId}"]`),
    ).toContainText(renamed.name);

    recorder.log(`Step ${step++}: deleting "${renamed.name}"`);
    await deleteLocation(page, recorder, renamed.name, locationId);
  });

  // #1654 regression: the per-row "more actions" trigger
  // (`location-card-menu`) shares pixel space with the absolute
  // overlay `<Link data-testid="location-card-link">` that makes
  // the whole card click-through to /locations/:id. CSS painting
  // order put the positioned Link above the in-flow actions
  // cluster, so a click landing on the trigger fell through to the
  // Link and navigated away instead of opening the dropdown. The
  // fix elevates the actions cluster with `relative z-10`; this
  // spec asserts the dropdown opens on click and the page stays on
  // /locations (no navigation).
  test('LocationCard "more actions" trigger opens the dropdown without navigating (#1654)', async ({
    page,
    recorder,
  }) => {
    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const target = {
      name: `E2E LocCard ${suffix}`,
      address: `7 Trigger Way, Test City`,
    };

    let step = 1;
    recorder.log(`Step ${step++}: navigating to /locations`);
    await navigateTo(page, recorder, TO_LOCATIONS);

    recorder.log(`Step ${step++}: creating "${target.name}"`);
    const locationId = await createLocation(page, recorder, target);

    recorder.log(`Step ${step++}: back to /locations for trigger-click test`);
    await navigateTo(page, recorder, TO_LOCATIONS);

    const card = page.locator(
      `[data-testid="location-card"][data-location-id="${locationId}"]`,
    );
    await card.waitFor({ state: 'visible', timeout: 10000 });

    const urlBeforeClick = page.url();

    // Hover reveals the trigger (the button is `opacity-0` until
    // `group-hover`, and Playwright's auto-actionability does hover
    // before clicking — but we hover explicitly here so the
    // recorder + screenshot reflect the real path the user takes).
    await card.hover();
    await recorder.takeScreenshot('location-more-01-hover');

    const trigger = card.locator('[data-testid="location-card-menu"]');
    // `Receives Events` actionability check exercises CSS hit
    // testing — if the overlay Link covers the trigger this fails
    // with a timeout instead of an outright incorrect navigation.
    await expect(trigger).toBeVisible();
    await trigger.click();
    await recorder.takeScreenshot('location-more-02-open');

    // The dropdown's Add-area item is rendered into the body via
    // Radix's portal once the menu opens.
    await expect(page.locator('[data-testid="location-card-add-area"]')).toBeVisible({
      timeout: 5000,
    });

    // Critical: we must still be on /locations, NOT on
    // /locations/:id — i.e. the trigger click did not fall through
    // to the overlay Link.
    expect(page.url()).toBe(urlBeforeClick);

    // Close the menu via Escape so cleanup below sees a clean
    // surface, then delete the fixture.
    await page.keyboard.press('Escape');
    await expect(page.locator('[data-testid="location-card-add-area"]')).toBeHidden({
      timeout: 5000,
    });

    recorder.log(`Step ${step++}: cleanup — delete "${target.name}"`);
    await deleteLocation(page, recorder, target.name, locationId);
  });
});
