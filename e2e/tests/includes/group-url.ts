import { Page } from '@playwright/test';

/**
 * Data-plane path prefixes that must be scoped under /g/<slug>/... after
 * #1321 removed the legacy flat route stubs. Anything outside this list
 * (/, /login, /profile, /groups/*, /invite/*, /no-group, /g/…) is
 * passed through unchanged.
 */
const FLAT_DATA_PREFIXES = [
  '/locations',
  '/areas',
  '/commodities',
  '/files',
  '/exports',
  '/system',
] as const;

function extractSlugFromUrl(url: string): string | null {
  try {
    const pathname = new URL(url).pathname;
    const match = pathname.match(/^\/g\/([^/]+)(?:\/|$)/);
    return match ? decodeURIComponent(match[1]) : null;
  } catch {
    return null;
  }
}

function isFlatDataPath(path: string): boolean {
  if (!path.startsWith('/')) return false;
  if (path.startsWith('/g/')) return false;
  return FLAT_DATA_PREFIXES.some((p) => path === p || path.startsWith(`${p}/`) || path.startsWith(`${p}?`));
}

/**
 * Returns the active group slug extracted from the page's current URL, or
 * null if the page is not on a /g/<slug>/... route. Sync — does not
 * navigate.
 */
export function currentGroupSlug(page: Page): string | null {
  return extractSlugFromUrl(page.url());
}

/**
 * Ensures the page is on a /g/<slug>/... route and returns the slug.
 * If the current URL already carries a slug it's returned immediately;
 * otherwise the helper navigates to `/` and lets the app redirect to the
 * user's default group before re-extracting.
 */
export async function ensureGroupSlug(page: Page): Promise<string> {
  const existing = currentGroupSlug(page);
  if (existing) return existing;

  await page.goto('/');
  await page.waitForURL(/\/g\/[^/]+/, { timeout: 15000 });
  const resolved = currentGroupSlug(page);
  if (!resolved) {
    throw new Error(
      `ensureGroupSlug: navigated to / but page URL ${page.url()} is still not /g/<slug>/... — is the user authenticated and a group member?`,
    );
  }
  return resolved;
}

/**
 * Rewrites a flat data-plane path (e.g. `/locations`, `/files/abc`) to
 * its /g/<slug>/... scoped equivalent using the slug extracted from the
 * page's current URL. Non-data paths (/, /login, /profile, etc.) and
 * already-scoped paths are returned unchanged.
 *
 * This is a sync helper that assumes the page has already navigated
 * into a group. When callers can't guarantee that, use `gotoScoped`
 * instead, which awaits `ensureGroupSlug` first.
 */
export function scopedPath(page: Page, path: string): string {
  if (!isFlatDataPath(path)) return path;
  const slug = currentGroupSlug(page);
  if (!slug) {
    throw new Error(
      `scopedPath: cannot scope "${path}" — page URL ${page.url()} is not /g/<slug>/... (call ensureGroupSlug first or use gotoScoped)`,
    );
  }
  return `/g/${encodeURIComponent(slug)}${path}`;
}

/**
 * `page.goto` wrapper that rewrites flat data-plane paths into their
 * /g/<slug>/... scoped equivalents. When the page isn't yet inside a
 * group (e.g. just after login on /login), navigates home first to
 * resolve the default-group slug. Non-data paths pass through.
 */
export async function gotoScoped(page: Page, path: string): Promise<void> {
  if (!isFlatDataPath(path)) {
    await page.goto(path);
    return;
  }
  const slug = await ensureGroupSlug(page);
  await page.goto(`/g/${encodeURIComponent(slug)}${path}`);
}

/**
 * Returns the group-scoped URL prefix for data-plane API endpoints, e.g.
 * `/api/v1/g/<slug>`. The frontend's axios instance injects this prefix via
 * an interceptor for `/api/v1/locations|commodities|files|exports|…`; tests
 * that drive the raw `page.request` / `request` fixtures bypass that axios
 * instance, so they have to prepend it themselves.
 *
 * After #1300 the active group lives on the URL itself
 * (/g/:groupSlug/...), not in localStorage. Parse the current URL to pull
 * the slug out. Throws if the page isn't on a /g/<slug>/... route — tests
 * that need group-scoped URLs must have navigated into a group first, and
 * silently producing an unprefixed URL would only re-introduce the 404
 * failure mode this helper exists to avoid.
 */
export async function groupApiBase(page: Page): Promise<string> {
  const slug = currentGroupSlug(page);
  if (!slug) {
    throw new Error(
      `groupApiBase: page URL ${page.url()} is not /g/<slug>/... — tests using group-scoped endpoints must navigate into a group first`,
    );
  }
  return `/api/v1/g/${slug}`;
}
