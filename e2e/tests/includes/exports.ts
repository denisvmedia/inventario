import {expect, Page} from "@playwright/test";
import {TestRecorder} from "../../utils/test-recorder.js";

export async function createExport(page: Page, recorder: TestRecorder, testExport: any, locationName?: string, areaName?: string, commodityName?: string) {
    await recorder.takeScreenshot('exports-create-01-before-create');

    // Click the New button to show the export form
    await page.click('a:has-text("New")');
    await page.waitForSelector('h1:has-text("Create Export")');

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
        const locationItem = page.locator(`.tree-item:has-text("${locationName}")`);
        await locationItem.waitFor();
        
        // Click the location to select it
        const locationCheckbox = locationItem.locator('.item-checkbox');
        await locationCheckbox.click();
        
        // Turn off "Include all areas and commodities" to drill down
        const locationToggle = locationItem.locator('.p-toggleswitch');
        if (await locationToggle.isVisible()) {
            await locationToggle.click();
        }
        
        // Wait for areas to appear and select the specific area
        const areaItem = page.locator(`.tree-item:has-text("${areaName}")`);
        await areaItem.waitFor();
        
        const areaCheckbox = areaItem.locator('.item-checkbox');
        await areaCheckbox.click();
        
        // Turn off "Include all commodities" to drill down to specific commodity
        const areaToggle = areaItem.locator('.p-toggleswitch');
        if (await areaToggle.isVisible()) {
            await areaToggle.click();
        }
        
        // Wait for commodities to appear and select the specific commodity
        const commodityItem = page.locator(`.tree-item:has-text("${commodityName}")`);
        await commodityItem.waitFor();
        
        const commodityCheckbox = commodityItem.locator('.item-checkbox');
        await commodityCheckbox.click();
        
        await recorder.takeScreenshot('exports-create-03-items-selected');
    }

    // Submit the form
    await page.click('button:has-text("Create Export")');

    // Wait to be redirected to the exports list
    await page.waitForSelector('h1:has-text("Exports")');
    await recorder.takeScreenshot('exports-create-04-after-create');
}

export async function deleteExport(page: Page, recorder: TestRecorder, exportDescription: string) {
    await recorder.takeScreenshot('exports-delete-01-before-delete');

    // Navigate to the export if not already there
    const exportRow = page.locator(`tr:has-text("${exportDescription}")`);
    if (await exportRow.isVisible()) {
        // Click on the export row to go to detail view
        await exportRow.click();
    } else {
        // If we're already in detail view, great
        await page.waitForSelector('h1:has-text("Export Details")');
    }

    // Click the delete button
    await page.click('button:has-text("Delete")');

    // Wait for confirmation dialog
    await page.waitForSelector('.confirmation-dialog');
    await recorder.takeScreenshot('exports-delete-02-confirmation');

    // Confirm deletion
    await page.click('.confirmation-dialog button:has-text("Delete")');

    // Wait to be redirected back to exports list
    await page.waitForSelector('h1:has-text("Exports")');
    await recorder.takeScreenshot('exports-delete-03-after-delete');
}

export async function verifyExportDetails(page: Page, recorder: TestRecorder, testExport: any) {
    await recorder.takeScreenshot('exports-verify-01-details');

    // Verify description
    await expect(page.locator('text=' + testExport.description)).toBeVisible();

    // Verify type
    const typeMap = {
        'full_database': 'Full Database',
        'selected_items': 'Selected Items',
        'locations': 'Locations',
        'areas': 'Areas',
        'commodities': 'Commodities'
    };
    const expectedTypeLabel = typeMap[testExport.type as keyof typeof typeMap] || testExport.type;
    await expect(page.locator(`text=${expectedTypeLabel}`)).toBeVisible();

    // Verify status (should be one of: Pending, In Progress, Completed, Failed)
    const statusLocator = page.locator('.status-badge');
    await expect(statusLocator).toBeVisible();
    
    // Verify creation date is present
    await expect(page.locator('text=Created Date')).toBeVisible();

    await recorder.takeScreenshot('exports-verify-02-verified');
}

// Constants for navigation
export const TO_EXPORT_CREATE = 'export-create';
export const TO_EXPORT_DETAIL = 'export-detail';
export const BACK_TO_EXPORTS = 'back-to-exports';