/**
 * E2E for the Settings → Notifications surface (#1643).
 *
 * The acceptance criterion on #1643 calls for an end-to-end probe of the
 * opt-out path: flip a notification toggle off, send-side observes the
 * suppression, flip back on, send-side fires again. The send-side half
 * (the warranty reminder worker calling
 * `(*notifications.Service).IsEnabledForGroup` — equivalently
 * `(*notifications.Cache).IsEnabledForGroup` from a per-sweep cache —
 * defined in `go/services/notifications/preferences.go`) is already
 * locked in by the Go unit test in
 * `services/warranty_reminder_service_test.go` and the per-group BE
 * tests under `#1648`. Reproducing it in Playwright would require an
 * admin-only "force-run worker" endpoint we don't have, plus a real
 * mailpit cycle that's flaky against the 1-hour default sweep cadence.
 *
 * What this spec covers is the FE→BE wire-up that those unit tests
 * cannot: the Switch click hits `PATCH /g/{slug}/settings/{field}` with
 * the correct body, and the value survives a reload. The send-side gate
 * is already covered by the unit tests above.
 */
import { expect, type Locator, type Page } from "@playwright/test";
import { test as authTest } from "../fixtures/app-fixture.js";

// The lib/http rewrite middleware prepends `/g/{slug}` to `/settings`,
// so any non-empty slug between `/g/` and `/settings(/...)?` matches.
const SETTINGS_PATH = /\/api\/v1\/g\/[^/]+\/settings(\/.+)?$/;

interface Settings {
  notificationsWarrantyExpiry?: boolean;
}

// openNotificationsSection navigates to /settings and waits for both
// the GET /settings response AND the Switch enabled state before
// returning the warranty-expiry toggle locator. Returns the toggle plus
// the BE-confirmed initial value so callers don't have to re-read
// `aria-checked` (which races against the React render cycle after
// the GET resolves — see the `disabled={!settings}` defence in
// SettingsPage's NotificationsSection).
async function openNotificationsSection(
  page: Page,
): Promise<{ toggle: Locator; serverWasOn: boolean }> {
  // Capture the next GET /settings response so we can read the
  // BE-confirmed value directly. The Switch's `aria-checked` is
  // controlled by the same value but React's commit phase to DOM is
  // one microtask away from the response landing — reading it from the
  // network avoids that race entirely.
  const settingsGet = page.waitForResponse(
    (resp) =>
      SETTINGS_PATH.test(resp.url()) &&
      !resp.url().includes("/settings/") &&
      resp.request().method() === "GET" &&
      resp.status() === 200,
    { timeout: 20000 },
  );
  await page.goto("/settings");
  const settingsResp = await settingsGet;
  const settingsBody = (await settingsResp.json()) as Settings;
  const serverWasOn = settingsBody.notificationsWarrantyExpiry ?? true;

  const nav = page.locator('[data-testid="settings-nav-notifications"]');
  await expect(nav).toBeVisible({ timeout: 15000 });
  await nav.click();

  const row = page.locator('[data-testid="notification-row-warranty-expiry"]');
  await expect(row).toBeVisible({ timeout: 15000 });
  const toggle = row.locator('button[role="switch"]');
  await expect(toggle).toBeEnabled({ timeout: 15000 });
  // Pin the DOM state to the server-confirmed value before letting
  // callers click. If aria-checked doesn't match server within the
  // expect timeout, Radix hasn't repainted yet and a click would emit
  // the wrong onCheckedChange value.
  if (serverWasOn) {
    await expect(toggle).toBeChecked();
  } else {
    await expect(toggle).not.toBeChecked();
  }

  return { toggle, serverWasOn };
}

authTest.describe("Settings — notification preferences persist (#1643)", () => {
  authTest(
    "warranty_expiry toggle persists across reloads",
    async ({ page }) => {
      // ---- Phase 1: capture starting state, flip it, prove BE saw it.
      const { toggle, serverWasOn: initialOn } =
        await openNotificationsSection(page);

      const flipPatch = page.waitForResponse(
        (resp) =>
          /\/api\/v1\/g\/[^/]+\/settings\/notifications\.warranty_expiry$/.test(
            resp.url(),
          ) && resp.request().method() === "PATCH",
        { timeout: 15000 },
      );
      await toggle.click();
      const flipResp = await flipPatch;
      expect(flipResp.status()).toBe(200);
      // The FE patches with the primitive value as the raw body — see
      // patchSetting in features/settings/api.ts. JSON.stringify of a
      // boolean is the literal `"true"` or `"false"` string.
      expect(flipResp.request().postData()).toBe(JSON.stringify(!initialOn));

      // ---- Phase 2: reload — fresh react-query cache, fresh GET — and
      // assert the new value is what the server returned, not a
      // lingering optimistic paint.
      await page.reload();
      const { serverWasOn: persistedOn, toggle: reloadedToggle } =
        await openNotificationsSection(page);
      expect(persistedOn).toBe(!initialOn);

      // ---- Phase 3: restore the baseline so subsequent runs / cases
      // observe the same starting state. We don't reload-and-assert
      // again — the BE-confirmed POST body is enough to prove the
      // round-trip flipped the correct direction; Phase 2 already
      // covered the persistence side.
      const restorePatch = page.waitForResponse(
        (resp) =>
          /\/api\/v1\/g\/[^/]+\/settings\/notifications\.warranty_expiry$/.test(
            resp.url(),
          ) && resp.request().method() === "PATCH",
        { timeout: 15000 },
      );
      await reloadedToggle.click();
      const restoreResp = await restorePatch;
      expect(restoreResp.status()).toBe(200);
      expect(restoreResp.request().postData()).toBe(JSON.stringify(initialOn));
    },
  );

  authTest(
    "subgroup chrome renders the full mock-spec category set",
    async ({ page }) => {
      // Acceptance criterion: 3 subgroups (Reminders / Updates / Channels)
      // with autosave Switch rows. The autosave half is exercised above —
      // this case asserts the surface (and that the previous
      // ComingSoonBanner stubs are gone).
      await page.goto("/settings");
      await page.locator('[data-testid="settings-nav-notifications"]').click();

      await expect(
        page.locator('[data-testid="notification-row-warranty-expiry"]'),
      ).toBeVisible({ timeout: 15000 });
      await expect(
        page.locator('[data-testid="notification-row-maintenance-reminder"]'),
      ).toBeVisible();
      await expect(
        page.locator('[data-testid="notification-row-weekly-digest"]'),
      ).toBeVisible();
      await expect(
        page.locator('[data-testid="notification-row-price-drop"]'),
      ).toBeVisible();
      await expect(
        page.locator('[data-testid="notification-row-channel-email"]'),
      ).toBeVisible();
      await expect(
        page.locator('[data-testid="notification-row-channel-push"]'),
      ).toBeVisible();

      // The two ComingSoonBanner stubs the PR-A scope is meant to replace
      // must NOT linger after the section has been wired.
      await expect(
        page.locator(
          '[data-testid="coming-soon-banner-notificationPreferences"]',
        ),
      ).toHaveCount(0);
      await expect(
        page.locator('[data-testid="coming-soon-banner-maintenanceReminders"]'),
      ).toHaveCount(0);
    },
  );
});
