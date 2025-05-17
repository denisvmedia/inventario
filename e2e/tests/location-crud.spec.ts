import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture';

test.describe('Location CRUD Operations', () => {
  // Test data
  const testLocation = {
    name: `Test Location ${Date.now()}`,
    address: '123 Test Street, Test City'
  };

  // Updated test data
  const updatedLocation = {
    name: `Updated Location ${Date.now()}`,
    address: '456 Updated Street, Updated City'
  };

  let createdLocationId: string;

  test('should create a new location', async ({ page, recorder }) => {
    // Navigate to locations page
    await page.goto('/locations');
    await recorder.takeScreenshot('locations-before-create');

    // Click the New button to show the location form
    await page.click('button:has-text("New")');

    // Fill in the location form
    await page.fill('#name', testLocation.name);
    await page.fill('#address', testLocation.address);
    await recorder.takeScreenshot('location-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Location")');

    // Wait for the location to be created and displayed
    await page.waitForSelector(`.location-card:has-text("${testLocation.name}")`);
    await recorder.takeScreenshot('location-created');

    // Verify the new location is displayed
    const locationCard = page.locator(`.location-card:has-text("${testLocation.name}")`);
    await expect(locationCard).toBeVisible();
    await expect(locationCard).toContainText(testLocation.address);

    // For now, we'll use a hardcoded ID for testing purposes
    // In a real scenario, we would extract this from the UI or API
    createdLocationId = 'test-location-id';
    console.log(`Using test location ID: ${createdLocationId}`);
  });

  test('should edit an existing location', async ({ page, recorder }) => {
    // Skip if the previous test didn't create a location
    test.skip(!createdLocationId, 'No location ID from previous test');

    // Navigate to the locations list
    await page.goto('/locations');
    await recorder.takeScreenshot('locations-before-edit');

    // Find the location card and click the edit icon
    const locationCard = page.locator(`.location-card:has-text("${testLocation.name}")`);
    await expect(locationCard).toBeVisible();

    // Click on the location to go to the detail page
    await locationCard.click();

    // Verify we're on the location detail page
    await expect(page.locator('h1')).toContainText(testLocation.name);

    // Click the edit button
    await page.click('button:has-text("Edit")');
    await recorder.takeScreenshot('location-edit-before');

    // Clear and fill in the updated values
    await page.fill('#name', '');
    await page.fill('#name', updatedLocation.name);
    await page.fill('#address', '');
    await page.fill('#address', updatedLocation.address);
    await recorder.takeScreenshot('location-edit-form-filled');

    // Save the changes
    await page.click('button:has-text("Save Changes")');

    // Wait to be redirected back to the location detail page
    await page.waitForURL(`/locations/${createdLocationId}`);
    await recorder.takeScreenshot('location-after-edit');

    // Verify the location was updated
    await expect(page.locator('h1')).toContainText(updatedLocation.name);
    await expect(page.locator('.location-address')).toContainText(updatedLocation.address);

    // Navigate back to locations list to verify it's updated there too
    await page.goto('/locations');
    const updatedLocationCard = page.locator(`.location-card:has-text("${updatedLocation.name}")`);
    await expect(updatedLocationCard).toBeVisible();
    await expect(updatedLocationCard).toContainText(updatedLocation.address);
  });

  test('should delete a location', async ({ page, recorder }) => {
    // Skip if the previous test didn't create a location
    test.skip(!createdLocationId, 'No location ID from previous test');

    // Navigate to the location detail page
    await page.goto(`/locations/${createdLocationId}`);
    await recorder.takeScreenshot('location-before-delete');

    // Click the delete button
    await page.click('button:has-text("Delete")');

    // Confirm deletion in the modal
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Wait to be redirected back to the locations list
    await page.waitForURL('/locations');
    await recorder.takeScreenshot('locations-after-delete');

    // Verify the location is no longer in the list
    const deletedLocationCard = page.locator(`.location-card:has-text("${updatedLocation.name}")`);
    await expect(deletedLocationCard).not.toBeVisible();
  });
});
