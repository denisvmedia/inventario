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
  navitateTo,
  TO_AREA_COMMODITIES,
  TO_LOCATIONS
} from "./includes/navigate.js";

test.describe('Commodity Simple CRUD Operations', () => {
  // Test data with timestamps to ensure uniqueness
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location for Commodity ${timestamp}`,
    address: '123 Test Street, Test City'
  };

  const testArea = {
    name: `Test Area for Commodity ${timestamp}`
  };

  const testCommodity = {
    name: `Test Commodity ${timestamp}`,
    shortName: 'TestCom',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0], // Today's date in YYYY-MM-DD format
    status: 'In Use'
  };

  const updatedCommodity = {
    name: `Updated Commodity ${timestamp}`,
    shortName: 'UpdCom',
    count: 2,
    originalPrice: 200
  };

  test('should perform full CRUD operations on a commodity', async ({ page, recorder }) => {
    // STEP 1: CREATE LOCATION - First create a location
    console.log('Step 1: Creating a new location');
    await navitateTo(page, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA - Create a new area in-place in the location list view
    console.log('Step 2: Creating a new area');
    await createArea(page, recorder, testArea)

    // STEP 3: CREATE COMMODITY - Create a new commodity
    console.log('Step 3: Creating a new commodity');
    await navitateTo(page, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    await createCommodity(page, recorder, testCommodity);

    // STEP 4: READ - Verify the commodity details
    console.log('Step 4: Verifying the commodity details');
    await verifyCommodityDetails(page, testCommodity);

    // STEP 5: UPDATE - Edit the commodity
    console.log('Step 5: Editing the commodity');
    await editCommodity(page, recorder, updatedCommodity);

    // STEP 6: READ - Verify the commodity details
    console.log('Step 6: Verifying updated commodity details');
    await verifyCommodityDetails(page, updatedCommodity);

    // STEP 7: DELETE - Delete the commodity
    console.log('Step 7: Deleting the commodity');
    await deleteCommodity(page, recorder, updatedCommodity.name, BACK_TO_AREAS);

    // STEP 7: CLEANUP - Delete the area and location
    console.log('Step 7: Cleaning up - deleting the area and location');
    await navitateTo(page, TO_LOCATIONS, FROM_COMMODITIES);

    // Wait for the areas section to be visible after location expansion
    await page.waitForSelector('.areas-header');
    await recorder.takeScreenshot('location-expanded');

    // Delete the area
    await deleteArea(page, recorder, testArea.name);

    // Delete the location
    await deleteLocation(page, recorder, testLocation.name);
  });
});
