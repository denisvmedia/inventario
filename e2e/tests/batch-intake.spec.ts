/**
 * E2E coverage for batch intake (#2810).
 *
 * One receipt with three products goes through the real stack: the e2e
 * backend runs the deterministic mock AI provider, which reads a file named
 * like a receipt as a purchase of an espresso machine, a milk frother and a
 * line of two espresso cups.
 *
 *   1. The member drops the receipt, leaves the frother out and creates the
 *      drafts. The answer to that request is lost on the way back, as on a
 *      dropped connection.
 *   2. A second device (a fresh browser context) opens the batch and sees
 *      the outcome the server kept: two drafts created, the frother left
 *      out, nothing left to do.
 *   3. The first device repeats the request. The server makes no second
 *      draft for a product that has one, so the group ends with exactly two
 *      new drafts.
 */
import fs from "node:fs";
import path from "node:path";

import { expect, type APIRequestContext } from "@playwright/test";

import { test } from "../fixtures/app-fixture.js";
import { ensureAuthenticated } from "./includes/auth.js";
import { extractApiAuth, resolveActiveGroup, type ApiAuth } from "./includes/commodities-api.js";

interface DraftRow {
  id: string;
  client_key: string;
  title: string;
}

interface CandidateRow {
  id: string;
  outcome: string;
  draft_id?: string;
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
  return ((await resp.json()) as { data: DraftRow[] }).data;
}

async function listCandidates(
  request: APIRequestContext,
  auth: ApiAuth,
  slug: string,
  sessionId: string,
): Promise<CandidateRow[]> {
  const resp = await request.get(
    `/api/v1/g/${encodeURIComponent(slug)}/intake/sessions/${encodeURIComponent(sessionId)}`,
    { headers: headers(auth) },
  );
  expect(resp.ok(), `GET /intake/sessions/{id} → ${resp.status()}`).toBe(true);
  return ((await resp.json()) as { data: { attributes: { candidates: CandidateRow[] } } }).data
    .attributes.candidates;
}

// The drafts batch intake made for this session's products: their client
// key names the candidate. Other specs running in parallel on the same group
// never match.
function sessionDrafts(drafts: DraftRow[], candidates: CandidateRow[]): DraftRow[] {
  const keys = new Set(candidates.map((c) => `intake:${c.id}`));
  return drafts.filter((d) => keys.has(d.client_key));
}

test.describe("Batch intake (#2810)", () => {
  let auth: ApiAuth;
  let slug: string;
  let run: string;
  let sessionId = "";

  test.beforeEach(async ({ page, request }) => {
    auth = await extractApiAuth(page);
    ({ slug } = await resolveActiveGroup(request, auth));
    run = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
  });

  test.afterEach(async ({ request }) => {
    if (!sessionId) return;
    const base = `/api/v1/g/${encodeURIComponent(slug)}`;
    const id = sessionId;
    sessionId = "";
    const candidates = await listCandidates(request, auth, slug, id).catch(() => []);
    const drafts = await listDrafts(request, auth, slug).catch(() => []);
    for (const d of sessionDrafts(drafts, candidates)) {
      await request
        .delete(`${base}/drafts/${encodeURIComponent(d.id)}`, { headers: headers(auth) })
        .catch(() => {});
    }
    await request
      .delete(`${base}/intake/sessions/${encodeURIComponent(id)}`, { headers: headers(auth) })
      .catch(() => {});
  });

  test("one receipt gives drafts for the kept products, on any device, without duplicates", async ({
    page,
    request,
    browser,
  }) => {
    test.setTimeout(120_000);

    // Device one: drop the receipt through "Add from files".
    await page.goto(`/g/${encodeURIComponent(slug)}/commodities`);
    await page.getByTestId("commodities-add-from-files").click();
    const dialog = page.getByTestId("intake-upload-dialog");
    await expect(dialog).toBeVisible();
    await page.getByTestId("intake-upload-input").setInputFiles({
      name: `receipt-${run}.pdf`,
      mimeType: "application/pdf",
      buffer: fs.readFileSync(path.join("fixtures", "files", "invoice.pdf")),
    });
    await expect(page.getByTestId("intake-upload-submit")).toHaveText("Scan 1 file");
    await page.getByTestId("intake-upload-submit").click();

    await page.waitForURL(/\/intake\/[^/]+$/);
    sessionId = new URL(page.url()).pathname.split("/").pop() ?? "";
    expect(sessionId).not.toBe("");

    // The worker scans the receipt in the background; the page follows.
    await expect(page.getByText("Espresso Machine")).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText("Milk Frother")).toBeVisible();
    await expect(page.getByText("Espresso Cups")).toBeVisible();
    await expect(page.getByTestId("intake-create-drafts")).toHaveText("Create 3 drafts");

    // Leave the frother out. The choice is stored on the server.
    await page.getByRole("checkbox", { name: "Make a draft of Milk Frother" }).click();
    await expect(page.getByTestId("intake-create-drafts")).toHaveText("Create 2 drafts");

    // The first request to create the drafts reaches the server, but its
    // answer never reaches the browser.
    let dropped = false;
    await page.route(`**/api/v1/g/${slug}/intake/sessions/*/drafts`, async (route) => {
      if (dropped) return route.continue();
      dropped = true;
      await route.fetch();
      await route.abort("failed");
    });
    await page.getByTestId("intake-create-drafts").click();
    await expect.poll(() => dropped).toBe(true);
    // This device still offers to create them: it never heard back.
    await expect(page.getByTestId("intake-create-drafts")).toBeEnabled();

    // Device two: a fresh browser context signs in on its own and sees what
    // the server kept.
    const other = await browser.newContext();
    try {
      const page2 = await other.newPage();
      await page2.goto("/");
      await ensureAuthenticated(page2);
      await page2.goto(`/g/${encodeURIComponent(slug)}/intake/${encodeURIComponent(sessionId)}`);
      await expect(page2.getByTestId("intake-page-summary")).toContainText("2 drafts created");
      const continues = page2.locator('[data-testid^="intake-continue-"]');
      await expect(continues).toHaveCount(2);
      await expect(page2.getByText("Left out")).toBeVisible();
      await expect(page2.getByTestId("intake-create-drafts")).toHaveCount(0);
    } finally {
      await other.close();
    }

    // Device one repeats the request; the products that have their draft
    // keep it. The error toast sits over the footer button, and a pointer
    // resting on a toast keeps it open, so move away and let it close first.
    await page.mouse.move(0, 0);
    await expect(page.locator("[data-sonner-toast]")).toHaveCount(0, { timeout: 15_000 });
    await page.getByTestId("intake-create-drafts").click();
    await expect(page.getByTestId("intake-page-summary")).toContainText("2 drafts created");
    await expect(page.locator('[data-testid^="intake-continue-"]')).toHaveCount(2);

    // Exactly one draft per kept product, and they are the drafts the
    // batch points at.
    const candidates = await listCandidates(request, auth, slug, sessionId);
    expect(candidates.map((c) => c.outcome).sort()).toEqual(["created", "created", "skipped"]);
    const drafts = sessionDrafts(await listDrafts(request, auth, slug), candidates);
    expect(drafts.map((d) => d.title).sort()).toEqual(["Espresso Cups", "Espresso Machine"]);
    expect(drafts.map((d) => d.id).sort()).toEqual(
      candidates
        .map((c) => c.draft_id)
        .filter((id): id is string => !!id)
        .sort(),
    );
  });
});
