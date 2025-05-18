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
    await page.click(`.location-card:has-text("${locationName}") button[title="Delete"]`);
    await recorder.takeScreenshot('location-delete-01-confirm');
    await page.click('.confirmation-modal button:has-text("Delete")');
    await recorder.takeScreenshot('location-delete-02-deleted');

    await expect(page).toHaveURL(/\/locations/);
    await expect(page.locator(`.location-card:has-text("${locationName}")`)).not.toBeVisible();
}