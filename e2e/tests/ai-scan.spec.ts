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
import { navigateTo, TO_COMMODITIES, TO_LOCATIONS } from './includes/navigate.js';
import {
  BACK_TO_COMMODITIES,
  deleteCommodity,
  fillCommodityWizardAndSubmit,
  type TestCommodity,
} from './includes/commodities.js';
import { expectCommodityFilesCount } from './includes/uploads.js';

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

// The two specs in this describe block share the admin@test-org.com
// fixture (login, default group, etc.). Running them in parallel
// against the same backend lets ensureAuthenticated / login race —
// the second test's session-establish step can race the first test's
// in-flight CSRF rotation and bounce to a guarded route before the
// login form renders, producing a "input[type='email'] not visible"
// timeout. `serial` opts both into the same worker; cleanup of the
// first runs before the setup of the second.
test.describe.serial('AI vision scan flow', () => {
  test('happy path: scan → review → use values prefills Basics', async ({ page, recorder }) => {
    const { location, area } = makeTestData();
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, location);
    await createArea(page, recorder, area, location.name);

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

    // Navigate to the commodities list so `commodities-add-button` is
    // in the DOM. createArea lands on the area-detail page, which has
    // its own "Add commodity" testid but not the top-level one this
    // spec targets — without this step the click below waits 30s and
    // the whole test times out.
    await navigateTo(page, recorder, TO_COMMODITIES);

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

    // Cleanup. deleteArea looks up the area tile on the location-detail
    // page, but we're currently on /commodities (the Add Item dialog
    // routed us here via use-values). Navigate back to /locations first
    // — the helper drills into the parent location card from there.
    await navigateTo(page, recorder, TO_LOCATIONS);
    await deleteArea(page, recorder, area.name, location.name);
    await deleteLocation(page, recorder, location.name);
  });

  test('degraded path: provider-disabled banner surfaces typed copy and Fill manually still works', async ({
    page,
    recorder,
  }) => {
    const { location, area } = makeTestData();
    await navigateTo(page, recorder, TO_LOCATIONS);
    await createLocation(page, recorder, location);
    await createArea(page, recorder, area, location.name);

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

    // Same /commodities navigation as the happy-path test —
    // `commodities-add-button` lives on the commodities list page,
    // not on the area-detail page createArea lands on.
    await navigateTo(page, recorder, TO_COMMODITIES);

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

    // Cleanup. After Fill-manually the dialog sits on the empty Basics
    // step — Escape would only fire the discard-confirm dialog if the
    // user had typed something, which we haven't. Navigating away does
    // the same job (route change unmounts the dialog) and lands us on
    // /locations where deleteArea expects to be.
    await navigateTo(page, recorder, TO_LOCATIONS);
    await deleteArea(page, recorder, area.name, location.name);
    await deleteLocation(page, recorder, location.name);
  });

  test('attach path: scanned image + PDF end up on the created commodity (#1983 Part A)', async ({
    page,
    recorder,
  }) => {
    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const commodity: TestCommodity = {
      name: `AI Attach Item ${suffix}`,
      shortName: `AIAttach-${suffix.slice(-6)}`,
      type: 'electronics',
      count: 1,
      // Non-draft create needs a purchase date + original price. Use the
      // e2e group's own currency (CZK) so original===group and the
      // converted-price field stays hidden (no foreign-currency branch).
      originalPrice: 100,
      originalPriceCurrency: 'CZK',
      currentPrice: 100,
      purchaseDate: '2024-01-15',
    };

    // Stub the scan so the review phase lands with at least one field —
    // the actual prefill is overwritten by fillCommodityWizardAndSubmit;
    // what we're exercising here is that the *source files* survive accept.
    await page.route(SCAN_URL_GLOB, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/vnd.api+json',
        body: JSON.stringify({
          data: {
            type: 'commodity_scan',
            attributes: {
              fields: { name: { value: 'Scanned Stub', confidence: 0.95 } },
              warnings: [],
            },
          },
        }),
      })
    );

    await navigateTo(page, recorder, TO_COMMODITIES);
    await page.locator('[data-testid="commodities-add-button"]').first().click();
    await page.locator('[data-testid="commodity-form-ai-step"]').waitFor();

    // Feed the AI step one photo + one PDF (a receipt). Both are scan
    // sources; after accept they must be retained and attached.
    await page
      .locator('[data-testid="commodity-form-ai-file-input"]')
      .setInputFiles(['fixtures/files/image.jpg', 'fixtures/files/invoice.pdf']);
    // Both files stage as a tile under the shared `commodity-form-ai-thumb`
    // wrapper (image AND PDF), so the total is 2; the PDF additionally
    // carries the `-pdf` document-tile testid, so exactly one of them is a PDF.
    await expect(page.locator('[data-testid="commodity-form-ai-thumb"]')).toHaveCount(2);
    await expect(page.locator('[data-testid="commodity-form-ai-thumb-pdf"]')).toHaveCount(1);

    await page.locator('[data-testid="commodity-form-ai-scan"]').click();
    await page.locator('[data-testid="commodity-form-ai-review"]').waitFor();
    // The review phase tells the user the scanned files will be kept.
    await expect(
      page.locator('[data-testid="commodity-form-ai-attach-note"]')
    ).toBeVisible();
    await page.locator('[data-testid="commodity-form-ai-use-values"]').click();

    // Finish the wizard (Basics → … → Files) and submit. The two scanned
    // files ride along in the Files step's pending queue.
    await fillCommodityWizardAndSubmit(page, commodity);
    await page.waitForSelector('[data-testid="page-commodity-detail"]');
    await page.waitForLoadState('networkidle');

    // Open the Files tab and assert both source files were attached with the
    // right categorisation: the photo as an image, the PDF as a document.
    await page.getByTestId('commodity-detail-tab-files').click();
    await expect(page.getByTestId('commodity-detail-files')).toBeVisible();
    await expectCommodityFilesCount(page, 2);
    await expect(page.getByTestId('commodity-files-chip-images-count')).toHaveText('1');
    await expect(page.getByTestId('commodity-files-chip-documents-count')).toHaveText('1');

    // Cleanup — deleting the commodity cascades to its two attached files.
    await deleteCommodity(page, recorder, commodity.name, BACK_TO_COMMODITIES);
  });
});
