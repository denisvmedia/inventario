/**
 * E2E coverage for saved views (#2793).
 *
 *   1. A filtered items list is saved as a view from the toolbar; the
 *      sidebar and the dashboard show it with the same item count as the
 *      opened list.
 *   2. An open view tells when the list no longer matches it, saves the
 *      change, and reverts to what it keeps.
 *   3. A view naming a tag that does not exist still opens and says so.
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

// Best-effort cleanup of the saved views whose name holds text.
async function deleteViewsMatching(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  text: string,
): Promise<void> {
  const base = `/api/v1/g/${encodeURIComponent(slug)}/saved-views`;
  const resp = await request.get(base, { headers: headers(auth) });
  if (!resp.ok()) return;
  const body = (await resp.json()) as {
    data?: Array<{ id: string; attributes: { name: string } }>;
  };
  for (const view of body.data ?? []) {
    if (!view.attributes.name.includes(text)) continue;
    await request
      .delete(`${base}/${encodeURIComponent(view.id)}`, { headers: headers(auth) })
      .catch(() => {});
  }
}

test.describe("Saved views (#2793)", () => {
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
    await deleteViewsMatching(request, auth, slug, run);
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

  const list = (query: string) => `/g/${encodeURIComponent(slug)}/commodities?${query}`;

  test("a saved view shows in the sidebar and on the dashboard with the list's count", async ({
    page,
    request,
  }) => {
    const name = `Tools ${run}`;
    await create(request, { name: `Drill ${run}`, type: "equipment" });
    await create(request, { name: `Saw ${run}`, type: "equipment" });
    await create(request, { name: `Lamp ${run}`, type: "other" });

    await page.goto(list(`q=${run}&type=equipment`));
    const cards = page.getByTestId("commodity-card");
    await expect(cards).toHaveCount(2);

    await page.getByTestId("saved-views-menu").click();
    await page.getByTestId("saved-views-save-new").click();
    const dialog = page.getByTestId("saved-view-dialog");
    await dialog.getByTestId("saved-view-name").fill(name);
    await dialog.getByTestId("saved-view-dashboard").click();
    await dialog.getByTestId("saved-view-submit").click();
    await expect(dialog).toBeHidden();
    await expect(page).toHaveURL(/[?&]saved_view=/);
    await expect(page.getByTestId("saved-views-menu-label")).toHaveText(name);
    await expect(page.getByTestId("saved-views-changed")).toHaveCount(0);

    const sidebarRow = page
      .getByTestId("sidebar-saved-views")
      .getByRole("listitem")
      .filter({ hasText: name });
    await expect(sidebarRow.getByTestId("sidebar-saved-view-count")).toHaveText("2");

    await page.goto(`/g/${encodeURIComponent(slug)}`);
    const card = page.getByTestId("dashboard-saved-view").filter({ hasText: name });
    await expect(card.getByTestId("dashboard-saved-view-count")).toHaveText("2 items");
    await expect(card.getByTestId("dashboard-saved-view-row")).toHaveCount(2);

    await page.getByTestId("sidebar-saved-views").getByRole("link", { name }).click();
    await expect(page).toHaveURL(new RegExp(`[?&]q=${run}(&|$)`));
    await expect(page).toHaveURL(/[?&]type=equipment(&|$)/);
    await expect(cards).toHaveCount(2);
  });

  test("an open view tells it changed, saves the change and reverts", async ({
    page,
    request,
  }) => {
    const name = `Kitchen ${run}`;
    await create(request, { name: `Kettle ${run}` });
    await create(request, { name: `Toaster ${run}`, status: "sold" });

    await page.goto(list(`q=${run}`));
    const cards = page.getByTestId("commodity-card");
    await expect(cards).toHaveCount(1);
    await page.getByTestId("saved-views-menu").click();
    await page.getByTestId("saved-views-save-new").click();
    const dialog = page.getByTestId("saved-view-dialog");
    await dialog.getByTestId("saved-view-name").fill(name);
    await dialog.getByTestId("saved-view-submit").click();
    await expect(page.getByTestId("saved-views-menu-label")).toHaveText(name);

    // Showing inactive items changes the list: the sold toaster appears.
    await page.getByTestId("commodities-toggle-inactive").click();
    await expect(cards).toHaveCount(2);
    await expect(page.getByTestId("saved-views-changed")).toBeVisible();
    await page.getByTestId("saved-views-menu").click();
    await page.getByTestId("saved-views-save-changes").click();
    await expect(page.getByTestId("saved-views-changed")).toHaveCount(0);

    // Opening the view again shows the saved change.
    await page.goto(`/g/${encodeURIComponent(slug)}/views`);
    const row = page.getByTestId("saved-view-row").filter({ hasText: name });
    await row.getByRole("link", { name }).click();
    await expect(cards).toHaveCount(2);

    // Revert puts back what the view keeps.
    await page.getByTestId("commodities-toggle-inactive").click();
    await expect(cards).toHaveCount(1);
    await page.getByTestId("saved-views-menu").click();
    await page.getByTestId("saved-views-revert").click();
    await expect(cards).toHaveCount(2);
    await expect(page.getByTestId("saved-views-changed")).toHaveCount(0);
  });

  test("a view naming a tag that is gone still opens and says so", async ({ page, request }) => {
    const name = `Gone ${run}`;
    await create(request, { name: `Box ${run}` });
    const resp = await request.post(`/api/v1/g/${encodeURIComponent(slug)}/saved-views`, {
      headers: headers(auth),
      data: {
        data: {
          type: "saved_views",
          attributes: {
            name,
            icon: "archive",
            query: `include_inactive=false&q=${run}&sort=name&tags=gone-${run}`,
          },
        },
      },
    });
    expect(resp.ok(), `POST /saved-views → ${resp.status()}`).toBe(true);

    await page.goto(`/g/${encodeURIComponent(slug)}/views`);
    const row = page.getByTestId("saved-view-row").filter({ hasText: name });
    await expect(row.locator('[data-stale="true"]')).toHaveText(`Tag gone-${run}`);
    await row.getByRole("link", { name }).click();

    await expect(page.getByTestId("commodities-empty-filtered")).toBeVisible();
    await page.getByTestId("saved-views-menu").click();
    await expect(page.getByTestId("saved-views-stale")).toContainText(`gone-${run}`);
  });
});
