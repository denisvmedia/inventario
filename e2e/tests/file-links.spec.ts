/**
 * E2E coverage for one file attached to several items (#2806).
 *
 * One receipt ends up attached to three items through the two ways a user
 * has to reuse a file:
 *
 *   1. Uploaded on item A.
 *   2. Uploaded again on item B: the upload dialog recognizes the content
 *      and "Attach the existing file instead" swaps the copy for the file
 *      item A already has.
 *   3. Attached to item C with "Attach an existing file".
 *
 * Then the attachment to A is removed from the file panel, item B is
 * deleted, and item C still opens the same file, downloadable, with C as
 * its only attachment.
 */
import * as fs from "node:fs";
import * as path from "node:path";

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
import {
  assertSheetDownloadable,
  expectCommodityFilesCount,
  getCommodityFileIds,
} from "./includes/uploads.js";

interface FileLinkJSON {
  entity_type?: string;
  entity_id?: string;
}

async function linksOf(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  fileId: string,
): Promise<{ status: number; links: FileLinkJSON[] }> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}`,
    {
      headers: {
        Accept: "application/vnd.api+json",
        Authorization: `Bearer ${auth.accessToken}`,
      },
    },
  );
  if (!resp.ok()) return { status: resp.status(), links: [] };
  const body = await resp.json();
  return {
    status: resp.status(),
    links: (body?.attributes?.links ?? []) as FileLinkJSON[],
  };
}

test.describe("One file, several items (#2806)", () => {
  let auth: ApiAuth;
  let slug: string;
  let groupCurrency: string;
  let areaId: string;
  const commodityIds: string[] = [];

  test.beforeEach(async ({ page, request }) => {
    auth = await extractApiAuth(page);
    const group = await resolveActiveGroup(request, auth);
    slug = group.slug;
    groupCurrency = group.groupCurrency;
    ({ areaId } = await ensureLocationAndArea(request, auth, slug));
  });

  test.afterEach(async ({ request }) => {
    for (const id of commodityIds.splice(0)) {
      await deleteCommodityViaAPI(request, auth, slug, id).catch(() => {});
    }
  });

  test("a receipt attached to three items survives unlinking one and deleting another", async ({
    page,
    request,
    recorder,
  }) => {
    const run = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const items = [];
    for (const label of ["A", "B", "C"]) {
      const created = await createCommodityViaAPI(
        request,
        auth,
        slug,
        { name: `Shared receipt item ${label} ${run}`, areaId },
        groupCurrency,
      );
      commodityIds.push(created.id);
      items.push(created);
    }
    const [itemA, itemB, itemC] = items;

    // Content unique to this run, so the duplicate check can only match the
    // file uploaded in step 1.
    const receiptBytes = Buffer.concat([
      fs.readFileSync(path.join("fixtures", "files", "invoice.pdf")),
      Buffer.from(`\n% e2e ${run}\n`),
    ]);
    const receiptTitle = `shared-receipt-${run}`;

    const openFilesTab = async (p: Page, id: string) => {
      await p.goto(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
      );
      await p.getByTestId("commodity-detail-tab-files").click();
      await expect(p.getByTestId("commodity-detail-files")).toBeVisible();
    };
    const uploadReceipt = async (p: Page) => {
      await p.getByTestId("commodity-files-upload-zone").click();
      await expect(p.getByTestId("files-upload-dialog")).toBeVisible();
      await p.getByTestId("files-upload-input").setInputFiles({
        name: `${receiptTitle}.pdf`,
        mimeType: "application/pdf",
        buffer: receiptBytes,
      });
      await p.getByTestId("files-upload-next").click();
      await expect(p.getByTestId("files-upload-metadata-list")).toBeVisible();
      await p.getByTestId("files-upload-start").click();
      await expect(p.getByTestId("files-upload-close")).toBeEnabled({
        timeout: 30_000,
      });
      await expect(
        p.locator(
          '[data-testid^="files-upload-progress-item-"][data-status="failed"]',
        ),
      ).toHaveCount(0);
    };
    const onlyFileId = async (p: Page) => {
      await expectCommodityFilesCount(p, 1);
      const ids = await getCommodityFileIds(p);
      expect(ids).toHaveLength(1);
      return ids[0];
    };
    // The tab may be in grid or list view (the choice is remembered).
    const openFile = async (p: Page, id: string) => {
      const card = p.getByTestId(`file-card-open-${id}`);
      await (
        (await card.count()) ? card : p.getByTestId(`file-row-${id}`)
      ).click();
    };

    let step = 1;
    recorder.log(`Step ${step++}: upload the receipt on item A`);
    await openFilesTab(page, itemA.id);
    await uploadReceipt(page);
    await expect(page.getByTestId("files-upload-duplicate")).toHaveCount(0);
    await page.getByTestId("files-upload-close").click();
    const receiptId = await onlyFileId(page);
    expect(receiptId).not.toBe("");

    recorder.log(
      `Step ${step++}: upload the same receipt on item B, keep the existing file`,
    );
    await openFilesTab(page, itemB.id);
    await uploadReceipt(page);
    const notice = page.getByTestId("files-upload-duplicate");
    await expect(notice).toBeVisible();
    await expect(notice).toContainText(receiptTitle);
    await recorder.takeScreenshot("file-links-duplicate-offer");
    await page.getByTestId("files-upload-duplicate-use-existing").click();
    await expect(page.getByTestId("files-upload-duplicate-done")).toBeVisible();
    await page.getByTestId("files-upload-close").click();
    expect(await onlyFileId(page)).toBe(receiptId);

    recorder.log(`Step ${step++}: attach the existing receipt to item C`);
    await openFilesTab(page, itemC.id);
    await page.getByTestId("commodity-files-attach-existing").click();
    const dialog = page.getByTestId("attach-existing-dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByTestId("attach-existing-search").fill(receiptTitle);
    await dialog.getByTestId(`attach-existing-attach-${receiptId}`).click();
    await expect(
      dialog.getByTestId(`attach-existing-row-${receiptId}`),
    ).toContainText("Attached");
    await dialog.getByTestId("attach-existing-done").click();
    expect(await onlyFileId(page)).toBe(receiptId);

    recorder.log(`Step ${step++}: the file panel lists all three items`);
    await openFile(page, receiptId);
    const sheet = page.getByTestId("file-detail-sheet");
    await expect(sheet).toBeVisible();
    const links = sheet.getByTestId("file-links");
    await expect(links.locator("li")).toHaveCount(3);
    for (const item of items) {
      await expect(
        links.getByTestId(`file-link-commodity-${item.id}`),
      ).toContainText(item.name);
    }
    await recorder.takeScreenshot("file-links-linked-to");

    recorder.log(`Step ${step++}: remove the attachment to item A`);
    await sheet.getByTestId(`file-link-remove-commodity-${itemA.id}`).click();
    await expect(links.locator("li")).toHaveCount(2);
    await expect(
      links.getByTestId(`file-link-commodity-${itemA.id}`),
    ).toHaveCount(0);

    recorder.log(`Step ${step++}: delete item B`);
    await deleteCommodityViaAPI(request, auth, slug, itemB.id);
    commodityIds.splice(commodityIds.indexOf(itemB.id), 1);

    recorder.log(`Step ${step++}: item C still opens the same file`);
    await openFilesTab(page, itemC.id);
    expect(await onlyFileId(page)).toBe(receiptId);
    await openFile(page, receiptId);
    await expect(sheet).toBeVisible();
    await expect(sheet.getByTestId("file-links").locator("li")).toHaveCount(1);
    await expect(
      sheet.getByTestId(`file-link-commodity-${itemC.id}`),
    ).toBeVisible();
    await assertSheetDownloadable(page, recorder, receiptId);

    const after = await linksOf(request, auth, slug, receiptId);
    expect(after.status).toBe(200);
    expect(after.links.map((l) => `${l.entity_type}/${l.entity_id}`)).toEqual([
      `commodity/${itemC.id}`,
    ]);

    recorder.log(`Step ${step++}: item A no longer shows the file`);
    await openFilesTab(page, itemA.id);
    await expectCommodityFilesCount(page, 0);
  });
});
