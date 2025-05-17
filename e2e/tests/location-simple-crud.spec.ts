import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture';

test.describe('Location Simple CRUD Operations', () => {
  // Test data with timestamps to ensure uniqueness
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location ${timestamp}`,
    address: '123 Test Street, Test City'
  };
  
  const updatedLocation = {
    name: `Updated Location ${timestamp}`,
    address: '456 Updated Street, Updated City'
  };

  test('should perform full CRUD operations on a location', async ({ page, recorder }) => {
    // STEP 1: CREATE - Create a new location
    console.log('Step 1: Creating a new location');
    await page.goto('/locations');
    await recorder.takeScreenshot('01-locations-before-create');
    
    // Click the New button to show the location form
    await page.click('button:has-text("New")');
    
    // Fill in the location form
    await page.fill('#name', testLocation.name);
    await page.fill('#address', testLocation.address);
    await recorder.takeScreenshot('02-location-form-filled');
    
    // Submit the form
    await page.click('button:has-text("Create Location")');
    
    // Wait for the location to be created and displayed
    await page.waitForSelector(`.location-card:has-text("${testLocation.name}")`);
    await recorder.takeScreenshot('03-location-created');
    
    // Verify the new location is displayed
    const locationCard = page.locator(`.location-card:has-text("${testLocation.name}")`);
    await expect(locationCard).toBeVisible();
    await expect(locationCard).toContainText(testLocation.address);
    
    // STEP 2: READ - View the location details
    console.log('Step 2: Viewing the location details');
    // Click on the location card to view details
    await locationCard.click();
    
    // Verify we're on the location detail page
    await expect(page).toHaveURL(/\/locations\/[a-zA-Z0-9-]+$/);
    await expect(page.locator('h1')).toContainText(testLocation.name);
    await expect(page.locator('.location-address')).toContainText(testLocation.address);
    await recorder.takeScreenshot('04-location-details');
    
    // STEP 3: UPDATE - Edit the location
    console.log('Step 3: Editing the location');
    // Click the Edit button
    await page.click('button:has-text("Edit")');
    
    // Verify we're on the edit page
    await expect(page).toHaveURL(/\/locations\/[a-zA-Z0-9-]+\/edit$/);
    await recorder.takeScreenshot('05-location-edit-form');
    
    // Clear and fill in the updated values
    await page.fill('#name', '');
    await page.fill('#name', updatedLocation.name);
    await page.fill('#address', '');
    await page.fill('#address', updatedLocation.address);
    await recorder.takeScreenshot('06-location-edit-form-filled');
    
    // Save the changes
    await page.click('button:has-text("Save Changes")');
    
    // Verify we're redirected back to the location detail page
    await expect(page).toHaveURL(/\/locations\/[a-zA-Z0-9-]+$/);
    await recorder.takeScreenshot('07-location-after-edit');
    
    // Verify the location was updated
    await expect(page.locator('h1')).toContainText(updatedLocation.name);
    await expect(page.locator('.location-address')).toContainText(updatedLocation.address);
    
    // STEP 4: DELETE - Delete the location
    console.log('Step 4: Deleting the location');
    // Click the Delete button
    await page.click('button:has-text("Delete")');
    
    // Confirm deletion in the modal
    await page.click('.confirmation-modal button:has-text("Delete")');
    
    // Verify we're redirected back to the locations list
    await expect(page).toHaveURL('/locations');
    await recorder.takeScreenshot('08-locations-after-delete');
    
    // Verify the location is no longer in the list
    await expect(page.locator(`.location-card:has-text("${updatedLocation.name}")`)).not.toBeVisible();
  });
});
