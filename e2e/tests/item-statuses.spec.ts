/**
 * E2E coverage for the Donated and Stolen statuses (#1607).
 *
 *   1. An item in use is marked donated from its page with a date and a
 *      note; the page shows the status card, and the items list keeps the
 *      item out of the default view and finds it by the status filter.
 *   2. A donated and a stolen item are found together by both statuses,
 *      each with its own status pill.
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
  type CreateCommodityParams,
} from "./includes/commodities-api.js";

test.describe("Item statuses (#1607)", () => {
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
    params: Omit<CreateCommodityParams, "areaId">,
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

  test("marks an item donated and finds it by the status filter", async ({ page, request }) => {
    const id = await create(request, { name: `Bike ${run}` });

    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`);
    await expect(page.getByTestId("page-commodity-detail")).toBeVisible();
    await page.getByTestId("commodity-detail-transition-donated").click();

    const dialog = page.getByTestId("status-transition-dialog");
    await expect(dialog.getByRole("heading", { name: "Mark as Donated" })).toBeVisible();
    await expect(dialog.getByTestId("status-transition-sale-price")).toHaveCount(0);
    await dialog.getByTestId("status-transition-date").fill("2026-11-26");
    await dialog.getByTestId("status-transition-note").fill(`School fair ${run}`);
    await dialog.getByTestId("status-transition-confirm").click();
    await expect(dialog).toBeHidden();

    const card = page.getByTestId("commodity-detail-terminal-status");
    await expect(card.getByTestId("commodity-detail-terminal-status-label")).toHaveText("Donated");
    await expect(card.getByTestId("commodity-detail-terminal-status-note")).toHaveText(
      `School fair ${run}`,
    );
    await expect(page.getByTestId("commodity-detail-change-status")).toHaveCount(0);

    const cards = page.getByTestId("commodity-card");
    await page.goto(list(`q=${run}`));
    await expect(page.getByTestId("commodities-empty-filtered")).toBeVisible();
    await expect(cards).toHaveCount(0);

    await page.goto(list(`q=${run}&status=donated`));
    await expect(cards).toHaveCount(1);
    await expect(cards.first()).toContainText(`Bike ${run}`);
    await expect(cards.first()).toContainText("Donated");
  });

  // #2915: the step-by-step dialog shows the status without changing it, and
  // a save from it keeps the date, note and sale price Change Status set.
  test("keeps the sale details when a sold item is saved from All fields", async ({
    page,
    request,
  }) => {
    const id = await create(request, { name: `Bike ${run}` });

    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`);
    await expect(page.getByTestId("page-commodity-detail")).toBeVisible();
    await page.getByTestId("commodity-detail-transition-sold").click();
    const transition = page.getByTestId("status-transition-dialog");
    await transition.getByTestId("status-transition-date").fill("2026-11-20");
    await transition.getByTestId("status-transition-sale-price").fill("150");
    await transition.getByTestId("status-transition-note").fill(`Sold to Bob ${run}`);
    await transition.getByTestId("status-transition-confirm").click();
    await expect(transition).toBeHidden();

    const card = page.getByTestId("commodity-detail-terminal-status");
    await expect(card.getByTestId("commodity-detail-terminal-status-label")).toHaveText("Sold");

    await page.getByTestId("commodity-detail-edit").click();
    await page.getByTestId("commodity-edit-all-fields").click();
    const form = page.getByTestId("commodity-form-dialog");
    const status = form.getByLabel("Status", { exact: true });
    await expect(status).toHaveText("Sold");
    await expect(status).toBeDisabled();
    const name = form.getByLabel("Name", { exact: true });
    await name.fill(`Road bike ${run}`);
    await form.getByTestId("commodity-form-submit").click();
    await expect(form).toBeHidden();

    await expect(page.getByRole("heading", { name: `Road bike ${run}` })).toBeVisible();
    await expect(card.getByTestId("commodity-detail-terminal-status-label")).toHaveText("Sold");
    await expect(card.getByTestId("commodity-detail-terminal-status-date")).toContainText("2026");
    await expect(card.getByTestId("commodity-detail-terminal-status-note")).toHaveText(
      `Sold to Bob ${run}`,
    );
    await expect(card.getByTestId("commodity-detail-terminal-status-sale-price")).toContainText(
      "150",
    );
  });

  test("finds donated and stolen items together, each with its own pill", async ({
    page,
    request,
  }) => {
    await create(request, { name: `Lamp ${run}`, status: "donated" });
    await create(request, { name: `Phone ${run}`, status: "stolen" });
    await create(request, { name: `Kettle ${run}`, status: "lost" });

    await page.goto(list(`q=${run}&status=donated&status=stolen`));
    const cards = page.getByTestId("commodity-card");
    await expect(cards).toHaveCount(2);
    await expect(cards.nth(0)).toContainText(`Lamp ${run}`);
    await expect(cards.nth(0)).toContainText("Donated");
    await expect(cards.nth(1)).toContainText(`Phone ${run}`);
    await expect(cards.nth(1)).toContainText("Stolen");

    await page.getByTestId("commodities-filter-status").click();
    await expect(page.getByRole("menuitemcheckbox", { name: "Donated" })).toBeChecked();
    await expect(page.getByRole("menuitemcheckbox", { name: "Stolen" })).toBeChecked();
    await expect(page.getByRole("menuitemcheckbox", { name: "Lost" })).not.toBeChecked();
  });
});
