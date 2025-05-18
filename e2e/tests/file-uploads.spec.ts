// filepath: d:\Work\coding\projects\buster\inventario\e2e\tests\file-uploads.spec.ts
import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import path from 'path';
import {createLocation} from "./includes/locations.js";
import {createArea, verifyAreaHasCommodities} from "./includes/areas.js";
import {createCommodity, verifyCommodityDetails} from "./includes/commodities.js";
import {FROM_LOCATIONS_AREA, navitateTo, TO_AREA_COMMODITIES, TO_LOCATIONS} from "./includes/navigate.js";
import {deleteFile, downloadFile, fileinfo, uploadFile} from "./includes/uploads.js";

test.describe('File Uploads and Properties Tests', () => {
  // Test data with timestamps to ensure uniqueness
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location for Files ${timestamp}`,
    address: '123 File Test Street, Test City'
  };

  const testArea = {
    name: `Test Area for Files ${timestamp}`
  };

  const testCommodity = {
    name: `Test Commodity for Files ${timestamp}`,
    shortName: 'TestFiles',
    type: 'Electronics',
    count: 1,
    originalPrice: 100,
    originalPriceCurrency: 'CZK',
    purchaseDate: new Date().toISOString().split('T')[0], // Today's date in YYYY-MM-DD format
    status: 'In Use'
  };

  // File paths for test uploads
  const testImagePath = path.join('fixtures', 'files', 'image.jpg');
  const testManualPath = path.join('fixtures', 'files', 'manual.pdf');
  const testInvoicePath = path.join('fixtures', 'files', 'invoice.pdf');

  test('should upload and validate image, manual, and invoice files', async ({ page, recorder }) => {
    let step = 1;

    // STEP 1: CREATE LOCATION - First create a location
    console.log(`Step ${step++}: Creating a new location`);
    await navitateTo(page, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: CREATE AREA - Create a new area
    console.log(`Step ${step++}: Creating a new area`);
    await createArea(page, recorder, testArea)

    // STEP 3: CREATE COMMODITY - Create a new commodity
    console.log(`Step ${step++}: Creating a new commodity`);
    await navitateTo(page, TO_AREA_COMMODITIES, FROM_LOCATIONS_AREA, testArea.name);
    await verifyAreaHasCommodities(page, recorder);
    await createCommodity(page, recorder, testCommodity);

    // STEP 4: READ - Verify the commodity details
    console.log(`Step ${step++}: Verifying the commodity details`);
    await verifyCommodityDetails(page, testCommodity);

    // STEP 5: UPLOAD IMAGE - Upload an image to the commodity
    console.log(`Step ${step++}: Uploading an image`);
    await uploadFile(page, recorder, '.commodity-images', testImagePath);

    // STEP 6: UPLOAD MANUAL - Upload a manual to the commodity
    console.log(`Step ${step++}: Uploading a manual`);
    await uploadFile(page, recorder, '.commodity-manuals', testManualPath);

    // STEP 7: UPLOAD INVOICE - Upload an invoice to the commodity
    console.log(`Step ${step++}: Uploading an invoice`);
    await uploadFile(page, recorder, '.commodity-invoices', testInvoicePath);

    // STEP 8: Check file properties by looking at displayed information
    console.log(`Step ${step++}: Testing file properties dialog`);

    // For each file type, test file details view
    for (const { selector, fileType } of [
      { selector: '.commodity-images', fileType: 'image' },
      { selector: '.commodity-manuals', fileType: 'manual' },
      { selector: '.commodity-invoices', fileType: 'invoice' }
    ]) {
      console.log(`Testing file details for ${fileType}`);
      await fileinfo(page, recorder, selector, fileType);
    }

    // Wait to ensure all uploads are processed and displayed
    await page.waitForTimeout(1000);

    // Verify files are visible in the UI
    for (const selector of ['.commodity-images', '.commodity-manuals', '.commodity-invoices']) {
      await expect(page.locator(`${selector} .file-item`)).toBeVisible();
    }

    // STEP 9: TEST FILE DOWNLOAD - Verify that files can be downloaded
    console.log(`Step ${step++}: Testing file downloads`);

    // For each file type, test downloads
    for (const { selector, fileType } of [
      { selector: '.commodity-images', fileType: 'image' },
      { selector: '.commodity-manuals', fileType: 'manual' },
      { selector: '.commodity-invoices', fileType: 'invoice' }
    ]) {
      console.log(`Testing download for ${fileType}`);
      await downloadFile(page, recorder, selector, fileType);
    }

    // STEP 10: TEST PDF VIEWER - Verify that PDFs can be viewed
    console.log(`Step ${step++}: Testing PDF viewer`);

    // STEP 11: TEST Image viewer - Verify that images can be viewed
    console.log(`Step ${step++}: Testing image viewer`);

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
    console.log(`Image style: ${imageStyleOriginal}`);

    // Test dragging the zoomed image
    await page.mouse.move(400, 300);
    await page.mouse.down();
    await page.waitForSelector('.image-container img[style*="cursor: grabbing"]');
    console.log("Cursor is changed to grabbing. Dragging image...")
    await page.mouse.move(500, 350);
    await page.mouse.up();
    await page.waitForSelector('.image-container img[style*="cursor: grab"]');
    console.log("Cursor is changed to grab.");
    // compare imageStyleOriginal with current image style
    const imageStyleAfterDrag = await previewImage.evaluate((img) => img.style.transform);
    console.log(`Image style after drag: ${imageStyleAfterDrag}`);
    expect(imageStyleAfterDrag).not.toEqual(imageStyleOriginal);
    await recorder.takeScreenshot('image-dragged');

    // Test zoom out functionality
    console.log("Clicking image to zoom out...")
    await previewImage.click();
    await page.waitForSelector('.image-container img[style*="cursor: zoom-in"]');
    console.log("Cursor is changed to zoom-in.");
    await recorder.takeScreenshot('image-zoomed-out');

    // Test closing the dialog
    const closeButton = imageViewerModal.locator('.file-actions .btn-secondary');
    await closeButton.click();
    await expect(imageViewerModal).not.toBeVisible();
    await recorder.takeScreenshot('image-viewer-closed');

    // STEP 12: CLEANUP - Delete the test image, manual, and invoice
    console.log(`Step ${step++}: Cleaning up - deleting the test files`);
    // For each file type, delete files
    for (const { selector, fileType } of [
      { selector: '.commodity-images', fileType: 'image' },
      { selector: '.commodity-manuals', fileType: 'manual' },
      { selector: '.commodity-invoices', fileType: 'invoice' }
    ]) {
      console.log(`Deleting ${fileType}`);
      await deleteFile(page, recorder, selector, fileType);
    }
  });
});
