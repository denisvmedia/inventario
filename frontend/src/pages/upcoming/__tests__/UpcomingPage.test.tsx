import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { SectionRoute } from "@/components/routing/SectionRoute"
import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { UpcomingPage } from "@/pages/upcoming/UpcomingPage"
import { groupHandlers, reminderHandlers } from "@/test/handlers"
import { pickRadixSelect } from "@/test/radix"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

const ITEMS: Schema<"jsonapi.UpcomingItem">[] = [
  {
    kind: "loan",
    date: "2026-11-23",
    days_left: -2,
    title: "Anna",
    source_id: "loan-1",
    commodity: { id: "c-drill", name: "Drill" },
  },
  {
    kind: "reminder",
    date: "2026-11-30",
    days_left: 5,
    title: "Inspection",
    source_id: "rem-1",
    commodity: { id: "c-car", name: "Car" },
  },
  {
    kind: "warranty",
    date: "2026-12-07",
    days_left: 12,
    source_id: "c-tv",
    commodity: { id: "c-tv", name: "TV" },
  },
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

function renderPage(path = `/g/${SLUG}/upcoming`) {
  return renderWithProviders({
    initialPath: path,
    routes: (
      <Route
        path="/g/:groupSlug/upcoming"
        element={
          <GroupProvider>
            <UpcomingPage />
          </GroupProvider>
        }
      />
    ),
  })
}

describe("<UpcomingPage />", () => {
  it("lists every kind in server order with links to the right item tab", async () => {
    server.use(...groupHandlers.list(groupFixture), ...reminderHandlers.upcoming(SLUG, ITEMS))
    renderPage()
    const list = await screen.findByTestId("upcoming-list")
    const rows = within(list).getAllByRole("link")
    expect(rows.map((r) => r.getAttribute("data-testid"))).toEqual([
      "upcoming-row-loan-loan-1",
      "upcoming-row-reminder-rem-1",
      "upcoming-row-warranty-c-tv",
    ])
    expect(rows[0]).toHaveAttribute("href", `/g/${SLUG}/commodities/c-drill?tab=lend`)
    expect(rows[1]).toHaveAttribute("href", `/g/${SLUG}/commodities/c-car`)
    expect(rows[2]).toHaveAttribute("href", `/g/${SLUG}/commodities/c-tv?tab=warranty`)
    expect(within(rows[0]).getByText("2 days overdue")).toBeInTheDocument()
    expect(within(rows[1]).getByText("Reminder · Inspection")).toBeInTheDocument()
    expect(within(rows[2]).getByText("In 12 days")).toBeInTheDocument()
  })

  it("filters by kind and shows the per-kind counts", async () => {
    server.use(...groupHandlers.list(groupFixture), ...reminderHandlers.upcoming(SLUG, ITEMS))
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("upcoming-list")
    expect(screen.getByTestId("upcoming-tab-all")).toHaveTextContent("All3")
    expect(screen.getByTestId("upcoming-tab-maintenance")).toHaveTextContent(/^Maintenance$/)

    await user.click(screen.getByTestId("upcoming-tab-reminder"))
    const list = screen.getByTestId("upcoming-list")
    expect(within(list).getAllByRole("link")).toHaveLength(1)
    expect(within(list).getByText("Car")).toBeInTheDocument()

    await user.click(screen.getByTestId("upcoming-tab-maintenance"))
    expect(screen.getByTestId("upcoming-empty")).toBeInTheDocument()
  })

  it("asks the server for the chosen period", async () => {
    const days: number[] = []
    server.use(...groupHandlers.list(groupFixture), ...reminderHandlers.upcoming(SLUG, [], days))
    const user = userEvent.setup()
    renderPage()
    expect(await screen.findByTestId("upcoming-empty")).toBeInTheDocument()
    expect(days).toEqual([30])

    await pickRadixSelect(user, /Period/i, { optionLabel: /Next 12 months/i })
    await waitFor(() => expect(days).toContain(365))
  })
})

describe("<UpcomingPage /> in a group that hides sections (#1607)", () => {
  function renderRoute() {
    return renderWithProviders({
      initialPath: `/g/${SLUG}/upcoming`,
      routes: (
        <Route
          path="/g/:groupSlug/upcoming"
          element={
            <GroupProvider>
              <SectionRoute section="upcoming">
                <UpcomingPage />
              </SectionRoute>
            </GroupProvider>
          }
        />
      ),
    })
  }

  it("leaves out the dates of hidden sections and their tabs", async () => {
    server.use(
      ...groupHandlers.list([{ ...groupFixture[0], hidden_sections: ["warranties", "loans"] }]),
      ...reminderHandlers.upcoming(SLUG, ITEMS)
    )
    renderRoute()

    const list = await screen.findByTestId("upcoming-list")
    await waitFor(() => expect(within(list).getAllByRole("listitem")).toHaveLength(1))
    expect(list).toHaveTextContent("Car")
    expect(screen.getByTestId("upcoming-tab-reminder")).toBeInTheDocument()
    expect(screen.getByTestId("upcoming-tab-maintenance")).toBeInTheDocument()
    expect(screen.queryByTestId("upcoming-tab-warranty")).not.toBeInTheDocument()
    expect(screen.queryByTestId("upcoming-tab-loan")).not.toBeInTheDocument()
  })

  it("shows a notice with a link to the group settings while the group hides Upcoming", async () => {
    server.use(
      ...groupHandlers.list([{ ...groupFixture[0], hidden_sections: ["upcoming"] }]),
      ...reminderHandlers.upcoming(SLUG, ITEMS)
    )
    renderRoute()

    const notice = await screen.findByTestId("hidden-section-notice")
    expect(notice).toHaveAttribute("data-section", "upcoming")
    expect(
      within(notice).getByRole("heading", { name: "Upcoming is hidden in this group" })
    ).toBeInTheDocument()
    expect(within(notice).getByTestId("hidden-section-settings")).toHaveAttribute(
      "href",
      "/groups/g1/settings?section=sections"
    )
    expect(screen.queryByTestId("page-upcoming")).not.toBeInTheDocument()
  })
})
