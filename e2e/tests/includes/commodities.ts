import {expect, Page} from "@playwright/test";
import {TestRecorder} from "../../utils/test-recorder.js";

export async function createCommodity(page: Page, recorder: TestRecorder,testCommodity: any) {
    await recorder.takeScreenshot('commodities-create-01-before-create');

    // Click the New button to show the commodity form
    await page.click('a:has-text("Add Commodity")');

    // Fill in the commodity form
    await page.waitForTimeout(1000); // In some cases we are too fast to fill in the form
    await page.fill('#name', testCommodity.name);
    await page.fill('#shortName', testCommodity.shortName);

    // Select type from dropdown
    await page.click('.p-select[id="type"]');
    await page.click(`.p-select-option-label:has-text("${testCommodity.type}")`);

    // Fill in other fields
    await page.fill('#count', testCommodity.count.toString());
    await page.fill('#originalPrice', testCommodity.originalPrice.toString());

    // Select currency from dropdown
    await page.click('.p-select[id="originalPriceCurrency"]');
    await page.click(`.p-select-option-label:has-text("${testCommodity.originalPriceCurrency}")`);

    // Set purchase date
    await page.fill('#purchaseDate', testCommodity.purchaseDate);

    await recorder.takeScreenshot('commodity-create-02-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Commodity")');

    // Wait to be redirected to the commodity detail page
    await page.waitForURL(/\/commodities\/[a-zA-Z0-9-]+\?/);
    await recorder.takeScreenshot('commodity-create-03-created');
}

export async function verifyCommodityDetails(page: Page, testCommodity: any) {
    // Verify the commodity details are displayed correctly
    await expect(page.locator('h1')).toContainText(testCommodity.name);
    await expect(page.locator('.commodity-short-name')).toContainText(testCommodity.shortName);
    await expect(page.locator('.commodity-type')).toContainText(testCommodity.type);
    await expect(page.locator('.commodity-count')).toContainText(testCommodity.count.toString());
    await expect(page.locator('.commodity-original-price')).toContainText(testCommodity.originalPrice.toString());
}

export async function editCommodity(page: Page, recorder: TestRecorder, updatedCommodity: any, buttonSelector?: string|boolean) {
    if (buttonSelector !== false) {
        // Click the edit button
        if (typeof buttonSelector === 'string' && buttonSelector.length > 0) {
            await page.click(buttonSelector);
        } else {
            await page.click('button:has-text("Edit")');
        }
    } else {
        // Expected to be already on the edit page
    }

    // Verify we're on the edit page
    await expect(page).toHaveURL(/\/commodities\/[a-zA-Z0-9-]+\/edit\?/);
    await recorder.takeScreenshot('commodity-edit-01-edit-form');

    // Update the commodity fields
    await page.fill('#name', updatedCommodity.name);
    await page.fill('#shortName', updatedCommodity.shortName);
    await page.fill('#count', updatedCommodity.count.toString());
    await page.fill('#originalPrice', updatedCommodity.originalPrice.toString());

    await recorder.takeScreenshot('commodity-edit-02-edit-form-filled');

    // Save the changes
    await page.click('button:has-text("Save Commodity")');

    // Wait to be redirected back to the commodity detail page
    await expect(page).toHaveURL(/\/commodities\/[a-zA-Z0-9-]+\?/);
    await recorder.takeScreenshot('commodity-edit-02-after-edit');
}

export const BACK_TO_COMMODITIES = 'commodities';
export const BACK_TO_AREAS = 'areas';
export type BackTo = typeof BACK_TO_COMMODITIES | typeof BACK_TO_AREAS;

export async function deleteCommodity(page: Page, recorder: TestRecorder, commodityName: string, backTo: BackTo) {
    // Click the Delete button
    await page.click('button:has-text("Delete")');

    // Confirm deletion in the modal
    await page.click('.confirmation-modal button:has-text("Delete")');
    await recorder.takeScreenshot('commodity-delete-01-on-delete-confirm');

    // Verify we're redirected back to the commodities list
    if (backTo === 'commodities') {
        await expect(page).toHaveURL('/commodities');
        await recorder.takeScreenshot('commodity-delete-02-after-delete');
    } else if (backTo === 'areas') {
        await expect(page).toHaveURL(/\/areas\/[a-zA-Z0-9-]+/);
        await recorder.takeScreenshot('commodity-delete-01-after-delete');
    }

    // Verify the commodity is no longer in the list
    await expect(page.locator(`.commodity-card:has-text("${commodityName}")`)).not.toBeVisible();
}
