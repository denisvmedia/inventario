/**
 * E2E for the onboarding product tour (#1543, #2787).
 *
 * The tour skips its auto-launch under WebDriver (navigator.webdriver), so
 * these tests open it the way a returning user does: from the user menu or
 * from Settings → Help. The seeded admin has a group, so every step has its
 * target in the sidebar.
 *
 * The main check is geometric: on each anchored step, the highlight ring
 * must sit on the element the step describes. #2787 was a tour whose steps
 * had no target on the page and fell back to centered cards; a unit test in
 * jsdom cannot see that, a bounding-box comparison in a real browser can.
 *
 * Below the mobile breakpoint (768px) the sidebar is a closed sheet, so the
 * tour hides. It closes, without counting as seen, only when the window
 * stays that narrow for a second; a shorter dip leaves it on its step.
 */
import { test } from '../fixtures/app-fixture.js';
import { expect, type Locator, type Page } from '@playwright/test';
import { navigateWithAuth } from './includes/auth.js';
import type { TestRecorder } from '../utils/test-recorder.js';

// Step key (the card's data-step) → the element the step highlights. The
// welcome step is a centered card with no highlight.
const STEP_TARGETS: Record<string, string | null> = {
  welcome: null,
  addItem: "[data-tour='add-item']",
  navDashboard: "[data-tour='nav-dashboard']",
  navLocations: "[data-tour='nav-locations']",
  navItems: "[data-tour='nav-items']",
  navWarranties: "[data-tour='nav-warranties']",
  navFiles: "[data-tour='nav-files']",
};

// Share of the target's box that the highlight covers, from 0 to 1. The
// highlight is the target's box grown by a few pixels of padding, so a
// highlight on its element covers all of it.
async function coverage(highlight: Locator, target: Locator): Promise<number> {
  const h = await highlight.boundingBox();
  const t = await target.boundingBox();
  if (!h || !t || t.width === 0 || t.height === 0) return 0;
  const w = Math.min(h.x + h.width, t.x + t.width) - Math.max(h.x, t.x);
  const hgt = Math.min(h.y + h.height, t.y + t.height) - Math.max(h.y, t.y);
  if (w <= 0 || hgt <= 0) return 0;
  return (w * hgt) / (t.width * t.height);
}

// Opens /locations and starts the tour from the user menu, on its welcome
// step.
async function openTourFromUserMenu(page: Page, recorder: TestRecorder) {
  await navigateWithAuth(page, '/locations', recorder);
  await expect(page.locator('h1')).toBeVisible();
  await expect(page.locator('[data-testid="sidebar-add-item"]')).toBeVisible();

  await page.click('[data-testid="user-menu"]');
  await page.click('[data-testid="restart-tour"]');

  const tour = page.locator('[data-testid="onboarding-tour"]');
  const card = page.locator('[data-testid="onboarding-card"]');
  const highlight = page.locator('[data-testid="onboarding-highlight"]');
  await expect(tour).toBeVisible();
  await expect(card).toHaveAttribute('data-step', 'welcome');
  return { tour, card, highlight };
}

// The "seen" flags the tour keeps in localStorage, one per user; a finish
// or a skip sets the user's flag to "1".
async function seenFlags(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    Object.keys(window.localStorage).filter(
      (k) => k.startsWith('inventario-tour-seen-v1:') && window.localStorage.getItem(k) === '1',
    ),
  );
}

test.describe('Onboarding tour', () => {
  test('opens from the user menu and highlights each step on its element', async ({ page, recorder }) => {
    const { tour, card, highlight } = await openTourFromUserMenu(page, recorder);

    const keys = Object.keys(STEP_TARGETS);
    for (const [i, key] of keys.entries()) {
      await expect(card).toHaveAttribute('data-step', key);

      const selector = STEP_TARGETS[key];
      if (selector === null) {
        await expect(highlight).toHaveCount(0);
      } else {
        const target = page.locator(selector);
        await expect(target).toBeVisible();
        await expect(highlight).toBeVisible();
        // The highlight animates to its spot (220ms) and the target may
        // scroll into view first, so poll until it settles.
        await expect
          .poll(() => coverage(highlight, target), {
            message: `highlight on step "${key}" should cover ${selector}`,
          })
          .toBeGreaterThan(0.95);
      }
      // Viewport-sized on purpose. The tour is a fixed overlay, so a
      // full-page capture adds nothing, and on a page taller than the
      // window Chromium shrinks the page to 1x1 while it captures one, which
      // hides the tour for that moment (see the next test).
      await recorder.takeScreenshot(`onboarding-tour-${String(i + 1).padStart(2, '0')}-${key}`, false);

      if (i === keys.length - 1) {
        await page.click('[data-testid="onboarding-done"]');
      } else {
        await page.click('[data-testid="onboarding-next"]');
      }
    }

    await expect(tour).toHaveCount(0);
  });

  test('keeps its step when the window is narrow for a moment only', async ({ page, recorder }) => {
    const { tour, card, highlight } = await openTourFromUserMenu(page, recorder);
    await page.click('[data-testid="onboarding-next"]');
    await expect(card).toHaveAttribute('data-step', 'addItem');
    const addItem = page.locator(STEP_TARGETS.addItem!);
    const highlightCoversAddItem = () =>
      expect.poll(() => coverage(highlight, addItem)).toBeGreaterThan(0.95);

    // A window dragged across the breakpoint and back.
    await page.setViewportSize({ width: 700, height: 720 });
    await expect(tour).toHaveCount(0);
    await page.setViewportSize({ width: 1280, height: 720 });
    await expect(card).toHaveAttribute('data-step', 'addItem');
    await highlightCoversAddItem();

    // A full-page screenshot of a page taller than the window: Chromium
    // shrinks the page to 1x1 while it captures. Other browsers do not, so
    // there this only checks that the screenshot leaves the tour alone.
    await page.setViewportSize({ width: 1280, height: 400 });
    expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBeGreaterThan(400);
    await page.screenshot({ fullPage: true });
    await expect(card).toHaveAttribute('data-step', 'addItem');
    await highlightCoversAddItem();
    expect(await seenFlags(page)).toEqual([]);
  });

  test('closes without counting as seen when the window stays narrow', async ({ page, recorder }) => {
    const { tour } = await openTourFromUserMenu(page, recorder);

    await page.setViewportSize({ width: 700, height: 720 });
    await expect(tour).toHaveCount(0);
    // The tour closes once the window has been narrow for a second
    // (TOUR_BLOCKER_GRACE_MS in the frontend). Nothing on the page shows
    // the moment it does, so wait past it.
    await page.waitForTimeout(1500);
    await page.setViewportSize({ width: 1280, height: 720 });

    // The sidebar and the tour come back in the same render, so once the
    // sidebar is back a tour that was only hidden would be visible too.
    await expect(page.locator('[data-testid="sidebar-add-item"]')).toBeVisible();
    await expect(tour).toHaveCount(0);
    expect(await seenFlags(page)).toEqual([]);
  });

  test('Settings → Help → Restart product tour opens the tour', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/settings', recorder);
    await expect(page.locator('h1')).toBeVisible();

    await page.locator('[data-testid="settings-nav-help"]').click();
    await expect(page.locator('[data-testid="section-help"]')).toBeVisible();

    const row = page.locator('[data-testid="help-row-tour"]');
    await expect(row).toBeEnabled();
    await row.click();

    await expect(page.locator('[data-testid="onboarding-tour"]')).toBeVisible();
    await expect(page.locator('[data-testid="onboarding-card"]')).toHaveAttribute('data-step', 'welcome');

    await page.keyboard.press('Escape');
    await expect(page.locator('[data-testid="onboarding-tour"]')).toHaveCount(0);
  });
});
