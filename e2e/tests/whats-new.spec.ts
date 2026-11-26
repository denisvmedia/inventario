/**
 * E2E for the What's new page (#1386).
 *
 * The page lists the releases the build contains, read from CHANGELOG.md at
 * build time. The last release a user has seen is the per-user setting
 * `whats_new.last_seen_version`; while the build has a newer release, the
 * Settings → Help row and the sidebar's Help entry carry a dot, and opening
 * the page stores the newest release, which clears both.
 *
 * The CI images are built for a PR or for master ("pr-123", "edge"), which
 * are development builds: they keep every released version, so the page has
 * releases whatever the next tag will be.
 */
import { expect, type APIRequestContext } from "@playwright/test";
import { test as authTest } from "../fixtures/app-fixture.js";
import {
  extractApiAuth,
  resolveActiveGroup,
} from "./includes/commodities-api.js";

const LAST_SEEN = "whats_new.last_seen_version";

authTest.describe("What's new (#1386)", () => {
  authTest(
    "opens from Settings, and opening it clears the unread dot",
    async ({ page, request }) => {
      const auth = await extractApiAuth(page);
      const group = await resolveActiveGroup(request, auth);
      const settingsUrl = `/api/v1/g/${encodeURIComponent(group.slug)}/settings`;
      const headers = {
        "Content-Type": "application/vnd.api+json",
        Authorization: `Bearer ${auth.accessToken}`,
        "X-CSRF-Token": auth.csrfToken,
      };
      const setLastSeen = (req: APIRequestContext, value: string | null) =>
        req.patch(`${settingsUrl}/${LAST_SEEN}`, {
          headers,
          data: JSON.stringify(value),
        });

      // Put the value back afterwards; start as a user who never opened
      // the page, who sees the dot once.
      const before = await request.get(settingsUrl, { headers });
      expect(before.status()).toBe(200);
      const previous =
        ((await before.json()) as { whatsNewLastSeenVersion?: string })
          .whatsNewLastSeenVersion ?? null;
      expect((await setLastSeen(request, null)).status()).toBe(200);

      try {
        await page.goto(
          `/settings?section=help&g=${encodeURIComponent(group.slug)}`,
        );
        const row = page.getByTestId("help-row-whatsNew");
        await expect(row).toBeVisible({ timeout: 30000 });
        await expect(page.getByTestId("help-row-whatsNew-unread")).toBeVisible({
          timeout: 30000,
        });
        await expect(page.getByTestId("sidebar-help-unread")).toBeVisible();

        const saved = page.waitForResponse(
          (r) =>
            r.request().method() === "PATCH" &&
            r.url().includes(`/settings/${LAST_SEEN}`),
        );
        await row.click();
        await expect(page).toHaveURL(/\/whats-new/);
        await expect(page.getByTestId("whats-new-page")).toBeVisible();
        const releases = page
          .getByTestId("whats-new-releases")
          .getByRole("region");
        await expect(releases.first()).toBeVisible();
        await expect(
          releases.first().getByRole("heading", { level: 2 }),
        ).toHaveText(/^v\d+\.\d+\.\d+/);
        expect((await saved).status()).toBe(200);
        await expect(page.getByTestId("sidebar-help-unread")).toHaveCount(0);

        // The stored value survives a reload: back on Settings, no dot.
        await page.goto(
          `/settings?section=help&g=${encodeURIComponent(group.slug)}`,
        );
        await expect(row).toBeVisible({ timeout: 30000 });
        await expect(page.getByTestId("help-row-whatsNew-unread")).toHaveCount(
          0,
        );
        await expect(page.getByTestId("sidebar-help-unread")).toHaveCount(0);
      } finally {
        await setLastSeen(request, previous);
      }
    },
  );
});
