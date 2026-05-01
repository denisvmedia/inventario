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

export async function deleteLocation(
    page: Page,
    recorder: TestRecorder,
    locationName: string,
    locationId?: string,
) {
    // Prefer the id when given — name lookups remain ambiguous across
    // warmup/retry orphans even though the React card surface uses
    // `[data-testid="location-card"]` instead of `.location-card`.
    const locationCard = locationId
        ? page.locator(`[data-testid="location-card"][data-location-id="${locationId}"]`)
        : page.locator(`[data-testid="location-card"]:has-text("${locationName}")`).last();
    await locationCard.waitFor({ state: 'visible', timeout: 10000 });

    const targetId = locationId ?? (await locationCard.getAttribute('data-location-id'));
    if (!targetId) {
        throw new Error(`deleteLocation: could not read data-location-id from card "${locationName}"`);
    }

    // Click the trash icon button on the card. The confirm dialog comes from
    // useConfirm (single shared root-mounted Dialog) — selector is
    // `[data-testid="confirm-dialog"]` with `confirm-accept` for the
    // destructive button.
    await locationCard.locator('[data-testid="location-card-delete"]').click();
    await recorder.takeScreenshot('location-delete-01-confirm');

    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'visible', timeout: 5000 });

    // Click confirm + wait for the DELETE round-trip. After the Location
    // Groups refactor every data path is `/api/v1/g/{slug}/locations/{id}`,
    // so match on the `/locations/<id>` suffix. Cascaded deletes (areas +
    // commodities) under CI load can exceed 10s — give it 30s.
    await Promise.all([
        page.waitForResponse(
            (response) =>
                new URL(response.url()).pathname.endsWith(`/locations/${targetId}`) &&
                response.request().method() === 'DELETE' &&
                response.status() === 204,
            { timeout: 30000 },
        ),
        page.click('[data-testid="confirm-accept"]'),
    ]);

    await page.locator('[data-testid="confirm-dialog"]').waitFor({ state: 'hidden', timeout: 5000 });

    // Assert the specific card is gone — name match alone is unreliable when
    // a sibling test left a same-named orphan.
    await expect(
        page.locator(`[data-testid="location-card"][data-location-id="${targetId}"]`),
    ).toHaveCount(0, { timeout: 15000 });

    await recorder.takeScreenshot('location-delete-02-deleted');
    await expect(page).toHaveURL(/\/locations/);
}
