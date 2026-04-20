import {expect, Page} from "@playwright/test";
import {TestRecorder} from "../../utils/test-recorder.js";

export async function createLocation(page: Page, recorder: TestRecorder, testLocation: any): Promise<string> {
    await recorder.takeScreenshot('locations-create-01-before-create');

    // Click the New button to show the location form
    await page.click('button:has-text("New")');

    // Fill in the location form
    await page.fill('#name', testLocation.name);
    await page.fill('#address', testLocation.address);
    await recorder.takeScreenshot('location-create-02-form-filled');

    // Submit the form
    await page.click('button:has-text("Create Location")');

    // Wait for the location to be created and displayed
    await page.waitForSelector(`.location-card:has-text("${testLocation.name}")`);
    await recorder.takeScreenshot('location-create-03-created');

    // Capture the newly-created location's ID so the caller can delete the
    // exact same card later. Using `.last()` picks the just-created entry when
    // earlier runs (e.g. the CI warmup invocation) left another location with
    // the same name behind — without an ID the subsequent deleteLocation would
    // hit the orphan and get a 422 "contains areas".
    const createdCard = page.locator(`.location-card:has-text("${testLocation.name}")`).last();
    const locationId = await createdCard.getAttribute('data-location-id');
    if (!locationId) {
        throw new Error(`createLocation: could not read data-location-id after creating "${testLocation.name}"`);
    }
    return locationId;
}

export async function deleteLocation(page: Page, recorder: TestRecorder, locationName: string, locationId?: string) {
    // Prefer the ID when the caller has one — name-based lookup is ambiguous
    // if previous test invocations (the CI warmup, a prior retry, a sibling
    // test sharing the same describe-level testLocation) left a same-named
    // orphan card behind.
    const locationCard = locationId
        ? page.locator(`.location-card[data-location-id="${locationId}"]`)
        : page.locator(`.location-card:has-text("${locationName}")`).last();
    await locationCard.waitFor({ state: 'visible', timeout: 10000 });

    // Without an explicit ID we still need one for the DELETE waitForResponse
    // predicate and the post-delete toHaveCount check.
    const targetId = locationId ?? await locationCard.getAttribute('data-location-id');
    if (!targetId) {
        throw new Error(`deleteLocation: could not read data-location-id from card "${locationName}"`);
    }

    // Click the delete button
    await locationCard.locator('button[title="Delete"]').click();
    await recorder.takeScreenshot('location-delete-01-confirm');

    // Wait for confirmation modal to be visible
    await page.locator('.confirmation-modal').waitFor({ state: 'visible', timeout: 5000 });

    // Click the delete button in the confirmation modal and wait for the API response.
    // Data API calls go through /api/v1/g/{groupSlug}/locations/... after the
    // Location Groups refactor (the axios interceptor rewrites the url), so match
    // on /locations/<id> rather than the pre-rewrite prefix. Timeout is 30s
    // because cascaded deletes (areas + commodities) can exceed 10s under CI load.
    await Promise.all([
        page.waitForResponse(response =>
            new URL(response.url()).pathname.endsWith(`/locations/${targetId}`) &&
            response.request().method() === 'DELETE' &&
            response.status() === 204,
            { timeout: 30000 }
        ),
        page.click('.confirmation-modal button:has-text("Delete")')
    ]);

    // Wait for the confirmation modal to disappear
    await page.locator('.confirmation-modal').waitFor({ state: 'hidden', timeout: 5000 });

    // Assert the *specific* location we deleted is gone. Checking by name
    // substring is unreliable: a previous retry can leave a sibling card with
    // the same base name behind, which makes the :has-text match non-unique
    // even though the deletion itself succeeded. Match on the stable ID.
    await expect(page.locator(`.location-card[data-location-id="${targetId}"]`)).toHaveCount(0, { timeout: 15000 });

    await recorder.takeScreenshot('location-delete-02-deleted');

    // Verify we're still on the locations page
    await expect(page).toHaveURL(/\/locations/);
}
