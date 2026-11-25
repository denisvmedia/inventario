/**
 * E2E coverage for item reminders and the Upcoming page (#2805):
 *   1. Open a fresh item and add a yearly reminder from the Reminders card
 *      on the Details tab.
 *   2. The reminder shows its repeat rule, lead times and next date.
 *   3. The group-wide Upcoming page lists it, linked back to the item.
 *   4. Edit the title, then delete the reminder.
 *
 * Sending is covered by the Go service tests
 * (services/item_reminder_service_test.go); the worker has no force-run
 * endpoint to drive from Playwright.
 */
import { expect, type Page } from "@playwright/test"

import { test } from "../fixtures/app-fixture.js"
import {
  createCommodityViaAPI,
  deleteCommodityViaAPI,
  ensureLocationAndArea,
  extractApiAuth,
  resolveActiveGroup,
} from "./includes/commodities-api.js"

function isoDateFromToday(days: number): string {
  const d = new Date()
  d.setUTCDate(d.getUTCDate() + days)
  return d.toISOString().slice(0, 10)
}

async function pickRepeat(page: Page, label: RegExp) {
  await page.getByTestId("reminder-repeat-select").click()
  const listbox = page.getByRole("listbox")
  await listbox.waitFor({ state: "visible", timeout: 5000 })
  await listbox.getByRole("option", { name: label }).click()
  await listbox.waitFor({ state: "detached", timeout: 5000 })
}

test.describe("Item reminders and the Upcoming page", () => {
  test("add, list on Upcoming, edit and delete a reminder", async ({ page, request }) => {
    const auth = await extractApiAuth(page)
    const group = await resolveActiveGroup(request, auth)
    const { areaId } = await ensureLocationAndArea(request, auth, group.slug)

    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const commodityName = `Reminder Item ${suffix}`
    const title = `Insurance renewal ${suffix}`
    const renamed = `Policy renewal ${suffix}`
    let commodityID = ""

    try {
      ;({ id: commodityID } = await createCommodityViaAPI(
        request,
        auth,
        group.slug,
        { name: commodityName, areaId, type: "equipment" },
        group.groupCurrency
      ))
      const itemPath = `/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(commodityID)}`

      // 1) Add a reminder five days out, yearly, with two lead times.
      await page.goto(itemPath)
      const card = page.getByTestId("commodity-reminders")
      await expect(card.getByTestId("reminders-empty")).toBeVisible()
      await card.getByTestId("reminder-add").click()
      await page.getByTestId("reminder-title-input").fill(title)
      await page.getByTestId("reminder-date-input").fill(isoDateFromToday(5))
      await page.getByTestId("reminder-leads-input").fill("7, 1")
      await pickRepeat(page, /^Yearly$/)
      await expect(page.getByTestId("reminder-interval-input")).toHaveValue("1")
      await page.getByTestId("reminder-submit").click()

      // 2) The row describes the rule and counts down to the date.
      const list = card.getByTestId("reminders-list")
      await expect(list.getByText(title)).toBeVisible()
      await expect(list.getByText("Every year · Email 7, 1 days before")).toBeVisible()
      await expect(list.getByText("In 5 days")).toBeVisible()

      // 3) The Upcoming page lists it, and the row links to the item.
      await page.goto(`/g/${encodeURIComponent(group.slug)}/upcoming`)
      await expect(page.getByTestId("page-upcoming")).toBeVisible()
      await page.getByTestId("upcoming-tab-reminder").click()
      const row = page.locator('[data-testid^="upcoming-row-reminder-"]', {
        hasText: commodityName,
      })
      await expect(row).toBeVisible()
      await expect(row).toContainText(`Reminder · ${title}`)
      await row.click()
      await expect(page).toHaveURL(new RegExp(`/commodities/${commodityID}$`))

      // 4) Edit the title, then delete the reminder.
      const reminderRow = page.locator('[data-testid^="reminder-row-"]', { hasText: title })
      await reminderRow.locator('[data-testid$="-edit"]').click()
      const titleInput = page.getByTestId("reminder-title-input")
      await titleInput.fill(renamed)
      await page.getByTestId("reminder-submit").click()
      await expect(card.getByTestId("reminders-list").getByText(renamed)).toBeVisible()

      await page
        .locator('[data-testid^="reminder-row-"]', { hasText: renamed })
        .locator('[data-testid$="-delete"]')
        .click()
      await expect(page.getByTestId("confirm-dialog")).toBeVisible()
      await page.getByTestId("confirm-accept").click()
      await expect(card.getByTestId("reminders-empty")).toBeVisible()
    } finally {
      if (commodityID) {
        await deleteCommodityViaAPI(request, auth, group.slug, commodityID).catch(() => {})
      }
    }
  })
})
