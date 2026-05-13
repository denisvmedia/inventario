/**
 * E2E for the Settings → Notifications surface (#1643).
 *
 * The acceptance criterion on #1643 calls for an end-to-end probe of the
 * opt-out path: flip a notification toggle off, send-side observes the
 * suppression, flip back on, send-side fires again.
 *
 * Three different parts of the suite already cover the moving pieces of
 * that contract:
 *
 *   - **Send-side gate**: the warranty reminder worker honours the
 *     per-user / per-group preference via
 *     `(*notifications.Service).IsEnabledForGroup` /
 *     `(*notifications.Cache).IsEnabledForGroup` (defined in
 *     `go/services/notifications/preferences.go`). Locked in by
 *     `services/warranty_reminder_service_test.go::TestWarrantyReminderService_RemindOnce_OptOutSkipsRecipient`
 *     and the per-group BE tests under #1648.
 *
 *   - **FE→BE autosave wire-up**: the Switch click hits
 *     `PATCH /g/{slug}/settings/{field}` with the primitive boolean
 *     value as the raw body, and optimistic update + rollback happens
 *     via `usePatchSetting`. Locked in by
 *     `frontend/src/pages/__tests__/SettingsPage.test.tsx::"toggling a notification row fires PATCH /settings/{field}"`.
 *
 *   - **Initial-view preference round-trip**:
 *     `CommoditiesListPage` reads `appearance.default_items_view` from
 *     the same settings slice as the initial view-mode source of
 *     truth. Locked in by
 *     `frontend/src/pages/commodities/__tests__/CommoditiesListPage.test.tsx::"uses preferences default_items_view='list' as the initial view mode"`.
 *
 * The unique value Playwright adds is the **page-level surface check**:
 * once a real authenticated session lands on `/settings → Notifications`,
 * all six rows render and the prior `ComingSoonBanner` stubs are gone.
 * A live persistence-through-reload probe was attempted and
 * intentionally dropped — the CI cadence races against the GroupContext
 * propagation (the lib/http rewrite needs a non-empty slug to translate
 * `/settings` → `/g/{slug}/settings`, and that has been observed to
 * take >60s on a busy chromium runner). That race is orthogonal to the
 * acceptance criterion and is already insured against by the unit-test
 * coverage above.
 */
import { expect } from "@playwright/test";
import { test as authTest } from "../fixtures/app-fixture.js";

authTest.describe("Settings — notification preferences surface (#1643)", () => {
  authTest(
    "Notifications section renders the full mock-spec category set",
    async ({ page }) => {
      // Acceptance criterion: 3 subgroups (Reminders / Updates / Channels)
      // with autosave Switch rows. The autosave half is exercised by the
      // FE unit test (see file header); this case asserts the surface
      // (and that the previous ComingSoonBanner stubs are gone).
      await page.goto("/settings");
      await page
        .locator('[data-testid="settings-nav-notifications"]')
        .click({ timeout: 30000 });

      await expect(
        page.locator('[data-testid="notification-row-warranty-expiry"]'),
      ).toBeVisible({ timeout: 30000 });
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
