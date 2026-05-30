/**
 * E2E coverage for the insurance report (#1370). The report is a
 * print-capable page mounted inside the Shell at
 * `/g/:slug/reports/insurance`, with two modes (item / location) driven by
 * the query string. This spec walks the user-facing surface:
 *
 *   1. Reports landing card → opens the insurance report.
 *   2. Commodity detail "Insurance report" action → item mode deep-link.
 *   3. Location detail "Insurance report" action → location mode deep-link.
 *   4. The toolbar collapses under print emulation (`.print-hide`).
 *
 * We deliberately don't drive the native `window.print()` dialog
 * (Playwright can't), so the Print button is only asserted visible /
 * print-hidden, not clicked.
 */
import { expect } from "@playwright/test";

import { test } from "../fixtures/app-fixture.js";
import { ensureGroupSlug } from "./includes/group-url.js";
import {
  createCommodityViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
} from "./includes/commodities-api.js";

test.describe("Insurance report (#1370)", () => {
  test("reachable from the Reports landing card", async ({ page }) => {
    const slug = await ensureGroupSlug(page);
    await page.goto(`/g/${encodeURIComponent(slug)}/reports`);
    await expect(page.getByTestId("page-reports")).toBeVisible();

    const card = page.getByTestId("reports-card-insurance");
    await expect(card).toBeVisible();
    await card.getByRole("link").first().click();

    await expect(page).toHaveURL(/\/reports\/insurance/);
    await expect(page.getByTestId("page-insurance-report")).toBeVisible();
    await expect(page.getByTestId("insurance-report-print")).toBeVisible();
  });

  test("commodity detail deep-links into item mode", async ({
    page,
    request,
  }) => {
    const auth = await extractApiAuth(page);
    const group = await resolveActiveGroup(request, auth);
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug);
    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const seededIDs: string[] = [];
    const cleanup = async () => {
      for (const id of seededIDs) {
        await deleteCommodityViaAPI(request, auth, group.slug, id).catch(
          () => {},
        );
      }
    };

    try {
      const { id: commodityID } = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: `Insurance Item ${suffix}`, areaId, type: "electronics" },
        group.groupCurrency,
      );
      seededIDs.push(commodityID);

      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(commodityID)}`,
      );
      const insuranceLink = page.getByTestId("commodity-detail-insurance");
      await expect(insuranceLink).toBeVisible();
      await insuranceLink.click();

      await expect(page).toHaveURL(/\/reports\/insurance\?mode=item&item=/);
      await expect(page.getByTestId("page-insurance-report")).toBeVisible();
      await expect(page.getByTestId("report-item")).toBeVisible();
    } finally {
      await cleanup();
    }
  });

  test("location detail deep-links into location mode", async ({
    page,
    request,
  }) => {
    const auth = await extractApiAuth(page);
    const group = await resolveActiveGroup(request, auth);
    const { locationId } = await ensureLocationAndArea(
      request,
      auth,
      group.slug,
    );

    await page.goto(
      `/g/${encodeURIComponent(group.slug)}/locations/${encodeURIComponent(locationId)}`,
    );
    const insuranceLink = page.getByTestId("location-detail-insurance");
    await expect(insuranceLink).toBeVisible();
    await insuranceLink.click();

    await expect(page).toHaveURL(
      /\/reports\/insurance\?mode=location&location=/,
    );
    await expect(page.getByTestId("page-insurance-report")).toBeVisible();
    await expect(page.getByTestId("report-location")).toBeVisible();
  });

  test("toolbar is hidden under print media", async ({ page }) => {
    const slug = await ensureGroupSlug(page);
    await page.goto(`/g/${encodeURIComponent(slug)}/reports/insurance`);
    await expect(page.getByTestId("page-insurance-report")).toBeVisible();

    await page.emulateMedia({ media: "print" });
    await expect(page.getByTestId("insurance-report-toolbar")).toBeHidden();
    await expect(page.getByTestId("insurance-report-print")).toBeHidden();
  });
});
