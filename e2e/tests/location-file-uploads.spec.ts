import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import path from 'path';
import { createLocation, deleteLocation } from './includes/locations.js';
import { navigateTo, TO_LOCATIONS } from './includes/navigate.js';
import { deleteFile, downloadFile, fileinfo, uploadFile } from './includes/uploads.js';

// Helper: navigate to location detail page by clicking the View button
async function navigateToLocationDetail(page: any, recorder: any, locationName: string) {
  const locationCard = page.locator(`.location-card:has-text("${locationName}")`).first();
  await locationCard.waitFor({ state: 'visible', timeout: 10000 });

  // Click the View button to navigate directly to /locations/{id}
  await locationCard.locator('button[title="View"]').click();
  await page.waitForURL(/\/locations\/[^/]+$/, { timeout: 10000 });
  await page.waitForSelector('.location-images', { timeout: 10000 });
  await recorder.takeScreenshot('location-detail-page');
}

test.describe('Location File Uploads Tests', () => {
  const timestamp = Date.now();
  const testLocation = {
    name: `Test Location for File Uploads ${timestamp}`,
    address: '42 Upload Test Street, Test City'
  };

  const testImagePath = path.join('fixtures', 'files', 'image.jpg');
  const testFilePath = path.join('fixtures', 'files', 'manual.pdf');

  test('should upload, view info, download and delete image and file on a location', async ({ page, recorder }) => {
    let step = 1;

    // STEP 1: Navigate to Locations and create a location
    recorder.log(`Step ${step++}: Creating a new location`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, testLocation);

    // STEP 2: Navigate to the location detail page
    recorder.log(`Step ${step++}: Navigating to location detail page`);
    await navigateToLocationDetail(page, recorder, testLocation.name);

    // STEP 3: Upload an image
    recorder.log(`Step ${step++}: Uploading an image to the location`);
    await uploadFile(page, recorder, '.location-images', testImagePath);

    // STEP 4: Upload a generic file (PDF)
    recorder.log(`Step ${step++}: Uploading a file to the location`);
    await uploadFile(page, recorder, '.location-files', testFilePath);

    // STEP 5: Verify both sections show the uploaded files
    recorder.log(`Step ${step++}: Verifying uploaded files are visible`);
    await expect(page.locator('.location-images .file-item')).toBeVisible();
    await expect(page.locator('.location-files .file-item')).toBeVisible();
    await recorder.takeScreenshot('location-files-uploaded');

    // STEP 6: Check file properties (info dialog) for both sections
    recorder.log(`Step ${step++}: Testing file info dialog`);
    for (const { selector, fileType } of [
      { selector: '.location-images', fileType: 'image' },
      { selector: '.location-files', fileType: 'file' }
    ]) {
      recorder.log(`Checking file info for ${fileType}`);
      await fileinfo(page, recorder, selector, fileType);
    }

    // STEP 7: Download both files via signed URL
    recorder.log(`Step ${step++}: Testing file downloads`);
    for (const { selector, fileType } of [
      { selector: '.location-images', fileType: 'image' },
      { selector: '.location-files', fileType: 'file' }
    ]) {
      recorder.log(`Downloading ${fileType}`);
      await downloadFile(page, recorder, selector, fileType);
    }

    // STEP 8: Delete both files
    recorder.log(`Step ${step++}: Deleting uploaded files`);
    for (const { selector, fileType } of [
      { selector: '.location-images', fileType: 'image' },
      { selector: '.location-files', fileType: 'file' }
    ]) {
      recorder.log(`Deleting ${fileType}`);
      await deleteFile(page, recorder, selector, fileType);
    }

    // STEP 9: Cleanup — delete the location
    recorder.log(`Step ${step++}: Cleaning up — deleting the location`);
    await navigateTo(page, recorder, TO_LOCATIONS);
    await deleteLocation(page, recorder, testLocation.name);
  });
});
