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

    await areaCard.waitFor({ state: 'visible' });

    // Get the count of area cards before deletion
    const areaCardsBefore = await page.locator('.area-card').count();

    await areaCard.locator('.area-actions button[title="Delete"]').click();
    await recorder.takeScreenshot('area-delete-01-confirm');
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Wait for the area to be removed from the DOM by checking the count decreased
    await page.waitForFunction(
        (expectedCount) => {
            const cards = document.querySelectorAll('.area-card');
            return cards.length === expectedCount;
        },
        areaCardsBefore - 1,
        { timeout: 10000 }
    );

    await recorder.takeScreenshot('area-delete-02-deleted');

    await expect(page).toHaveURL(/\/locations/);
    // Verify the specific area is no longer in the DOM
    await expect(page.locator(`.area-card:has-text("${areaName}")`)).toHaveCount(0);
}

export async function verifyAreaHasCommodities(page: Page, recorder: TestRecorder) {
    await page.waitForSelector('.no-commodities p:has-text("No commodities found in this area.")');
    await recorder.takeScreenshot('area-verify-no-commodities-01');
}
