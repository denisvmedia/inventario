/**
 * E2E coverage for the commodity-type Lucide icon mapping (#1392).
 *
 * Each row on `/commodities` carries a `CommodityThumb` that falls
 * back to a type-keyed Lucide icon when no cover photo is set. The
 * thumb exposes the type via `data-commodity-type` so the test can
 * assert "every type renders its own distinct icon" without peeking
 * at the SVG path. One commodity per backend type, all seeded via
 * the API with `cover === undefined` so the thumb renders the icon
 * branch and not the photo branch.
 *
 * Why this lives in e2e rather than vitest: the issue's acceptance
 * criterion calls out "e2e: list view shows the right icon next to
 * each item". Vitest covers the per-component fallback logic in
 * `frontend/src/features/commodities/__tests__/CommodityThumb.test.tsx`;
 * this spec walks the live React route over a real BE so a future
 * refactor (e.g., a new wrapping component that drops the
 * `data-commodity-type` hook) is caught here.
 *
 * Isolation: a per-run suffix keeps the visible set scoped via the
 * search box (`?q=<suffix>`), so the assertions stay deterministic
 * regardless of pre-existing seed data on the shared e2e DB.
 */
import { expect, type Page } from "@playwright/test"

import { test } from "../fixtures/app-fixture.js"
import {
  createCommodityViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
  type ResolvedGroup,
} from "./includes/commodities-api.js"

const TYPES = [
  "white_goods",
  "electronics",
  "equipment",
  "furniture",
  "clothes",
  "other",
] as const

async function gotoCommoditiesScoped(
  page: Page,
  group: ResolvedGroup,
  q: string,
): Promise<void> {
  await page.goto(
    `/g/${encodeURIComponent(group.slug)}/commodities?q=${encodeURIComponent(q)}`,
  )
  await expect(page.getByTestId("page-commodities")).toBeVisible()
}

test.describe("Commodity type icons (#1392)", () => {
  test("each backend type renders its own fallback icon on the list", async ({
    page,
    request,
  }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const seeded: { id: string; name: string; type: (typeof TYPES)[number] }[] = []
    const cleanup = async () => {
      for (const row of seeded) {
        await deleteCommodityViaAPI(request, auth, group.slug, row.id).catch(
          () => {},
        )
      }
    }

    try {
      for (const tp of TYPES) {
        const row = await createCommodityViaAPI(
          request,
          auth,
          group.slug,
          { name: `Icon ${tp} ${suffix}`, areaId, type: tp },
          group.groupCurrency,
        )
        seeded.push({ ...row, type: tp })
      }

      await gotoCommoditiesScoped(page, group, suffix)

      // Wait for every seeded card to materialise before asserting.
      for (const row of seeded) {
        await expect(
          page.locator(`[data-commodity-id="${row.id}"]`),
        ).toBeVisible({ timeout: 15000 })
      }

      // Per-type assertions: the thumb is in fallback mode (no cover
      // was uploaded), the `data-commodity-type` attribute matches
      // the seeded type, and a Lucide `<svg>` is rendered inside.
      for (const row of seeded) {
        const card = page.locator(`[data-commodity-id="${row.id}"]`)
        const thumb = card.locator('[data-testid="commodity-card-thumb"]')
        await expect(thumb).toHaveAttribute("data-state", "fallback")
        await expect(thumb).toHaveAttribute("data-commodity-type", row.type)
        await expect(thumb.locator("svg")).toHaveCount(1)
      }

      // The six rendered icons must be visually distinct — same
      // `<svg>` element under the hood, but Lucide gives each glyph
      // a unique `class` token (`lucide-refrigerator`, `lucide-laptop`,
      // …). Collecting the class lists and asserting uniqueness keeps
      // the test framework-agnostic and survives Lucide version bumps
      // that change the actual path data.
      const classes = await Promise.all(
        seeded.map(async (row) => {
          const svg = page
            .locator(`[data-commodity-id="${row.id}"]`)
            .locator('[data-testid="commodity-card-thumb"] svg')
          const cls = (await svg.getAttribute("class")) ?? ""
          return cls
        }),
      )
      const unique = new Set(classes.filter((c) => c.length > 0))
      expect(unique.size).toBe(seeded.length)
    } finally {
      await cleanup()
    }
  })
})
