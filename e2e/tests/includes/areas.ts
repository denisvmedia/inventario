import { expect, Page } from "@playwright/test";
import { TestRecorder } from "../../utils/test-recorder.js";

// React port of the Vue-era areas helper. Areas now live as inline items
// inside each LocationCard (`[data-testid="location-card-area"]`); creation
// goes through `AreaFormDialog`, deletion through the per-item trash button
// + the shared useConfirm dialog (`[data-testid="confirm-dialog"]`).

export async function createArea(
    page: Page,
    recorder: TestRecorder,
    testArea: { name: string },
) {
    // Open the AreaFormDialog from the inline button on the parent location's
    // card. Each LocationCard renders its own `location-card-add-area` —
    // the test ensures a single location is in the visible set, so the first
    // matching button is the right one.
    await page.locator('[data-testid="location-card-add-area"]').first().click();
    await page.waitForSelector('[data-testid="area-form-dialog"]');

    // Fill + submit. The `area-location` select is hidden when only one
    // location exists (typical in a fresh test run); we don't fight it.
    await page.fill('#area-name', testArea.name);
    await recorder.takeScreenshot('area-create-01-form-filled');

    await page.click('[data-testid="area-form-submit"]');

    // Wait for the rendered area row inside the location card.
    await page.waitForSelector(
        `[data-testid="location-card-area"]:has-text("${testArea.name}")`,
    );
    await recorder.takeScreenshot('area-create-02-created');
}

export async function deleteArea(
    page: Page,
    recorder: TestRecorder,
    areaName: string,
    locationName?: string,
) {
    // The area row lives inside its location card on the locations list page.
    // We don't need the explicit location-card-by-name lookup the Vue helper
    // had — only the Vue legacy area-card layout needed it; in React the row
    // is uniquely identified by its inline text under any visible card.
    const areaRow = page.locator(
        `[data-testid="location-card-area"]:has-text("${areaName}")`,
    );

    if (locationName && !(await areaRow.isVisible())) {
        // If the caller hints which parent card to expand, click into it
        // first. The locations list shows everything inline by default; this
        // is a safety net for tests that navigated away.
        await page
            .locator(`[data-testid="location-card"]:has-text("${locationName}")`)
            .click();
    }

    await areaRow.waitFor({ state: 'visible', timeout: 10000 });

    // The delete button is the only button in the row. Each area row renders
    // a Link (the area title) plus a single Button (trash icon, aria-labelled
    // with the area name).
    await areaRow.locator('button').click();
    await recorder.takeScreenshot('area-delete-01-confirm');

    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'visible', timeout: 5000 });

    // Confirm + wait for DELETE. Match on `/areas/<uuid>$` — the URL prefix
    // is `/api/v1/g/{slug}/areas/...` after the Location Groups refactor.
    await Promise.all([
        page.waitForResponse(
            (response) =>
                /\/areas\/[0-9a-f-]+$/.test(new URL(response.url()).pathname) &&
                response.request().method() === 'DELETE' &&
                response.status() === 204,
            { timeout: 10000 },
        ),
        page.click('[data-testid="confirm-accept"]'),
    ]);

    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'hidden', timeout: 5000 });

    await expect(
        page.locator(`[data-testid="location-card-area"]:has-text("${areaName}")`),
    ).toHaveCount(0, { timeout: 15000 });

    await recorder.takeScreenshot('area-delete-02-deleted');
    await expect(page).toHaveURL(/\/locations/);
}

// Renamed-and-repurposed: post-cutover the area detail page is a
// ComingSoon stub and item creation flows through the top-level
// /commodities page. The Vue-era contract was "this area has no
// commodities yet" (verified via the now-gone area-detail page); the
// React /commodities list is flat and always shows every commodity in
// the group, so the original assertion is no longer meaningful when
// the e2e seed dataset starts with several pre-existing items.
//
// What we *can* still assert: the commodities list page rendered (so
// the next createCommodity() call has a known DOM to drive). Any of
// the list states — empty, filtered-empty, grid, table, or even the
// loading skeleton — qualifies; we only care that we're on the right
// page wrapper, not what's inside it.
export async function verifyAreaHasCommodities(page: Page, recorder: TestRecorder) {
    await page.locator('[data-testid="page-commodities"]').waitFor({
        state: 'visible',
        timeout: 10000,
    });
    await recorder.takeScreenshot('area-verify-no-commodities-01');
}
