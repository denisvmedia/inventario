/**
 * Long-form dialogs on a phone (#1627).
 *
 * Below 640px the Add item and location dialogs fill the screen: the
 * header with the title and close button sticks to the top, the footer
 * with the actions sticks to the bottom, and the form scrolls between
 * them. From 640px up they stay the centered card.
 */
import { test } from '../fixtures/app-fixture.js';
import { expect, type Locator, type Page } from '@playwright/test';
import { navigateWithAuth } from './includes/auth.js';

const PHONE = { width: 390, height: 844 };
// Short enough that every form here scrolls.
const SHORT_PHONE = { width: 390, height: 420 };
const DESKTOP = { width: 1280, height: 800 };

type Size = { width: number; height: number };

async function box(locator: Locator) {
  const b = await locator.boundingBox();
  if (!b) throw new Error('element has no bounding box');
  return b;
}

function rounded(b: { x: number; y: number; width: number; height: number }) {
  return [Math.round(b.x), Math.round(b.y), Math.round(b.width), Math.round(b.height)];
}

async function expectPinned(dialog: Locator, size: Size) {
  const header = dialog.locator('[data-slot="dialog-header"]');
  const footer = dialog.locator('[data-slot="dialog-footer"]');
  await expect.poll(async () => Math.round((await box(header)).y)).toBe(0);
  await expect
    .poll(async () => {
      const f = await box(footer);
      return Math.round(f.y + f.height);
    })
    .toBe(size.height);
  await expect(header.getByRole('button', { name: 'Close' })).toBeInViewport();
  await expect(footer.getByRole('button').last()).toBeInViewport();
}

async function expectFullScreen(page: Page, dialog: Locator) {
  await expect(dialog).toHaveAttribute('data-state', 'open');
  // Poll until the slide-up has finished.
  await expect.poll(async () => rounded(await box(dialog))).toEqual([0, 0, PHONE.width, PHONE.height]);
  expect(await dialog.evaluate((el) => getComputedStyle(el).borderTopLeftRadius)).toBe('0px');
  await expectPinned(dialog, PHONE);

  // On a short screen the form scrolls between the header and the footer.
  await page.setViewportSize(SHORT_PHONE);
  await expect
    .poll(() => dialog.evaluate((el) => el.scrollHeight > el.clientHeight))
    .toBe(true);
  for (const position of ['middle', 'end'] as const) {
    await dialog.evaluate((el, at) => {
      el.scrollTo(0, at === 'end' ? el.scrollHeight : el.scrollHeight / 2);
    }, position);
    await expectPinned(dialog, SHORT_PHONE);
  }
  await dialog.evaluate((el) => el.scrollTo(0, 0));
  await page.setViewportSize(PHONE);
  return dialog.locator('[data-slot="dialog-header"]').getByRole('button', { name: 'Close' });
}

async function expectCenteredCard(dialog: Locator, maxWidth: number) {
  await expect(dialog).toHaveAttribute('data-state', 'open');
  await expect.poll(async () => Math.round((await box(dialog)).width)).toBe(maxWidth);
  const b = await box(dialog);
  expect(Math.abs(b.x + b.width / 2 - DESKTOP.width / 2)).toBeLessThanOrEqual(1);
  expect(b.y).toBeGreaterThan(0);
  expect(b.y + b.height).toBeLessThan(DESKTOP.height);
  expect(await dialog.evaluate((el) => getComputedStyle(el).borderTopLeftRadius)).not.toBe('0px');
  // The header's close button is the phone one; the card keeps its corner one.
  await expect(
    dialog.locator('[data-slot="dialog-header"]').getByRole('button', { name: 'Close' }),
  ).toBeHidden();
  await expect(dialog.getByRole('button', { name: 'Close' })).toBeVisible();
}

test.describe('Long-form dialogs on a phone (#1627)', () => {
  test('Add item fills a phone screen and is a centered card on a desktop', async ({
    page,
    recorder,
  }) => {
    await navigateWithAuth(page, '/commodities', recorder);
    await page.setViewportSize(PHONE);

    await page.getByTestId('commodities-add-button').click();
    const dialog = page.getByTestId('commodity-form-dialog');
    await expect(dialog.getByTestId('commodity-form-ai-step')).toBeVisible();
    await expectFullScreen(page, dialog);

    // Basics has more fields than the scan step.
    await dialog.getByTestId('commodity-form-ai-fill-manually').click();
    await expect(dialog.getByLabel(/^Name$/i)).toBeVisible();
    const close = await expectFullScreen(page, dialog);
    await recorder.takeScreenshot('mobile-dialogs-01-add-item-phone');
    await close.click();
    await expect(dialog).toBeHidden();

    await page.setViewportSize(DESKTOP);
    await page.getByTestId('commodities-add-button').click();
    await expectCenteredCard(dialog, 672);
    await recorder.takeScreenshot('mobile-dialogs-02-add-item-desktop');
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
  });

  test('New location fills a phone screen and is a centered card on a desktop', async ({
    page,
    recorder,
  }) => {
    await navigateWithAuth(page, '/locations', recorder);
    await page.setViewportSize(PHONE);

    await page.getByTestId('locations-add-button').click();
    const dialog = page.getByTestId('location-form-dialog');
    const close = await expectFullScreen(page, dialog);
    await recorder.takeScreenshot('mobile-dialogs-03-location-phone');
    await close.click();
    await expect(dialog).toBeHidden();

    await page.setViewportSize(DESKTOP);
    await page.getByTestId('locations-add-button').click();
    await expectCenteredCard(dialog, 512);
    await recorder.takeScreenshot('mobile-dialogs-04-location-desktop');
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
  });
});
