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
});
