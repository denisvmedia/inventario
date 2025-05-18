import {expect, Page} from "@playwright/test";
import {TestRecorder} from "../../utils/test-recorder.js";

export const uploadFile = async (page: Page, recorder: TestRecorder, selectorBase: string, filePath: string) => {
    // Scroll to the ${selectorBase} section
    await page.evaluate((selectorBase: string) => {
        const imagesSection = document.querySelector(`${selectorBase}`);
        if (imagesSection) {
            imagesSection.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
    }, selectorBase);

    await page.click(`${selectorBase} .section-header .btn-primary`);
    await page.setInputFiles(`${selectorBase} input[type="file"]`, filePath);
    await page.evaluate((selectorBase: string) => {
        const fileInput = document.querySelector(`${selectorBase} .file-input`);
        if (fileInput) {
            // Create and dispatch a change event
            const event = new Event('change', {bubbles: true});
            fileInput.dispatchEvent(event);
        }
    }, selectorBase);
    // strip non-latin characters from selector
    const screenshotBase = selectorBase.replace(/[^a-zA-Z0-9-]/g, '');

    await recorder.takeScreenshot(`${screenshotBase}-upload`);
    await page.click(`${selectorBase} .upload-actions button:has-text("Upload Files")`);

    // Verify image is displayed
    await expect(page.locator(`${selectorBase} .file-item`)).toBeVisible();
    await recorder.takeScreenshot(`${screenshotBase}-displayed`);
};

export const downloadFile = async (page: Page, recorder: TestRecorder, selector: string, fileType: string) => {
    // First get the file item that should be visible now
    const fileItem = page.locator(`${selector} .file-item`).first();
    await expect(fileItem).toBeVisible();

    // Click the download button within the file item - adjust this selector based on your UI
    const downloadPromise = page.waitForEvent('download');
    await fileItem.locator('.file-actions .btn-primary').click();

    // Wait for the download to complete with timeout
    const download = await downloadPromise;

    // Get the suggested filename
    const suggestedFilename = download.suggestedFilename();
    console.log(`Downloaded file: ${suggestedFilename}`);

    // Save to a temp path to verify it exists
    const filePath = await download.path();
    expect(filePath).toBeTruthy();

    // Take screenshot after download
    await recorder.takeScreenshot(`${fileType}-download-success`);
    console.log(`${fileType} downloaded successfully`);
};

export const deleteFile = async (page: Page, recorder: TestRecorder, selector: string, fileType: string) => {
    // Get the file item
    const fileItem = page.locator(`${selector} .file-item`).first();
    await expect(fileItem).toBeVisible();

    // Find and click the delete button
    await fileItem.locator('.file-actions .btn-danger').click();

    await recorder.takeScreenshot(`file-delete-${fileType}-confirm`);
    await page.click('.confirmation-modal button:has-text("Delete")');
    await recorder.takeScreenshot(`filed-delete-${fileType}-deleted`);

    // Verify file is no longer visible
    await expect(page.locator(`${selector} .file-item`)).not.toBeVisible();

    // Take screenshot after deletion
    await recorder.takeScreenshot(`${fileType}-deletion-success`);
    console.log(`${fileType} deleted successfully`);
};
