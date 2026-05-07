import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { Outlet, Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest"

import { SelectedItemsPicker } from "@/components/exports/SelectedItemsPicker"
import type { ExportSelectedItem } from "@/features/export/api"
import { GroupProvider } from "@/features/group/GroupContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, locationHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const SLUG = "household"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  server.use(...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household" }]))
})

interface LocationFixture {
  id: string
  name: string
  address?: string
}

function toResource(loc: LocationFixture) {
  return {
    id: loc.id,
    type: "locations",
    attributes: { name: loc.name, address: loc.address ?? "" },
  }
}

function renderPicker(options: {
  locations: LocationFixture[]
  initialValue?: ExportSelectedItem[]
  onChange?: (next: ExportSelectedItem[]) => void
}) {
  server.use(...locationHandlers.list(SLUG, options.locations.map(toResource)))
  const onChange = options.onChange ?? vi.fn()
  return renderWithProviders({
    initialPath: `/g/${SLUG}/exports/new`,
    routes: (
      <Route
        path="/g/:groupSlug"
        element={
          <GroupProvider>
            <main>
              <Outlet />
            </main>
          </GroupProvider>
        }
      >
        <Route
          path="exports/new"
          element={<SelectedItemsPicker value={options.initialValue ?? []} onChange={onChange} />}
        />
      </Route>
    ),
  })
}

describe("<SelectedItemsPicker />", () => {
  it("renders the search input above the list once locations have loaded", async () => {
    renderPicker({
      locations: [
        { id: "loc-1", name: "Main House" },
        { id: "loc-2", name: "Garage" },
      ],
    })
    expect(await screen.findByTestId("selected-items-picker-search")).toBeVisible()
    expect(screen.getByTestId("selected-items-picker-row-loc-1")).toBeVisible()
    expect(screen.getByTestId("selected-items-picker-row-loc-2")).toBeVisible()
  })

  it("filters the list by name as the user types (case-insensitive)", async () => {
    const user = userEvent.setup()
    renderPicker({
      locations: [
        { id: "loc-1", name: "Main House" },
        { id: "loc-2", name: "Garage" },
        { id: "loc-3", name: "Cottage" },
      ],
    })
    const search = await screen.findByTestId("selected-items-picker-search")
    await user.type(search, "gar")
    await waitFor(() =>
      expect(screen.queryByTestId("selected-items-picker-row-loc-1")).not.toBeInTheDocument()
    )
    expect(screen.getByTestId("selected-items-picker-row-loc-2")).toBeVisible()
    expect(screen.queryByTestId("selected-items-picker-row-loc-3")).not.toBeInTheDocument()
  })

  it("matches against the address as well as the name", async () => {
    const user = userEvent.setup()
    renderPicker({
      locations: [
        { id: "loc-1", name: "Main House", address: "12 Elm Street" },
        { id: "loc-2", name: "Garage", address: "Backyard" },
      ],
    })
    const search = await screen.findByTestId("selected-items-picker-search")
    await user.type(search, "elm")
    await waitFor(() =>
      expect(screen.queryByTestId("selected-items-picker-row-loc-2")).not.toBeInTheDocument()
    )
    expect(screen.getByTestId("selected-items-picker-row-loc-1")).toBeVisible()
  })

  it("keeps already-picked rows visible even when they don't match the query", async () => {
    const user = userEvent.setup()
    renderPicker({
      locations: [
        { id: "loc-1", name: "Main House" },
        { id: "loc-2", name: "Garage" },
        { id: "loc-3", name: "Cottage" },
      ],
      initialValue: [{ type: "location", id: "loc-1", name: "Main House", include_all: true }],
    })
    const search = await screen.findByTestId("selected-items-picker-search")
    await user.type(search, "gar")
    // loc-1 is picked → stays visible despite not matching "gar".
    expect(screen.getByTestId("selected-items-picker-row-loc-1")).toBeVisible()
    expect(screen.getByTestId("selected-items-picker-row-loc-2")).toBeVisible()
    expect(screen.queryByTestId("selected-items-picker-row-loc-3")).not.toBeInTheDocument()
  })

  it("renders the search-empty message when nothing matches and nothing is picked", async () => {
    const user = userEvent.setup()
    renderPicker({
      locations: [
        { id: "loc-1", name: "Main House" },
        { id: "loc-2", name: "Garage" },
      ],
    })
    const search = await screen.findByTestId("selected-items-picker-search")
    await user.type(search, "zzz")
    expect(await screen.findByTestId("selected-items-picker-search-empty")).toBeVisible()
    expect(screen.queryByTestId("selected-items-picker-row-loc-1")).not.toBeInTheDocument()
  })

  it("does NOT render the search-empty message when a picked row keeps the list non-empty", async () => {
    const user = userEvent.setup()
    renderPicker({
      locations: [{ id: "loc-1", name: "Main House" }],
      initialValue: [{ type: "location", id: "loc-1", name: "Main House", include_all: true }],
    })
    const search = await screen.findByTestId("selected-items-picker-search")
    await user.type(search, "zzz")
    // Picked row keeps the list non-empty → no search-empty banner.
    expect(screen.queryByTestId("selected-items-picker-search-empty")).not.toBeInTheDocument()
    expect(screen.getByTestId("selected-items-picker-row-loc-1")).toBeVisible()
  })

  it("does not render the search input when there are no locations at all", async () => {
    renderPicker({ locations: [] })
    await waitFor(() => {
      expect(screen.getByTestId("selected-items-picker-empty")).toBeVisible()
    })
    expect(screen.queryByTestId("selected-items-picker-search")).not.toBeInTheDocument()
  })
})
