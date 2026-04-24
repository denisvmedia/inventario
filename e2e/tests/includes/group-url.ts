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
 * Resolves a group slug for the authenticated user via /api/v1/groups.
 * The frontend authenticates with a JWT stored in localStorage
 * (`inventario_token`) and attached as `Authorization: Bearer`, so we run
 * the fetch inside the page context where that token is in scope.
 * Prefers the user's default_group_id (via /api/v1/auth/me) when
 * available, otherwise falls back to the first group returned — the seed
 * dataset ships a single group so either path lands on the same slug.
 */
async function resolveGroupSlugFromApi(page: Page): Promise<string | null> {
  try {
    return await page.evaluate(async () => {
      const token = localStorage.getItem('inventario_token');
      if (!token) return null;
      const headers: Record<string, string> = {
        Accept: 'application/vnd.api+json',
        Authorization: `Bearer ${token}`,
      };
      const groupsResp = await fetch('/api/v1/groups', { headers });
      if (!groupsResp.ok) return null;
      const groupsBody = await groupsResp.json();
      const items: Array<{ id: string; attributes?: { slug?: string } }> =
        groupsBody?.data ?? [];
      if (items.length === 0) return null;

      let preferredId: string | null = null;
      try {
        const meResp = await fetch('/api/v1/auth/me', { headers });
        if (meResp.ok) {
          const meBody = await meResp.json();
          preferredId =
            meBody?.data?.attributes?.default_group_id ??
            meBody?.default_group_id ??
            null;
        }
      } catch {
        // best-effort — fall through to first-item fallback
      }

      if (preferredId) {
        const match = items.find((g) => g.id === preferredId);
        if (match?.attributes?.slug) return match.attributes.slug;
      }
      return items[0]?.attributes?.slug ?? null;
    });
  } catch {
    return null;
  }
}

/**
 * Ensures a group slug is available and returns it. Prefers the slug
 * already present on the page URL; otherwise queries /api/v1/groups from
 * inside the page context (reusing the JWT the frontend stashed in
 * localStorage at login time). Does not rely on any implicit `/` →
 * `/g/<slug>` redirect — the app's `/` is a valid unscoped home, not a
 * redirector.
 */
export async function ensureGroupSlug(page: Page): Promise<string> {
  const existing = currentGroupSlug(page);
  if (existing) return existing;

  const resolved = await resolveGroupSlugFromApi(page);
  if (!resolved) {
    throw new Error(
      `ensureGroupSlug: could not resolve a group slug from /api/v1/groups — is the user authenticated and a member of at least one group? current URL: ${page.url()}`,
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
