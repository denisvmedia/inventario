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

    // const button = await page.locator(`${selectorBase} .upload-actions button:has-text("Upload Files")`);
    // const box = await button.boundingBox();
    // if (box) {
    //     const {locationX, locationY} = {locationX: box.x + box.width / 2, locationY: box.y + box.height / 2};
    //     await page.mouse.move(locationX, locationY);
    //     await page.mouse.click(locationX, locationY);
    // }

    // Verify image is displayed
    await expect(page.locator(`${selectorBase} .file-item`).first()).toBeVisible();
    await recorder.takeScreenshot(`${screenshotBase}-displayed`);
};

export const downloadFile = async (page: Page, recorder: TestRecorder, selector: string, fileType: string) => {
    // First get the file item that should be visible now
    const fileItem = page.locator(`${selector} .file-item`).first();
    await expect(fileItem).toBeVisible();

    // Get the file ID from data attributes
    const fileId = await fileItem.getAttribute('data-file-id');

    if (!fileId) {
        throw new Error(`Could not get file ID for ${fileType}. ID: ${fileId}`);
    }

    recorder.log(`Testing download for ${fileType} with file ID: ${fileId}`);

    // Get authentication token for API requests
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token'));

    // Step 1: Generate signed URL by calling the signing API
    // Import CSRF helper
    const { getCsrfToken } = await import('./csrf.js');
    const csrfToken = getCsrfToken();

    const headers: Record<string, string> = {
        'Authorization': `Bearer ${authToken}`,
        'Content-Type': 'application/json'
    };

    // Add CSRF token for state-changing requests
    if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
    }

    const signedUrlResponse = await page.request.post(`/api/v1/files/${fileId}/signed-url`, {
        headers
    });

    expect(signedUrlResponse.status()).toBe(200);

    const signedUrlData = await signedUrlResponse.json();
    recorder.log(`Signed URL response:`, signedUrlData);

    // Extract the signed URL from JSON:API response format
    const signedUrl = signedUrlData.attributes.url;

    if (!signedUrl) {
        throw new Error(`Could not get signed URL for ${fileType}. Response: ${JSON.stringify(signedUrlData)}`);
    }

    recorder.log(`Signed download URL for ${fileType}: ${signedUrl}`);

    // Step 2: Verify the signed URL is accessible by making a GET request with range header
    // This will only download the first byte to verify the file exists and is accessible
    const response = await page.request.get(signedUrl, {
        headers: {
            'Range': 'bytes=0-0'
        }
    });

    // Accept both 200 (full content) and 206 (partial content) as success
    expect([200, 206]).toContain(response.status());

    // Get the content-disposition header to verify filename
    const contentDisposition = response.headers()['content-disposition'];
    if (contentDisposition) {
        recorder.log(`Content-Disposition header: ${contentDisposition}`);
    }

    // Step 3: Click the download button to trigger the download (this tests the frontend flow)
    await fileItem.locator('.file-actions .btn-primary').click();

    // Take screenshot after download action
    await recorder.takeScreenshot(`${fileType}-download-success`);
    recorder.log(`${fileType} download verified successfully`);
};

export const deleteFile = async (page: Page, recorder: TestRecorder, selector: string, fileType: string) => {
    // Get the file item
    const fileItem = page.locator(`${selector} .file-item`).first();
    await expect(fileItem).toBeVisible();

    // Get the count of file items before deletion
    const fileItemsBefore = await page.locator(`${selector} .file-item`).count();

    // Find and click the delete button
    await fileItem.locator('.file-actions .btn-danger').click();

    await recorder.takeScreenshot(`file-delete-${fileType}-confirm`);
    await page.click('.confirmation-modal button:has-text("Delete")');

    // Wait for the file to be removed from the DOM by checking the count decreased
    await page.waitForFunction(
        ({ selector, expectedCount }) => {
            const items = document.querySelectorAll(`${selector} .file-item`);
            return items.length === expectedCount;
        },
        { selector, expectedCount: fileItemsBefore - 1 },
        { timeout: 10000 }
    );

    await recorder.takeScreenshot(`filed-delete-${fileType}-deleted`);

    // Verify file is no longer visible (should have one less item)
    await expect(page.locator(`${selector} .file-item`)).toHaveCount(fileItemsBefore - 1);

    // Take screenshot after deletion
    await recorder.takeScreenshot(`${fileType}-deletion-success`);
    recorder.log(`${fileType} deleted successfully`);
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

export const imageviewer = async (page: Page, recorder: TestRecorder) => {
    // Get the image file item
    const imageFileItem = page.locator('.commodity-images .file-item').first();
    await expect(imageFileItem).toBeVisible();

    // Click the file preview to open the image viewer
    await imageFileItem.locator('.file-preview').click();
    await recorder.takeScreenshot('image-viewer-opened');

    // Verify the modal dialog is visible
    const imageViewerModal = page.locator('.file-modal');
    await expect(imageViewerModal).toBeVisible();

    // Verify the image is displayed
    const previewImage = imageViewerModal.locator('.image-container img');
    await expect(previewImage).toBeVisible();

    // Verify image name is in the dialog title
    const modalTitle = imageViewerModal.locator('.modal-header h3');
    await expect(modalTitle).toBeVisible();
    await expect(modalTitle).toHaveText(/.+/); // Title should contain text

    const imageCursorInitial = await previewImage.evaluate((img) => img.style.cursor);
    expect(imageCursorInitial).toEqual('zoom-in');

    // Test zoom in functionality
    // test click zooms in
    await previewImage.click();
    // check if previewImage has class .zoomed
    await page.waitForSelector('.image-container img.zoomed');
    // wait for selector that will check image cursor grab
    await page.waitForSelector('.image-container img[style*="cursor: grab"]');

    await page.waitForSelector('.image-container img.zoomed');
    const imageCursorZoomed = await previewImage.evaluate((img) => img.style.cursor);
    expect(imageCursorZoomed).toEqual('grab');
    await recorder.takeScreenshot('image-zoomed-in');

    // read img style attribute
    const imageStyleOriginal = await previewImage.evaluate((img) => img.style.transform);
    recorder.log(`Image style: ${imageStyleOriginal}`);

    // Test dragging the zoomed image
    await page.mouse.move(400, 300);
    await page.mouse.down();
    await page.waitForSelector('.image-container img[style*="cursor: grabbing"]');
    recorder.log("Cursor is changed to grabbing. Dragging image...")
    await page.mouse.move(500, 350);
    await page.mouse.up();
    await page.waitForSelector('.image-container img[style*="cursor: grab"]');
    recorder.log("Cursor is changed to grab.");
    // compare imageStyleOriginal with current image style
    const imageStyleAfterDrag = await previewImage.evaluate((img) => img.style.transform);
    recorder.log(`Image style after drag: ${imageStyleAfterDrag}`);
    expect(imageStyleAfterDrag).not.toEqual(imageStyleOriginal);
    await recorder.takeScreenshot('image-dragged');

    // Test zoom out functionality
    recorder.log("Clicking image to zoom out...")
    await previewImage.click();
    await page.waitForSelector('.image-container img[style*="cursor: zoom-in"]');
    recorder.log("Cursor is changed to zoom-in.");
    await recorder.takeScreenshot('image-zoomed-out');

    // Test closing the dialog
    const closeButton = imageViewerModal.locator('.file-actions .btn-secondary');
    await closeButton.click();
    await expect(imageViewerModal).not.toBeVisible();
    await recorder.takeScreenshot('image-viewer-closed');
};
