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
import { generateSync } from 'otplib';

import { ensureAuthenticated, TEST_CREDENTIALS, login } from './includes/auth.js';

// otplib defaults match our backend: SHA-1, 30s step, 6 digits.
function generateTOTP(secret: string): string {
  return generateSync({ secret, digits: 6, period: 30 });
}

// Sleep into the next 30s TOTP step, so the code generated after this call
// belongs to a step the replay guard (#2124) has not seen.
async function waitForNextTotpStep(): Promise<void> {
  const msIntoStep = Date.now() % 30_000;
  await new Promise((resolve) => setTimeout(resolve, 30_000 - msIntoStep + 500));
}

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
  const code = generateTOTP(secret);
  await dialog.locator('[data-testid="mfa-setup-code"]').fill(code);
  // Capture the verify response so the test can assert "the FE rendered
  // every code the BE issued" without baking the configured count into
  // the spec — if services.MFABackupCodeCount is bumped this still holds.
  const verifyRespPromise = page.waitForResponse(
    (r) => r.url().includes('/auth/mfa/verify') && r.status() === 200,
  );
  await dialog.locator('[data-testid="mfa-setup-verify"]').click();
  const verifyResp = await verifyRespPromise;
  const verifyBody = (await verifyResp.json()) as { backup_codes?: string[] };
  const apiCodes = verifyBody.backup_codes ?? [];
  expect(apiCodes.length).toBeGreaterThan(0);

  // The dialog flips to the backup-codes panel; assert the rendered grid
  // matches the API response one-to-one.
  const codesGrid = dialog.locator('[data-testid="mfa-backup-codes"]');
  await expect(codesGrid).toBeVisible();
  const backupCodes = (await codesGrid.locator('span').allInnerTexts()).map((s) => s.trim());
  expect(backupCodes.length).toBe(apiCodes.length);

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
  // Disable invalidates every session for the user (mirrors change-password —
  // see auth_mfa.go disable handler). The FE invalidates the MFA status
  // query, the refetch hits a blacklisted access token, the refresh attempt
  // also fails (refresh tokens were revoked too), and `handle401` clears auth
  // + redirects to /login. Either bounce — back to /login or the row flipping
  // to inactive before the bounce — is a valid terminal state for this step.
  await Promise.race([
    page.waitForURL(/\/login(\?|$)/, { timeout: 15000 }),
    page
      .locator('[data-testid="privacy-mfa-row"][data-mfa-state="inactive"]')
      .waitFor({ timeout: 15000 }),
  ]);
}

// ---------------------------------------------------------------------------
// Test
// ---------------------------------------------------------------------------

test.describe.serial('MFA / TOTP enrollment + login', () => {
  // Held for the teardown below. admin@test-org.com is shared with every other
  // spec, so MFA left enabled here does not fail this test — it stalls each
  // later login on the challenge until the 120s global timeout, one test at a
  // time, until the lane runs out of budget and is cancelled mid-suite (#2249).
  // Cleared on the clean path, where the test disables MFA itself.
  let enrolledSecret: string | null = null;

  // Single end-to-end run: each step depends on the previous one's state.
  test('enroll → login with TOTP → login with backup code → reject reuse → disable', async ({ page }) => {
    await page.goto('/');
    await ensureAuthenticated(page);

    const { secret, backupCodes } = await enrollMFA(page);
    enrolledSecret = secret;

    // Logout and re-login with a fresh TOTP code. This consumes the current
    // 30s time-step: the #2124 replay guard records last_used_step, so any
    // later TOTP in the same window would now be rejected as a replay. That
    // is why the recover + disable steps below use backup codes instead of a
    // second same-window TOTP (the pre-#2124 spec relied on in-window TOTP
    // replay being accepted, which it no longer is — RFC 6238 §5.2).
    await logout(page);
    const totp1 = generateTOTP(secret);
    await loginWithMFA(page, { totp: totp1 }, true);

    // Logout and re-login using a backup code.
    await logout(page);
    const backup0 = backupCodes[0];
    await loginWithMFA(page, { backup: backup0 }, true);

    // Logout and try to replay the same backup code — must be rejected.
    await logout(page);
    await loginWithMFA(page, { backup: backup0 }, false);

    // Recover so the session can continue. The login TOTP above already
    // consumed this 30s step, so a same-window TOTP would be rejected by the
    // #2124 replay guard — recover with a fresh, unused backup code. The
    // challenge is already in backup-code mode from the rejected replay above.
    await page.fill('[data-testid="mfa-code-input"]', backupCodes[1]);
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes('/auth/login/mfa') && r.status() === 200,
      ),
      page.click('[data-testid="mfa-submit"]'),
    ]);
    await page.waitForFunction(() => !window.location.pathname.startsWith('/login'), {
      timeout: 15000,
    });

    // Cleanup so a re-run doesn't trip over leftover state. Disable with
    // another fresh backup code (a same-window TOTP would again be rejected
    // by the replay guard).
    await disableMFA(page, { backup: backupCodes[2] });
    enrolledSecret = null;
  });

  // Runs whatever the test did. A failure anywhere after enrollMFA used to
  // leave the shared user enrolled, and the damage lands on the specs that
  // follow rather than here — see the note on enrolledSecret.
  //
  // TOTP rather than a backup code: the codes consumed above depend on how far
  // the test got, and guessing wrong leaves MFA on. The #2124 replay guard
  // rejects a code from an already-used 30s step, so this waits out the
  // current step before each submission rather than tracking which steps were
  // used.
  test.afterAll(async ({ browser }) => {
    if (enrolledSecret === null) {
      return;
    }

    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      await waitForNextTotpStep();
      await loginWithMFA(page, { totp: generateTOTP(enrolledSecret) }, true);
      await waitForNextTotpStep();
      await disableMFA(page, { totp: generateTOTP(enrolledSecret) });
      enrolledSecret = null;
    } catch (err) {
      // Rethrown deliberately. A silent miss here is the failure this teardown
      // exists to prevent, and it surfaces later as unrelated specs timing out
      // on a login prompt nobody expected.
      console.error(
        '[mfa.spec] teardown could not disable MFA on the shared user; ' +
          'later specs that log in as ' + TEST_CREDENTIALS.email + ' will stall ' +
          'on the MFA challenge until they time out',
        err,
      );
      throw err;
    } finally {
      await context.close();
    }
  });
});

// Re-export to satisfy the fixture-less test suite — keeps the file using
// the bare @playwright/test runner (login() above does its own auth).
export { login };
