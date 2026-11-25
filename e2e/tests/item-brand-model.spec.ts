/**
 * E2E coverage for an item's brand and model (#2804).
 *
 *   1. The edit form suggests a brand another item of the group carries;
 *      the arrow keys and Enter pick it, and the saved brand and model show
 *      under the item's name on its card.
 *   2. The items list finds an item by its model and says the model
 *      matched.
 *
 * Every brand, model and name carries a suffix unique to the run, so the
 * suggestions and the searched list hold only this run's items.
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

async function itemAttrs(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<Record<string, unknown>> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
    {
      headers: {
        Accept: "application/vnd.api+json",
        Authorization: `Bearer ${auth.accessToken}`,
      },
    },
  );
  expect(resp.ok(), `GET /commodities/${id} → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as { data: { attributes: Record<string, unknown> } };
  return body.data.attributes;
}

test.describe("Item brand and model (#2804)", () => {
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

  async function create(
    request: APIRequestContext,
    params: Omit<Parameters<typeof createCommodityViaAPI>[3], "areaId">,
  ): Promise<string> {
    const { id } = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { ...params, areaId, shortName: params.shortName ?? "Item" },
      groupCurrency,
    );
    commodityIds.push(id);
    return id;
  }

  test("suggests a brand the group uses and shows brand · model on the card", async ({
    page,
    request,
  }) => {
    const brand = `Acme${run}`;
    const model = `AX-${run}`;
    await create(request, { name: `Brand source ${run}`, brand });
    const target = await create(request, { name: `Brand target ${run}` });

    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${target}/edit`);
    const brandInput = page.locator("#edit-brand");
    await expect(brandInput).toBeVisible();
    // Part of the brand, in another case: suggestions match anywhere and
    // ignore case.
    await brandInput.pressSequentially(`CME${run.toUpperCase()}`);
    const suggestions = page.getByTestId("brand-suggestions");
    await expect(suggestions.getByRole("option")).toHaveText([brand]);
    await brandInput.press("ArrowDown");
    await brandInput.press("Enter");
    await expect(brandInput).toHaveValue(brand);
    await expect(suggestions).toBeHidden();

    await page.locator("#edit-model").fill(model);
    await page.getByTestId("commodity-edit-save").click();
    await expect(page.getByTestId("commodity-edit-form")).toBeHidden();
    await expect(page.getByTestId("commodity-detail-brand-model")).toHaveText(
      `${brand} · ${model}`,
    );
    const attrs = await itemAttrs(request, auth, slug, target);
    expect(attrs.brand).toBe(brand);
    expect(attrs.model).toBe(model);

    await page.goto(
      `/g/${encodeURIComponent(slug)}/commodities?q=${encodeURIComponent(`Brand target ${run}`)}`,
    );
    const card = page.getByTestId("commodity-card");
    await expect(card).toHaveCount(1);
    await expect(card.getByTestId("commodity-card-subtitle")).toHaveText(`${brand} · ${model}`);
  });

  test("finds an item by its model and says the model matched", async ({ page, request }) => {
    const model = `MDL-${run}`.toUpperCase();
    await create(request, { name: `Router ${run}`, brand: `Netgear${run}`, model });

    // Lower case on purpose: matching ignores case.
    await page.goto(
      `/g/${encodeURIComponent(slug)}/commodities?q=${encodeURIComponent(model.toLowerCase())}`,
    );
    const card = page.getByTestId("commodity-card");
    await expect(card).toHaveCount(1);
    await expect(card).toContainText(`Router ${run}`);
    await expect(card.getByTestId("commodity-search-match")).toHaveText(`Model: ${model}`);
  });
});
