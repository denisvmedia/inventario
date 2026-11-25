/**
 * E2E coverage for filtering by tag (#2792).
 *
 *   1. The Tags menu on the items list shows a count per tag, requires a
 *      tag on the first click, excludes it on the second, and offers
 *      "No tags"; every choice lands in the URL.
 *   2. A tag pill on an item card adds that tag to the list's filters.
 *   3. On the item page, tags, the area and the type link to the list
 *      filtered by them alone, inactive items included.
 *   4. A row on the Tags page links to the list it filters, inactive
 *      items included.
 *   5. The file browser's tag picker offers file tags, not item tags.
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

function headers(auth: ApiAuth): Record<string, string> {
  return {
    Accept: "application/vnd.api+json",
    "Content-Type": "application/vnd.api+json",
    Authorization: `Bearer ${auth.accessToken}`,
    "X-CSRF-Token": auth.csrfToken,
  };
}

// Best-effort cleanup of the tags of one kind whose label or slug holds
// text.
async function deleteTagsMatching(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  kind: "commodity" | "file",
  text: string,
): Promise<void> {
  const base = `/api/v1/g/${encodeURIComponent(slug)}/tags`;
  const resp = await request.get(
    `${base}?kind=${kind}&per_page=100&q=${encodeURIComponent(text)}`,
    { headers: headers(auth) },
  );
  if (!resp.ok()) return;
  const body = (await resp.json()) as { data?: Array<{ id: string }> };
  for (const tag of body.data ?? []) {
    await request
      .delete(`${base}/${encodeURIComponent(tag.id)}?force=true`, { headers: headers(auth) })
      .catch(() => {});
  }
}

test.describe("Filter items by tag (#2792)", () => {
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
    await deleteTagsMatching(request, auth, slug, "commodity", run);
    await deleteTagsMatching(request, auth, slug, "file", run);
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

  const list = (query: string) => `/g/${encodeURIComponent(slug)}/commodities?${query}`;

  test("the Tags menu counts, requires, excludes and picks untagged items", async ({
    page,
    request,
  }) => {
    const garage = `garage-${run}`;
    const fragile = `fragile-${run}`;
    await create(request, { name: `Drill ${run}`, tags: [`Garage ${run}`] });
    await create(request, { name: `Vase ${run}`, tags: [`Fragile ${run}`, `Garage ${run}`] });
    await create(request, { name: `Bucket ${run}` });

    // The search keeps the counts and the list to this run's items.
    await page.goto(list(`q=${run}`));
    const cards = page.getByTestId("commodity-card");
    await expect(cards).toHaveCount(3);

    await page.getByTestId("commodities-filter-tags").click();
    const menu = page.getByTestId("commodities-filter-tags-menu");
    await expect(menu.getByTestId(`tag-filter-count-${garage}`)).toHaveText("2");
    await expect(menu.getByTestId(`tag-filter-count-${fragile}`)).toHaveText("1");
    await expect(menu.getByTestId("commodities-filter-tags-untagged")).toContainText("1");

    const garageOption = menu.getByTestId(`commodities-filter-tags-option-${garage}`);
    await garageOption.click();
    await expect(page).toHaveURL(new RegExp(`[?&]tags=${garage}(&|$)`));
    await expect(cards).toHaveCount(2);

    await garageOption.click();
    await expect(page).toHaveURL(new RegExp(`[?&]tags_exclude=${garage}(&|$)`));
    await expect(page).not.toHaveURL(/[?&]tags=/);
    await expect(cards).toHaveCount(1);
    await expect(cards.first()).toContainText(`Bucket ${run}`);

    await garageOption.click();
    await menu.getByTestId("commodities-filter-tags-untagged").click();
    await expect(page).toHaveURL(/[?&]no_tags=1/);
    await expect(cards).toHaveCount(1);
    await expect(cards.first()).toContainText(`Bucket ${run}`);

    // The filter survives a reload.
    await page.reload();
    await expect(cards).toHaveCount(1);
    await expect(page.getByTestId("commodities-filter-tags")).toContainText("1");
  });

  test("a tag on a card adds it to the list's filters", async ({ page, request }) => {
    const label = `Camping ${run}`;
    await create(request, { name: `Tent ${run}`, tags: [label] });
    await create(request, { name: `Lamp ${run}` });

    await page.goto(list(`q=${run}`));
    const tent = page.getByTestId("commodity-card").filter({ hasText: `Tent ${run}` });
    await tent.getByRole("link", { name: label }).click();
    // The search stays.
    await expect(page).toHaveURL(new RegExp(`/commodities\\?q=${run}&tags=camping-${run}$`));
    await expect(page.getByTestId("commodity-card")).toHaveCount(1);
  });

  test("tags, area and type on the item page link to the filtered list", async ({
    page,
    request,
  }) => {
    const label = `Workshop ${run}`;
    // A sold item is inactive; the links still list it.
    const id = await create(request, {
      name: `Saw ${run}`,
      type: "equipment",
      status: "sold",
      tags: [label],
    });

    const detail = `/g/${encodeURIComponent(slug)}/commodities/${id}`;
    await page.goto(detail);
    await page.getByTestId("commodity-detail-details").getByRole("link", { name: label }).click();
    await expect(page).toHaveURL(new RegExp(`/commodities\\?inactive=1&tags=workshop-${run}$`));
    await expect(page.getByTestId("commodity-card")).toHaveCount(1);

    await page.goto(detail);
    await page.getByTestId("commodity-detail-area-link").click();
    await expect(page).toHaveURL(new RegExp(`/commodities\\?area=${areaId}&inactive=1$`));

    await page.goto(detail);
    await page.getByTestId("commodity-detail-type-link").click();
    await expect(page).toHaveURL(/\/commodities\?type=equipment&inactive=1$/);
  });

  test("a row on the Tags page links to the items with that tag", async ({ page, request }) => {
    const label = `Attic ${run}`;
    await create(request, { name: `Box ${run}`, tags: [label] });
    await create(request, { name: `Crate ${run}`, status: "sold", tags: [label] });

    await page.goto(`/g/${encodeURIComponent(slug)}/tags?kind=commodity&q=${run}`);
    await page.getByTestId(`tag-row-attic-${run}-link`).click();
    await expect(page).toHaveURL(new RegExp(`/commodities\\?inactive=1&tags=attic-${run}$`));
    await expect(page.getByTestId("commodity-card")).toHaveCount(2);
  });

  test("the file browser picks from file tags only", async ({ page, request }) => {
    await create(request, { name: `Kettle ${run}`, tags: [`Itemonly ${run}`] });
    const resp = await request.post(`/api/v1/g/${encodeURIComponent(slug)}/tags`, {
      headers: headers(auth),
      data: {
        data: {
          type: "tags",
          attributes: { kind: "file", slug: `receipt-${run}`, label: `Receipt ${run}`, color: "amber" },
        },
      },
    });
    expect(resp.ok(), `POST /tags → ${resp.status()}`).toBe(true);

    await page.goto(`/g/${encodeURIComponent(slug)}/files`);
    await page.getByTestId("files-tag-picker").click();
    const menu = page.getByTestId("files-tag-picker-menu");
    await menu.getByPlaceholder("Search tags…").fill(run);
    await expect(menu.getByTestId(`files-tag-picker-option-itemonly-${run}`)).toHaveCount(0);
    await menu.getByTestId(`files-tag-picker-option-receipt-${run}`).click();
    await expect(page).toHaveURL(new RegExp(`[?&]tags=receipt-${run}(&|$)`));
    await page.keyboard.press("Escape");
    await expect(page.getByTestId(`files-tag-pill-receipt-${run}`)).toHaveText(
      new RegExp(`Receipt ${run}`),
    );
  });
});
