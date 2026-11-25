/**
 * E2E coverage for the inbox (#2794).
 *
 * A server draft and an item marked as draft both need attention. The
 * dashboard, the sidebar and the Items page lead to the inbox, which lists
 * each with its reason. Marking the item as reviewed and discarding the
 * draft clear the two rows. The items to review can be walked one after
 * another from their own pages, a draft continues on the step it still
 * needs, and the Missing details card leads to the items list filters.
 *
 * Other specs share the group and may add their own drafts at the same
 * time, so rows are matched by id and counts are compared, never asserted
 * exactly. Names carry a suffix unique to the run.
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
    "Content-Type": "application/vnd.api+json",
    Accept: "application/vnd.api+json",
    Authorization: `Bearer ${auth.accessToken}`,
    "X-CSRF-Token": auth.csrfToken,
  };
}

async function createDraft(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  key: string,
  name: string,
): Promise<string> {
  const resp = await request.post(`/api/v1/g/${encodeURIComponent(slug)}/drafts`, {
    headers: headers(auth),
    data: { data: { type: "commodity_drafts", attributes: { client_key: key, values: { name } } } },
  });
  expect(resp.status(), `POST /drafts → ${resp.status()}`).toBe(201);
  return ((await resp.json()) as { data: { id: string } }).data.id;
}

// draftListed reports whether the group's open drafts include id. A
// promoted or discarded draft is not listed.
async function draftListed(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<boolean> {
  const resp = await request.get(`/api/v1/g/${encodeURIComponent(slug)}/drafts?per_page=100`, {
    headers: headers(auth),
  });
  expect(resp.ok(), `GET /drafts → ${resp.status()}`).toBe(true);
  return ((await resp.json()) as { data: { id: string }[] }).data.some((d) => d.id === id);
}

async function itemIsDraft(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<boolean> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
    { headers: headers(auth) },
  );
  expect(resp.ok(), `GET /commodities/${id} → ${resp.status()}`).toBe(true);
  return ((await resp.json()) as { data: { attributes: { draft: boolean } } }).data.attributes
    .draft;
}

// createUnpricedItem adds a live item with an unknown purchase price and
// date (#2813), which the Missing details lists count.
async function createUnpricedItem(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  name: string,
  areaId: string,
  currency: string,
): Promise<string> {
  const resp = await request.post(`/api/v1/g/${encodeURIComponent(slug)}/commodities`, {
    headers: headers(auth),
    data: {
      data: {
        type: "commodities",
        attributes: {
          name,
          short_name: name.slice(-20),
          type: "other",
          status: "in_use",
          count: 1,
          area_id: areaId,
          original_price_currency: currency,
          draft: false,
        },
      },
    },
  });
  expect(resp.status(), `POST /commodities → ${resp.status()}`).toBe(201);
  return ((await resp.json()) as { data: { id: string } }).data.id;
}

async function inboxTotal(request: APIRequestContext, auth: ApiAuth, slug: string) {
  const resp = await request.get(`/api/v1/g/${encodeURIComponent(slug)}/inbox/count`, {
    headers: headers(auth),
  });
  expect(resp.ok(), `GET /inbox/count → ${resp.status()}`).toBe(true);
  return ((await resp.json()) as { data: { total: number } }).data.total;
}

test.describe("Inbox (#2794)", () => {
  let auth: ApiAuth;
  let slug: string;
  let groupCurrency: string;
  let run: string;
  let draftId = "";
  let itemId = "";
  let extraIds: string[] = [];

  test.beforeEach(async ({ page, request }) => {
    auth = await extractApiAuth(page);
    ({ slug, groupCurrency } = await resolveActiveGroup(request, auth));
    run = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  });

  test.afterEach(async ({ request }) => {
    if (itemId) await deleteCommodityViaAPI(request, auth, slug, itemId).catch(() => {});
    for (const id of extraIds) {
      await deleteCommodityViaAPI(request, auth, slug, id).catch(() => {});
    }
    extraIds = [];
    if (draftId) {
      await request
        .delete(`/api/v1/g/${encodeURIComponent(slug)}/drafts/${encodeURIComponent(draftId)}`, {
          headers: headers(auth),
        })
        .catch(() => {});
    }
    itemId = "";
    draftId = "";
  });

  test("collects a draft and an item marked as draft, and clears both", async ({
    page,
    request,
  }) => {
    const draftName = `Inbox drill ${run}`;
    const itemName = `Inbox lamp ${run}`;
    const { areaId } = await ensureLocationAndArea(request, auth, slug);
    draftId = await createDraft(request, auth, slug, `e2e-inbox-${run}`, draftName);
    // Priced in the group currency, so the item passes the checks a
    // reviewed item has to pass.
    ({ id: itemId } = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { name: itemName, areaId, draft: true },
      groupCurrency,
    ));
    const before = await inboxTotal(request, auth, slug);
    expect(before).toBeGreaterThanOrEqual(2);

    // The dashboard and the sidebar show the count and lead to the inbox.
    await page.goto(`/g/${encodeURIComponent(slug)}`);
    await expect(page.getByTestId("dashboard-needs-attention")).toContainText("Needs attention", {
      timeout: 15000,
    });
    const badge = page.getByTestId("sidebar-inbox-count");
    await expect(badge).toBeVisible();
    expect(Number(await badge.textContent())).toBeGreaterThanOrEqual(2);

    // The Items page leads there too.
    await page.goto(`/g/${encodeURIComponent(slug)}/commodities`);
    await page.getByTestId("items-inbox-notice-open").click();
    await page.waitForURL((url) => url.pathname.endsWith("/inbox"));

    const draftRow = page.getByTestId(`inbox-row-draft-${draftId}`);
    const itemRow = page.getByTestId(`inbox-row-commodity-${itemId}`);
    // Land on All, whatever tab the notice picked.
    await page.getByTestId("inbox-tab-all").click();
    await expect(draftRow).toContainText(draftName);
    await expect(draftRow.getByTestId("inbox-reason")).toHaveText("Draft");
    await expect(itemRow).toContainText(itemName);
    await expect(itemRow.getByTestId("inbox-reason")).toHaveText("Marked as draft");

    // The reason tabs filter on the server.
    await page.getByTestId("inbox-tab-item_draft").click();
    await expect(itemRow).toBeVisible();
    await expect(draftRow).toBeHidden();

    // Marking the item as reviewed clears its row.
    await itemRow.getByTestId(`inbox-review-${itemId}`).click();
    await expect(itemRow).toBeHidden();
    await expect.poll(() => itemIsDraft(request, auth, slug, itemId)).toBe(false);

    // Discarding the draft clears the other.
    await page.getByTestId("inbox-tab-all").click();
    await draftRow.getByTestId(`inbox-discard-${draftId}`).click();
    await page.getByTestId("confirm-accept").click();
    await expect(draftRow).toBeHidden();
    await expect.poll(() => draftListed(request, auth, slug, draftId)).toBe(false);
    draftId = "";
  });

  test("continues a draft over the inbox and finishes it into an item", async ({
    page,
    request,
  }) => {
    const name = `Inbox kettle ${run}`;
    draftId = await createDraft(request, auth, slug, `e2e-inbox-${run}`, name);

    await page.goto(`/g/${encodeURIComponent(slug)}/inbox?reason=draft`);
    const row = page.getByTestId(`inbox-row-draft-${draftId}`);
    await row.getByTestId(`inbox-resume-${draftId}`).click();
    await expect(page.locator("#commodity-name")).toHaveValue(name);
    // The inbox stays behind the dialog.
    await expect(page.getByTestId("page-inbox")).toBeAttached();

    // Finishing needs what any item needs: a short name and a type. The
    // draft was saved with a name only.
    await page.locator("#commodity-short-name").fill("Kettle");
    await page.locator("#commodity-type").click();
    await page.getByRole("listbox").getByRole("option", { name: /Equipment/i }).click();
    const files = page.getByTestId("commodity-form-files-step");
    for (let i = 0; i < 5 && !(await files.isVisible()); i++) {
      await page.getByTestId("commodity-form-next").click();
    }
    await expect(files).toBeVisible();
    await page.getByTestId("commodity-form-submit").click();
    await page.waitForURL(
      (url) => /\/commodities\/[^/]+$/.test(url.pathname) && !url.pathname.endsWith("/new"),
    );
    itemId = new URL(page.url()).pathname.split("/").pop() ?? "";
    expect(await draftListed(request, auth, slug, draftId)).toBe(false);
    draftId = "";
  });

  test("walks the items to review from their pages", async ({ page, request }) => {
    const { areaId } = await ensureLocationAndArea(request, auth, slug);
    const first = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { name: `Review bike ${run}`, areaId, draft: true },
      groupCurrency,
    );
    const second = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { name: `Review lamp ${run}`, areaId, draft: true },
      groupCurrency,
    );
    extraIds = [first.id, second.id];

    // Review one by one opens the first item to review over the inbox. With
    // at least the two above in the list, it has a next one to skip to.
    await page.goto(`/g/${encodeURIComponent(slug)}/inbox`);
    await page.getByTestId("inbox-review-walk").click();
    const bar = page.getByTestId("commodity-review-bar");
    await expect(bar).toBeVisible({ timeout: 15000 });
    await expect(page.getByTestId("commodity-review-position")).toContainText("Item 1 of");
    const opened = new URL(page.url()).pathname;
    await page.getByTestId("commodity-review-skip").click();
    await expect.poll(() => new URL(page.url()).pathname).not.toBe(opened);
    await expect(page.getByTestId("commodity-review-position")).toContainText("Item 2 of");

    // Mark as reviewed clears the mark and moves on, to the next item or
    // back to the inbox after the last one.
    await page.goto(`/g/${encodeURIComponent(slug)}/inbox?reason=item_draft`);
    await page.getByTestId(`inbox-open-${first.id}`).click();
    await expect(bar).toBeVisible({ timeout: 15000 });
    await page.getByTestId("commodity-review-mark").click();
    await expect.poll(() => itemIsDraft(request, auth, slug, first.id)).toBe(false);
    await expect.poll(() => page.url()).not.toContain(`/commodities/${first.id}`);
  });

  test("continues a draft on the step it still needs", async ({ page, request }) => {
    const resp = await request.post(`/api/v1/g/${encodeURIComponent(slug)}/drafts`, {
      headers: headers(auth),
      data: {
        data: {
          type: "commodity_drafts",
          attributes: {
            client_key: `e2e-inbox-step-${run}`,
            values: { name: `Inbox chair ${run}`, short_name: "Chair", type: "other" },
          },
        },
      },
    });
    expect(resp.status(), `POST /drafts → ${resp.status()}`).toBe(201);
    draftId = ((await resp.json()) as { data: { id: string } }).data.id;

    await page.goto(`/g/${encodeURIComponent(slug)}/inbox?reason=draft`);
    await page.getByTestId(`inbox-resume-${draftId}`).click();
    // Basics is complete, so the form opens on Purchase; Basics is one click back.
    await expect(page.getByTestId("commodity-form-purchase-step")).toBeVisible({
      timeout: 15000,
    });
    await page.getByRole("button", { name: "Basics", exact: true }).click();
    await expect(page.locator("#commodity-name")).toHaveValue(`Inbox chair ${run}`);
  });

  test("leads from the missing details to the items list filters", async ({ page, request }) => {
    const { areaId } = await ensureLocationAndArea(request, auth, slug);
    const name = `Unpriced stool ${run}`;
    itemId = await createUnpricedItem(request, auth, slug, name, areaId, groupCurrency);
    const priced = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { name: `Priced stool ${run}`, areaId, originalPrice: 25 },
      groupCurrency,
    );
    extraIds = [priced.id];

    await page.goto(`/g/${encodeURIComponent(slug)}/inbox`);
    const link = page.getByTestId("inbox-missing-no-purchase-price");
    await expect(page.getByTestId("inbox-missing-no-purchase-price-count")).toBeVisible({
      timeout: 15000,
    });
    expect(
      Number(await page.getByTestId("inbox-missing-no-purchase-price-count").textContent()),
    ).toBeGreaterThanOrEqual(1);
    await link.click();
    await page.waitForURL((url) => url.searchParams.get("no_purchase_price") === "1");

    // Narrowed to this run, the filter keeps the unpriced item only.
    const listed = (id: string) => page.locator(`[data-commodity-id="${id}"]`);
    await page.goto(
      `/g/${encodeURIComponent(slug)}/commodities?q=${encodeURIComponent(run)}&no_purchase_price=1`,
    );
    await expect(listed(itemId)).toBeVisible({ timeout: 15000 });
    await expect(listed(priced.id)).toHaveCount(0);

    // The toolbar's Missing menu sets the same filter.
    await page.goto(`/g/${encodeURIComponent(slug)}/commodities?q=${encodeURIComponent(run)}`);
    await expect(listed(priced.id)).toBeVisible({ timeout: 15000 });
    await page.getByTestId("commodities-filter-missing").click();
    await page.getByTestId("commodities-filter-missing-noPurchaseDate").click();
    await page.waitForURL((url) => url.searchParams.get("no_purchase_date") === "1");
    await expect(listed(priced.id)).toHaveCount(0, { timeout: 15000 });
    await expect(listed(itemId)).toBeVisible();
  });
});
