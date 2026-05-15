/**
 * E2E for TOTP/MFA enrollment + login flow (#1380 / #1645).
 *
 * Acceptance criteria covered:
 *
 *   - enable MFA via Settings → Privacy & Security → 2FA row
 *   - log out, log in with password → expect MFA prompt
 *   - submit a TOTP code → land in the app
 *   - log out again, log in, submit a backup code → land in the app
 *   - log out again, log in, replay the same backup code → reject with 401
 *
 * The spec drives a single user end-to-end and disables MFA at the end so
 * a re-run from a clean DB lands in the same baseline state.
 */
import { test, expect, type Page } from '@playwright/test';
import { authenticator } from 'otplib';

import { ensureAuthenticated, TEST_CREDENTIALS, login } from './includes/auth.js';

// otplib defaults match our backend: SHA-1, 30s step, 6 digits.
authenticator.options = { digits: 6, step: 30 };

// ---------------------------------------------------------------------------
// Helpers — kept inline because nothing else in the suite enrolls MFA.
// ---------------------------------------------------------------------------

async function logout(page: Page) {
  // Open the user menu and trigger Sign out; mirrors profile.spec.ts.
  // `.dropdown-item--logout` is the class added to the logout menu entry
  // in AppSidebar.tsx; selecting on that keeps the helper resilient to
  // label-copy changes.
  await page.click('[data-testid="user-menu"]');
  await Promise.all([
    page.waitForURL(/\/login(\?|$)/, { timeout: 15000 }),
    page.click('.dropdown-item--logout'),
  ]);
}

async function openSettingsPrivacy(page: Page) {
  await page.goto('/settings');
  await page.click('[data-testid="settings-nav-privacy"]');
  await expect(page.locator('[data-testid="section-privacy"]')).toBeVisible();
}

async function enrollMFA(page: Page): Promise<{ secret: string; backupCodes: string[] }> {
  await openSettingsPrivacy(page);
  // Click the MFA row → setup dialog.
  await page.click('[data-testid="privacy-mfa-row"]');
  const dialog = page.locator('[data-testid="mfa-setup-dialog"]');
  await expect(dialog).toBeVisible();

  // The manual setup key input mirrors the QR — easier to read in tests.
  await expect(dialog.locator('[data-testid="mfa-setup-secret"]')).toHaveValue(/.+/);
  const secret = await dialog.locator('[data-testid="mfa-setup-secret"]').inputValue();

  // Compute the current TOTP code from the issued secret.
  const code = authenticator.generate(secret);
  await dialog.locator('[data-testid="mfa-setup-code"]').fill(code);
  await Promise.all([
    page.waitForResponse((r) => r.url().includes('/auth/mfa/verify') && r.status() === 200),
    dialog.locator('[data-testid="mfa-setup-verify"]').click(),
  ]);

  // The dialog flips to the backup-codes panel. Read all 10 codes off the grid.
  const codesGrid = dialog.locator('[data-testid="mfa-backup-codes"]');
  await expect(codesGrid).toBeVisible();
  const backupCodes = (await codesGrid.locator('span').allInnerTexts()).map((s) => s.trim());
  expect(backupCodes.length).toBe(10);

  await dialog.locator('[data-testid="mfa-ack-saved"]').click();
  await dialog.locator('[data-testid="mfa-finish"]').click();
  // After finish the dialog closes and the status row flips to Active.
  await expect(page.locator('[data-testid="privacy-mfa-row"]')).toHaveAttribute(
    'data-mfa-state',
    'active',
    { timeout: 10000 },
  );
  return { secret, backupCodes };
}

async function loginWithMFA(
  page: Page,
  args: { totp?: string; backup?: string },
  expectSuccess: boolean,
): Promise<void> {
  await page.goto('/login');
  await page.fill('input[type="email"]', TEST_CREDENTIALS.email);
  await page.fill('input[type="password"]', TEST_CREDENTIALS.password);
  await page.click('button[type="submit"]');
  // Step 1 returns 200 with mfa_required — the page swaps to the
  // challenge surface.
  await expect(page.locator('[data-testid="mfa-challenge"]')).toBeVisible();

  if (args.backup !== undefined) {
    // Toggle to backup-code mode before typing.
    await page.click('[data-testid="mfa-toggle-mode"]');
    await expect(page.locator('[data-testid="mfa-code-input"]')).toHaveAttribute('data-mode', 'backup');
    await page.fill('[data-testid="mfa-code-input"]', args.backup);
  } else if (args.totp !== undefined) {
    await page.fill('[data-testid="mfa-code-input"]', args.totp);
  }

  const responsePromise = page.waitForResponse(
    (r) => r.url().includes('/auth/login/mfa'),
    { timeout: 20000 },
  );
  await page.click('[data-testid="mfa-submit"]');
  const resp = await responsePromise;

  if (expectSuccess) {
    expect(resp.status()).toBe(200);
    await page.waitForFunction(() => !window.location.pathname.startsWith('/login'), {
      timeout: 15000,
    });
  } else {
    expect(resp.status()).toBe(401);
    // We stay on the MFA challenge screen with an inline error.
    await expect(page.locator('[data-testid="mfa-server-error"]')).toBeVisible();
  }
}

async function disableMFA(page: Page, args: { totp?: string; backup?: string }) {
  await openSettingsPrivacy(page);
  await page.click('[data-testid="privacy-mfa-row"]');
  const dialog = page.locator('[data-testid="mfa-disable-dialog"]');
  await expect(dialog).toBeVisible();
  await dialog.locator('[data-testid="mfa-disable-password"]').fill(TEST_CREDENTIALS.password);
  if (args.backup !== undefined) {
    await dialog.locator('[data-testid="mfa-disable-toggle"]').click();
    await dialog.locator('[data-testid="mfa-disable-code"]').fill(args.backup);
  } else if (args.totp !== undefined) {
    await dialog.locator('[data-testid="mfa-disable-code"]').fill(args.totp);
  }
  await Promise.all([
    page.waitForResponse((r) => r.url().includes('/auth/mfa/disable') && r.status() === 200),
    dialog.locator('[data-testid="mfa-disable-confirm"]').click(),
  ]);
  await expect(page.locator('[data-testid="privacy-mfa-row"]')).toHaveAttribute(
    'data-mfa-state',
    'inactive',
    { timeout: 10000 },
  );
}

// ---------------------------------------------------------------------------
// Test
// ---------------------------------------------------------------------------

test.describe.serial('MFA / TOTP enrollment + login', () => {
  // Single end-to-end run: each step depends on the previous one's state.
  test('enroll → login with TOTP → login with backup code → reject reuse → disable', async ({ page }) => {
    await page.goto('/');
    await ensureAuthenticated(page);

    const { secret, backupCodes } = await enrollMFA(page);

    // Logout and re-login with a TOTP code. We regenerate the code here
    // rather than reusing the enrollment-verify code: even though the
    // server allows ±1 step and accepts in-window replay, deriving the
    // code at the moment of the call is robust to test slowness and
    // doesn't depend on the previous code being in the same window.
    await logout(page);
    const totp1 = authenticator.generate(secret);
    await loginWithMFA(page, { totp: totp1 }, true);

    // Logout and re-login using a backup code.
    await logout(page);
    const backup0 = backupCodes[0];
    await loginWithMFA(page, { backup: backup0 }, true);

    // Logout and try to replay the same backup code — must be rejected.
    await logout(page);
    await loginWithMFA(page, { backup: backup0 }, false);

    // Recover by typing a fresh TOTP code so the session can continue.
    await page.fill('[data-testid="mfa-code-input"]', '');
    await page.click('[data-testid="mfa-toggle-mode"]');
    const totp2 = authenticator.generate(secret);
    await page.fill('[data-testid="mfa-code-input"]', totp2);
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes('/auth/login/mfa') && r.status() === 200,
      ),
      page.click('[data-testid="mfa-submit"]'),
    ]);
    await page.waitForFunction(() => !window.location.pathname.startsWith('/login'), {
      timeout: 15000,
    });

    // Cleanup so a re-run doesn't trip over leftover state.
    const totpForDisable = authenticator.generate(secret);
    await disableMFA(page, { totp: totpForDisable });
  });
});

// Re-export to satisfy the fixture-less test suite — keeps the file using
// the bare @playwright/test runner (login() above does its own auth).
export { login };
