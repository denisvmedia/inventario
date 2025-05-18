// filepath: d:\Work\coding\projects\buster\inventario\e2e\tests\file-uploads.spec.ts
import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import path from 'path';
import {createLocation} from "./includes/locations.js";
import {createArea, verifyAreaHasCommodities} from "./includes/areas.js";
import {createCommodity, verifyCommodityDetails} from "./includes/commodities.js";
import {FROM_LOCATIONS_AREA, navitateTo, TO_AREA_COMMODITIES, TO_LOCATIONS} from "./includes/navigate.js";
import {deleteFile, downloadFile, fileinfo, imageviewer, uploadFile} from "./includes/uploads.js";

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
    await page.click('.commodity-manuals .file-item .file-preview');
    await page.waitForSelector('.file-modal', { state: 'visible' });
    await recorder.takeScreenshot('pdf-viewer-opened');

    // Test paginated mode (default)
    const nextButton = page.locator('.pdf-navigation-next');
    const prevButton = page.locator('.pdf-navigation-prev');
    const pageIndicator = page.locator('.page-info');

    // Check initial page info
    await expect(pageIndicator).toBeVisible();
    const initialPageText = await pageIndicator.textContent();
    console.log(`Initial page text: ${initialPageText}`);
    expect(initialPageText).toMatch(/1 \/ \d+/);

    // Extract total pages
    const totalPagesMatch = initialPageText?.match(/\/ (\d+)/) ?? [];
    const totalPages = totalPagesMatch ? parseInt(totalPagesMatch[1] || '0') : 0;

    // Test pagination if multiple pages
    if (totalPages > 1) {
      console.log(`Total pages: ${totalPages}`);
      await nextButton.click();
      await expect(pageIndicator).toContainText('2 /');
      await recorder.takeScreenshot('pdf-viewer-page-2');

      await prevButton.click();
      await expect(pageIndicator).toContainText('1 /');
    } else {
      console.log('Only one page, skipping pagination test');
    }

    // Test container scrollability
    console.log('Testing container scrollability');
    const pdfContainer = page.locator('.pdf-view > .pdf-container');
    await expect(pdfContainer).toBeVisible();
    const initialScrollTop = await pdfContainer.evaluate(el => el.scrollTop);
    await pdfContainer.evaluate(el => el.scrollBy(0, 100));
    const afterScrollTop = await pdfContainer.evaluate(el => el.scrollTop);
    expect(afterScrollTop).toBeGreaterThan(initialScrollTop);

    // Test zoom in paginated mode
    const zoomInButton = page.locator('.pdf-zoom-in');
    const zoomOutButton = page.locator('.pdf-zoom-out');

    console.log('Testing zoom in/out in paginated mode');
    await zoomInButton.click();
    await recorder.takeScreenshot('pdf-viewer-zoomed-in');
    await zoomOutButton.click();

    // Switch to "view all pages" mode
    const pdfViewModeAllPages = page.locator('.pdf-view-mode-all-pages');
    console.log('Switching to view all pages mode');
    await pdfViewModeAllPages.click();
    await recorder.takeScreenshot('pdf-viewer-all-pages-mode');

    // Verify pagination buttons are disabled in all-pages mode
    console.log('Verifying pagination buttons are disabled in all-pages mode');
    await expect(nextButton).toBeDisabled();
    await expect(prevButton).toBeDisabled();

    // Page indicator should still show pages info
    await expect(pageIndicator).toContainText(`/ ${totalPages}`);

    // Test scrolling updates current page in all-pages mode
    if (totalPages > 1) {
      console.log('Testing scrolling updates current page in all-pages mode');

      // Get height of a single page
      const pageHeight = await page.evaluate(() => {
        const firstPage = document.querySelector('.pdf-page') as HTMLElement;

        return firstPage ? firstPage.offsetHeight : 0;
      });

      // Scroll to second page
      await pdfContainer.evaluate((el, height) => {
        el.scrollTop = height + 10;
      }, pageHeight);

      console.log('Scrolling to second page...');
      // Wait for page indicator to update
      await page.waitForFunction(
        () => document.querySelector('.page-info')?.textContent?.includes('2 /'),
        { timeout: 5000 }
      );

      await recorder.takeScreenshot('pdf-viewer-scrolled-to-page-2');
    }

    // Test zoom in all-pages mode
    console.log('Testing zoom in/out in all-pages mode');
    await zoomInButton.click();
    await recorder.takeScreenshot('pdf-viewer-all-pages-zoomed-in');

    // Close the viewer
    console.log('Closing the PDF viewer');
    await page.click('.file-modal .btn-secondary');
    await expect(page.locator('.file-modal')).toBeHidden();

    // STEP 11: TEST Image viewer - Verify that images can be viewed
    console.log(`Step ${step++}: Testing image viewer`);
    await imageviewer(page, recorder);

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
