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
 */
import { test } from '../fixtures/app-fixture.js';
import { expect, type Locator } from '@playwright/test';
import { navigateWithAuth } from './includes/auth.js';

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

test.describe('Onboarding tour', () => {
  test('opens from the user menu and highlights each step on its element', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder);
    await expect(page.locator('h1')).toBeVisible();
    await expect(page.locator('[data-testid="sidebar-add-item"]')).toBeVisible();

    await page.click('[data-testid="user-menu"]');
    await page.click('[data-testid="restart-tour"]');

    const tour = page.locator('[data-testid="onboarding-tour"]');
    const card = page.locator('[data-testid="onboarding-card"]');
    const highlight = page.locator('[data-testid="onboarding-highlight"]');
    await expect(tour).toBeVisible();

    const seen: string[] = [];
    for (let i = 0; i < Object.keys(STEP_TARGETS).length; i += 1) {
      const key = (await card.getAttribute('data-step')) ?? '';
      seen.push(key);
      expect(Object.keys(STEP_TARGETS), `unexpected step "${key}"`).toContain(key);

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
      await recorder.takeScreenshot(`onboarding-tour-${String(i + 1).padStart(2, '0')}-${key}`);

      const done = page.locator('[data-testid="onboarding-done"]');
      if (await done.isVisible()) {
        await done.click();
        break;
      }
      await page.click('[data-testid="onboarding-next"]');
      await expect(card).not.toHaveAttribute('data-step', key);
    }

    expect(seen).toEqual(Object.keys(STEP_TARGETS));
    await expect(tour).toHaveCount(0);
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
