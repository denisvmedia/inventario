/**
 * E2E coverage for the group's sellers (#2804).
 *
 *   1. The item edit form creates a seller from its Seller picker, selects
 *      it, and the item page then names the seller.
 *   2. The Sellers page counts each seller's items and links to them, and
 *      deleting a seller after the confirmation that names the count keeps
 *      the items without a seller.
 *
 * Seller and item names carry a suffix unique to the run: seller names are
 * unique in a group and the e2e group is shared between specs.
 */
import { expect, type APIRequestContext } from "@playwright/test";

import { test } from "../fixtures/app-fixture.js";
import {
  createCommodityViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
} from "./includes/commodities-api.js";

function jsonHeaders(auth: ApiAuth): Record<string, string> {
  return {
    "Content-Type": "application/vnd.api+json",
    Accept: "application/vnd.api+json",
    Authorization: `Bearer ${auth.accessToken}`,
    "X-CSRF-Token": auth.csrfToken,
  };
}

async function itemAttrs(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<Record<string, unknown>> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
    { headers: jsonHeaders(auth) },
  );
  expect(resp.ok(), `GET /commodities/${id} → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as { data: { attributes: Record<string, unknown> } };
  return body.data.attributes;
}

async function createSeller(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  name: string,
): Promise<string> {
  const resp = await request.post(`/api/v1/g/${encodeURIComponent(slug)}/sellers`, {
    headers: jsonHeaders(auth),
    data: { data: { type: "sellers", attributes: { name } } },
  });
  expect(resp.status(), `POST /sellers → ${resp.status()}`).toBe(201);
  const body = (await resp.json()) as { data: { id: string } };
  return body.data.id;
}

// The id of the seller with this name, or null.
async function findSeller(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  name: string,
): Promise<string | null> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/sellers?q=${encodeURIComponent(name)}`,
    { headers: jsonHeaders(auth) },
  );
  if (!resp.ok()) return null;
  const body = (await resp.json()) as {
    data?: Array<{ id: string; attributes: { name?: string } }>;
  };
  return body.data?.find((row) => row.attributes.name === name)?.id ?? null;
}

async function deleteSeller(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<void> {
  await request.delete(
    `/api/v1/g/${encodeURIComponent(slug)}/sellers/${encodeURIComponent(id)}`,
    { headers: jsonHeaders(auth) },
  );
}

test.describe("Sellers (#2804)", () => {
  let auth: ApiAuth;
  let slug: string;
  let groupCurrency: string;
  let areaId: string;
  let run: string;
  const commodityIds: string[] = [];
  const sellerNames: string[] = [];

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
    for (const name of sellerNames.splice(0)) {
      const id = await findSeller(request, auth, slug, name);
      if (id) await deleteSeller(request, auth, slug, id);
    }
  });

  async function createItem(
    request: APIRequestContext,
    name: string,
    extra: { sellerId?: string; draft?: boolean } = {},
  ): Promise<string> {
    const { id } = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { name, shortName: "Item", areaId, ...extra },
      groupCurrency,
    );
    commodityIds.push(id);
    return id;
  }

  test("the edit form creates a seller and the item page names it", async ({
    page,
    request,
  }) => {
    const seller = `Hardware ${run}`;
    sellerNames.push(seller);
    const item = await createItem(request, `Drill ${run}`);

    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${item}/edit`);
    await expect(page.getByTestId("commodity-edit-form")).toBeVisible();
    const picker = page.locator("#edit-seller");
    await picker.click();
    await page.getByTestId("edit-seller-new").click();
    const dialog = page.getByTestId("seller-form-dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByTestId("seller-name-input").fill(seller);
    await dialog.getByTestId("seller-website-input").fill("https://hardware.example");
    await dialog.getByTestId("seller-form-submit").click();
    await expect(dialog).toBeHidden();
    await expect(picker).toContainText(seller);

    // The form is still open; saving it stores the seller on the item.
    await page.getByTestId("commodity-edit-save").click();
    await expect(page.getByTestId("commodity-edit-form")).toBeHidden();
    const sellerId = await findSeller(request, auth, slug, seller);
    expect(sellerId).toBeTruthy();
    expect((await itemAttrs(request, auth, slug, item)).seller_id).toBe(sellerId);
    await expect(page.getByTestId("commodity-detail-seller")).toContainText(seller);
  });

  test("the Sellers page counts items, links to them and deleting keeps them", async ({
    page,
    request,
  }) => {
    const seller = `Market ${run}`;
    sellerNames.push(seller);
    const sellerId = await createSeller(request, auth, slug, seller);
    const lamp = await createItem(request, `Lamp ${run}`, { sellerId });
    // A draft counts and is listed: the link includes inactive items.
    const rug = await createItem(request, `Rug ${run}`, { sellerId, draft: true });
    await createItem(request, `Vase ${run}`);

    await page.goto(`/g/${encodeURIComponent(slug)}/sellers?q=${encodeURIComponent(seller)}`);
    const row = page.getByTestId(`seller-row-${sellerId}`);
    await expect(row).toBeVisible();
    await expect(row.getByTestId(`seller-row-${sellerId}-usage`)).toHaveText("2 items");

    await row.getByTestId(`seller-row-${sellerId}-link`).click();
    await expect(page).toHaveURL(new RegExp(`[?&]seller=${sellerId}(&|$)`));
    await expect(page).toHaveURL(/[?&]inactive=1(&|$)/);
    const cards = page.getByTestId("commodity-card");
    await expect(cards).toHaveCount(2);
    await expect(cards.filter({ hasText: `Lamp ${run}` })).toHaveCount(1);
    await expect(cards.filter({ hasText: `Rug ${run}` })).toHaveCount(1);

    await page.goto(`/g/${encodeURIComponent(slug)}/sellers?q=${encodeURIComponent(seller)}`);
    await row.hover();
    await row.getByTestId(`seller-row-${sellerId}-delete`).click();
    const confirm = page.getByTestId("confirm-dialog");
    await expect(confirm).toContainText(`2 items were bought from ${seller}.`);
    await confirm.getByTestId("confirm-accept").click();
    await expect(row).toBeHidden();
    await expect(page.getByTestId("sellers-list-empty")).toBeVisible();

    for (const id of [lamp, rug]) {
      expect((await itemAttrs(request, auth, slug, id)).seller_id ?? null).toBeNull();
    }
  });
});
