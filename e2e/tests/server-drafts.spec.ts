/**
 * E2E coverage for server drafts (#2812).
 *
 *   1. A draft saved with a photo on one device shows up in the inbox on
 *      another (a fresh browser context, so nothing is shared through
 *      local storage), opens with its values and the photo, and finishing
 *      it creates one item that carries the photo.
 *   2. A save whose response is lost is repeated by Retry and still makes
 *      one draft.
 *   3. Add item uploads the files to the item's draft before it creates the
 *      item (#1626): the item page shows them with the titles typed in the
 *      dialog.
 *   4. A failed upload adds no item; Retry uploads what is missing and
 *      then adds the item with every file.
 *   5. Discard after a failed Add item deletes the draft it made and moves
 *      the files it uploaded to Trash.
 *
 * Names carry a suffix unique to the run, so rows from parallel specs in
 * the same group do not match.
 */
import { expect, type APIRequestContext, type Page } from "@playwright/test";

import { test } from "../fixtures/app-fixture.js";
import { ensureAuthenticated } from "./includes/auth.js";
import {
  deleteCommodityViaAPI,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
} from "./includes/commodities-api.js";

interface DraftRow {
  id: string;
  title: string;
  file_count: number;
}

function headers(auth: ApiAuth): Record<string, string> {
  return {
    "Content-Type": "application/vnd.api+json",
    Accept: "application/vnd.api+json",
    Authorization: `Bearer ${auth.accessToken}`,
    "X-CSRF-Token": auth.csrfToken,
  };
}

async function listDrafts(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
): Promise<DraftRow[]> {
  const resp = await request.get(`/api/v1/g/${encodeURIComponent(slug)}/drafts?per_page=100`, {
    headers: headers(auth),
  });
  expect(resp.ok(), `GET /drafts → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as { data: DraftRow[] };
  return body.data;
}

async function itemFiles(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  itemId: string,
): Promise<{ title: string; path: string; ext: string }[]> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/files?linked_entity_type=commodity&linked_entity_id=${encodeURIComponent(itemId)}`,
    { headers: headers(auth) },
  );
  expect(resp.ok(), `GET /files → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as { data: { title: string; path: string; ext: string }[] };
  return body.data;
}

async function itemFileCount(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  itemId: string,
): Promise<number> {
  return (await itemFiles(request, auth, slug, itemId)).length;
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

// pickType picks the item type on Basics.
async function pickType(page: Page, name: RegExp) {
  await page.locator("#commodity-type").click();
  await page.getByRole("listbox").getByRole("option", { name }).click();
}

// itemIdFromUrl waits for the item page Add item lands on and returns the
// item's id.
async function itemIdFromUrl(page: Page): Promise<string> {
  await page.waitForURL(
    (url) => /\/commodities\/[^/]+$/.test(url.pathname) && !url.pathname.endsWith("/new"),
  );
  return new URL(page.url()).pathname.split("/").pop() ?? "";
}

// openNewItemForm opens the Add item dialog on its Basics step.
async function openNewItemForm(page: Page, slug: string) {
  await page.goto(`/g/${encodeURIComponent(slug)}/commodities`);
  await page.getByTestId("commodities-add-button").click();
  await page.getByTestId("commodity-form-ai-fill-manually").click();
  await expect(page.locator("#commodity-name")).toBeVisible();
}

// walkToFiles presses Continue until the Files step shows.
async function walkToFiles(page: Page) {
  const files = page.getByTestId("commodity-form-files-step");
  for (let i = 0; i < 5 && !(await files.isVisible()); i++) {
    await page.getByTestId("commodity-form-next").click();
  }
  await expect(files).toBeVisible();
}

test.describe("Server drafts (#2812)", () => {
  let auth: ApiAuth;
  let slug: string;
  let run: string;
  const itemIds: string[] = [];

  test.beforeEach(async ({ page, request }) => {
    auth = await extractApiAuth(page);
    ({ slug } = await resolveActiveGroup(request, auth));
    run = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  });

  test.afterEach(async ({ request }) => {
    for (const id of itemIds.splice(0)) {
      await deleteCommodityViaAPI(request, auth, slug, id).catch(() => {});
    }
    for (const d of await listDrafts(request, auth, slug).catch(() => [])) {
      if (!d.title.includes(run)) continue;
      await request
        .delete(`/api/v1/g/${encodeURIComponent(slug)}/drafts/${encodeURIComponent(d.id)}`, {
          headers: headers(auth),
        })
        .catch(() => {});
    }
  });

  test("a draft saved on one device is finished on another with its photo", async ({
    page,
    request,
    browser,
  }) => {
    const name = `Cordless drill ${run}`;

    // Device one: a name and a photo, nothing else.
    await openNewItemForm(page, slug);
    await page.locator("#commodity-name").fill(name);
    await expect(page.locator("#commodity-short-name")).toHaveValue("Cordless drill");
    await page.locator("#commodity-draft").click();
    await walkToFiles(page);
    await page
      .locator('[data-testid="commodity-files-dropzone"] input[type="file"]')
      .setInputFiles("fixtures/files/image.jpg");
    await page.getByTestId("commodity-form-save-draft").click();
    await expect(page.getByTestId("commodity-form-files-step")).toBeHidden();

    const saved = (await listDrafts(request, auth, slug)).filter((d) => d.title === name);
    expect(saved).toHaveLength(1);
    expect(saved[0].file_count).toBe(1);
    const draftId = saved[0].id;

    // Device two: a fresh browser context signs in on its own.
    const other = await browser.newContext();
    try {
      const page2 = await other.newPage();
      await page2.goto("/");
      await ensureAuthenticated(page2);
      await page2.goto(`/g/${encodeURIComponent(slug)}/inbox?reason=draft`);
      const row = page2.getByTestId(`inbox-row-draft-${draftId}`);
      await expect(row).toContainText(name);
      await expect(row).toContainText("1 file");
      await row.getByTestId(`inbox-resume-${draftId}`).click();

      await expect(page2.locator("#commodity-name")).toHaveValue(name);
      await expect(page2.locator("#commodity-short-name")).toHaveValue("Cordless drill");
      // Finishing needs what any item needs: a type.
      await page2.locator("#commodity-type").click();
      await page2.getByRole("listbox").getByRole("option", { name: /Equipment/i }).click();
      await walkToFiles(page2);
      await expect(page2.getByTestId("draft-saved-files")).toContainText("image");
      await page2.getByTestId("commodity-form-submit").click();

      // The item page replaces /commodities/new?draft=… once the item exists.
      const itemId = await itemIdFromUrl(page2);
      itemIds.push(itemId);
      expect(await itemFileCount(request, auth, slug, itemId)).toBe(1);
      expect((await listDrafts(request, auth, slug)).some((d) => d.id === draftId)).toBe(false);
    } finally {
      await other.close();
    }
  });

  test("a save whose response is lost is retried into one draft", async ({ page, request }) => {
    const name = `Kettle ${run}`;
    let dropped = false;
    // The first create reaches the server, but its answer never reaches
    // the browser.
    await page.route(`**/api/v1/g/${slug}/drafts`, async (route) => {
      if (route.request().method() !== "POST" || dropped) return route.continue();
      dropped = true;
      await route.fetch();
      await route.abort("failed");
    });

    await openNewItemForm(page, slug);
    await page.locator("#commodity-name").fill(name);
    await page.locator("#commodity-draft").click();
    await page.getByTestId("commodity-form-save-draft").click();
    await expect(page.getByTestId("draft-save-error")).toBeVisible();
    await expect(page.locator("#commodity-name")).toHaveValue(name);

    await page.getByTestId("draft-save-retry").click();
    await expect(page.locator("#commodity-name")).toBeHidden();
    expect(dropped).toBe(true);
    const drafts = (await listDrafts(request, auth, slug)).filter((d) => d.title === name);
    expect(drafts).toHaveLength(1);
  });

  test("a new item gets its files, with their titles, through its draft", async ({
    page,
    request,
  }) => {
    const name = `Espresso machine ${run}`;
    await openNewItemForm(page, slug);
    await page.locator("#commodity-name").fill(name);
    await pickType(page, /Equipment/i);
    await walkToFiles(page);
    await page
      .locator('[data-testid="commodity-files-dropzone"] input[type="file"]')
      .setInputFiles(["fixtures/files/image.jpg", "fixtures/files/manual.pdf"]);
    const rows = page.getByTestId("commodity-files-pending");
    await expect(rows).toHaveCount(2);
    const title = page.getByLabel("Title of image.jpg");
    await expect(title).toHaveValue("image");
    await title.fill(`Front view ${run}`);
    await page.getByTestId("commodity-form-submit").click();

    const itemId = await itemIdFromUrl(page);
    itemIds.push(itemId);
    // The files were on the item before the dialog let go of it.
    const files = await itemFiles(request, auth, slug, itemId);
    expect(files.map((f) => f.title).sort()).toEqual([`Front view ${run}`, "manual"].sort());
    expect((await listDrafts(request, auth, slug)).some((d) => d.title === name)).toBe(false);

    await page.getByTestId("commodity-detail-tab-files").click();
    await expect(page.getByTestId("commodity-detail-files")).toContainText(`Front view ${run}`);
  });

  test("a failed upload adds nothing, and Retry adds the item with every file", async ({
    page,
    request,
  }) => {
    const name = `Toaster ${run}`;
    let failed = false;
    // The first upload fails; the ones after it go through.
    await page.route(`**/api/v1/g/${slug}/uploads/file`, async (route) => {
      if (route.request().method() !== "POST" || failed) return route.continue();
      failed = true;
      await route.fulfill({
        status: 500,
        contentType: "application/vnd.api+json",
        body: JSON.stringify({ errors: [{ status: "500", title: "Internal Server Error" }] }),
      });
    });

    await openNewItemForm(page, slug);
    await page.locator("#commodity-name").fill(name);
    await pickType(page, /Equipment/i);
    await walkToFiles(page);
    await page
      .locator('[data-testid="commodity-files-dropzone"] input[type="file"]')
      .setInputFiles(["fixtures/files/image.jpg", "fixtures/files/invoice.pdf"]);
    await page.getByTestId("commodity-form-submit").click();

    const alert = page.getByTestId("draft-save-error");
    await expect(alert).toContainText("Item not added");
    await expect(alert).toContainText("image.jpg");
    const failedRow = page.locator('[data-testid="commodity-files-pending"][data-status="failed"]');
    await expect(failedRow).toContainText("image.jpg");
    await expect(page.getByTestId("draft-saved-files")).toContainText("invoice");
    expect(page.url()).toContain("/commodities");
    expect(failed).toBe(true);
    // The item is not there yet: its values and the uploaded file wait on a
    // draft.
    const waiting = (await listDrafts(request, auth, slug)).filter((d) => d.title === name);
    expect(waiting).toHaveLength(1);
    expect(waiting[0].file_count).toBe(1);

    await page.getByTestId("draft-save-retry").click();
    const itemId = await itemIdFromUrl(page);
    itemIds.push(itemId);
    expect(await itemFileCount(request, auth, slug, itemId)).toBe(2);
    expect((await listDrafts(request, auth, slug)).some((d) => d.title === name)).toBe(false);
  });

  test("Discard after a failed Add item moves its uploads to Trash", async ({ page, request }) => {
    const name = `Kettle ${run}`;
    let failed = false;
    // The first attach fails: image.jpg is uploaded but on nothing, while
    // invoice.pdf reaches the draft Add item made.
    await page.route(`**/api/v1/g/${slug}/files/*/links`, async (route) => {
      if (route.request().method() !== "POST" || failed) return route.continue();
      failed = true;
      await route.fulfill({
        status: 500,
        contentType: "application/vnd.api+json",
        body: JSON.stringify({ errors: [{ status: "500", title: "Internal Server Error" }] }),
      });
    });

    await openNewItemForm(page, slug);
    await page.locator("#commodity-name").fill(name);
    await pickType(page, /Equipment/i);
    await walkToFiles(page);
    await page
      .locator('[data-testid="commodity-files-dropzone"] input[type="file"]')
      .setInputFiles(["fixtures/files/image.jpg", "fixtures/files/invoice.pdf"]);
    await page.getByLabel("Title of image.jpg").fill(`Front view ${run}`);
    await page.getByLabel("Title of invoice.pdf").fill(`Invoice ${run}`);
    await page.getByTestId("commodity-form-submit").click();
    await expect(page.getByTestId("draft-save-error")).toContainText("image.jpg");
    expect(failed).toBe(true);
    expect((await listDrafts(request, auth, slug)).filter((d) => d.title === name)).toHaveLength(1);

    await page.getByTestId("commodity-form-cancel").click();
    await page.getByTestId("commodity-form-close-confirm-discard").click();
    await expect(page.getByTestId("commodity-form-files-step")).toBeHidden();

    const ours = async () =>
      (await trashedFiles(request, auth, slug)).filter((f) => f.attributes.title.includes(run));
    await expect
      .poll(async () => (await ours()).map((f) => f.attributes.title).sort())
      .toEqual([`Front view ${run}`, `Invoice ${run}`].sort());
    expect((await listDrafts(request, auth, slug)).some((d) => d.title === name)).toBe(false);

    for (const f of await ours()) {
      await request
        .delete(`/api/v1/g/${encodeURIComponent(slug)}/trash/files/${encodeURIComponent(f.id)}`, {
          headers: headers(auth),
        })
        .catch(() => {});
    }
  });
});
