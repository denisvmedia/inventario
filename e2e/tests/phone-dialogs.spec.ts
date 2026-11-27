/**
 * Full-screen dialogs on a phone: the back gesture and toasts (#2931).
 *
 * Below 640px a long-form dialog fills the screen. A step back in the
 * browser history (the Android back gesture, the iOS edge swipe) closes it
 * the way its close button does and leaves the page where it was, and a
 * toast shown while it is open sits above its footer. Playwright's
 * page.goBack() is the same history step the gesture takes. The on-screen
 * keyboard cannot be opened in a desktop browser; the dialog's response to
 * it is covered in frontend/src/components/ui/__tests__/dialog-phone.test.tsx.
 */
import { test } from '../fixtures/app-fixture.js';
import { expect, type Locator, type Page } from '@playwright/test';
import { navigateWithAuth } from './includes/auth.js';
import { createCommodity } from './includes/commodities.js';

const PHONE = { width: 390, height: 844 };
const DESKTOP = { width: 1280, height: 800 };

async function box(locator: Locator) {
  const b = await locator.boundingBox();
  if (!b) throw new Error('element has no bounding box');
  return b;
}

function dialogEntries(page: Page) {
  return page.evaluate(() => {
    const state = window.history.state as { inventarioDialogs?: string[] } | null;
    return state?.inventarioDialogs ?? [];
  });
}

test.describe('Full-screen dialogs on a phone (#2931)', () => {
  test('the back gesture closes New location and stays on the page', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder);
    await page.setViewportSize(PHONE);
    const url = page.url();

    await page.getByTestId('locations-add-button').click();
    const dialog = page.getByTestId('location-form-dialog');
    await expect(dialog).toHaveAttribute('data-state', 'open');
    await expect.poll(() => dialogEntries(page)).toHaveLength(1);

    await page.goBack();
    await expect(dialog).toBeHidden();
    expect(page.url()).toBe(url);
    await expect(page.getByTestId('locations-add-button')).toBeVisible();
    expect(await dialogEntries(page)).toEqual([]);

    // Closed with its own button, the dialog takes its history entry off.
    await page.getByTestId('locations-add-button').click();
    await expect(dialog).toHaveAttribute('data-state', 'open');
    await expect.poll(() => dialogEntries(page)).toHaveLength(1);
    await dialog.locator('[data-slot="dialog-header"]').getByRole('button', { name: 'Close' }).click();
    await expect(dialog).toBeHidden();
    await expect.poll(() => dialogEntries(page)).toEqual([]);
    expect(page.url()).toBe(url);
  });

  test('the back gesture asks before Add item drops what was typed', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/commodities', recorder);
    await page.setViewportSize(PHONE);
    const url = page.url();

    await page.getByTestId('commodities-add-button').click();
    const dialog = page.getByTestId('commodity-form-dialog');
    await dialog.getByTestId('commodity-form-ai-fill-manually').click();
    const name = dialog.getByLabel(/^Name$/i);
    await name.fill('Back gesture test item');
    await expect.poll(() => dialogEntries(page)).toHaveLength(1);

    await page.goBack();
    const confirm = page.getByTestId('commodity-form-close-confirm');
    await expect(confirm).toBeVisible();
    await confirm.getByTestId('commodity-form-close-confirm-cancel').click();
    await expect(confirm).toBeHidden();
    await expect(dialog).toBeVisible();
    await expect(name).toHaveValue('Back gesture test item');
    expect(page.url()).toBe(url);

    // The next step back asks again.
    await expect.poll(() => dialogEntries(page)).toHaveLength(1);
    await page.goBack();
    await expect(confirm).toBeVisible();
    await confirm.getByTestId('commodity-form-close-confirm-discard').click();
    await expect(dialog).toBeHidden();
    expect(page.url()).toBe(url);
  });

  test('saving Add item and opening the new item leaves no extra step back', async ({
    page,
    recorder,
  }) => {
    await navigateWithAuth(page, '/commodities', recorder);
    await page.setViewportSize(PHONE);
    const listUrl = page.url();

    // The list closes the dialog and opens the new item in one handler.
    await createCommodity(page, recorder, {
      name: `Phone back step ${Date.now()}`,
      shortName: 'PBS',
      type: 'electronics',
      count: 1,
    });

    // One step back returns to the list page's own entry, not to one the
    // dialog left behind with the same URL.
    await page.goBack();
    await expect(page).toHaveURL(listUrl);
    expect(await dialogEntries(page)).toEqual([]);
  });

  test('a page pushed while the dialog steps back stays, and Back returns to the list', async ({
    page,
    recorder,
  }) => {
    await navigateWithAuth(page, '/locations', recorder);
    await page.setViewportSize(PHONE);
    const listUrl = page.url();

    await page.getByTestId('locations-add-button').click();
    const dialog = page.getByTestId('location-form-dialog');
    await expect(dialog).toHaveAttribute('data-state', 'open');
    await expect.poll(() => dialogEntries(page)).toHaveLength(1);

    // The next history.back() is the dialog's own step back; a push right
    // after it lands before the step does, as a fast navigation after a close.
    await page.evaluate(() => {
      const back = history.back.bind(history);
      history.back = () => {
        history.back = back;
        back();
        const state = history.state as { idx?: number } | null;
        history.pushState({ usr: null, key: 'race', idx: (state?.idx ?? 0) + 1 }, '', '/race-target');
      };
    });
    await dialog.locator('[data-slot="dialog-header"]').getByRole('button', { name: 'Close' }).click();
    await expect(dialog).toBeHidden();

    await expect.poll(() => page.evaluate(() => location.pathname)).toBe('/race-target');
    // Give the step back time to land, then check it left the push in place.
    await page.waitForTimeout(500);
    expect(await page.evaluate(() => location.pathname)).toBe('/race-target');
    expect(await page.evaluate(() => (history.state as { key?: string } | null)?.key)).toBe('race');

    await page.goBack();
    await expect(page).toHaveURL(listUrl);
    expect(await dialogEntries(page)).toEqual([]);
  });

  test('a toast shown over a full-screen dialog sits above its footer', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/settings', recorder);
    await page.setViewportSize(PHONE);
    // A refused send keeps the dialog open and shows an error toast.
    await page.route('**/api/v1/feedback', (route) =>
      route.fulfill({
        status: 429,
        contentType: 'application/vnd.api+json',
        body: JSON.stringify({ errors: [{ status: '429', title: 'Too Many Requests' }] }),
      }),
    );

    await page.getByTestId('settings-nav-help').click();
    await page.getByTestId('help-row-feedback').click();
    const dialog = page.getByTestId('feedback-dialog');
    await expect(dialog).toHaveAttribute('data-state', 'open');
    await dialog.getByTestId('feedback-message').fill('E2E: a toast over the phone dialog.');
    await dialog.getByTestId('feedback-submit').click();

    const toast = page.locator('[data-sonner-toast]').filter({ hasText: 'sent a lot of feedback' });
    await expect(toast).toBeVisible();
    await expect(dialog).toBeVisible();
    const footer = dialog.locator('[data-slot="dialog-footer"]');
    const footerTop = (await box(footer)).y;
    // Poll until the toast has slid in.
    await expect
      .poll(async () => {
        const t = await box(toast);
        return Math.round(t.y + t.height) <= Math.round(footerTop);
      })
      .toBe(true);
    await expect(footer.getByTestId('feedback-submit')).toBeInViewport();
  });

  test('a desktop dialog leaves the history alone', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder);
    await page.setViewportSize(DESKTOP);
    await page.getByTestId('locations-add-button').click();
    const dialog = page.getByTestId('location-form-dialog');
    await expect(dialog).toHaveAttribute('data-state', 'open');
    expect(await dialogEntries(page)).toEqual([]);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
  });
});
