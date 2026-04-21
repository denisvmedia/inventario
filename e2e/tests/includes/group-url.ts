import { Page } from '@playwright/test';

/**
 * Returns the group-scoped URL prefix for data-plane routes, e.g.
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
  const pathname = new URL(page.url()).pathname;
  const match = pathname.match(/^\/g\/([^/]+)(?:\/|$)/);
  if (!match) {
    throw new Error(
      `groupApiBase: page URL ${page.url()} is not /g/<slug>/... — tests using group-scoped endpoints must navigate into a group first`,
    );
  }
  return `/api/v1/g/${decodeURIComponent(match[1])}`;
}
