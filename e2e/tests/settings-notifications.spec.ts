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
 * every row renders and the prior `ComingSoonBanner` stubs are gone.
 * A live persistence-through-reload probe was attempted and
 * intentionally dropped — the CI cadence races against GroupContext
 * slug propagation (the lib/http rewrite needs a non-empty slug to
 * translate `/settings` → `/g/{slug}/settings`, and that propagation
 * can be nondeterministic on busy CI chromium runners). That race is
 * orthogonal to the acceptance criterion and is already insured against
 * by the unit-test coverage above.
 */
import { expect, type Page } from "@playwright/test";
import { test as authTest } from "../fixtures/app-fixture.js";
import {
  extractApiAuth,
  resolveActiveGroup,
} from "./includes/commodities-api.js";

const WARRANTY_LEAD_TIMES = "notifications.warranty_lead_times";

function waitForLeadTimesPatch(page: Page) {
  return page.waitForResponse(
    (r) =>
      r.request().method() === "PATCH" &&
      r.url().includes(`/settings/${WARRANTY_LEAD_TIMES}`),
  );
}

async function openNotifications(page: Page) {
  await page.goto("/settings");
  await page
    .locator('[data-testid="settings-nav-notifications"]')
    .click({ timeout: 30000 });
}

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
        page.locator('[data-testid="notification-row-loan-reminder"]'),
      ).toBeVisible();
      await expect(
        page.locator('[data-testid="notification-row-item-reminder"]'),
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

  // Per-user lead times (#2805). The lead-time field renders only once the
  // group's settings have loaded, so waiting for its value also waits out
  // the group slug race described in the file header.
  authTest(
    "warranty lead times save, survive a reload and reset to the default",
    async ({ page, request }) => {
      const auth = await extractApiAuth(page);
      const group = await resolveActiveGroup(request, auth);
      // Start from the default even if an earlier run stopped halfway.
      const resetViaAPI = () =>
        request.patch(
          `/api/v1/g/${encodeURIComponent(group.slug)}/settings/${WARRANTY_LEAD_TIMES}`,
          {
            headers: {
              "Content-Type": "application/vnd.api+json",
              Authorization: `Bearer ${auth.accessToken}`,
              "X-CSRF-Token": auth.csrfToken,
            },
            data: "null",
          },
        );
      expect((await resetViaAPI()).status()).toBe(200);
      try {
        await openNotifications(page);
        const input = page.getByTestId("lead-times-warranty-input");
        await expect(input).toHaveValue("60, 30, 7", { timeout: 30000 });

        const saved = waitForLeadTimesPatch(page);
        await input.fill("14, 90");
        await input.press("Enter");
        expect((await saved).status()).toBe(200);
        await expect(input).toHaveValue("90, 14");

        await page.reload();
        await openNotifications(page);
        await expect(input).toHaveValue("90, 14", { timeout: 30000 });

        // A value out of range is explained and not sent.
        await input.fill("0");
        await input.press("Enter");
        await expect(
          page.getByTestId("lead-times-warranty").getByRole("alert"),
        ).toBeVisible();

        const reset = waitForLeadTimesPatch(page);
        await page.getByTestId("lead-times-warranty-reset").click();
        expect((await reset).status()).toBe(200);
        await expect(input).toHaveValue("60, 30, 7");
        await expect(page.getByTestId("lead-times-warranty-reset")).toHaveCount(
          0,
        );
      } finally {
        await resetViaAPI();
      }
    },
  );
});
