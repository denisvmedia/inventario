/**
 * E2E coverage for item search (#2800).
 *
 *   1. The items list finds an item by its serial number and part number,
 *      and the card says which field matched.
 *   2. Several words must all match, each in any field: a word from the
 *      notes plus a word from a tag label.
 *   3. The search page shows the matched field, and its "see all" link
 *      opens the list with the same items, inactive ones included.
 *   4. A tag filter in the list URL keeps only items with that tag.
 *   5. A filter the server does not support shows an explanation with a
 *      way back, not an empty or unfiltered list.
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
    Authorization: `Bearer ${auth.accessToken}`,
  };
}

// Best-effort cleanup of the item tags this spec created through the
// items it saved.
async function deleteTagsMatching(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  text: string,
): Promise<void> {
  const base = `/api/v1/g/${encodeURIComponent(slug)}/tags`;
  const resp = await request.get(
    `${base}?kind=commodity&per_page=100&q=${encodeURIComponent(text)}`,
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

test.describe("Item search (#2800)", () => {
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
    await deleteTagsMatching(request, auth, slug, run);
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

  test("finds an item by serial or part number and says which field matched", async ({
    page,
    request,
  }) => {
    const serial = `SN-${run}`.toUpperCase();
    const part = `PN-${run}-7`;
    await create(request, { name: `Hedge trimmer ${run}`, serialNumber: serial, partNumbers: [part] });

    // Lower case on purpose: matching ignores case.
    await page.goto(list(`q=${encodeURIComponent(serial.toLowerCase())}`));
    const card = page.getByTestId("commodity-card");
    await expect(card).toHaveCount(1);
    await expect(card.getByTestId("commodity-search-match")).toHaveText(
      `Serial number: ${serial}`,
    );

    await page.goto(list(`q=${encodeURIComponent(part)}`));
    await expect(card).toHaveCount(1);
    await expect(card.getByTestId("commodity-search-match")).toHaveText(`Part number: ${part}`);
  });

  test("every word must match, each in any field", async ({ page, request }) => {
    const tagLabel = `Garden ${run}`;
    await create(request, {
      name: `Leaf blower ${run}`,
      comments: `Use the orange extension cord ${run}`,
      tags: [tagLabel],
    });
    await create(request, { name: `Garden hose ${run}`, comments: `Blue, 20 m ${run}` });

    // "orange" is only in the first item's notes; the tag label holds
    // "Garden" plus the run id, so the second item, whose name holds
    // "Garden", does not match both words.
    await page.goto(list(`q=${encodeURIComponent(`orange "${tagLabel}"`)}`));
    const cards = page.getByTestId("commodity-card");
    await expect(cards).toHaveCount(1);
    await expect(cards.first()).toContainText(`Leaf blower ${run}`);

    // The tag alone finds the item by its label, and the hint shows the
    // label rather than the slug.
    await page.goto(list(`q=${encodeURIComponent(`"${tagLabel}"`)}`));
    await expect(cards).toHaveCount(1);
    await expect(cards.first().getByTestId("commodity-search-match")).toHaveText(
      `Tag: ${tagLabel}`,
    );
  });

  test("the search page shows the matched field and links to the same items", async ({
    page,
    request,
  }) => {
    const serial = `XS-${run}`.toUpperCase();
    const activeId = await create(request, { name: `Router ${run}`, serialNumber: serial });
    const soldId = await create(request, {
      name: `Old router ${run}`,
      extraSerialNumbers: [`${serial}-B`],
      status: "sold",
    });

    await page.goto(`/g/${encodeURIComponent(slug)}/search?q=${encodeURIComponent(serial)}`);
    await expect(page.getByTestId(`result-commodity-${activeId}`)).toContainText(
      `Serial number: ${serial}`,
    );
    await expect(page.getByTestId(`result-commodity-${soldId}`)).toContainText(
      `Serial number: ${serial}-B`,
    );

    // The search counts the sold item, so the list it opens shows it too.
    await page.getByTestId("group-commodities-see-all").click();
    await expect(page).toHaveURL(/[?&]inactive=1/);
    await expect(page.getByTestId("commodity-card")).toHaveCount(2);
  });

  test("a tag filter in the URL keeps only items with that tag", async ({ page, request }) => {
    const tagLabel = `Fragile ${run}`;
    const tagSlug = `fragile-${run}`;
    await create(request, { name: `Vase ${run}`, tags: [tagLabel] });
    await create(request, { name: `Bucket ${run}` });

    await page.goto(list(`q=${run}&tags=${encodeURIComponent(tagSlug)}`));
    const cards = page.getByTestId("commodity-card");
    await expect(cards).toHaveCount(1);
    await expect(cards.first()).toContainText(`Vase ${run}`);

    await page.goto(list(`q=${run}&tags_exclude=${encodeURIComponent(tagSlug)}`));
    await expect(cards).toHaveCount(1);
    await expect(cards.first()).toContainText(`Bucket ${run}`);
  });

  test("an unsupported filter is explained, with a way back to the list", async ({
    page,
    request,
  }) => {
    await create(request, { name: `Kettle ${run}` });

    await page.goto(list(`status=broken`));
    const alert = page.getByTestId("commodities-invalid-query");
    await expect(alert).toBeVisible();
    await expect(alert).toContainText("status");

    await alert.getByRole("button", { name: "Clear filters" }).click();
    await expect(alert).toBeHidden();
    await expect(page).not.toHaveURL(/status=/);
    await expect(page.getByTestId("commodity-card").first()).toBeVisible();
  });
});
