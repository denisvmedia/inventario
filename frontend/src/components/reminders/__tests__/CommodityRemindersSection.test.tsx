import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { CommodityRemindersSection } from "@/components/reminders/CommodityRemindersSection"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { capture } from "@/test/capture"
import { groupHandlers, reminderHandlers } from "@/test/handlers"
import { pickRadixSelect } from "@/test/radix"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const COMMODITY_ID = "commodity-1"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
})

function renderSection() {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${COMMODITY_ID}`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <CommodityRemindersSection commodityId={COMMODITY_ID} />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

describe("<CommodityRemindersSection />", () => {
  it("shows the empty state", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...reminderHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    renderSection()
    expect(await screen.findByTestId("reminders-empty")).toHaveTextContent("No reminders yet.")
    expect(screen.getByTestId("reminder-add")).toBeInTheDocument()
  })

  it("describes the repeat rule, lead times and next date of each reminder", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...reminderHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        {
          id: "r1",
          commodity_id: COMMODITY_ID,
          title: "Insurance renewal",
          due_on: "2026-01-31",
          remind_days_before: [30, 7, 0],
          repeat_unit: "month",
          repeat_interval: 3,
          next_occurrence: "2027-01-31",
          days_until_next: 12,
        },
        {
          id: "r2",
          commodity_id: COMMODITY_ID,
          title: "Return window ends",
          due_on: "2026-01-02",
          remind_days_before: [1],
          repeat_unit: "none",
          repeat_interval: 1,
        },
      ])
    )
    renderSection()
    const first = await screen.findByTestId("reminder-row-r1")
    expect(within(first).getByText("Insurance renewal")).toBeInTheDocument()
    expect(
      within(first).getByText("Every 3 months · Email 30, 7 days before, on the day")
    ).toBeInTheDocument()
    expect(within(first).getByText("In 12 days")).toBeInTheDocument()
    // Delete is the red ghost icon trigger of a row (#2914).
    const deleteTrigger = screen.getByTestId("reminder-row-r1-delete")
    expect(deleteTrigger).toHaveAttribute("data-variant", "ghost")
    expect(deleteTrigger).toHaveClass(
      "text-destructive",
      "hover:bg-destructive/10",
      "hover:text-destructive"
    )

    const second = screen.getByTestId("reminder-row-r2")
    expect(within(second).getByText("One-off · Email 1 day before")).toBeInTheDocument()
    expect(within(second).getByText("Passed")).toBeInTheDocument()
  })

  it("creates a reminder with parsed lead times and a repeat rule", async () => {
    const body = capture<{ data: { attributes: Record<string, unknown> } }>()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...reminderHandlers.listForCommodity(SLUG, COMMODITY_ID, []),
      ...reminderHandlers.create(
        SLUG,
        COMMODITY_ID,
        {
          id: "r9",
          commodity_id: COMMODITY_ID,
          title: "Inspection",
          due_on: "2027-03-01",
          remind_days_before: [14, 1],
          repeat_unit: "year",
          repeat_interval: 2,
        },
        body
      )
    )
    const user = userEvent.setup()
    renderSection()
    await user.click(await screen.findByTestId("reminder-add"))

    await user.type(screen.getByTestId("reminder-title-input"), "Inspection")
    await user.type(screen.getByTestId("reminder-date-input"), "2027-03-01")
    const leads = screen.getByTestId("reminder-leads-input")
    await user.clear(leads)
    await user.type(leads, "1, 14")
    await pickRadixSelect(user, /Repeat/i, { optionLabel: /^Yearly$/i })
    const interval = await screen.findByTestId("reminder-interval-input")
    await user.clear(interval)
    await user.type(interval, "2")
    await user.click(screen.getByTestId("reminder-submit"))

    await waitFor(() => expect(body.value).toBeDefined())
    expect(body.value?.data.attributes).toEqual({
      title: "Inspection",
      due_on: "2027-03-01",
      remind_days_before: [1, 14],
      repeat_unit: "year",
      repeat_interval: 2,
    })
  })

  it("rejects lead times it cannot parse before calling the server", async () => {
    const body = capture<unknown>()
    server.use(
      ...groupHandlers.list(groupFixture),
      ...reminderHandlers.listForCommodity(SLUG, COMMODITY_ID, []),
      ...reminderHandlers.create(
        SLUG,
        COMMODITY_ID,
        { id: "x", commodity_id: COMMODITY_ID, title: "x" },
        body
      )
    )
    const user = userEvent.setup()
    renderSection()
    await user.click(await screen.findByTestId("reminder-add"))
    await user.type(screen.getByTestId("reminder-title-input"), "Inspection")
    await user.type(screen.getByTestId("reminder-date-input"), "2027-03-01")
    const leads = screen.getByTestId("reminder-leads-input")
    await user.clear(leads)
    await user.type(leads, "7, soon")
    await user.click(screen.getByTestId("reminder-submit"))

    expect(await screen.findByText("Use whole numbers separated by commas.")).toBeInTheDocument()
    expect(body.value).toBeUndefined()
  })
})
