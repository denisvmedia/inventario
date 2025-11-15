import {expect, Page} from "@playwright/test";
import {TestRecorder} from "../../utils/test-recorder.js";

export async function createArea(page: Page, recorder: TestRecorder, testArea: any) {
    // Click the Add Area button within the expanded location
    await page.click('.areas-header button:has-text("Add Area")');

    // Fill in the area form
    await page.fill('#name', testArea.name);
    await recorder.takeScreenshot('area-create-01-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Area")');

    // Wait for the area to be created and displayed in-place
    await page.waitForSelector(`.area-card:has-text("${testArea.name}")`);
    await recorder.takeScreenshot('area-create-02-created');
}

export async function deleteArea(page: Page, recorder: TestRecorder, areaName: string, locationName?: string) {
    const areaCard = page.locator(`.area-card:has-text("${areaName}")`);

    if (locationName && !await areaCard.isVisible()) {
        // Navigate to the location detail page
        await page.click(`.location-card:has-text("${locationName}")`);
    }

    // Ensure the area card is visible
    await areaCard.waitFor({ state: 'visible', timeout: 10000 });

    // Click the delete button
    await areaCard.locator('.area-actions button[title="Delete"]').click();
    await recorder.takeScreenshot('area-delete-01-confirm');

    // Wait for confirmation modal to be visible
    await page.locator('.confirmation-modal').waitFor({ state: 'visible', timeout: 5000 });

    // Click the delete button in the confirmation modal and wait for the API response
    await Promise.all([
        page.waitForResponse(response =>
            response.url().includes('/api/v1/areas/') &&
            response.request().method() === 'DELETE' &&
            response.status() === 204,
            { timeout: 10000 }
        ),
        page.click('.confirmation-modal button:has-text("Delete")')
    ]);

    // Wait for the confirmation modal to disappear
    await page.locator('.confirmation-modal').waitFor({ state: 'hidden', timeout: 5000 });

    // Wait for the specific area card to be removed from the DOM
    // Re-query the locator to ensure we're checking the current DOM state
    await expect(page.locator(`.area-card:has-text("${areaName}")`)).toHaveCount(0, { timeout: 15000 });

    await recorder.takeScreenshot('area-delete-02-deleted');

    // Verify we're still on the locations page
    await expect(page).toHaveURL(/\/locations/);
}

export async function verifyAreaHasCommodities(page: Page, recorder: TestRecorder) {
    await page.waitForSelector('.no-commodities p:has-text("No commodities found in this area.")');
    await recorder.takeScreenshot('area-verify-no-commodities-01');
}
