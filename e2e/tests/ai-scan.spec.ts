/**
 * AI vision scan flow for the Add Item dialog (#1720).
 *
 * Why network interception instead of the mock provider?
 *
 * Running the docker-compose e2e stack with `INVENTARIO_RUN_AI_VISION_
 * PROVIDER=mock` would exercise the real BE handler end-to-end, but
 * routing that env var into the compose config means coordinating two
 * separate provider modes across the full matrix of e2e jobs. For the
 * spirit of the AC (FE handles the typed response + typed error
 * codes) it is enough to stub the wire boundary via `page.route`. The
 * BE half is already covered by the Go unit tests under
 * `go/services/commodity_scan_service/`. Switching to the real mock
 * provider later is a one-line replacement (`page.route` → real
 * stack).
 */
import { test } from '../fixtures/app-fixture.js';
import { expect } from '@playwright/test';
import { createLocation, deleteLocation } from './includes/locations.js';
import { createArea, deleteArea } from './includes/areas.js';
import { navigateTo, TO_LOCATIONS } from './includes/navigate.js';

function makeTestData() {
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  return {
    location: { name: `AI Scan Location ${suffix}`, address: 'Test address' },
    area: { name: `AI Scan Area ${suffix}` },
  };
}

// Wire interception matcher — the dialog calls POST /api/v1/g/<slug>/
// commodities/scan. The slug is dynamic per-test (created via the
// helper); use a wildcard glob.
const SCAN_URL_GLOB = '**/api/v1/g/*/commodities/scan';

test.describe('AI vision scan flow', () => {
  test('happy path: scan → review → use values prefills Basics', async ({ page }) => {
    const { location, area } = makeTestData();
    await navigateTo(page, TO_LOCATIONS);
    await createLocation(page, location);
    await createArea(page, location.name, area);

    // Stub the scan endpoint — return one high-confidence guess so
    // the review row stays default-checked and the prefill is
    // deterministic.
    await page.route(SCAN_URL_GLOB, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/vnd.api+json',
        body: JSON.stringify({
          data: {
            type: 'commodity_scan',
            attributes: {
              fields: {
                name: { value: 'Stub Item', confidence: 0.95 },
                short_name: { value: 'Stub', confidence: 0.9 },
              },
              warnings: [],
            },
          },
        }),
      })
    );

    // Open the Add item dialog from the commodities list.
    await page.locator('[data-testid="commodities-add-button"]').first().click();
    await page.locator('[data-testid="commodity-form-ai-step"]').waitFor();

    // Drop two sample images via the file input. setInputFiles
    // bypasses the dropzone click affordance — it talks directly to
    // the hidden <input type="file"> the dropzone wraps.
    await page
      .locator('[data-testid="commodity-form-ai-file-input"]')
      .setInputFiles([
        'fixtures/files/image.jpg',
        'fixtures/files/image.jpg',
      ]);

    // Two thumbnails should now be staged.
    await expect(page.locator('[data-testid="commodity-form-ai-thumb"]')).toHaveCount(2);

    // Fire the scan.
    await page.locator('[data-testid="commodity-form-ai-scan"]').click();

    // Wait for the review phase to land — high-confidence rows
    // default-checked.
    await page.locator('[data-testid="commodity-form-ai-review"]').waitFor();
    await expect(
      page.locator('[data-testid="commodity-form-ai-row-name-check"]')
    ).toHaveAttribute('data-state', 'checked');

    // Accept the values and verify the Name input on Basics carries
    // the stubbed guess.
    await page.locator('[data-testid="commodity-form-ai-use-values"]').click();
    await expect(page.locator('input#commodity-name')).toHaveValue('Stub Item');

    // Cleanup.
    await deleteArea(page, location.name, area.name);
    await deleteLocation(page, location.name);
  });

  test('degraded path: provider-disabled banner surfaces typed copy and Fill manually still works', async ({
    page,
  }) => {
    const { location, area } = makeTestData();
    await navigateTo(page, TO_LOCATIONS);
    await createLocation(page, location);
    await createArea(page, location.name, area);

    // Stub the scan endpoint with the typed 503 error envelope.
    await page.route(SCAN_URL_GLOB, (route) =>
      route.fulfill({
        status: 503,
        contentType: 'application/vnd.api+json',
        body: JSON.stringify({
          errors: [
            {
              code: 'commodity_scan.provider_disabled',
              status: '503',
              title: 'provider disabled',
              detail: 'provider off',
            },
          ],
        }),
      })
    );

    await page.locator('[data-testid="commodities-add-button"]').first().click();
    await page.locator('[data-testid="commodity-form-ai-step"]').waitFor();

    await page
      .locator('[data-testid="commodity-form-ai-file-input"]')
      .setInputFiles('fixtures/files/image.jpg');
    await page.locator('[data-testid="commodity-form-ai-scan"]').click();

    // Banner appears with the typed title for provider_disabled.
    await page.locator('[data-testid="commodity-form-ai-error"]').waitFor();
    await expect(page.locator('[data-testid="commodity-form-ai-error"]')).toContainText(
      'AI vision is unavailable'
    );

    // Fill manually still routes to Basics.
    await page.locator('[data-testid="commodity-form-ai-fill-manually"]').click();
    await expect(page.locator('input#commodity-name')).toBeVisible();

    // Cleanup.
    await page.keyboard.press('Escape');
    await page.locator('[data-testid="commodity-form-close-confirm-discard"]').click();
    await deleteArea(page, location.name, area.name);
    await deleteLocation(page, location.name);
  });
});
