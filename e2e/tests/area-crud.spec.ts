import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture';

test.describe('Area CRUD Operations', () => {
  // Test data
  const testLocation = {
    name: `Test Location for Area ${Date.now()}`,
    address: '123 Test Street, Test City'
  };

  const testArea = {
    name: `Test Area ${Date.now()}`
  };

  const updatedArea = {
    name: `Updated Area ${Date.now()}`
  };

  let createdLocationId: string;
  let createdAreaId: string;

  // Setup: Create a location first
  test('should create a location for area tests', async ({ page, recorder }) => {
    // Navigate to locations page
    await page.goto('/locations');

    // Click the New button to show the location form
    await page.click('button:has-text("New")');

    // Fill in the location form
    await page.fill('#name', testLocation.name);
    await page.fill('#address', testLocation.address);

    // Submit the form
    await page.click('button:has-text("Create Location")');

    // Wait for the location to be created and displayed
    await page.waitForSelector(`.location-card:has-text("${testLocation.name}")`);
    await recorder.takeScreenshot('location-for-area-created');

    // For now, we'll use a hardcoded ID for testing purposes
    // In a real scenario, we would extract this from the UI or API
    createdLocationId = 'test-location-id';
    console.log(`Using test location ID: ${createdLocationId}`);
  });

  test('should create a new area', async ({ page, recorder }) => {
    // Skip if the previous test didn't create a location
    test.skip(!createdLocationId, 'No location ID from previous test');

    // Navigate to the location detail page
    await page.goto(`/locations/${createdLocationId}`);
    await recorder.takeScreenshot('location-before-area-create');

    // Click the "Add Area" button
    await page.click('button:has-text("Add Area")');

    // Fill in the area form
    await page.fill('#name', testArea.name);
    await recorder.takeScreenshot('area-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Area")');

    // Wait for the area to be created and displayed
    await page.waitForSelector(`.area-card:has-text("${testArea.name}")`);
    await recorder.takeScreenshot('area-created');

    // Verify the new area is displayed
    const areaCard = page.locator(`.area-card:has-text("${testArea.name}")`);
    await expect(areaCard).toBeVisible();

    // For now, we'll use a hardcoded ID for testing purposes
    // In a real scenario, we would extract this from the UI or API
    createdAreaId = 'test-area-id';
    console.log(`Using test area ID: ${createdAreaId}`);

    // Go back to the location page for the next test
    await page.goto(`/locations/${createdLocationId}`);
  });

  test('should edit an existing area', async ({ page, recorder }) => {
    // Skip if the previous test didn't create an area
    test.skip(!createdAreaId, 'No area ID from previous test');

    // Navigate to the location detail page
    await page.goto(`/locations/${createdLocationId}`);
    await recorder.takeScreenshot('location-before-area-edit');

    // Find the area card
    const areaCard = page.locator(`.area-card:has-text("${testArea.name}")`);
    await expect(areaCard).toBeVisible();

    // Click on the area to go to the detail page
    await areaCard.click();

    // Verify we're on the area detail page
    await expect(page).toHaveURL(new RegExp(`/areas/${createdAreaId}`));

    // Click the edit button
    await page.click('button:has-text("Edit")');
    await recorder.takeScreenshot('area-edit-before');

    // Clear and fill in the updated values
    await page.fill('#name', '');
    await page.fill('#name', updatedArea.name);
    await recorder.takeScreenshot('area-edit-form-filled');

    // Save the changes
    await page.click('button:has-text("Save Changes")');

    // Wait to be redirected back to the locations page
    await page.waitForURL('/locations');
    await recorder.takeScreenshot('locations-after-area-edit');

    // Verify the area was updated
    // First expand the location if it's collapsed
    const locationCard = page.locator(`.location-card:has-text("${testLocation.name}")`);
    if (await locationCard.locator('.collapsed').isVisible()) {
      await locationCard.click();
    }

    // Now check for the updated area
    const updatedAreaCard = page.locator(`.area-card:has-text("${updatedArea.name}")`);
    await expect(updatedAreaCard).toBeVisible();
  });

  test('should delete an area', async ({ page, recorder }) => {
    // Skip if the previous test didn't create an area
    test.skip(!createdAreaId || !createdLocationId, 'No area or location ID from previous test');

    // Navigate to the location detail page
    await page.goto(`/locations/${createdLocationId}`);
    await recorder.takeScreenshot('location-before-area-delete');

    // Find the area card
    const areaCard = page.locator(`.area-card:has-text("${updatedArea.name}")`);

    // Click the delete button on the area card
    await areaCard.locator('.delete-icon').click();

    // Confirm deletion in the modal
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Wait for the area to be removed
    await page.waitForTimeout(1000); // Give it a moment to process
    await recorder.takeScreenshot('location-after-area-delete');

    // Verify the area is no longer in the list
    await expect(page.locator(`.area-card:has-text("${updatedArea.name}")`)).not.toBeVisible();

    // Clean up: Delete the test location
    await page.click('button:has-text("Delete")');
    await page.click('.confirmation-modal button:has-text("Delete")');
    await page.waitForURL('/locations');
  });
});
