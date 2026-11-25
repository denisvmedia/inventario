/**
 * E2E coverage for the group's custom fields (#2803).
 *
 *   1. The Custom fields page creates a field, the item edit form fills it
 *      in, the item page shows the value, and renaming the field keeps it.
 *   2. Deleting a field that an item has a value for asks to confirm that
 *      the values go too, then deletes the field and the value.
 *
 * Field and item names carry a suffix unique to the run: field names are
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

interface FieldRow {
  id: string;
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

function fieldsUrl(slug: string, rest = ""): string {
  return `/api/v1/g/${encodeURIComponent(slug)}/custom-fields${rest}`;
}

function itemUrl(slug: string, id: string): string {
  return `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`;
}

async function listFields(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
): Promise<FieldRow[]> {
  const resp = await request.get(fieldsUrl(slug), { headers: jsonHeaders(auth) });
  expect(resp.ok(), `GET /custom-fields → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as {
    data: Array<{ id: string; attributes: { name?: string } }>;
  };
  return body.data.map((row) => ({ id: row.id, name: row.attributes.name ?? "" }));
}

async function createField(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  name: string,
): Promise<FieldRow> {
  const resp = await request.post(fieldsUrl(slug), {
    headers: jsonHeaders(auth),
    data: { data: { type: "custom_fields", attributes: { name, type: "text" } } },
  });
  expect(resp.status(), `POST /custom-fields → ${resp.status()}`).toBe(201);
  const body = (await resp.json()) as { data: { id: string } };
  return { id: body.data.id, name };
}

async function itemValues(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  id: string,
): Promise<Record<string, unknown>> {
  const resp = await request.get(itemUrl(slug, id), { headers: jsonHeaders(auth) });
  expect(resp.ok(), `GET /commodities/${id} → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as {
    data: { attributes: { custom_fields?: Record<string, unknown> } };
  };
  return body.data.attributes.custom_fields ?? {};
}

test.describe("Custom fields (#2803)", () => {
  let auth: ApiAuth;
  let slug: string;
  let groupCurrency: string;
  let areaId: string;
  let run: string;
  const commodityIds: string[] = [];
  const fieldNames: string[] = [];

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
    const names = new Set(fieldNames.splice(0));
    const fields = await listFields(request, auth, slug).catch(() => []);
    for (const field of fields.filter((f) => names.has(f.name))) {
      await request
        .delete(fieldsUrl(slug, `/${encodeURIComponent(field.id)}?delete_values=true`), {
          headers: jsonHeaders(auth),
        })
        .catch(() => {});
    }
  });

  async function createItem(request: APIRequestContext, name: string): Promise<string> {
    const { id } = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { name, shortName: "Item", areaId },
      groupCurrency,
    );
    commodityIds.push(id);
    return id;
  }

  test("a field created on the page is filled in on an item and renamed", async ({
    page,
    request,
  }) => {
    const name = `Plate ${run}`;
    const renamed = `Registration ${run}`;
    fieldNames.push(name, renamed);
    const item = await createItem(request, `Van ${run}`);

    await page.goto(`/g/${encodeURIComponent(slug)}/custom-fields`);
    await expect(page.getByTestId("page-custom-fields")).toBeVisible();
    await page.getByTestId("custom-fields-create-button").click();
    const dialog = page.getByTestId("custom-field-form-dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByTestId("custom-field-name-input").fill(name);
    await dialog.getByTestId("custom-field-form-submit").click();
    await expect(dialog).toBeHidden();
    const created = (await listFields(request, auth, slug)).find((f) => f.name === name);
    expect(created, `field ${name} exists`).toBeTruthy();
    const row = page.getByTestId(`custom-field-row-${created!.id}`);
    await expect(row).toContainText(name);
    await expect(row.getByTestId(`custom-field-row-${created!.id}-usage`)).toHaveText(
      "No values yet",
    );

    // The item edit form adds the field and saves its value.
    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${item}/edit`);
    await expect(page.getByTestId("commodity-edit-form")).toBeVisible();
    await page.getByTestId("custom-field-add").click();
    await page.getByTestId(`custom-field-add-${created!.id}`).click();
    await page.getByTestId(`custom-field-${created!.id}-input`).fill("AB 123");
    await page.getByTestId("commodity-edit-save").click();
    await expect(page.getByTestId("commodity-edit-form")).toBeHidden();
    expect(await itemValues(request, auth, slug, item)).toEqual({ [created!.id]: "AB 123" });
    const detailRow = page.getByTestId(`commodity-detail-custom-field-${created!.id}`);
    await expect(detailRow).toContainText(name);
    await expect(detailRow).toContainText("AB 123");

    // Renaming keeps the value.
    await page.goto(
      `/g/${encodeURIComponent(slug)}/custom-fields?q=${encodeURIComponent(name)}`,
    );
    await row.hover();
    await row.getByTestId(`custom-field-row-${created!.id}-edit`).click();
    await expect(dialog).toBeVisible();
    await dialog.getByTestId("custom-field-name-input").fill(renamed);
    await dialog.getByTestId("custom-field-form-submit").click();
    await expect(dialog).toBeHidden();

    await page.goto(`/g/${encodeURIComponent(slug)}/commodities/${item}`);
    await expect(detailRow).toContainText(renamed);
    await expect(detailRow).toContainText("AB 123");
  });

  test("deleting a field in use deletes its values once confirmed", async ({
    page,
    request,
  }) => {
    const field = await createField(request, auth, slug, `Contract ${run}`);
    fieldNames.push(field.name);
    const item = await createItem(request, `Boiler ${run}`);
    // An item update takes the whole record, as the item form sends it.
    const current = await request.get(itemUrl(slug, item), { headers: jsonHeaders(auth) });
    expect(current.ok(), `GET /commodities/${item} → ${current.status()}`).toBe(true);
    const { attributes } = ((await current.json()) as {
      data: { attributes: Record<string, unknown> };
    }).data;
    const put = await request.put(itemUrl(slug, item), {
      headers: jsonHeaders(auth),
      data: {
        data: {
          id: item,
          type: "commodities",
          attributes: { ...attributes, custom_fields: { [field.id]: "C-42" } },
        },
      },
    });
    expect(put.ok(), `PUT /commodities/${item} → ${put.status()}`).toBe(true);

    await page.goto(
      `/g/${encodeURIComponent(slug)}/custom-fields?q=${encodeURIComponent(field.name)}`,
    );
    const row = page.getByTestId(`custom-field-row-${field.id}`);
    await expect(row.getByTestId(`custom-field-row-${field.id}-usage`)).toHaveText("1 item");
    await row.hover();
    await row.getByTestId(`custom-field-row-${field.id}-delete`).click();

    const dialog = page.getByTestId("custom-field-delete-dialog");
    await expect(dialog).toContainText("1 item has a value for this field.");
    const confirm = dialog.getByTestId("custom-field-delete-confirm");
    await expect(confirm).toBeDisabled();
    await dialog.getByTestId("custom-field-delete-values").click();
    await confirm.click();
    await expect(dialog).toBeHidden();
    await expect(row).toBeHidden();

    expect(await itemValues(request, auth, slug, item)).toEqual({});
    const left = await listFields(request, auth, slug);
    expect(left.find((f) => f.id === field.id)).toBeUndefined();
  });
});
