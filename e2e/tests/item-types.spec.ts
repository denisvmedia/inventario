/**
 * E2E coverage for the group's item types (#2802).
 *
 *   1. The Item types page creates a type, the item edit form picks it, the
 *      item page names it, and renaming the type renames it on the item.
 *   2. Deleting a type that an item has moves the item to the type picked
 *      in the dialog, then deletes the type.
 *
 * Type and item names carry a suffix unique to the run: type names are
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

interface ItemTypeRow {
  id: string;
  slug: string;
  name: string;
}

function jsonHeaders(auth: ApiAuth): Record<string, string> {
  return {
    "Content-Type": "application/vnd.api+json",
    Accept: "application/vnd.api+json",
    Authorization: `Bearer ${auth.accessToken}`,
    "X-CSRF-Token": auth.csrfToken,
  };
}

function typesUrl(slug: string, rest = ""): string {
  return `/api/v1/g/${encodeURIComponent(slug)}/item-types${rest}`;
}

async function listTypes(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
): Promise<ItemTypeRow[]> {
  const resp = await request.get(typesUrl(slug), { headers: jsonHeaders(auth) });
  expect(resp.ok(), `GET /item-types → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as {
    data: Array<{ id: string; attributes: { slug: string; name?: string } }>;
  };
  return body.data.map((row) => ({
    id: row.id,
    slug: row.attributes.slug,
    name: row.attributes.name ?? "",
  }));
}

async function createType(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  name: string,
): Promise<ItemTypeRow> {
  const resp = await request.post(typesUrl(slug), {
    headers: jsonHeaders(auth),
    data: { data: { type: "item_types", attributes: { name, icon: "package" } } },
  });
  expect(resp.status(), `POST /item-types → ${resp.status()}`).toBe(201);
  const body = (await resp.json()) as { data: { id: string; attributes: { slug: string } } };
  return { id: body.data.id, slug: body.data.attributes.slug, name };
}

async function itemType(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<string> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
    { headers: jsonHeaders(auth) },
  );
  expect(resp.ok(), `GET /commodities/${id} → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as { data: { attributes: { type?: string } } };
  return body.data.attributes.type ?? "";
}

test.describe("Item types (#2802)", () => {
  let auth: ApiAuth;
  let slug: string;
  let groupCurrency: string;
  let areaId: string;
  let run: string;
  const commodityIds: string[] = [];
  // The names of the types a test created; a rename replaces the entry.
  const typeNames: string[] = [];

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
    const names = new Set(typeNames.splice(0));
    const types = await listTypes(request, auth, slug).catch(() => []);
    // Items in Trash still have their type, so the delete moves them.
    const fallback = types.find((t) => t.slug === "other");
    for (const type of types.filter((t) => names.has(t.name))) {
      const target = fallback ? `?reassign_to=${encodeURIComponent(fallback.id)}` : "";
      await request
        .delete(typesUrl(slug, `/${encodeURIComponent(type.id)}${target}`), {
          headers: jsonHeaders(auth),
        })
        .catch(() => {});
    }
  });

  async function createItem(
    request: APIRequestContext,
    name: string,
    type?: string,
  ): Promise<string> {
    const { id } = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { name, shortName: "Item", areaId, type },
      groupCurrency,
    );
    commodityIds.push(id);
    return id;
  }

  test("a type created on the page is picked on an item and renamed", async ({
    page,
    request,
  }) => {
    const name = `Boat ${run}`;
    const renamed = `Sailboat ${run}`;
    typeNames.push(name, renamed);
    const item = await createItem(request, `Dinghy ${run}`);

    await page.goto(`/g/${encodeURIComponent(slug)}/item-types`);
    await expect(page.getByTestId("page-item-types")).toBeVisible();
    await page.getByTestId("item-types-create-button").click();
    const dialog = page.getByTestId("item-type-form-dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByTestId("item-type-name-input").fill(name);
    await dialog.getByTestId("item-type-icon-sailboat").click();
    await dialog.getByTestId("item-type-form-submit").click();
    await expect(dialog).toBeHidden();
    const created = (await listTypes(request, auth, slug)).find((t) => t.name === name);
    expect(created, `type ${name} exists`).toBeTruthy();
    const row = page.getByTestId(`item-type-row-${created!.slug}`);
    await expect(row).toContainText(name);
    await expect(row.getByTestId(`item-type-row-${created!.slug}-usage`)).toHaveText(
      "No items yet",
    );

    // The item edit form lists the new type.
    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${item}/edit`);
    await expect(page.getByTestId("commodity-edit-form")).toBeVisible();
    await page.locator("#edit-category").click();
    await page.getByRole("option", { name }).click();
    await page.getByTestId("commodity-edit-save").click();
    await expect(page.getByTestId("commodity-edit-form")).toBeHidden();
    expect(await itemType(request, auth, slug, item)).toBe(created!.slug);
    await expect(page.getByTestId("commodity-detail-type-link")).toHaveText(name);

    // Renaming keeps the slug, so the item shows the new name.
    await page.goto(
      `/g/${encodeURIComponent(slug)}/item-types?q=${encodeURIComponent(name)}`,
    );
    await row.hover();
    await row.getByTestId(`item-type-row-${created!.slug}-edit`).click();
    await expect(dialog).toBeVisible();
    await dialog.getByTestId("item-type-name-input").fill(renamed);
    await dialog.getByTestId("item-type-form-submit").click();
    await expect(dialog).toBeHidden();

    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${item}`);
    await expect(page.getByTestId("commodity-detail-type-link")).toHaveText(renamed);
    expect(await itemType(request, auth, slug, item)).toBe(created!.slug);
  });

  test("deleting a type moves its items to the type picked", async ({ page, request }) => {
    const from = await createType(request, auth, slug, `Kayak ${run}`);
    const to = await createType(request, auth, slug, `Canoe ${run}`);
    typeNames.push(from.name, to.name);
    const item = await createItem(request, `Paddle ${run}`, from.slug);

    await page.goto(
      `/g/${encodeURIComponent(slug)}/item-types?q=${encodeURIComponent(from.name)}`,
    );
    const row = page.getByTestId(`item-type-row-${from.slug}`);
    await expect(row.getByTestId(`item-type-row-${from.slug}-usage`)).toHaveText("1 item");
    await row.hover();
    await row.getByTestId(`item-type-row-${from.slug}-delete`).click();

    const dialog = page.getByTestId("item-type-delete-dialog");
    await expect(dialog).toContainText(`1 item has the type ${from.name}.`);
    const confirm = dialog.getByTestId("item-type-delete-confirm");
    await expect(confirm).toBeDisabled();
    await dialog.getByTestId("item-type-delete-target").click();
    await page.getByRole("option", { name: to.name }).click();
    await confirm.click();
    await expect(dialog).toBeHidden();
    await expect(row).toBeHidden();

    expect(await itemType(request, auth, slug, item)).toBe(to.slug);
    const left = await listTypes(request, auth, slug);
    expect(left.some((t) => t.id === from.id)).toBe(false);
  });
});
