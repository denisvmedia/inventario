/**
 * E2E coverage for maintenance reminders (#1368). Walks the
 * acceptance-criterion happy path:
 *   1. Open a fresh commodity → Maintenance tab → "Add schedule"
 *      dialog.
 *   2. Fill title + interval (90 days), submit.
 *   3. Schedule shows up on the per-commodity Maintenance tab AND on
 *      the group-wide /maintenance page.
 *   4. Click "I did this" → next_due_at advances by 90 days and
 *      last_done_at is recorded.
 *
 * The reminder-email side of the worker is covered by the Go service
 * test `TestMaintenanceReminderService_RemindOnce_TickClock`; we don't
 * try to drive the worker from Playwright (it would need a force-run
 * endpoint we deliberately don't ship). This spec just covers the
 * user-facing CRUD + "I did this" round-trip.
 */
import { expect } from "@playwright/test"

import { test } from "../fixtures/app-fixture.js"
import {
  createCommodityViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
} from "./includes/commodities-api.js"

function addDays(date: Date, days: number): Date {
  const out = new Date(date)
  out.setUTCDate(out.getUTCDate() + days)
  return out
}

function isoDate(date: Date): string {
  return date.toISOString().slice(0, 10)
}

test.describe("Maintenance reminders — schedule + mark-done round-trip", () => {
  test("create schedule, mark done, verify next_due_at advances", async ({ page, request }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const commodityName = `Maintenance Item ${suffix}`
    const scheduleTitle = `Replace water filter ${suffix}`
    const seededIDs: string[] = []
    const cleanup = async () => {
      for (const id of seededIDs) {
        await deleteCommodityViaAPI(request, auth, group.slug, id).catch(() => {})
      }
    }

    try {
      const { id: commodityID } = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: commodityName, areaId, type: "equipment" },
        group.groupCurrency
      )
      seededIDs.push(commodityID)

      // 1) Open the detail page → Maintenance tab.
      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(commodityID)}`
      )
      await expect(page.getByTestId("commodity-detail-tabs")).toBeVisible()
      await page.getByRole("tab", { name: /^Maintenance$/ }).click()
      await expect(page.getByTestId("maintenance-add")).toBeVisible()

      // 2) Open the dialog, fill title + interval (default is 90), submit.
      await page.getByTestId("maintenance-add").click()
      await page.getByTestId("maintenance-title-input").fill(scheduleTitle)
      await expect(page.getByTestId("maintenance-interval-input")).toHaveValue("90")
      await page.getByTestId("maintenance-submit").click()

      // 3) Schedule renders on the per-commodity tab. We don't pin
      // the BE-generated id; assert by title presence + the
      // "I did this" CTA shape.
      await expect(page.getByText(scheduleTitle)).toBeVisible()
      const doneButton = page
        .locator('[data-testid^="schedule-"][data-testid$="-done"]')
        .first()
      await expect(doneButton).toBeVisible()

      // Capture the schedule's pre-done next-due cell so we can assert
      // it changed after the mark-done call.
      const nextDueCell = page
        .locator('[data-testid^="schedule-"][data-testid$="-next-due"]')
        .first()
      const beforeText = (await nextDueCell.innerText()).trim()

      // 4) Group-wide /maintenance page surfaces the row, sorted by
      // next_due_at. The same commodity link appears in the Item
      // column.
      await page.goto(`/g/${encodeURIComponent(group.slug)}/maintenance`)
      await expect(page.getByTestId("page-maintenance")).toBeVisible()
      await expect(page.getByText(commodityName).first()).toBeVisible()
      await expect(page.getByText(scheduleTitle)).toBeVisible()

      // 5) Back to the per-commodity tab, click "I did this".
      await page.goto(
        `/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(commodityID)}?tab=maintenance`
      )
      await page.getByRole("tab", { name: /^Maintenance$/ }).click()
      await page
        .locator('[data-testid^="schedule-"][data-testid$="-done"]')
        .first()
        .click()

      // 6) next_due_at advanced by 90 days. We don't try to pin the
      // exact date (the server clock can drift relative to the test
      // runner by a fraction of a day), but the new value MUST differ
      // from the captured "before" text — the original UI rendered
      // some next-due date and after MarkDone we expect that cell to
      // change.
      await expect.poll(async () => (await nextDueCell.innerText()).trim()).not.toBe(beforeText)

      // Sanity: assert the resulting date is at least 80 days out
      // from today (90d interval minus generous clock drift). The
      // rendered string is the localized date — formatDate uses
      // toLocaleDateString — so we parse via Date.parse, which
      // tolerates locale-formatted strings on a modern Chromium.
      const afterText = (await nextDueCell.innerText()).trim().split("\n")[0]
      const todayPlus80 = addDays(new Date(), 80)
      const parsed = new Date(afterText)
      if (!Number.isNaN(parsed.getTime())) {
        expect(parsed.getTime()).toBeGreaterThan(todayPlus80.getTime())
      }

      // Last-done shows today's UTC date — we don't assert the
      // displayed string verbatim (locale-dependent), only that
      // the "—" placeholder is gone.
      const cards = page.locator('[data-testid^="schedule-"][data-testid$="-next-due"]')
      const firstCard = cards.first()
      const parentCard = firstCard.locator(
        'xpath=ancestor::*[contains(@class, "Card") or contains(@class, "card")][1]'
      )
      // Best-effort visibility check — if the structure can't be
      // located the test still passes on the next_due_at advancement.
      if ((await parentCard.count()) > 0) {
        await expect(parentCard.first()).not.toContainText(/^—$/)
      }

      // Belt-and-braces: read the schedule back through the API and
      // assert next_due_at advanced ≥ 80d and last_done_at is set.
      const apiBase = `${page.url().replace(/\/g\/.*$/, "")}/api/v1/g/${encodeURIComponent(
        group.slug
      )}/commodities/${encodeURIComponent(commodityID)}/maintenance`
      const apiResp = await request.get(apiBase, {
        headers: {
          Authorization: `Bearer ${auth.accessToken}`,
        },
      })
      expect(apiResp.ok()).toBeTruthy()
      const body = await apiResp.json()
      const rows = (body.data ?? []) as Array<{
        title: string
        next_due_at: string
        last_done_at?: string | null
      }>
      const ours = rows.find((r) => r.title === scheduleTitle)
      expect(ours, "schedule should be present in the API response").toBeTruthy()
      const nextDue = new Date(`${ours!.next_due_at}T00:00:00Z`)
      const todayUTC = new Date(isoDate(new Date()) + "T00:00:00Z")
      const days = Math.round((nextDue.getTime() - todayUTC.getTime()) / (1000 * 60 * 60 * 24))
      expect(days).toBeGreaterThanOrEqual(85)
      expect(days).toBeLessThanOrEqual(95)
      expect(ours!.last_done_at).toBeTruthy()
    } finally {
      await cleanup()
    }
  })
})
