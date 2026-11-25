/**
 * E2E coverage for documents from other systems (#2809).
 *
 * A script plays Paperless-ngx: with an API token limited to the group it
 * sends an invoice to POST /g/{slug}/intake as a multipart form, the way the
 * Paperless-ngx webhook does with "Include document" on. The document becomes
 * a draft in the inbox that says where it came from and links to it there.
 * Sending the same document again makes nothing, and sending it with other
 * values is refused. The item finished from the draft keeps the source, and
 * the document's state follows it.
 *
 * The script uses a fresh request context, so nothing from the browser
 * session can make a call succeed that the token alone would not. Other
 * specs share the group, so the row is matched by its draft id and the
 * document id is unique to the run.
 */
import fs from "node:fs";
import path from "node:path";

import {
  expect,
  request as playwrightRequest,
  type APIRequestContext,
} from "@playwright/test";

import { test } from "../fixtures/app-fixture.js";
import { TEST_CREDENTIALS } from "./includes/auth.js";
import {
  deleteCommodityViaAPI,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
} from "./includes/commodities-api.js";
import { BASE_URL } from "../setup/urls.js";

function sessionHeaders(auth: ApiAuth): Record<string, string> {
  return {
    "Content-Type": "application/json",
    Accept: "application/json",
    Authorization: `Bearer ${auth.accessToken}`,
    "X-CSRF-Token": auth.csrfToken,
  };
}

interface DeliveredDocument {
  id: string;
  attributes: {
    draft_id: string | null;
    commodity_id: string | null;
    state: string;
    external_document_id: string;
  };
}

test.describe("Documents from other systems (#2809)", () => {
  let auth: ApiAuth;
  let slug: string;
  let groupCurrency: string;
  let tokenId = "";
  let secret = "";
  let draftId = "";
  let itemId = "";
  let script: APIRequestContext | null = null;

  test.beforeEach(async ({ page, request }) => {
    auth = await extractApiAuth(page);
    const group = await resolveActiveGroup(request, auth);
    slug = group.slug;
    groupCurrency = group.groupCurrency;
    const minted = await request.post("/api/v1/users/me/api-tokens", {
      headers: sessionHeaders(auth),
      data: {
        name: `e2e intake ${Date.now()}`,
        scope: "read_write",
        group_id: group.id,
        current_password: TEST_CREDENTIALS.password,
      },
    });
    expect(
      minted.status(),
      `POST /users/me/api-tokens → ${minted.status()}`,
    ).toBe(201);
    const body = (await minted.json()) as {
      token: { id: string };
      secret: string;
    };
    tokenId = body.token.id;
    secret = body.secret;
    script = await playwrightRequest.newContext({ baseURL: BASE_URL });
  });

  test.afterEach(async ({ request }) => {
    await script?.dispose();
    script = null;
    if (itemId)
      await deleteCommodityViaAPI(request, auth, slug, itemId).catch(() => {});
    if (draftId) {
      await request
        .delete(
          `/api/v1/g/${encodeURIComponent(slug)}/drafts/${encodeURIComponent(draftId)}`,
          {
            headers: sessionHeaders(auth),
          },
        )
        .catch(() => {});
    }
    if (tokenId) {
      await request
        .delete(`/api/v1/users/me/api-tokens/${encodeURIComponent(tokenId)}`, {
          headers: sessionHeaders(auth),
        })
        .catch(() => {});
    }
    draftId = "";
    itemId = "";
    tokenId = "";
  });

  test("a delivered invoice becomes a draft that names its source", async ({
    page,
    request,
  }) => {
    const run = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
    const title = `Delivered invoice ${run}`;
    const sourceURL = `https://paperless.example.com/documents/${run}/`;
    const url = `/api/v1/g/${encodeURIComponent(slug)}/intake`;
    const invoice = fs.readFileSync(
      path.join("fixtures", "files", "invoice.pdf"),
    );
    const fields = {
      title,
      document_id: run,
      source_system: "paperless-ngx",
      source_name: "Paperless-ngx",
      source_url: sourceURL,
      seller: "Hardware Store",
      purchase_date: "2026-11-20",
      // Priced in the group currency, so the item can be finished without a
      // converted price.
      amount: `${groupCurrency}129.90`,
    };
    const deliver = (extra: Record<string, string> = {}) =>
      script!.post(url, {
        headers: { Authorization: `Bearer ${secret}` },
        multipart: {
          ...fields,
          ...extra,
          file: {
            name: "invoice.pdf",
            mimeType: "application/pdf",
            buffer: invoice,
          },
        },
      });

    // The browser session may not deliver: the endpoint takes a token only.
    const bySession = await request.post(url, {
      headers: sessionHeaders(auth),
      data: { title, document_id: run, source_system: "paperless-ngx" },
    });
    expect(bySession.status()).toBe(403);

    const first = await deliver();
    expect(first.status(), await first.text()).toBe(201);
    const doc = ((await first.json()) as { data: DeliveredDocument }).data;
    expect(doc.attributes.state).toBe("draft");
    expect(doc.attributes.external_document_id).toBe(run);
    draftId = doc.attributes.draft_id ?? "";
    expect(draftId).not.toBe("");

    // The same document again makes nothing and answers what became of it.
    const replay = await deliver();
    expect(replay.status()).toBe(200);
    const again = ((await replay.json()) as { data: DeliveredDocument }).data;
    expect(again.id).toBe(doc.id);
    expect(again.attributes.draft_id).toBe(draftId);

    // The same document with another value is refused and changes nothing.
    const changed = await deliver({ amount: `${groupCurrency}99.00` });
    expect(changed.status()).toBe(409);
    expect(await changed.text()).toContain("intake.document_changed");

    // The draft waits in the inbox and says where it came from.
    await page.goto(`/g/${encodeURIComponent(slug)}/inbox?reason=draft`);
    const row = page.getByTestId(`inbox-row-draft-${draftId}`);
    await expect(row).toContainText(title, { timeout: 15000 });
    await expect(row).toContainText("1 file");
    const source = row.getByTestId(`inbox-source-${draftId}`);
    await expect(source).toContainText("From Paperless-ngx");
    const link = source.getByRole("link");
    await expect(link).toHaveAttribute("href", sourceURL);
    await expect(link).toHaveAttribute("target", "_blank");

    // Continuing the draft shows the values the other system sent. Finishing
    // it needs what any item needs: a short name and a type.
    await row.getByTestId(`inbox-resume-${draftId}`).click();
    await expect(page.locator("#commodity-name")).toHaveValue(title);
    await page.locator("#commodity-short-name").fill("Invoice");
    await page.locator("#commodity-type").click();
    await page
      .getByRole("listbox")
      .getByRole("option", { name: /Equipment/i })
      .click();
    const files = page.getByTestId("commodity-form-files-step");
    for (let i = 0; i < 5 && !(await files.isVisible()); i++) {
      await page.getByTestId("commodity-form-next").click();
    }
    await expect(files).toBeVisible();
    await page.getByTestId("commodity-form-submit").click();
    await page.waitForURL(
      (u) =>
        /\/commodities\/[^/]+$/.test(u.pathname) &&
        !u.pathname.endsWith("/new"),
    );
    itemId = new URL(page.url()).pathname.split("/").pop() ?? "";
    draftId = "";

    // The item names its source on the Details tab.
    const sourceRow = page.getByTestId("commodity-detail-source");
    await expect(sourceRow).toContainText("Paperless-ngx", { timeout: 15000 });
    await expect(sourceRow.getByRole("link")).toHaveAttribute(
      "href",
      sourceURL,
    );

    // Sending the document now answers that it became this item.
    const afterPromotion = await deliver();
    expect(afterPromotion.status()).toBe(200);
    const promoted = (
      (await afterPromotion.json()) as { data: DeliveredDocument }
    ).data;
    expect(promoted.attributes.state).toBe("item");
    expect(promoted.attributes.commodity_id).toBe(itemId);
  });
});
