import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture';

test.describe('Area Simple CRUD Operations', () => {
  // Test data with timestamps to ensure uniqueness
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location for Area ${timestamp}`,
    address: '123 Test Street, Test City'
  };
  
  const testArea = {
    name: `Test Area ${timestamp}`
  };
  
  const updatedArea = {
    name: `Updated Area ${timestamp}`
  };

  test('should perform full CRUD operations on an area', async ({ page, recorder }) => {
    // STEP 1: CREATE LOCATION - First create a location to contain the area
    console.log('Step 1: Creating a new location for the area');
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
    
    // Click on the location card to view details
    const locationCard = page.locator(`.location-card:has-text("${testLocation.name}")`);
    await locationCard.click();
    
    // STEP 2: CREATE AREA - Create a new area in the location
    console.log('Step 2: Creating a new area');
    // Verify we're on the location detail page
    await expect(page).toHaveURL(/\/locations\/[a-zA-Z0-9-]+$/);
    await expect(page.locator('h1')).toContainText(testLocation.name);
    await recorder.takeScreenshot('04-location-details');
    
    // Click the Add Area button
    await page.click('button:has-text("Add Area")');
    
    // Fill in the area form
    await page.fill('#name', testArea.name);
    await recorder.takeScreenshot('05-area-form-filled');
    
    // Submit the form
    await page.click('button:has-text("Create Area")');
    
    // Wait for the area to be created and displayed
    await page.waitForSelector(`.area-card:has-text("${testArea.name}")`);
    await recorder.takeScreenshot('06-area-created');
    
    // Verify the new area is displayed
    const areaCard = page.locator(`.area-card:has-text("${testArea.name}")`);
    await expect(areaCard).toBeVisible();
    
    // STEP 3: READ - View the area details
    console.log('Step 3: Viewing the area details');
    // Click on the area card to view details
    await areaCard.click();
    
    // Verify we're on the area detail page
    await expect(page).toHaveURL(/\/areas\/[a-zA-Z0-9-]+$/);
    await expect(page.locator('h1')).toContainText(testArea.name);
    await recorder.takeScreenshot('07-area-details');
    
    // STEP 4: UPDATE - Edit the area
    console.log('Step 4: Editing the area');
    // Click the Edit button
    await page.click('button:has-text("Edit")');
    
    // Verify we're on the edit page
    await expect(page).toHaveURL(/\/areas\/[a-zA-Z0-9-]+\/edit$/);
    await recorder.takeScreenshot('08-area-edit-form');
    
    // Clear and fill in the updated values
    await page.fill('#name', '');
    await page.fill('#name', updatedArea.name);
    await recorder.takeScreenshot('09-area-edit-form-filled');
    
    // Save the changes
    await page.click('button:has-text("Save Changes")');
    
    // Verify we're redirected back to the locations page
    await expect(page).toHaveURL('/locations');
    await recorder.takeScreenshot('10-locations-after-area-edit');
    
    // Find the location and check if it contains the updated area
    await locationCard.click();
    
    // Verify the area was updated
    const updatedAreaCard = page.locator(`.area-card:has-text("${updatedArea.name}")`);
    await expect(updatedAreaCard).toBeVisible();
    
    // STEP 5: DELETE - Delete the area
    console.log('Step 5: Deleting the area');
    // Click the delete icon on the area card
    await updatedAreaCard.locator('.delete-icon').click();
    
    // Confirm deletion in the modal
    await page.click('.confirmation-modal button:has-text("Delete")');
    
    // Verify the area is no longer in the list
    await expect(page.locator(`.area-card:has-text("${updatedArea.name}")`)).not.toBeVisible();
    
    // STEP 6: CLEANUP - Delete the location
    console.log('Step 6: Cleaning up - deleting the location');
    // Click the Delete button for the location
    await page.click('button:has-text("Delete")');
    
    // Confirm deletion in the modal
    await page.click('.confirmation-modal button:has-text("Delete")');
    
    // Verify we're redirected back to the locations list
    await expect(page).toHaveURL('/locations');
    await recorder.takeScreenshot('11-locations-after-cleanup');
    
    // Verify the location is no longer in the list
    await expect(page.locator(`.location-card:has-text("${testLocation.name}")`)).not.toBeVisible();
  });
});
