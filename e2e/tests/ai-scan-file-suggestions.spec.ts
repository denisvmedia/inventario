/**
 * The AI scan's tag and title for each file in the Add item dialog (#1626).
 *
 * The scan endpoint is stubbed with `page.route`, as in ai-scan.spec.ts: the
 * server half (the `files` part of the scan response) is covered by the Go
 * tests, and what this spec checks is that the dialog puts each suggestion on
 * the file it names, that a removed suggestion stays removed, and that the
 * tags and titles reach the item's files.
 */
import { test } from '../fixtures/app-fixture.js';
import { expect, type Page } from '@playwright/test';
import { navigateTo, TO_COMMODITIES } from './includes/navigate.js';
import { BACK_TO_COMMODITIES, deleteCommodity } from './includes/commodities.js';
import { expectCommodityFilesCount } from './includes/uploads.js';

const SCAN_URL_GLOB = '**/api/v1/g/*/commodities/scan';

type Step = 'basics' | 'purchase' | 'warranty' | 'extras' | 'files';

async function next(page: Page, step: Step) {
  await page.click('[data-testid="commodity-form-next"]');
  await page.waitForSelector(`[data-testid="commodity-form-${step}-step"]`, {
    state: 'visible',
    timeout: 10000,
  });
}

// rowOf is the staged-file row on the Files step that names file.
function rowOf(page: Page, file: string) {
  return page.locator('[data-testid="commodity-files-pending"]').filter({ hasText: file });
}

test('AI scan puts its tag and title on each file of the new item', async ({ page, recorder }) => {
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const name = `AI Suggest Item ${suffix}`;

  // The request sends the photo first and the invoice second, so index 0 is
  // the photo and index 1 the invoice. Every field the form requires is in
  // the scan, so Continue walks to the Files step.
  await page.route(SCAN_URL_GLOB, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/vnd.api+json',
      body: JSON.stringify({
        data: {
          type: 'commodity_scan',
          attributes: {
            fields: {
              name: { value: name, confidence: 0.95 },
              short_name: { value: `AISug-${suffix.slice(-6)}`, confidence: 0.95 },
              type: { value: 'electronics', confidence: 0.95 },
              original_price: { value: 100, confidence: 0.95 },
              original_price_currency: { value: 'CZK', confidence: 0.95 },
              purchase_date: { value: '2024-01-15', confidence: 0.95 },
            },
            files: [
              {
                index: 1,
                document_type: { value: 'invoice', confidence: 0.9 },
                title: { value: `Invoice ${suffix}`, confidence: 0.9 },
              },
              {
                index: 0,
                document_type: { value: 'photo', confidence: 0.9 },
                title: { value: `Photo ${suffix}`, confidence: 0.9 },
              },
            ],
            warnings: [],
          },
        },
      }),
    })
  );

  await navigateTo(page, recorder, TO_COMMODITIES);
  await page.locator('[data-testid="commodities-add-button"]').first().click();
  await page.locator('[data-testid="commodity-form-ai-step"]').waitFor();
  await page
    .locator('[data-testid="commodity-form-ai-file-input"]')
    .setInputFiles(['fixtures/files/image.jpg', 'fixtures/files/invoice.pdf']);
  await expect(page.locator('[data-testid="commodity-form-ai-thumb"]')).toHaveCount(2);
  await page.locator('[data-testid="commodity-form-ai-scan"]').click();
  await page.locator('[data-testid="commodity-form-ai-review"]').waitFor();
  await expect(page.getByTestId('commodity-form-ai-file-suggestions-note')).toContainText('2 files');
  await page.locator('[data-testid="commodity-form-ai-use-values"]').click();

  await page.waitForSelector('[data-testid="commodity-form-basics-step"]', { state: 'visible' });
  await next(page, 'purchase');
  await next(page, 'warranty');
  await next(page, 'extras');
  await next(page, 'files');

  const invoiceRow = rowOf(page, 'invoice.pdf');
  const invoiceChip = invoiceRow.locator('[data-suggested="true"]');
  await expect(invoiceChip).toHaveCount(1);
  await expect(invoiceChip).toContainText(/invoice/i);
  await expect(invoiceRow.getByRole('textbox', { name: 'Title of invoice.pdf' })).toHaveValue(
    `Invoice ${suffix}`
  );

  // The photo tag is wrong for this file: remove it, and keep the title.
  const photoRow = rowOf(page, 'image.jpg');
  await expect(photoRow.locator('[data-suggested="true"]')).toContainText(/photo/i);
  await photoRow.getByRole('button', { name: /^remove photo/i }).click();
  await expect(photoRow.locator('[data-suggested="true"]')).toHaveCount(0);
  await recorder.takeScreenshot('ai-scan-file-suggestions-01-files-step');

  await page.click('[data-testid="commodity-form-submit"]');
  await page.waitForSelector('[data-testid="page-commodity-detail"]');
  await page.waitForLoadState('networkidle');

  await page.getByTestId('commodity-detail-tab-files').click();
  const files = page.getByTestId('commodity-detail-files');
  await expect(files).toBeVisible();
  await expectCommodityFilesCount(page, 2);
  await expect(files).toContainText(`Invoice ${suffix}`);
  await expect(files).toContainText(`Photo ${suffix}`);
  // Only the invoice kept a document tag.
  await expect(page.getByTestId('commodity-files-chip-invoices-count')).toHaveText('1');

  await deleteCommodity(page, recorder, name, BACK_TO_COMMODITIES);
});
