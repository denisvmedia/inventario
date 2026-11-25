/**
 * E2E coverage for item photos on the items list and the item page (#2797).
 *
 *   1. The items list switches between photo cards, compact cards and the
 *      list, remembers the choice, and offers 24, 48 or 96 items per page,
 *      remembered on the device.
 *   2. An item with photos shows its cover on a photo card and on the item
 *      page; the cover on the item page opens the viewer, which walks
 *      through the item's photos.
 */
import * as fs from "node:fs";
import * as path from "node:path";

import { expect, type APIRequestContext, type Locator, type Page } from "@playwright/test";

import { test } from "../fixtures/app-fixture.js";
import {
  createCommodityViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
} from "./includes/commodities-api.js";

test.describe("Item photos on cards and on the item page (#2797)", () => {
  let auth: ApiAuth;
  let slug: string;
  let groupCurrency: string;
  let areaId: string;
  let run: string;
  const commodityIds: string[] = [];

  test.beforeEach(async ({ page, request }) => {
    auth = await extractApiAuth(page);
    const group = await resolveActiveGroup(request, auth);
    slug = group.slug;
    groupCurrency = group.groupCurrency;
    ({ areaId } = await ensureLocationAndArea(request, auth, slug));
    run = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  });

  test.afterEach(async ({ request }) => {
    for (const id of commodityIds.splice(0)) {
      await deleteCommodityViaAPI(request, auth, slug, id).catch(() => {});
    }
  });

  async function create(request: APIRequestContext, name: string): Promise<string> {
    const { id } = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { name, areaId, shortName: "Item" },
      groupCurrency,
    );
    commodityIds.push(id);
    return id;
  }

  const list = (query: string) => `/g/${encodeURIComponent(slug)}/commodities?${query}`;

  test("the list switches layouts and page sizes and remembers both", async ({
    page,
    request,
  }) => {
    // One more item than the smallest page, so the page size choice shows.
    for (let i = 0; i < 25; i += 1) {
      await create(request, `Shelf item ${String(i).padStart(2, "0")} ${run}`);
    }

    // `view=grid` pins the photo cards whatever this user's default view is.
    await page.goto(list(`q=${run}&view=grid`));
    const cards = page.getByTestId("commodity-card");
    await expect(cards).toHaveCount(24);
    await expect(page.getByTestId("commodities-grid")).toHaveAttribute("data-view", "grid");
    await expect(cards.first()).toHaveAttribute("data-variant", "photo");
    // An item without a photo shows its type icon in the photo box.
    await expect(cards.first().getByTestId("commodity-card-thumb")).toHaveAttribute(
      "data-state",
      "fallback",
    );
    // Two cards a row on a phone, four or more on a wide window.
    await page.setViewportSize({ width: 390, height: 844 });
    await expect.poll(() => cardsInFirstRow(page)).toBe(2);
    await page.setViewportSize({ width: 1600, height: 1000 });
    await expect.poll(() => cardsInFirstRow(page)).toBeGreaterThanOrEqual(4);

    await page.getByTestId("commodities-view-compact").click();
    await expect(page.getByTestId("commodities-grid")).toHaveAttribute("data-view", "compact");
    await expect(cards.first()).toHaveAttribute("data-variant", "compact");

    // Without `view` in the URL, the last layout picked on this device applies.
    await page.goto(list(`q=${run}`));
    await expect(page.getByTestId("commodities-grid")).toHaveAttribute("data-view", "compact");

    await page.getByTestId("commodities-view-list").click();
    await expect(page.getByTestId("commodities-table")).toBeVisible();
    await page.getByTestId("commodities-view-grid").click();
    await expect(page.getByTestId("commodities-grid")).toHaveAttribute("data-view", "grid");

    await expect(page.getByTestId("commodities-pagination")).toBeVisible();
    await page.getByTestId("commodities-page-size").click();
    await page.getByRole("option", { name: "48", exact: true }).click();
    await expect(page).toHaveURL(/[?&]per_page=48\b/);
    await expect(cards).toHaveCount(25);
    await expect(page.getByTestId("commodities-pagination")).toHaveCount(0);

    // Without `per_page` in the URL, the last page size picked applies.
    await page.goto(list(`q=${run}`));
    await expect(cards).toHaveCount(25);
    await expect(page.getByTestId("commodities-page-size")).toHaveText(/48/);
  });

  test("the cover shows on the card and the item page, and opens the item's photos", async ({
    page,
    request,
  }) => {
    const name = `Camera ${run}`;
    const id = await create(request, name);
    const itemPage = `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`;

    await page.goto(itemPage);
    await expect(page.getByTestId("page-commodity-detail")).toBeVisible();
    // No photo yet: the small type tile, no cover.
    await expect(page.getByTestId("commodity-detail-thumb")).toHaveAttribute(
      "data-state",
      "fallback",
    );
    await expect(page.getByTestId("commodity-detail-cover")).toHaveCount(0);

    await page.getByTestId("commodity-detail-tab-files").click();
    await uploadPhotos(page, run, 2);

    await page.reload();
    const cover = page.getByTestId("commodity-detail-cover");
    await expect(cover).toBeVisible();
    await expect(cover.getByTestId("commodity-detail-thumb")).toHaveAttribute(
      "data-state",
      "image",
    );
    await expect(cover.getByTestId("commodity-detail-photo-count")).toHaveText(/2/);
    // Beside the name on a wide window, above it on a phone.
    const heading = page.getByRole("heading", { level: 1, name });
    await page.setViewportSize({ width: 1280, height: 900 });
    await expect
      .poll(async () => (await box(cover)).right <= (await box(heading)).left)
      .toBe(true);
    await page.setViewportSize({ width: 390, height: 844 });
    await expect
      .poll(async () => (await box(cover)).bottom <= (await box(heading)).top)
      .toBe(true);
    await page.setViewportSize({ width: 1280, height: 900 });

    // The cover is a button: Enter opens the viewer, and Next walks to the
    // other photo.
    await cover.focus();
    await page.keyboard.press("Enter");
    const viewer = page.getByTestId("file-image-viewer");
    await expect(viewer).toBeVisible();
    const position = viewer.getByTestId("image-viewer-position");
    await expect(position).toHaveText(/[12] \/ 2/);
    const before = await position.textContent();
    await viewer.getByTestId("image-viewer-next").click();
    await expect(position).not.toHaveText(before ?? "");
    await page.keyboard.press("Escape");
    await expect(viewer).toHaveCount(0);

    // The photo card on the list shows the same cover, with every
    // thumbnail size offered to the browser.
    await page.goto(list(`q=${run}&view=grid`));
    const thumb = page
      .locator(`[data-commodity-id="${id}"]`)
      .getByTestId("commodity-card-thumb");
    await expect(thumb).toHaveAttribute("data-state", "image");
    await expect(thumb.locator("img")).toHaveAttribute("srcset", /\/large\b.* 600w/);
  });
});

// cardsInFirstRow counts the item cards level with the first one.
async function cardsInFirstRow(page: Page): Promise<number> {
  return page.getByTestId("commodity-card").evaluateAll((els) => {
    const top = els[0]?.getBoundingClientRect().top ?? 0;
    return els.filter((el) => Math.abs(el.getBoundingClientRect().top - top) < 2).length;
  });
}

interface Box {
  left: number;
  right: number;
  top: number;
  bottom: number;
}

async function box(locator: Locator): Promise<Box> {
  const b = await locator.boundingBox();
  if (!b) throw new Error("element has no bounding box");
  return { left: b.x, right: b.x + b.width, top: b.y, bottom: b.y + b.height };
}

// uploadPhotos attaches `count` photos to the open item through the Files
// tab. Each copy gets trailing bytes unique to the run, so the upload
// dialog does not offer to reuse a file it has already seen.
async function uploadPhotos(page: Page, run: string, count: number): Promise<void> {
  const image = fs.readFileSync(path.join("fixtures", "files", "image.jpg"));
  await page.getByTestId("commodity-files-upload-zone").click();
  await expect(page.getByTestId("files-upload-dialog")).toBeVisible();
  await page.getByTestId("files-upload-input").setInputFiles(
    Array.from({ length: count }, (_, i) => ({
      name: `photo-${i + 1}-${run}.jpg`,
      mimeType: "image/jpeg",
      buffer: Buffer.concat([image, Buffer.from(`e2e ${run} ${i}`)]),
    })),
  );
  await page.getByTestId("files-upload-next").click();
  const rows = page.locator('[data-testid^="files-upload-metadata-item-"]');
  await expect(rows).toHaveCount(count);
  for (let i = 0; i < count; i += 1) {
    await rows
      .nth(i)
      .locator('[data-testid^="files-upload-meta-category-"]')
      .selectOption("images");
  }
  await page.getByTestId("files-upload-start").click();
  await expect(page.getByTestId("files-upload-close")).toBeEnabled({ timeout: 30_000 });
  await expect(
    page.locator('[data-testid^="files-upload-progress-item-"][data-status="failed"]'),
  ).toHaveCount(0);
  await page.getByTestId("files-upload-close").click();
  await expect(page.getByTestId("files-upload-dialog")).toHaveCount(0);
}
