/**
 * E2E coverage for asset IDs and QR labels (#2807).
 *
 *   1. A new item gets an asset ID, shown on its page and found by the
 *      items list `q` and by the search page.
 *   2. The item's Print label action opens the label page with the asset
 *      ID and a QR code that encodes the item URL; under print emulation
 *      the toolbar is gone and the label starts at the corner of the page.
 *   3. The area page prints the area label plus one label per item on a
 *      sheet, with the first label at the sheet's die-cut position.
 *   4. Opening an item URL (what a scanned label holds) while signed out
 *      goes through login and lands on the item.
 *
 * The native print dialog is out of reach for Playwright, so the Print
 * button is never clicked.
 */
import { expect, type APIRequestContext } from "@playwright/test";

import { test } from "../fixtures/app-fixture.js";
import { TEST_CREDENTIALS, login } from "./includes/auth.js";
import {
  createAreaViaAPI,
  createCommodityViaAPI,
  deleteAreaViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
} from "./includes/commodities-api.js";

function formatAssetId(n: number): string {
  return `A-${String(n).padStart(4, "0")}`;
}

async function assetIdOf(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<string> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
    {
      headers: {
        Accept: "application/vnd.api+json",
        Authorization: `Bearer ${auth.accessToken}`,
      },
    },
  );
  expect(resp.ok(), `GET /commodities/${id} → ${resp.status()}`).toBeTruthy();
  const n = (await resp.json())?.data?.attributes?.asset_number;
  expect(typeof n, "the API returns an asset_number for a new item").toBe("number");
  return formatAssetId(n as number);
}

test.describe("Asset IDs and QR labels (#2807)", () => {
  let auth: ApiAuth;
  let slug: string;
  let groupCurrency: string;
  let locationId: string;
  let areaId: string;
  const commodityIds: string[] = [];
  const areaIds: string[] = [];

  test.beforeEach(async ({ page, request }) => {
    auth = await extractApiAuth(page);
    const group = await resolveActiveGroup(request, auth);
    slug = group.slug;
    groupCurrency = group.groupCurrency;
    ({ locationId, areaId } = await ensureLocationAndArea(request, auth, slug));
  });

  test.afterEach(async ({ request }) => {
    for (const id of commodityIds.splice(0)) {
      await deleteCommodityViaAPI(request, auth, slug, id).catch(() => {});
    }
    for (const id of areaIds.splice(0)) {
      await deleteAreaViaAPI(request, auth, slug, id).catch(() => {});
    }
  });

  const suffix = () => `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

  test("a new item shows its asset ID and is found by it", async ({ page, request }) => {
    const name = `Asset ID Item ${suffix()}`;
    const { id } = await createCommodityViaAPI(request, auth, slug, { name, areaId }, groupCurrency);
    commodityIds.push(id);
    const assetId = await assetIdOf(request, auth, slug, id);

    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`);
    // The badge also carries a screen-reader prefix ("Asset ID").
    await expect(page.getByTestId("commodity-detail-asset-id")).toContainText(assetId);

    await page.goto(`/g/${encodeURIComponent(slug)}/commodities?q=${encodeURIComponent(assetId)}`);
    await expect(page.getByText(name).first()).toBeVisible();

    await page.goto(`/g/${encodeURIComponent(slug)}/search?q=${encodeURIComponent(assetId)}`);
    const result = page.getByTestId(`result-commodity-${id}`);
    await expect(result).toBeVisible();
    await expect(result).toContainText(assetId);
  });

  test("the item label carries the asset ID and a QR code for the item page", async ({
    page,
    request,
  }) => {
    const name = `Label Item ${suffix()}`;
    const { id } = await createCommodityViaAPI(request, auth, slug, { name, areaId }, groupCurrency);
    commodityIds.push(id);
    const assetId = await assetIdOf(request, auth, slug, id);

    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`);
    await page.getByTestId("commodity-detail-print-label").click();
    await expect(page).toHaveURL(/\/labels\?item=/);

    const preview = page.getByTestId("labels-preview");
    await expect(preview).toHaveAttribute("data-format", "single");
    const label = preview.getByTestId("label");
    await expect(label).toHaveCount(1);
    await expect(label.getByTestId("label-code")).toHaveText(assetId);
    await expect(label).toContainText(name);
    const origin = new URL(page.url()).origin;
    await expect(label).toHaveAttribute(
      "data-label-url",
      `${origin}/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
    );
    await expect(label.locator("svg")).toBeVisible();

    // On paper the shell and the toolbar are gone and the label starts at
    // the corner of the page: any leftover padding would print every
    // label off its die-cut position.
    await page.emulateMedia({ media: "print" });
    await expect(page.getByTestId("labels-toolbar")).toBeHidden();
    const box = await preview.getByTestId("label-sheet").boundingBox();
    expect(box?.x).toBeCloseTo(0, 0);
    expect(box?.y).toBeCloseTo(0, 0);
    await page.emulateMedia({ media: "screen" });
  });

  test("the area page prints the area label and one label per item", async ({ page, request }) => {
    const area = await createAreaViaAPI(request, auth, slug, locationId, `Label Box ${suffix()}`);
    areaIds.push(area.id);
    for (const name of ["Box Drill", "Box Ladder"]) {
      const { id } = await createCommodityViaAPI(
        request,
        auth,
        slug,
        { name: `${name} ${suffix()}`, areaId: area.id },
        groupCurrency,
      );
      // Items go first in cleanup: the area cannot be deleted while it holds any.
      commodityIds.push(id);
    }

    await page.goto(`/g/${encodeURIComponent(slug)}/areas/${encodeURIComponent(area.id)}`);
    await page.getByTestId("area-detail-print-labels").click();
    await page.getByTestId("area-detail-print-item-labels").click();
    await expect(page).toHaveURL(/\/labels\?area=.*&area_items=/);

    const labels = page.getByTestId("labels-preview").getByTestId("label");
    await expect(labels).toHaveCount(3);
    await expect(labels.first()).toHaveAttribute(
      "data-label-url",
      new RegExp(`/areas/${area.id}$`),
    );
    await expect(labels.first()).toContainText(area.name);
    await expect(labels.nth(1).getByTestId("label-code")).toHaveText(/^A-\d{4,}$/);

    await page.getByTestId("labels-format-select").click();
    await page.getByTestId("labels-format-a4-3x7").click();
    const preview = page.getByTestId("labels-preview");
    await expect(preview).toHaveAttribute("data-format", "a4-3x7");
    await expect(preview.getByTestId("label-sheet")).toHaveCount(1);

    // Avery L7160: the first label sits 7.21 mm from the left and
    // 15.15 mm from the top of the sheet (96 CSS px per inch).
    await page.emulateMedia({ media: "print" });
    const mm = 96 / 25.4;
    const first = await labels.first().boundingBox();
    expect(first?.x).toBeCloseTo(7.21 * mm, 0);
    expect(first?.y).toBeCloseTo(15.15 * mm, 0);
    expect(first?.width).toBeCloseTo(63.5 * mm, 0);
    expect(first?.height).toBeCloseTo(38.1 * mm, 0);
    await page.emulateMedia({ media: "screen" });
  });

  test("an item URL opened while signed out goes through login to the item", async ({
    page,
    request,
    browser,
  }) => {
    const name = `Scanned Item ${suffix()}`;
    const { id } = await createCommodityViaAPI(request, auth, slug, { name, areaId }, groupCurrency);
    commodityIds.push(id);
    const itemPath = `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`;
    // The label's QR code holds this absolute URL.
    const scannedUrl = new URL(itemPath, page.url()).toString();

    const context = await browser.newContext();
    try {
      const phone = await context.newPage();
      await phone.goto(scannedUrl);
      await expect(phone).toHaveURL(/\/login\?/);
      expect(new URL(phone.url()).searchParams.get("redirect")).toBe(itemPath);

      await login(phone, undefined, TEST_CREDENTIALS);
      await expect(phone).toHaveURL(new RegExp(`${itemPath}$`));
      await expect(phone.getByTestId("commodity-detail-name")).toHaveText(name);
    } finally {
      await context.close();
    }
  });
});
