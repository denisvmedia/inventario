import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { CommodityCreateModalRoute } from "@/pages/commodities/CommodityCreateModal"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { areaHandlers, commodityHandlers, groupHandlers, locationHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const BACKDROP_PATH = `/g/${SLUG}/commodities`
const MODAL_PATH = `/g/${SLUG}/commodities/new`

const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]
const areaFixture = [
  { id: "a1", type: "areas", attributes: { id: "a1", name: "Garage", location_id: "l1" } },
]
const locationFixture = [{ id: "l1", type: "locations", attributes: { id: "l1", name: "Home" } }]

// Stub the form dialog with a minimal harness that exposes
// `onOpenChange` (Cancel) and `onSubmit` (Submit). The route
// wrapper's own logic — useState/useRef/useEffect timer
// cleanup, scheduleNavigate, close(), handleSubmit — is what
// we're covering here. The real <CommodityFormDialog /> has
// its own dedicated test file; reproducing the multi-step
// wizard walk-through inside this test would just exercise
// the dialog twice and re-trip the same Radix-Select-in-JSDOM
// gap tracked in #1629.
vi.mock("@/components/items/CommodityFormDialog", () => ({
  CommodityFormDialog: ({
    open,
    onOpenChange,
    onSubmit,
  }: {
    open: boolean
    onOpenChange: (open: boolean) => void
    onSubmit: (values: Record<string, unknown>) => Promise<unknown> | unknown
  }) =>
    open ? (
      <div data-testid="mock-dialog">
        <button data-testid="mock-cancel" type="button" onClick={() => onOpenChange(false)}>
          Cancel
        </button>
        <button
          data-testid="mock-submit"
          type="button"
          onClick={() =>
            void onSubmit({
              name: "Test item",
              short_name: "Test",
              type: "other",
              area_id: "a1",
              count: 1,
              status: "in_use",
            })
          }
        >
          Submit
        </button>
      </div>
    ) : null,
}))

function renderModal() {
  setAccessToken("good-token")
  function LocationProbe() {
    const loc = useLocation()
    return <div data-testid="loc">{loc.pathname}</div>
  }
  return renderWithProviders({
    // Two entries so `navigate(-1)` has a previous history slot to
    // pop back to (matches the production setup where the sidebar /
    // dashboard CTA pushes the modal route on top of the list page).
    initialEntries: [
      { pathname: BACKDROP_PATH, state: null },
      {
        pathname: MODAL_PATH,
        state: { background: { pathname: BACKDROP_PATH } },
      },
    ],
    routes: (
      <>
        <Route path="/g/:groupSlug/commodities" element={<LocationProbe />} />
        <Route
          path="/g/:groupSlug/commodities/new"
          element={
            <GroupProvider>
              <CommodityCreateModalRoute />
            </GroupProvider>
          }
        />
        <Route path="/g/:groupSlug/commodities/:id" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  // Fake timers so the deferred `navigate(...)` calls (200 ms wait
  // for the Radix close animation) can be advanced deterministically.
  // `shouldAdvanceTime: true` keeps MSW + react-query happy by letting
  // their internal microtasks run on the host scheduler.
  vi.useFakeTimers({ shouldAdvanceTime: true })
})

afterEach(() => {
  vi.useRealTimers()
})

describe("<CommodityCreateModalRoute />", () => {
  it("renders the form dialog when mounted as the modal route", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...locationHandlers.list(SLUG, locationFixture)
    )
    renderModal()
    expect(await screen.findByTestId("mock-dialog")).toBeInTheDocument()
  })

  it("pops the route back to the backdrop after the close animation when the user cancels", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...locationHandlers.list(SLUG, locationFixture)
    )
    renderModal()
    await screen.findByTestId("mock-dialog")
    await user.click(screen.getByTestId("mock-cancel"))
    // Dialog unmounts as soon as `open` flips to false; the
    // deferred navigate(-1) fires after the 200 ms animation timer.
    await waitFor(() => expect(screen.queryByTestId("mock-dialog")).toBeNull())
    vi.advanceTimersByTime(220)
    await waitFor(() => {
      expect(screen.getByTestId("loc").textContent).toBe(BACKDROP_PATH)
    })
  })

  it("navigates to the new commodity detail page after a successful submit", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...locationHandlers.list(SLUG, locationFixture),
      ...commodityHandlers.create(SLUG, {
        id: "c-new",
        type: "commodities",
        attributes: {
          id: "c-new",
          name: "Test item",
          short_name: "Test",
          type: "other",
          area_id: "a1",
          count: 1,
          status: "in_use",
        },
      })
    )
    renderModal()
    await screen.findByTestId("mock-dialog")
    await user.click(screen.getByTestId("mock-submit"))
    // Wait for the create mutation to settle and the dialog to start
    // its close animation (open → false unmounts our mock).
    await waitFor(() => expect(screen.queryByTestId("mock-dialog")).toBeNull())
    vi.advanceTimersByTime(220)
    await waitFor(() => {
      expect(screen.getByTestId("loc").textContent).toBe(`${BACKDROP_PATH}/c-new`)
    })
  })

  it("cancels the pending navigate timer on unmount so a late tick can't fire after the route is gone", async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...locationHandlers.list(SLUG, locationFixture)
    )
    const utils = renderModal()
    await screen.findByTestId("mock-dialog")
    // Start a close (schedules the 200 ms navigate timer) ...
    await user.click(screen.getByTestId("mock-cancel"))
    // ... then unmount BEFORE the timer fires. The unmount cleanup
    // must clear the pending timer; advancing time past 200 ms after
    // unmount should NOT throw / log a "navigate after unmount" error.
    utils.unmount()
    expect(() => vi.advanceTimersByTime(500)).not.toThrow()
  })
})
