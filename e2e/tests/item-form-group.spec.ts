/**
 * E2E coverage for the group of the Add item form (#2920).
 *
 * A save still running when the member leaves the form with browser Back
 * and switches to another group goes on in the group the form opened in:
 * the file being sent reaches that group's draft, and nothing is sent to
 * the other group.
 */
import { expect, type APIRequestContext, type Page } from "@playwright/test";

import { test } from "../fixtures/app-fixture.js";
import {
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ApiAuth,
} from "./includes/commodities-api.js";

const ADMIN_PASSWORD = "TestPassword123";

function headers(auth: ApiAuth): Record<string, string> {
  return {
    "Content-Type": "application/vnd.api+json",
    Accept: "application/vnd.api+json",
    Authorization: `Bearer ${auth.accessToken}`,
    "X-CSRF-Token": auth.csrfToken,
  };
}

interface Group {
  id: string;
  name: string;
  slug: string;
}

async function groupName(request: APIRequestContext, auth: ApiAuth, id: string): Promise<string> {
  const resp = await request.get(`/api/v1/groups/${encodeURIComponent(id)}`, {
    headers: headers(auth),
  });
  expect(resp.ok(), `GET /groups/${id} → ${resp.status()}`).toBe(true);
  const body = (await resp.json()) as { data: { attributes: { name: string } } };
  return body.data.attributes.name;
}

async function createGroup(request: APIRequestContext, auth: ApiAuth, name: string): Promise<Group> {
  const resp = await request.post("/api/v1/groups", {
    headers: headers(auth),
    data: { data: { type: "groups", attributes: { name, icon: "🧪" } } },
  });
  expect(resp.status(), await resp.text()).toBe(201);
  const body = (await resp.json()) as { data: { id: string; attributes: { slug: string } } };
  return { id: body.data.id, name, slug: body.data.attributes.slug };
}

interface DraftRow {
  id: string;
  title: string;
  file_count: number;
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

async function pickType(page: Page, name: RegExp) {
  await page.locator("#commodity-type").click();
  await page.getByRole("listbox").getByRole("option", { name }).click();
}

async function walkToFiles(page: Page) {
  const files = page.getByTestId("commodity-form-files-step");
  for (let i = 0; i < 5 && !(await files.isVisible()); i++) {
    await page.getByTestId("commodity-form-next").click();
  }
  await expect(files).toBeVisible();
}

test.describe("Add item form group (#2920)", () => {
  let auth: ApiAuth;
  let form: Group | null = null;

  test.beforeEach(async ({ page }) => {
    auth = await extractApiAuth(page);
  });

  test.afterEach(async ({ request }) => {
    if (!form) return;
    await request
      .delete(`/api/v1/groups/${form.id}`, {
        headers: headers(auth),
        data: { confirm_word: form.name, password: ADMIN_PASSWORD },
      })
      .catch(() => {});
    form = null;
  });

  test("finishes the file in the form's group after Back and a group switch", async ({
    page,
    request,
  }) => {
    const run = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
    const home = await resolveActiveGroup(request, auth);
    const homeName = await groupName(request, auth, home.id);
    form = await createGroup(request, auth, `Workshop ${run}`);
    await ensureLocationAndArea(request, auth, form.slug);
    const name = `Planer ${run}`;

    // The upload answers once the member is in the other group.
    let release = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    let uploading = false;
    await page.route(`**/api/v1/g/${form.slug}/uploads/file`, async (route) => {
      if (route.request().method() !== "POST") return route.continue();
      uploading = true;
      await held;
      await route.continue();
    });
    const sentHome: string[] = [];
    page.on("request", (req) => {
      if (req.method() !== "GET" && req.url().includes(`/api/v1/g/${home.slug}/`)) {
        sentHome.push(`${req.method()} ${new URL(req.url()).pathname}`);
      }
    });

    // The sidebar's Add item opens the form on its own address, so browser
    // Back closes it without leaving the app.
    await page.goto(`/g/${encodeURIComponent(form.slug)}/commodities`);
    await page.getByTestId("sidebar-add-item").click();
    await page.waitForURL((url) => url.pathname.endsWith("/commodities/new"));
    await page.getByTestId("commodity-form-ai-fill-manually").click();
    await page.locator("#commodity-name").fill(name);
    await pickType(page, /Equipment/i);
    await walkToFiles(page);
    await page
      .locator('[data-testid="commodity-files-dropzone"] input[type="file"]')
      .setInputFiles("fixtures/files/image.jpg");
    await page.getByTestId("commodity-form-submit").click();
    await expect.poll(() => uploading).toBe(true);

    // Browser Back closes the form, and the member switches group.
    await page.goBack();
    await page.waitForURL((url) => url.pathname.endsWith("/commodities"));
    await expect(page.getByTestId("commodity-form-dialog")).toBeHidden();
    await page.locator(".group-selector__trigger").click();
    await page.locator(".group-selector__item", { hasText: homeName }).click();
    await expect(page.locator(".group-selector__trigger")).toContainText(homeName);
    await page.waitForURL((url) => url.pathname.startsWith(`/g/${home.slug}`));
    release();

    const fileCount = async () =>
      (await listDrafts(request, auth, form!.slug)).find((d) => d.title === name)?.file_count;
    await expect.poll(fileCount).toBe(1);
    expect(sentHome).toEqual([]);
  });
});
