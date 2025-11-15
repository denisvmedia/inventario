import {expect, Page} from "@playwright/test";
import {TestRecorder} from "../../utils/test-recorder.js";

export async function createLocation(page: Page, recorder: TestRecorder, testLocation: any) {
    await recorder.takeScreenshot('locations-create-01-before-create');

    // Click the New button to show the location form
    await page.click('button:has-text("New")');

    // Fill in the location form
    await page.fill('#name', testLocation.name);
    await page.fill('#address', testLocation.address);
    await recorder.takeScreenshot('location-create-02-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Location")');

    // Wait for the location to be created and displayed
    await page.waitForSelector(`.location-card:has-text("${testLocation.name}")`);
    await recorder.takeScreenshot('location-create-03-created');

    // Click on the location card to expand it
    // await page.click(`.location-card:has-text("${testLocation.name}")`);
}

export async function deleteLocation(page: Page, recorder: TestRecorder, locationName: string) {
    // First, ensure the location card is visible
    const locationCard = page.locator(`.location-card:has-text("${locationName}")`);
    await locationCard.waitFor({ state: 'visible', timeout: 10000 });

    // Click the delete button
    await locationCard.locator('button[title="Delete"]').click();
    await recorder.takeScreenshot('location-delete-01-confirm');

    // Wait for confirmation modal to be visible
    await page.locator('.confirmation-modal').waitFor({ state: 'visible', timeout: 5000 });

    // Click the delete button in the confirmation modal
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Wait for the confirmation modal to disappear
    await page.locator('.confirmation-modal').waitFor({ state: 'hidden', timeout: 5000 });

    // Wait for the specific location card to be removed from the DOM
    await expect(locationCard).toHaveCount(0, { timeout: 15000 });

    await recorder.takeScreenshot('location-delete-02-deleted');

    // Verify we're still on the locations page
    await expect(page).toHaveURL(/\/locations/);
}
