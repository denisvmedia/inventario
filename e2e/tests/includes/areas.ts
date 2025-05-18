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

export async function deleteArea(page: Page, recorder: TestRecorder, areaName: string) {
    const areaCard = page.locator(`.area-card:has-text("${areaName}")`);
    await areaCard.locator('.area-actions button[title="Delete"]').click();
    await recorder.takeScreenshot('area-delete-01-confirm');
    await page.click('.confirmation-modal button:has-text("Delete")');
    await recorder.takeScreenshot('area-delete-02-deleted');

    await expect(page).toHaveURL(/\/locations/);
    await expect(areaCard).not.toBeVisible();
}

export async function verifyAreaHasCommodities(page: Page, recorder: TestRecorder) {
    await page.waitForSelector('.no-commodities p:has-text("No commodities found in this area.")');
    await recorder.takeScreenshot('area-verify-no-commodities-01');
}
