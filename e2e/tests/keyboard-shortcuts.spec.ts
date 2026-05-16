/**
 * E2E for the keyboard-shortcuts cheat-sheet modal (#1385).
 *
 * The dialog is mounted by KeyboardShortcutsProvider at the Shell root,
 * so any authenticated route should expose the global `?` keybinding.
 * We drive it from `/locations` for parity with the command-palette
 * spec — the underlying page being mounted is incidental to what we're
 * asserting (the modal is application-wide chrome).
 */
import { test } from '../fixtures/app-fixture.js';
import { expect } from '@playwright/test';
import { navigateWithAuth } from './includes/auth.js';

test.describe('Keyboard shortcuts dialog — `?` cheat sheet', () => {
  test('opens with `?` and closes with Esc', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder);
    await expect(page.locator('h1')).toBeVisible();

    // No dialog initially.
    const dialog = page.locator('[data-testid="keyboard-shortcuts-dialog"]');
    await expect(dialog).toHaveCount(0);

    // Press `?` (Shift+/) → dialog appears.
    await page.keyboard.press('Shift+Slash');
    await expect(dialog).toBeVisible();
    await recorder.takeScreenshot('keyboard-shortcuts-01-open');

    // Acceptance criterion: the Navigation/Search section is visible.
    // We ship `search` (Cmd+K), `layout` (Cmd+B), and `help` (?) at MVP;
    // assert all three section markers + the Cmd+K row are present.
    await expect(dialog.locator('[data-testid="keyboard-shortcuts-section-search"]')).toBeVisible();
    await expect(dialog.locator('[data-testid="keyboard-shortcuts-section-layout"]')).toBeVisible();
    await expect(dialog.locator('[data-testid="keyboard-shortcuts-section-help"]')).toBeVisible();
    await expect(dialog.locator('[data-testid="keyboard-shortcut-command-palette.open"]')).toBeVisible();

    // Esc closes.
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
  });

  test('filters the list as the user types', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/locations', recorder);
    await expect(page.locator('h1')).toBeVisible();

    await page.keyboard.press('Shift+Slash');
    const dialog = page.locator('[data-testid="keyboard-shortcuts-dialog"]');
    await expect(dialog).toBeVisible();

    const filter = dialog.locator('[data-testid="keyboard-shortcuts-filter"]');
    await filter.fill('sidebar');
    await expect(dialog.locator('[data-testid="keyboard-shortcut-sidebar.toggle"]')).toBeVisible();
    await expect(dialog.locator('[data-testid="keyboard-shortcut-command-palette.open"]')).toHaveCount(0);

    // A query that matches nothing surfaces the empty state.
    await filter.fill('zzzzznosuchshortcut');
    await expect(dialog.locator('[data-testid="keyboard-shortcuts-empty"]')).toBeVisible();
  });

  test('Settings → Help → Keyboard shortcuts row opens the dialog', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/settings', recorder);
    await expect(page.locator('h1')).toBeVisible();

    // SettingsPage uses a section nav. Click the Help nav entry, which
    // renders the help section (data-testid="section-help") with the
    // shortcuts row inside it.
    await page.locator('[data-testid="settings-nav-help"]').click();
    await expect(page.locator('[data-testid="section-help"]')).toBeVisible();

    await page.locator('[data-testid="help-row-shortcuts"]').click();
    await expect(page.locator('[data-testid="keyboard-shortcuts-dialog"]')).toBeVisible();
  });
});
