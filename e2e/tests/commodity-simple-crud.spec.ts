import {test} from '../fixtures/app-fixture.js';
import {createLocation, deleteLocation} from "./includes/locations.js";
import {createArea, deleteArea, verifyAreaHasCommodities} from "./includes/areas.js";
import {
  BACK_TO_AREAS,
  createCommodity,
  deleteCommodity,
  editCommodity,
  verifyCommodityDetails
} from "./includes/commodities.js";
import {
  FROM_COMMODITIES,
  FROM_LOCATIONS_AREA,
  navigateTo,
  TO_AREA_COMMODITIES,
  TO_LOCATIONS
} from "./includes/navigate.js";

// Per-test fixture factory — each `test()` invokes this so its row
// names are uniquely suffixed even when the warmup invocation runs
// first against the same backend DB and leaves orphans behind, and
// even when sibling tests in this describe run sequentially against
// the same stack. `Date.now()` collisions inside the same millisecond
// are guarded by a random suffix; `Math.random` is plenty for naming.
function makeTestData() {
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  return {
    testLocation: {
      name: `Test Location for Commodity ${suffix}`,
      address: '123 Test Street, Test City'
    },
    testArea: {
      name: `Test Area for Commodity ${suffix}`
    },
    testCommodity: {
      name: `Test Commodity ${suffix}`,
      shortName: 'TestCom',
      type: 'Electronics',
      count: 1,
      originalPrice: 100,
      originalPriceCurrency: 'CZK',
      purchaseDate: new Date().toISOString().split('T')[0],
      status: 'In Use',
      serialNumber: `SN-${suffix}`,
      extraSerialNumbers: [`ESN1-${suffix}`, `ESN2-${suffix}`],
      partNumbers: [`PN-${suffix}`],
      tags: ['test', 'example', 'e2e'],
      urls: ['https://example.com/product']
    },
    updatedCommodity: {
      name: `Updated Commodity ${suffix}`,
      shortName: 'UpdCom',
      type: 'Electronics',
      count: 2,
      originalPrice: 200,
      serialNumber: `Updated-SN-${suffix}`,
      extraSerialNumbers: [`Updated-ESN-${suffix}`],
      partNumbers: [`Updated-PN-${suffix}`, `Additional-PN-${suffix}`],
      tags: ['updated', 'modified'],
      urls: ['https://example.com/updated', 'https://example.com/documentation']
    },
  };
}

test.describe('Commodity Simple CRUD Operations', () => {

  // Fast-fail test to debug the specific issue. Post-cutover (#1423) the
  // "create commodity from area detail" Vue flow is gone — the React area
  // detail page is a ComingSoon stub. Commodity creation is a top-level
  // /commodities flow that asks for the area on the form, so the helpers
  // navigate to /commodities and we pass `areaName` explicitly.
  test('should update and immediately retrieve a commodity (fast-fail debug)', async ({ page, recorder }) => {
    const { testLocation, testArea, testCommodity, updatedCommodity } = makeTestData();
    // STEP 1: CREATE LOCATION - First create a location
    recorder.log('Step 1: Creating a new location');
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA - Create a new area in-place in the location list view
    recorder.log('Step 2: Creating a new area');
    await createArea(page, recorder, testArea)

    // STEP 3: CREATE COMMODITY - Create a new commodity
    recorder.log('Step 3: Creating a new commodity');
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    await createCommodity(page, recorder, { ...testCommodity, areaName: testArea.name });

    // STEP 4: READ - Verify the commodity details
    recorder.log('Step 4: Verifying the commodity details');
    await verifyCommodityDetails(page, testCommodity);

    // STEP 5: UPDATE - Edit the commodity
    recorder.log('Step 5: Editing the commodity');
    await editCommodity(page, recorder, { ...updatedCommodity, areaName: testArea.name });

    // STEP 6: READ - Verify the commodity details (this is where it fails in CI)
    recorder.log('Step 6: Verifying updated commodity details');
    await verifyCommodityDetails(page, updatedCommodity);
  });

  test('should perform full CRUD operations on a commodity', async ({ page, recorder }) => {
    const { testLocation, testArea, testCommodity, updatedCommodity } = makeTestData();
    // STEP 1: CREATE LOCATION - First create a location
    recorder.log('Step 1: Creating a new location');
    await navigateTo(page, recorder, TO_LOCATIONS);
    // Capture the ID up front so the cleanup step targets *this* test's
    // location, not an orphan with the same name left by the CI warmup
    // invocation or by the sibling fast-fail-debug test (same describe →
    // same testLocation.name).
    const locationId = await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA - Create a new area in-place in the location list view
    recorder.log('Step 2: Creating a new area');
    await createArea(page, recorder, testArea)

    // STEP 3: CREATE COMMODITY - Create a new commodity. Post-cutover the
    // helper navigates to /commodities and the form's area select is the
    // source of truth, so we pass areaName explicitly.
    recorder.log('Step 3: Creating a new commodity');
    await navigateTo(page, recorder, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    await createCommodity(page, recorder, { ...testCommodity, areaName: testArea.name });

    // STEP 4: READ - Verify the commodity details
    recorder.log('Step 4: Verifying the commodity details');
    await verifyCommodityDetails(page, testCommodity);

    // STEP 5: UPDATE - Edit the commodity
    recorder.log('Step 5: Editing the commodity');
    await editCommodity(page, recorder, { ...updatedCommodity, areaName: testArea.name });

    // STEP 6: READ - Verify the commodity details
    recorder.log('Step 6: Verifying updated commodity details');
    await verifyCommodityDetails(page, updatedCommodity);

    // STEP 7: DELETE - Delete the commodity. The post-delete redirect lands
    // on /commodities (area-detail is a ComingSoon stub today; the helper
    // accepts BACK_TO_AREAS but the actual destination is the same).
    recorder.log('Step 7: Deleting the commodity');
    await deleteCommodity(page, recorder, updatedCommodity.name, BACK_TO_AREAS);

    // STEP 7: CLEANUP - Delete the area and location. The React locations
    // list always shows areas inline under their parent card — no
    // `.areas-header` chrome to wait on after navigation.
    recorder.log('Step 7: Cleaning up - deleting the area and location');
    await navigateTo(page, recorder, TO_LOCATIONS, FROM_COMMODITIES);
    await page.waitForSelector(`[data-testid="location-card"][data-location-id="${locationId}"]`);
    await recorder.takeScreenshot('location-expanded');

    // Delete the area (inline-rendered under the location card)
    await deleteArea(page, recorder, testArea.name);

    // Delete the location (pass the ID so we delete *this* test's clone)
    await deleteLocation(page, recorder, testLocation.name, locationId);
  });
});
