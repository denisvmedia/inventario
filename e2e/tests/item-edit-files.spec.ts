/**
 * E2E coverage for files added while editing an item (#2916).
 *
 *   1. A file added in the All fields dialog is attached to the item before
 *      the dialog closes. When its attach fails, the dialog stays open and
 *      says so; Retry attaches it and saves the changes.
 *   2. Cancel after a failed attach moves the upload to Trash, so no file is
 *      left in the group without an item.
 *   3. A file still being sent when browser Back takes the member off the
 *      item is not attached; once its upload ends it goes to Trash.
 *
 * Names carry a suffix unique to the run, so rows from parallel specs in
 * the same group do not match.
 */
import { expect, type APIRequestContext, type Page } from "@playwright/test";

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

async function itemFileTitles(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  itemId: string,
): Promise<string[]> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/files?linked_entity_type=commodity&linked_entity_id=${encodeURIComponent(itemId)}`,
    { headers: headers(auth) },
  );
  expect(resp.ok(), `GET /files → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as { data: { title: string }[] };
  return body.data.map((f) => f.title);
}

interface TrashedFileRow {
  id: string;
  attributes: { title: string };
}

async function trashedFiles(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
): Promise<TrashedFileRow[]> {
  const resp = await request.get(`/api/v1/g/${encodeURIComponent(slug)}/trash/files?per_page=100`, {
    headers: headers(auth),
  });
  expect(resp.ok(), `GET /trash/files → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as { data: TrashedFileRow[] };
  return body.data;
}

// failAttaches answers POST /files/{id}/links with 500, the first time only
// when once is set.
async function failAttaches(page: Page, slug: string, once: boolean) {
  let failed = false;
  await page.route(`**/api/v1/g/${slug}/files/*/links`, async (route) => {
    if (route.request().method() !== "POST" || (once && failed)) return route.continue();
    failed = true;
    await route.fulfill({
      status: 500,
      contentType: "application/vnd.api+json",
      body: JSON.stringify({ errors: [{ status: "500", title: "Internal Server Error" }] }),
    });
  });
}

// addFileInAllFields opens the item's All fields dialog on the Files step
// and stages image.jpg with the given title.
async function addFileInAllFields(page: Page, slug: string, itemId: string, title: string) {
  await page.goto(
    `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(itemId)}/edit`,
  );
  await page.getByTestId("commodity-edit-all-fields").click();
  const form = page.getByTestId("commodity-form-dialog");
  await form
    .getByRole("list", { name: /form steps/i })
    .getByRole("button", { name: "Files" })
    .click();
  await form
    .locator('[data-testid="commodity-files-dropzone"] input[type="file"]')
    .setInputFiles(["fixtures/files/image.jpg"]);
  await form.getByLabel("Title of image.jpg").fill(title);
  return form;
}

test.describe("Files added while editing an item (#2916)", () => {
  let auth: ApiAuth;
  let slug: string;
  let groupCurrency: string;
  let areaId: string;
  let run: string;
  const itemIds: string[] = [];

  test.beforeEach(async ({ page, request }) => {
    auth = await extractApiAuth(page);
    const group = await resolveActiveGroup(request, auth);
    slug = group.slug;
    groupCurrency = group.groupCurrency;
    ({ areaId } = await ensureLocationAndArea(request, auth, slug));
    run = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  });

  test.afterEach(async ({ request }) => {
    for (const id of itemIds.splice(0)) {
      await deleteCommodityViaAPI(request, auth, slug, id).catch(() => {});
    }
  });

  async function create(request: APIRequestContext, name: string): Promise<string> {
    const { id } = await createCommodityViaAPI(
      request,
      auth,
      slug,
      { name, shortName: "Kettle", areaId },
      groupCurrency,
    );
    itemIds.push(id);
    return id;
  }

  test("attaches the file before the dialog closes, with Retry after a failed attach", async ({
    page,
    request,
  }) => {
    const id = await create(request, `Kettle ${run}`);
    await failAttaches(page, slug, true);
    const form = await addFileInAllFields(page, slug, id, `Front view ${run}`);
    await form.getByTestId("commodity-form-submit").click();

    const alert = form.getByTestId("draft-save-error");
    await expect(alert).toContainText("Changes not saved");
    await expect(alert).toContainText("image.jpg");
    await expect(
      form.locator('[data-testid="commodity-files-pending"][data-status="failed"]'),
    ).toContainText("image.jpg");
    expect(await itemFileTitles(request, auth, slug, id)).toEqual([]);

    await alert.getByTestId("draft-save-retry").click();
    await expect(form).toBeHidden();
    expect(await itemFileTitles(request, auth, slug, id)).toEqual([`Front view ${run}`]);
  });

  test("moves the upload to Trash when the dialog is cancelled after a failed attach", async ({
    page,
    request,
  }) => {
    const id = await create(request, `Kettle ${run}`);
    await failAttaches(page, slug, false);
    const form = await addFileInAllFields(page, slug, id, `Front view ${run}`);
    await form.getByTestId("commodity-form-submit").click();
    await expect(form.getByTestId("draft-save-error")).toContainText("image.jpg");

    await form.getByTestId("commodity-form-cancel").click();
    await expect(form).toBeHidden();

    const ours = async () =>
      (await trashedFiles(request, auth, slug)).filter(
        (f) => f.attributes.title === `Front view ${run}`,
      );
    await expect.poll(async () => (await ours()).length).toBe(1);
    expect(await itemFileTitles(request, auth, slug, id)).toEqual([]);

    for (const f of await ours()) {
      await request
        .delete(`/api/v1/g/${encodeURIComponent(slug)}/trash/files/${encodeURIComponent(f.id)}`, {
          headers: headers(auth),
        })
        .catch(() => {});
    }
  });

  test("moves a file still being sent when Back leaves the item to Trash", async ({
    page,
    request,
  }) => {
    const id = await create(request, `Kettle ${run}`);
    // The upload answers once the member has left the dialog.
    let release = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    let uploading = false;
    await page.route(`**/api/v1/g/${slug}/uploads/file`, async (route) => {
      if (route.request().method() !== "POST") return route.continue();
      uploading = true;
      await held;
      await route.continue();
    });
    const uploaded = page.waitForResponse(
      (r) => r.url().includes(`/g/${slug}/uploads/file`) && r.request().method() === "POST",
    );

    // The item opens from the Items list inside the app, so Back stays in it.
    await page.goto(`/g/${encodeURIComponent(slug)}/commodities?q=${run}`);
    await page
      .getByTestId("commodity-card")
      .filter({ hasText: `Kettle ${run}` })
      .getByTestId("commodity-card-link")
      .click();
    await expect(page.getByTestId("commodity-detail-sheet")).toBeVisible();
    await page.getByTestId("commodity-detail-edit").click();
    await page.waitForURL((url) => url.pathname.endsWith("/edit"));
    await page.getByTestId("commodity-edit-all-fields").click();
    const form = page.getByTestId("commodity-form-dialog");
    await form
      .getByRole("list", { name: /form steps/i })
      .getByRole("button", { name: "Files" })
      .click();
    await form
      .locator('[data-testid="commodity-files-dropzone"] input[type="file"]')
      .setInputFiles(["fixtures/files/image.jpg"]);
    await form.getByTestId("commodity-form-submit").click();
    await expect.poll(() => uploading).toBe(true);

    // Back to the item, then Back to the list: the item, with its dialog,
    // goes away.
    await page.goBack();
    await page.waitForURL((url) => !url.pathname.endsWith("/edit"));
    await page.goBack();
    await page.waitForURL((url) => url.pathname.endsWith("/commodities"));
    await expect(form).toBeHidden();
    release();

    const body = (await (await uploaded).json()) as { data?: { id: string }; id?: string };
    const fileId = body.data?.id ?? body.id;
    expect(fileId, "upload answer carries the file id").toBeTruthy();
    await expect
      .poll(async () => (await trashedFiles(request, auth, slug)).some((f) => f.id === fileId))
      .toBe(true);
    expect(await itemFileTitles(request, auth, slug, id)).toEqual([]);

    await request
      .delete(`/api/v1/g/${encodeURIComponent(slug)}/trash/files/${encodeURIComponent(fileId!)}`, {
        headers: headers(auth),
      })
      .catch(() => {});
  });
});
