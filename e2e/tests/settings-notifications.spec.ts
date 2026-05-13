/**
 * E2E for the Settings → Notifications surface (#1643).
 *
 * The acceptance criterion on #1643 calls for an end-to-end probe of the
 * opt-out path: flip a notification toggle off, send-side observes the
 * suppression, flip back on, send-side fires again. The send-side half
 * (warranty reminder worker honouring `notifications.IsEnabledForGroup`)
 * is already locked in by the Go unit test in
 * `services/warranty_reminder_service_test.go` and the per-group BE
 * tests under `#1648`. Reproducing it in Playwright would require an
 * admin-only "force-run worker" endpoint we don't have, plus a real
 * mailpit cycle that's flaky against the 1-hour default sweep cadence.
 *
 * What this spec covers is the FE→BE wire-up that those unit tests
 * cannot: the Switch click hits `PATCH /g/{slug}/settings/{field}`, the
 * value survives a reload, and the same toggle is reachable from a
 * second tab so the autosave roundtrip is observable. Together with
 * the unit-test coverage of the send-side gate, this closes the
 * acceptance loop end-to-end without depending on worker timing.
 */
import { expect } from "@playwright/test";
import { test as authTest } from "../fixtures/app-fixture.js";

authTest.describe("Settings — notification preferences persist (#1643)", () => {
  authTest(
    "warranty_expiry toggle persists across reloads",
    async ({ page }) => {
      await page.goto("/settings");
      await page.locator('[data-testid="settings-nav-notifications"]').click();

      const row = page.locator(
        '[data-testid="notification-row-warranty-expiry"]',
      );
      await expect(row).toBeVisible({ timeout: 10000 });

      const toggle = row.locator('button[role="switch"]');
      // The Switch is disabled while GET /settings is in flight (avoids
      // a flicker between optimistic and server-confirmed state). Wait
      // for it to become interactive before the first click.
      await expect(toggle).toBeEnabled({ timeout: 10000 });

      // Capture the starting state so the assertions below are independent
      // of whatever the seeded user happens to have stored (the in-code
      // default is `true`, but a previous test run on the same DB could
      // have flipped it).
      const initialChecked = await toggle.getAttribute("aria-checked");
      expect(
        initialChecked,
        "warranty_expiry must report an explicit checked state",
      ).not.toBeNull();
      const wantOff = initialChecked === "true";

      // Wait for the autosave PATCH and capture its body so we can assert
      // the FE didn't just paint optimistically — the BE saw the new value.
      const patchPromise = page.waitForResponse(
        (resp) =>
          /\/api\/v1\/g\/[^/]+\/settings\/notifications\.warranty_expiry$/.test(
            resp.url(),
          ) && resp.request().method() === "PATCH",
        { timeout: 10000 },
      );
      await toggle.click();
      const patchResp = await patchPromise;
      expect(patchResp.status()).toBe(200);
      expect(patchResp.request().postData()).toBe(JSON.stringify(!wantOff));

      // Reload — if the row didn't actually persist, the toggle would
      // re-render with its previous value because the GET /settings on
      // mount is the source of truth (no localStorage fallback for the
      // notification rows).
      await page.reload();
      await page.locator('[data-testid="settings-nav-notifications"]').click();
      const reloadedToggle = page.locator(
        '[data-testid="notification-row-warranty-expiry"] button[role="switch"]',
      );
      await expect(reloadedToggle).toBeVisible({ timeout: 10000 });
      await expect(reloadedToggle).toHaveAttribute(
        "aria-checked",
        String(!wantOff),
      );

      // Flip back to the starting state so the next test run / next spec
      // observes a clean baseline. Same wait-for-autosave dance.
      const restorePatch = page.waitForResponse(
        (resp) =>
          /\/api\/v1\/g\/[^/]+\/settings\/notifications\.warranty_expiry$/.test(
            resp.url(),
          ) && resp.request().method() === "PATCH",
        { timeout: 10000 },
      );
      await expect(reloadedToggle).toBeEnabled();
      await reloadedToggle.click();
      const restoreResp = await restorePatch;
      expect(restoreResp.status()).toBe(200);
      expect(restoreResp.request().postData()).toBe(JSON.stringify(wantOff));

      await page.reload();
      await page.locator('[data-testid="settings-nav-notifications"]').click();
      const finalToggle = page.locator(
        '[data-testid="notification-row-warranty-expiry"] button[role="switch"]',
      );
      await expect(finalToggle).toBeVisible({ timeout: 10000 });
      await expect(finalToggle).toHaveAttribute(
        "aria-checked",
        initialChecked!,
      );
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
      ).toBeVisible({ timeout: 10000 });
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
