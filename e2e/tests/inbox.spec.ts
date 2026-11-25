/**
 * E2E coverage for the inbox (#2794).
 *
 * A server draft and an item marked as draft both need attention. The
 * dashboard, the sidebar and the Items page lead to the inbox, which lists
 * each with its reason. Marking the item as reviewed and discarding the
 * draft clear the two rows.
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

  test.beforeEach(async ({ page, request }) => {
    auth = await extractApiAuth(page);
    ({ slug, groupCurrency } = await resolveActiveGroup(request, auth));
    run = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  });

  test.afterEach(async ({ request }) => {
    if (itemId) await deleteCommodityViaAPI(request, auth, slug, itemId).catch(() => {});
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
});
