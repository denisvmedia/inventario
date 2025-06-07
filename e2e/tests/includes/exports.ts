import {expect, Page} from "@playwright/test";
import {TestRecorder} from "../../utils/test-recorder.js";

export async function createExport(page: Page, recorder: TestRecorder, testExport: any, locationName?: string, areaName?: string, commodityName?: string) {
    await recorder.takeScreenshot('exports-create-01-before-create');

    // Click the New button to show the export form
    await page.click('a:has-text("New")');
    await page.waitForSelector('h1:has-text("Create New Export")');

    // Fill in the export form
    await page.fill('#description', testExport.description);

    // Select type from dropdown
    await page.click('.p-select[id="type"]');
    const typeMap = {
        'full_database': 'Full Database',
        'selected_items': 'Selected Items',
        'locations': 'Locations',
        'areas': 'Areas',
        'commodities': 'Commodities'
    };
    const typeLabel = typeMap[testExport.type as keyof typeof typeMap] || testExport.type;
    await page.click(`.p-select-option-label:has-text("${typeLabel}")`);

    // Set include file data if specified
    if (testExport.includeFileData) {
        await page.check('#includeFileData');
    }

    // If it's a selected items export, we need to select specific items
    if (testExport.type === 'selected_items' && locationName && areaName && commodityName) {
        await recorder.takeScreenshot('exports-create-02-selecting-items');
        
        // Wait for the hierarchical selection tree to load
        await page.waitForSelector('.selection-tree');
        
        // Find and expand the location
        const locationItem = page.locator(`xpath=//span[contains(@class, "item-name") and text()="${locationName}"]/ancestor::div[contains(@class, "location-item")]`);
        await locationItem.waitFor();

        // Get the location ID for later use
        const locationId = await locationItem.getAttribute('data-location_id');
        console.log('Location ID: ' + locationId);
        
        // Click the location to select it
        await locationItem.click();
        
        // Turn off "Include all areas and commodities" to drill down
        const locationToggle = page.locator(`.item-content[data-location_id="${locationId}"]`);
        await locationToggle.waitFor({ state: 'visible' });
        await locationToggle.click();

        // Wait for areas to appear and select the specific area
        // const areaItem = page.locator(`.tree-item.area-item .item-name:has-text("${areaName}")`);
        // await areaItem.waitFor();

        // using xpath to find the area item
        const areaItem = page.locator(`xpath=//span[contains(@class, "item-name") and text()="${areaName}"]/ancestor::div[contains(@class, "area-item")]`);
        await areaItem.waitFor();

        // Get the area ID for later use
        const areaId = await areaItem.getAttribute('data-area_id');
        console.log('Area ID: ' + areaId);

        // Click the area to select it
        await areaItem.click();
        
        // Turn off "Include all commodities" to drill down to specific commodity
        const areaToggle = page.locator(`.item-content[data-area_id="${areaId}"]`);
        await areaToggle.waitFor({ state: 'visible' });
        await areaToggle.click();

        // Wait for commodities to appear and select the specific commodity
        const commodityItem = page.locator(`.tree-item.commodity-item[data-area_id="${areaId}"] .item-name:has-text("${commodityName}")`);
        await commodityItem.waitFor();

        // Click the commodity to select it
        await commodityItem.click();

        // Verify that items are selected (check for visual indicators)
        await expect(locationItem.locator('> .item-header input[type="checkbox"]')).toHaveAttribute('checked');
        await expect(areaItem.locator('> .item-header input[type="checkbox"]')).toHaveAttribute('checked');
        await expect(commodityItem.locator('..').locator('input[type="checkbox"]')).toHaveAttribute('checked');

        await recorder.takeScreenshot('exports-create-03-items-selected');
    }

    // Submit the form
    await page.click('button.btn[type=submit]');

    // Wait to be redirected to the export detail page
    await page.waitForSelector('h1:has-text("Export Details")');
    await page.waitForSelector('h2:has-text("Export Information")');

    // Wait for export to be processed (status should become completed)
    await page.waitForSelector('.card-header .status-badge.status-completed', { timeout: 30000 });

    // Verify the export was created successfully
    await expect(page.locator(`text=${testExport.description}`)).toBeVisible();

    await recorder.takeScreenshot('exports-create-04-after-create');
}

export async function deleteExport(page: Page, recorder: TestRecorder, exportDescription: string) {
    await recorder.takeScreenshot('exports-delete-01-before-delete');

    // Check if we're in detail view
    await page.waitForSelector('h1:has-text("Export Details")');

    // Click the delete button
    await page.click('button:has-text("Delete")');

    // Wait for confirmation dialog
    await page.waitForSelector('.confirmation-modal');
    await recorder.takeScreenshot('exports-delete-02-confirmation');

    // Confirm deletion
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Wait to be redirected back to exports list
    await page.waitForSelector('h1:has-text("Exports")');
    await recorder.takeScreenshot('exports-delete-03-after-delete');
}

export async function verifyExportDetails(page: Page, recorder: TestRecorder, testExport: any) {
    await recorder.takeScreenshot('exports-verify-01-details');
    // Verify type
    const typeMap = {
        'full_database': 'Full Database',
        'selected_items': 'Selected Items',
        'locations': 'Locations',
        'areas': 'Areas',
        'commodities': 'Commodities'
    };
    const expectedTypeLabel = typeMap[testExport.type as keyof typeof typeMap] || testExport.type;

    // Verify Export Information section
    await expect(page.locator('h1:has-text("Export Details")')).toBeVisible();

    // Verify Export Information section
    await expect(page.locator('h2:has-text("Export Information")')).toBeVisible();

    // Verify status (should be one of: Pending, In Progress, Completed, Failed)
    const statusLocator = page.locator('.status-badge');
    await expect(statusLocator).toBeVisible();
    // Verify status text
    await expect(statusLocator).toHaveText('Completed');

    // Verify description
    await expect(page.locator('text=' + testExport.description)).toBeVisible();

    // Verify include file data setting
    await expect(page.locator('text=Include File Data')).toBeVisible();
    const expectedFileDataText = testExport.includeFileData ? 'Yes' : 'No';
    await expect(page.locator(`.bool-badge:has-text("${expectedFileDataText}")`)).toBeVisible();

    // Verify completed date is present
    await expect(page.locator('.info-item > label:has-text("Completed")')).toBeVisible();

    // Verify type is present
    await expect(page.locator('text=Type')).toBeVisible();
    await expect(page.locator(`.type-badge:has-text("${expectedTypeLabel}")`)).toBeVisible();

    // Verify creation date is present
    await expect(page.locator('text=Created')).toBeVisible();

    // Verify file path is present
    await expect(page.locator('text=File Location')).toBeVisible();
    await expect(page.locator('.file-path')).toBeVisible();

    // Verify download button is available
    const downloadButtonCount = await page.locator('button:has-text("Download")').count();
    expect(downloadButtonCount).toBe(2);

    if (testExport.type === 'selected_items') {
        await expect(page.locator(`h2:has-text("${expectedTypeLabel}")`)).toBeVisible();
    }

    // // If there's an error, verify error message section
    // const errorSection = page.locator('.error-card');
    // if (await errorSection.isVisible()) {
    //     await expect(page.locator('h2:has-text("Error Details")')).toBeVisible();
    //     await expect(page.locator('.error-message')).toBeVisible();
    // }

    await recorder.takeScreenshot('exports-verify-02-verified');
}

export async function verifySelectedItems(page: Page, recorder: TestRecorder, expectedItems: {locationName?: string, areaName?: string, commodityName?: string}) {
    await recorder.takeScreenshot('exports-verify-selected-items-01-before');

    // Verify Selected Items section is present
    await expect(page.locator('h2:has-text("Selected Items")')).toBeVisible();

    // Verify count badge shows at least 1 item
    const countBadge = page.locator('.count-badge');
    await expect(countBadge).toBeVisible();
    const countText = await countBadge.textContent();
    expect(countText).toMatch(/\d+ items?/);

    // Wait for items to load (in case there's a loading state)
    await page.waitForSelector('.selected-items-hierarchy', { timeout: 10000 });

    // Verify hierarchical structure is displayed
    await expect(page.locator('.selected-items-hierarchy')).toBeVisible();

    // If specific items are expected, verify they are present
    if (expectedItems.locationName) {
        await expect(page.locator(`.location-item .item-name:has-text("${expectedItems.locationName}")`)).toBeVisible();
        await expect(page.locator(`.location-item .item-type:has-text("Location")`)).toBeVisible();
    }

    if (expectedItems.areaName) {
        await expect(page.locator(`.area-item .item-name:has-text("${expectedItems.areaName}")`)).toBeVisible();
        await expect(page.locator(`.area-item .item-type:has-text("Area")`)).toBeVisible();
    }

    if (expectedItems.commodityName) {
        await expect(page.locator(`.commodity-item .item-name:has-text("${expectedItems.commodityName}")`)).toBeVisible();
        await expect(page.locator(`.commodity-item .item-type:has-text("Commodity")`)).toBeVisible();
    }

    await recorder.takeScreenshot('exports-verify-selected-items-02-verified');
}

