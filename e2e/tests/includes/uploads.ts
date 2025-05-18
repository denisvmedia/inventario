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

export const fileinfo = async (page: Page, recorder: TestRecorder, selector: string, fileType: string) => {
    // Get the file item
    const fileItem = page.locator(`${selector} .file-item`).first();
    await expect(fileItem).toBeVisible();

    // Click the details/info button
    await fileItem.locator('.file-actions button.btn-info').click();
    await recorder.takeScreenshot(`${fileType}-details-dialog`);

    // Verify the dialog is displayed
    const detailsDialog = page.locator('.file-details-modal');
    await expect(detailsDialog).toBeVisible();

    // Verify file name and original name are displayed
    await expect(detailsDialog.locator('.file-name')).toBeVisible();
    await expect(detailsDialog.locator('.file-original-name')).toBeVisible();

    // Verify appropriate preview is shown based on file type
    if (fileType === 'image') {
        // For images, verify an actual image is displayed
        await expect(detailsDialog.locator('.image-preview img')).toBeVisible();
    } else {
        // For PDFs, verify the PDF icon is displayed
        await expect(detailsDialog.locator('.file-icon-preview .fa-file-pdf')).toBeVisible();
    }

    // Close the dialog
    await detailsDialog.locator('button.action-close').click();
    await expect(detailsDialog).not.toBeVisible();

    await recorder.takeScreenshot(`${fileType}-details-closed`);
};
