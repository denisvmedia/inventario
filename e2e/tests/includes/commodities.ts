import {expect, Page} from "@playwright/test";
import {TestRecorder} from "../../utils/test-recorder.js";

export async function createCommodity(page: Page, recorder: TestRecorder,testCommodity: any): Promise<string> {
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

    // Set purchase date using DatePicker component
    // PrimeVue DatePicker v4 typically uses this pattern
    await page.locator('#purchaseDate input').fill(testCommodity.purchaseDate);

    // Add serial number if provided
    if (testCommodity.serialNumber) {
        await page.fill('#serialNumber', testCommodity.serialNumber);
    }

    // Add extra serial numbers if provided
    if (testCommodity.extraSerialNumbers && testCommodity.extraSerialNumbers.length > 0) {
        for (let i = 0; i < testCommodity.extraSerialNumbers.length; i++) {
            recorder.log(`Adding extra serial number ${i + 1}`);
            await page.click('button:has-text("Add Serial Number")');
            recorder.log(`Filling in extra serial number ${i + 1}`);
            await page.fill(`.array-input:has(button:has-text("Add Serial Number")) .array-item:nth-child(${i + 1}) input`, testCommodity.extraSerialNumbers[i]);
        }
    }

    // Add part numbers if provided
    if (testCommodity.partNumbers && testCommodity.partNumbers.length > 0) {
        for (let i = 0; i < testCommodity.partNumbers.length; i++) {
            await page.click('button:has-text("Add Part Number")');
            await page.fill(`.array-input:has(button:has-text("Add Part Number")) .array-item:nth-child(${i + 1}) input`, testCommodity.partNumbers[i]);
        }
    }

    // Add tags if provided
    if (testCommodity.tags && testCommodity.tags.length > 0) {
        for (let i = 0; i < testCommodity.tags.length; i++) {
            await page.click('button:has-text("Add Tag")');
            await page.fill(`.array-input:has(button:has-text("Add Tag")) .array-item:nth-child(${i + 1}) input`, testCommodity.tags[i]);
        }
    }

    // Add URLs if provided
    if (testCommodity.urls && testCommodity.urls.length > 0) {
        for (let i = 0; i < testCommodity.urls.length; i++) {
            await page.click('button:has-text("Add URL")');
            await page.fill(`.array-input:has(button:has-text("Add URL")) .array-item:nth-child(${i + 1}) input`, testCommodity.urls[i]);
        }
    }

    await recorder.takeScreenshot('commodity-create-02-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Commodity")');

    // Wait to be redirected to the commodity detail page
    await page.waitForURL(/\/commodities\/[0-9a-fA-F-]{36}/);
    await recorder.takeScreenshot('commodity-create-03-created');

    return page.url();
}

export async function verifyCommodityDetails(page: Page, testCommodity: any) {
    // Verify the commodity details are displayed correctly
    await expect(page.locator('h1')).toContainText(testCommodity.name);
    await expect(page.locator('.commodity-short-name')).toContainText(testCommodity.shortName);
    await expect(page.locator('.commodity-type')).toContainText(testCommodity.type);
    await expect(page.locator('.commodity-count')).toContainText(testCommodity.count.toString());
    await expect(page.locator('.commodity-original-price')).toContainText(testCommodity.originalPrice.toString());

    // Verify serial number if provided
    if (testCommodity.serialNumber) {
        await expect(page.locator('.commodity-serial-number')).toContainText(testCommodity.serialNumber);
    }

    // Verify extra serial numbers if provided
    if (testCommodity.extraSerialNumbers && testCommodity.extraSerialNumbers.length > 0) {
        for (const serialNumber of testCommodity.extraSerialNumbers) {
            await expect(page.locator('.commodity-extra-serial-numbers')).toContainText(serialNumber);
        }
    }

    // Verify part numbers if provided
    if (testCommodity.partNumbers && testCommodity.partNumbers.length > 0) {
        for (const partNumber of testCommodity.partNumbers) {
            await expect(page.locator('.commodity-part-numbers')).toContainText(partNumber);
        }
    }

    // Verify tags if provided
    if (testCommodity.tags && testCommodity.tags.length > 0) {
        for (const tag of testCommodity.tags) {
            await expect(page.locator('.commodity-tags')).toContainText(tag);
        }
    }

    // Verify URLs if provided
    if (testCommodity.urls && testCommodity.urls.length > 0) {
        for (const url of testCommodity.urls) {
            await expect(page.locator('.commodity-urls')).toContainText(url);
        }
    }
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

    // Update serial number if provided
    if (updatedCommodity.serialNumber !== undefined) {
        await page.fill('#serialNumber', updatedCommodity.serialNumber);
    }

    // Handle extra serial numbers if provided
    if (updatedCommodity.extraSerialNumbers !== undefined) {
        // First, remove existing extra serial numbers
        const existingSerialNumbers = await page.$$('.array-input:has(button:has-text("Add Serial Number")) .array-item');
        for (let i = existingSerialNumbers.length - 1; i >= 0; i--) {
            await page.click(`.array-input:has(button:has-text("Add Serial Number")) .array-item:nth-child(${i + 1}) button:has-text("Remove")`);
        }

        // Then add new ones
        for (let i = 0; i < updatedCommodity.extraSerialNumbers.length; i++) {
            await page.click('button:has-text("Add Serial Number")');
            await page.fill(`.array-input:has(button:has-text("Add Serial Number")) .array-item:nth-child(${i + 1}) input`, updatedCommodity.extraSerialNumbers[i]);
        }
    }

    // Handle part numbers if provided
    if (updatedCommodity.partNumbers !== undefined) {
        // First, remove existing part numbers
        const existingPartNumbers = await page.$$('.array-input:has(button:has-text("Add Part Number")) .array-item');
        for (let i = existingPartNumbers.length - 1; i >= 0; i--) {
            await page.click(`.array-input:has(button:has-text("Add Part Number")) .array-item:nth-child(${i + 1}) button:has-text("Remove")`);
        }

        // Then add new ones
        for (let i = 0; i < updatedCommodity.partNumbers.length; i++) {
            await page.click('button:has-text("Add Part Number")');
            await page.fill(`.array-input:has(button:has-text("Add Part Number")) .array-item:nth-child(${i + 1}) input`, updatedCommodity.partNumbers[i]);
        }
    }

    // Handle tags if provided
    if (updatedCommodity.tags !== undefined) {
        // First, remove existing tags
        const existingTags = await page.$$('.array-input:has(button:has-text("Add Tag")) .array-item');
        for (let i = existingTags.length - 1; i >= 0; i--) {
            await page.click(`.array-input:has(button:has-text("Add Tag")) .array-item:nth-child(${i + 1}) button:has-text("Remove")`);
        }

        // Then add new ones
        for (let i = 0; i < updatedCommodity.tags.length; i++) {
            await page.click('button:has-text("Add Tag")');
            await page.fill(`.array-input:has(button:has-text("Add Tag")) .array-item:nth-child(${i + 1}) input`, updatedCommodity.tags[i]);
        }
    }

    // Handle URLs if provided
    if (updatedCommodity.urls !== undefined) {
        // First, remove existing URLs
        const existingUrls = await page.$$('.array-input:has(button:has-text("Add URL")) .array-item');
        for (let i = existingUrls.length - 1; i >= 0; i--) {
            await page.click(`.array-input:has(button:has-text("Add URL")) .array-item:nth-child(${i + 1}) button:has-text("Remove")`);
        }

        // Then add new ones
        for (let i = 0; i < updatedCommodity.urls.length; i++) {
            await page.click('button:has-text("Add URL")');
            await page.fill(`.array-input:has(button:has-text("Add URL")) .array-item:nth-child(${i + 1}) input`, updatedCommodity.urls[i]);
        }
    }

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

    // Wait for confirmation modal to be visible
    await page.locator('.confirmation-modal').waitFor({ state: 'visible', timeout: 5000 });

    // Confirm deletion in the modal
    await page.click('.confirmation-modal button:has-text("Delete")');
    await recorder.takeScreenshot('commodity-delete-01-on-delete-confirm');

    // Wait for the confirmation modal to disappear
    await page.locator('.confirmation-modal').waitFor({ state: 'hidden', timeout: 5000 });

    // Verify we're redirected back to the correct page
    if (backTo === 'commodities') {
        await expect(page).toHaveURL('/commodities', { timeout: 10000 });
        await recorder.takeScreenshot('commodity-delete-02-after-delete');
    } else if (backTo === 'areas') {
        await expect(page).toHaveURL(/\/areas\/[a-zA-Z0-9-]+/, { timeout: 10000 });
        await recorder.takeScreenshot('commodity-delete-01-after-delete');
    }

    // Wait for the specific commodity card to be removed from the DOM
    const commodityCard = page.locator(`.commodity-card:has-text("${commodityName}")`);
    await expect(commodityCard).toHaveCount(0, { timeout: 15000 });
}
