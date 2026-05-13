import { expect, Page } from "@playwright/test";
import { TestRecorder } from "../../utils/test-recorder.js";

// React port of the Vue-era helper. After cutover #1423 the legacy
// PrimeVue `.location-card`, `#name/#address`, `.confirmation-modal`
// selectors are gone; LocationsListPage / LocationFormDialog / useConfirm
// expose data-testid handles that this helper drives.

export async function createLocation(
    page: Page,
    recorder: TestRecorder,
    testLocation: { name: string; address: string },
): Promise<string> {
    await recorder.takeScreenshot('locations-create-01-before-create');

    // Open the create dialog.
    await page.click('[data-testid="locations-add-button"]');
    await page.waitForSelector('[data-testid="location-form-dialog"]');

    // Fill the form.
    await page.fill('#location-name', testLocation.name);
    await page.fill('#location-address', testLocation.address);
    await recorder.takeScreenshot('location-create-02-form-filled');

    // Submit and capture the create response so the new id is unambiguous —
    // DOM lookups by name flap if a previous run/orphan card shares the name.
    const [createResponse] = await Promise.all([
        page.waitForResponse(
            (response) =>
                new URL(response.url()).pathname.endsWith('/locations') &&
                response.request().method() === 'POST' &&
                response.status() === 201,
            { timeout: 30000 },
        ),
        page.click('[data-testid="location-form-submit"]'),
    ]);
    const createBody = await createResponse.json();
    const locationId = createBody?.data?.id;
    if (!locationId) {
        throw new Error(`createLocation: POST response missing data.id (body: ${JSON.stringify(createBody)})`);
    }

    // Wait for the rendered card to land.
    await page.waitForSelector(`[data-testid="location-card"][data-location-id="${locationId}"]`);
    await recorder.takeScreenshot('location-create-03-created');

    return locationId;
}

// Edit an existing location's name + address from the list page.
// Drives the LocationCard "Edit" affordance (post-#1531 the dropdown
// holds Add area + Delete but the title <Link> is the canonical
// detail-page entry; edit lives on the detail header). We navigate to
// detail, click the header Edit button, fill the dialog and submit.
// Captures the PUT response so the new attributes are read straight
// from the wire instead of fishing them back out of the DOM (which
// flaps under optimistic-update timing).
export async function editLocationViaDetail(
    page: Page,
    recorder: TestRecorder,
    locationId: string,
    updated: { name: string; address?: string },
): Promise<void> {
    await recorder.takeScreenshot('locations-edit-01-before-edit');
    await page.locator(`[data-testid="location-card"][data-location-id="${locationId}"] a`).first().click();
    await page.locator('[data-testid="page-location-detail"]').waitFor({ state: 'visible', timeout: 10000 });
    await page.locator('[data-testid="location-detail-edit"]').click();
    await page.locator('[data-testid="location-form-dialog"]').waitFor({ state: 'visible' });

    await page.fill('#location-name', updated.name);
    if (updated.address !== undefined) {
        await page.fill('#location-address', updated.address);
    }
    await recorder.takeScreenshot('location-edit-02-form-filled');

    const [putResponse] = await Promise.all([
        page.waitForResponse(
            (response) =>
                new URL(response.url()).pathname.endsWith(`/locations/${locationId}`) &&
                response.request().method() === 'PUT' &&
                response.status() === 200,
            { timeout: 30000 },
        ),
        page.click('[data-testid="location-form-submit"]'),
    ]);
    const body = await putResponse.json();
    if (body?.data?.attributes?.name !== updated.name) {
        throw new Error(`editLocationViaDetail: BE returned name "${body?.data?.attributes?.name}", expected "${updated.name}"`);
    }

    // Dialog closes on success; the detail heading reflects the new name.
    await page.locator('[data-testid="location-form-dialog"]').waitFor({ state: 'hidden', timeout: 5000 });
    await expect(page.locator('[data-testid="page-location-detail"] h1')).toContainText(updated.name);
    await recorder.takeScreenshot('location-edit-03-saved');
}

export async function deleteLocation(
    page: Page,
    recorder: TestRecorder,
    locationName: string,
    locationId?: string,
) {
    // Two entry points after #1531:
    //   - From `/locations` (list page): open the LocationCard dropdown
    //     and pick "Delete" (`location-card-menu` → `location-card-delete`).
    //   - From `/locations/:id` (detail page, where deleteArea now
    //     leaves us): use the header's Delete button
    //     (`location-detail-delete`). The location-card surface doesn't
    //     exist on the detail page.
    // Tests routinely chain deleteArea → deleteLocation, so this helper
    // detects the current page and picks the right path.
    const onDetail = await page
        .locator('[data-testid="page-location-detail"]')
        .first()
        .isVisible()
        .catch(() => false);

    // Resolve the id once so the post-delete assertion is unambiguous —
    // name match alone flaps when warmup/retry orphans share the name.
    let targetId = locationId;

    if (onDetail) {
        targetId = targetId ?? (await page.url().match(/\/locations\/([^/?#]+)/)?.[1]);
        await page.locator('[data-testid="location-detail-delete"]').click();
        await recorder.takeScreenshot('location-delete-01-confirm');
    } else {
        const locationCard = locationId
            ? page.locator(`[data-testid="location-card"][data-location-id="${locationId}"]`)
            : page.locator(`[data-testid="location-card"]:has-text("${locationName}")`).last();
        await locationCard.waitFor({ state: 'visible', timeout: 10000 });
        targetId = targetId ?? (await locationCard.getAttribute('data-location-id')) ?? undefined;
        if (!targetId) {
            throw new Error(`deleteLocation: could not read data-location-id from card "${locationName}"`);
        }
        // Post-#1531 the trash icon moved into the LocationCard dropdown.
        // Open the dropdown first, then pick "Delete" — the testid kept its
        // legacy name so any future repositioning of the action leaves this
        // helper intact.
        await locationCard.locator('[data-testid="location-card-menu"]').click();
        await page.locator('[data-testid="location-card-delete"]').click();
        await recorder.takeScreenshot('location-delete-01-confirm');
    }

    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'visible', timeout: 5000 });

    // Confirm-accept fires the DELETE; the card vanishes once the
    // mutation lands. Don't pre-arm a `waitForResponse` — it races the
    // click+fetch and sometimes attaches after the response (same
    // class of issue as createCommodity's POST listener race). The
    // dialog closing + the card's `toHaveCount(0)` is a deterministic
    // signal that the BE finished cascading the delete (areas +
    // commodities + the location itself). 30s timeout because cascade
    // can be slow under CI load.
    await page.click('[data-testid="confirm-accept"]');
    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'hidden', timeout: 30000 });

    // After delete from the detail page, the React app navigates back
    // to /locations; from the list page, the card simply unmounts. Both
    // paths converge on /locations with no card carrying targetId — that's
    // the deterministic settle signal.
    await page.waitForURL((url) => /\/locations(\?|$)/.test(url.pathname), { timeout: 30000 });
    if (targetId) {
        await expect(
            page.locator(`[data-testid="location-card"][data-location-id="${targetId}"]`),
        ).toHaveCount(0, { timeout: 30000 });
    }

    await recorder.takeScreenshot('location-delete-02-deleted');
    await expect(page).toHaveURL(/\/locations/);
}
