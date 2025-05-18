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
