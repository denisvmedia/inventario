import { expect, Page } from "@playwright/test";
import { TestRecorder } from "../../utils/test-recorder.js";

// Post-#1531 (item 2), the locations list no longer renders areas inline.
// Areas show as tile cards inside the location detail page
// (`[data-testid="location-detail-area"]`). Add-area still has an entry
// point on the list — it now lives inside the location card's dropdown
// menu (`location-card-menu` → `location-card-add-area`). Deletion fires
// from the per-tile dropdown on the location detail page
// (`location-detail-area-menu` → `location-detail-area-delete`).

export async function createArea(
    page: Page,
    recorder: TestRecorder,
    testArea: { name: string },
) {
    // Open the LocationCard dropdown on the first visible card, then
    // pick "Add area" — the menu item carries the legacy
    // `location-card-add-area` testid so any future visual repositioning
    // (button row vs menu vs detail page) leaves this helper intact.
    await page.locator('[data-testid="location-card-menu"]').first().click();
    await page.locator('[data-testid="location-card-add-area"]').first().click();
    await page.waitForSelector('[data-testid="area-form-dialog"]');

    // Fill + submit. The `area-location` select is hidden when only one
    // location exists (typical in a fresh test run); we don't fight it.
    await page.fill('#area-name', testArea.name);
    await recorder.takeScreenshot('area-create-01-form-filled');

    // Submit and wait for the POST to land. AreaFormDialog calls
    // onOpenChange(false) right after the mutation resolves; we wait
    // for the response so the `area-form-dialog` selector below has
    // something to detach.
    const [areaResponse] = await Promise.all([
        page.waitForResponse(
            (response) =>
                new URL(response.url()).pathname.endsWith('/areas') &&
                response.request().method() === 'POST' &&
                response.status() === 201,
            { timeout: 30000 },
        ),
        page.click('[data-testid="area-form-submit"]'),
    ]);
    const areaBody = await areaResponse.json().catch(() => null);
    const newAreaId = areaBody?.data?.id as string | undefined;

    // Wait for the dialog to fully unmount before interacting with the
    // page underneath — Radix's overlay still intercepts pointer
    // events during the close transition, which silently swallows the
    // next click.
    await page
        .locator('[data-testid="area-form-dialog"]')
        .waitFor({ state: 'detached', timeout: 10000 });

    // The list page doesn't surface the new area inline anymore; drill
    // into the parent location's detail page and assert the tile shows
    // up. Choosing the same "first visible card" the create step opened
    // keeps the helper deterministic across the suite.
    await page.locator('[data-testid="location-card-link"]').first().click();
    await page.waitForSelector('[data-testid="page-location-detail"]', { timeout: 10000 });
    const tileSelector = newAreaId
        ? `[data-testid="location-detail-area"][data-area-id="${newAreaId}"]`
        : `[data-testid="location-detail-area"]:has-text("${testArea.name}")`;
    await page.waitForSelector(tileSelector, { timeout: 15000 });
    await recorder.takeScreenshot('area-create-02-created');
}

export async function deleteArea(
    page: Page,
    recorder: TestRecorder,
    areaName: string,
    locationName?: string,
) {
    // The area tile lives on the parent location's detail page now. If
    // we're still on the list page, drill in first.
    if (await page.locator('[data-testid="page-locations"]').isVisible()) {
        const cardLink = locationName
            ? page
                .locator(`[data-testid="location-card"]:has-text("${locationName}")`)
                .locator('[data-testid="location-card-link"]')
            : page.locator('[data-testid="location-card-link"]').first();
        await cardLink.click();
        await page.waitForSelector('[data-testid="page-location-detail"]');
    }

    const areaTile = page.locator(
        `[data-testid="location-detail-area"]:has-text("${areaName}")`,
    );
    await areaTile.waitFor({ state: 'visible', timeout: 10000 });

    // Each tile renders an overlay <Link> for navigation and a dropdown
    // trigger above it; the trigger is hidden until hover but the
    // testid-based click bypasses the opacity transition.
    await areaTile.locator('[data-testid="location-detail-area-menu"]').click();
    await page.locator('[data-testid="location-detail-area-delete"]').click();
    await recorder.takeScreenshot('area-delete-01-confirm');

    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'visible', timeout: 5000 });

    // Confirm-accept fires the DELETE; the dialog closes and the tile
    // vanishes from the location-detail grid. Don't pre-arm a
    // `waitForResponse` listener — it races the click+fetch and
    // sometimes attaches *after* the response lands. The
    // `confirm-dialog` becoming hidden + the tile's `toHaveCount(0)` is
    // a deterministic settle signal that the React mutation completed.
    await page.click('[data-testid="confirm-accept"]');
    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'hidden', timeout: 10000 });

    await expect(
        page.locator(`[data-testid="location-detail-area"]:has-text("${areaName}")`),
    ).toHaveCount(0, { timeout: 15000 });

    await recorder.takeScreenshot('area-delete-02-deleted');
    // The user remains on the location detail page after delete.
    await expect(page).toHaveURL(/\/locations\//);
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
