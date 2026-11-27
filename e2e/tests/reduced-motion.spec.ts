/**
 * Reduced motion (#2932).
 *
 * The enter and exit animations come from tw-animate-css, whose keyframes
 * read their start and end values from `--tw-enter-*` and `--tw-exit-*`.
 * Under `prefers-reduced-motion: reduce` the ones that move (scale,
 * translate, rotate) are at rest, so a dialog or a menu fades in place
 * instead of zooming or sliding. Without the preference they move.
 */
import { test } from '../fixtures/app-fixture.js';
import { expect, type Locator, type Page } from '@playwright/test';
import { navigateWithAuth } from './includes/auth.js';

const DESKTOP = { width: 1280, height: 800 };

function readMotion(locator: Locator) {
  return locator.evaluate((el) => {
    const style = getComputedStyle(el);
    const prop = (name: string) => style.getPropertyValue(name).trim();
    return {
      enterScale: prop('--tw-enter-scale'),
      enterTranslateY: prop('--tw-enter-translate-y'),
      enterOpacity: prop('--tw-enter-opacity'),
      animationName: style.animationName,
    };
  });
}

async function openLocationDialog(page: Page) {
  await page.getByTestId('locations-add-button').click();
  const dialog = page.getByTestId('location-form-dialog');
  await expect(dialog).toHaveAttribute('data-state', 'open');
  return dialog;
}

async function openUserMenu(page: Page) {
  await page.getByTestId('user-menu').click();
  const menu = page.getByRole('menu');
  await expect(menu).toHaveAttribute('data-state', 'open');
  return menu;
}

test.describe('Reduced motion (#2932)', () => {
  test('a dialog and a menu fade without moving under reduced motion', async ({ page, recorder }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await page.setViewportSize(DESKTOP);
    await navigateWithAuth(page, '/locations', recorder);

    const dialog = await openLocationDialog(page);
    const dialogMotion = await readMotion(dialog);
    expect(dialogMotion.animationName).toBe('enter');
    expect(dialogMotion.enterScale).toBe('1');
    expect(dialogMotion.enterTranslateY).toBe('0');
    // The fade stays.
    expect(dialogMotion.enterOpacity).not.toBe('');
    expect(dialogMotion.enterOpacity).not.toBe('1');
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    const menu = await openUserMenu(page);
    const menuMotion = await readMotion(menu);
    expect(menuMotion.animationName).toBe('enter');
    expect(menuMotion.enterScale).toBe('1');
    expect(menuMotion.enterTranslateY).toBe('0');
    await page.keyboard.press('Escape');
    await expect(menu).toBeHidden();
  });

  test('a dialog and a menu zoom and slide without the preference', async ({ page, recorder }) => {
    await page.emulateMedia({ reducedMotion: 'no-preference' });
    await page.setViewportSize(DESKTOP);
    await navigateWithAuth(page, '/locations', recorder);

    const dialog = await openLocationDialog(page);
    expect((await readMotion(dialog)).enterScale).not.toBe('1');
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    const menu = await openUserMenu(page);
    const menuMotion = await readMotion(menu);
    expect(menuMotion.enterScale).not.toBe('1');
    expect(menuMotion.enterTranslateY).not.toBe('0');
    await page.keyboard.press('Escape');
    await expect(menu).toBeHidden();
  });
});
