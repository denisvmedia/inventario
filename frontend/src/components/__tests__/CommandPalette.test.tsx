import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { TopBar } from "@/components/TopBar"
import { RouteTitleProvider } from "@/components/routing/RouteTitle"
import { SidebarProvider } from "@/components/ui/sidebar"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAllRecent, getRecent, pushRecent, recentScope } from "@/features/search/recent"
import { authHandlers, groupHandlers, memberHandlers, tagHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

// #2801 — the command palette opens from the TopBar's search entry as well
// as from Cmd/Ctrl+K, and lists the matches of the active group by kind.

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const location = useLocation()
  return <p data-testid="location">{location.pathname + location.search}</p>
}

function renderTopBar(initialPath = "/g/household/commodities") {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/g/:groupSlug/*"
        element={
          <AuthProvider>
            <GroupProvider>
              <RouteTitleProvider>
                <SidebarProvider>
                  <TopBar />
                  <LocationProbe />
                </SidebarProvider>
              </RouteTitleProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

// One result per kind for "garage"; nothing for any other query.
const searchHandler = msw.get(api("/g/household/search"), ({ request }) => {
  const url = new URL(request.url)
  const type = url.searchParams.get("type")
  if (url.searchParams.get("q") !== "garage") {
    return HttpResponse.json({ data: [], meta: { total: 0, entity_type: type } })
  }
  const rows: Record<string, unknown[]> = {
    commodities: [{ id: "c1", name: "Hedge trimmer", short_name: "Trimmer", asset_number: 7 }],
    locations: [{ id: "l1", name: "Garage", address: "Main street 1" }],
    areas: [{ id: "a1", name: "Garage shelf", location_id: "l1" }],
    files: [{ id: "f1", title: "Garage door manual", path: "door.pdf" }],
    tags: [{ id: "t1", slug: "garage", label: "Garage", kind: "commodity", color: "blue" }],
  }
  const data = rows[type ?? ""] ?? []
  return HttpResponse.json({
    data,
    meta: {
      total: type === "commodities" ? 12 : data.length,
      entity_type: type,
      matches: type === "commodities" ? { c1: [{ field: "tags", value: "garage" }] } : undefined,
    },
  })
})

function useHandlers() {
  server.use(
    ...authHandlers.signedIn(),
    ...groupHandlers.list(),
    ...memberHandlers.list("g1"),
    ...tagHandlers.list("household", [
      { id: "t1", slug: "garage", label: "Garage", color: "blue" },
    ]),
    searchHandler
  )
}

async function openFromHeader(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByTestId("header-search"))
  return screen.findByTestId("command-palette")
}

async function typeQuery(
  user: ReturnType<typeof userEvent.setup>,
  palette: HTMLElement,
  q: string
) {
  await user.type(within(palette).getByRole("combobox"), q)
}

afterEach(() => {
  vi.restoreAllMocks()
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  clearAllRecent()
})

describe("<CommandPalette /> from the TopBar", () => {
  it("opens from the search entry, the compact button and Cmd/Ctrl+K", async () => {
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()

    const entry = await screen.findByTestId("header-search")
    expect(entry).toHaveAccessibleName("Search")
    expect(entry).toHaveAttribute("aria-keyshortcuts")

    const palette = await openFromHeader(user)
    expect(within(palette).getByRole("combobox")).toHaveFocus()
    expect(within(palette).getByTestId("palette-hint")).toHaveTextContent(
      "Type at least 2 characters to search."
    )
    await user.keyboard("{Escape}")
    await waitFor(() => expect(screen.queryByTestId("command-palette")).not.toBeInTheDocument())
    // Focus goes back to the entry that opened the palette.
    expect(entry).toHaveFocus()

    await user.click(screen.getByTestId("header-search-compact"))
    expect(await screen.findByTestId("command-palette")).toBeInTheDocument()
    await user.keyboard("{Escape}")
    await waitFor(() => expect(screen.queryByTestId("command-palette")).not.toBeInTheDocument())

    await user.keyboard("{Control>}k{/Control}")
    expect(await screen.findByTestId("command-palette")).toBeInTheDocument()
  })

  it("returns focus to the entry after a reopen during the closing animation", async () => {
    // Radix keeps a closing dialog mounted until its exit animation ends.
    // jsdom runs no animations, so the dialog content reports one while
    // `animate` is on.
    let animate = true
    const realGetComputedStyle = window.getComputedStyle.bind(window)
    vi.spyOn(window, "getComputedStyle").mockImplementation((element, pseudo) => {
      const style = realGetComputedStyle(element, pseudo)
      if (!(element instanceof HTMLElement) || element.dataset.slot !== "dialog-content") {
        return style
      }
      return new Proxy(style, {
        get(target, prop) {
          if (prop === "animationName") {
            if (!animate) return "none"
            return element.dataset.state === "closed" ? "exit" : "enter"
          }
          const value: unknown = Reflect.get(target, prop)
          return typeof value === "function" ? value.bind(target) : value
        },
      })
    })
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    const entry = await screen.findByTestId("header-search")

    await openFromHeader(user)
    await user.keyboard("{Escape}")
    // Still on screen, closing, with focus in its input.
    const closing = screen.getByTestId("command-palette")
    expect(closing).toHaveAttribute("data-state", "closed")
    expect(within(closing).getByRole("combobox")).toHaveFocus()

    await user.keyboard("{Control>}k{/Control}")
    await waitFor(() =>
      expect(screen.getByTestId("command-palette")).toHaveAttribute("data-state", "open")
    )
    await waitFor(() =>
      expect(within(screen.getByTestId("command-palette")).getByRole("combobox")).toHaveFocus()
    )

    animate = false
    await user.keyboard("{Escape}")
    await waitFor(() => expect(screen.queryByTestId("command-palette")).not.toBeInTheDocument())
    await waitFor(() => expect(entry).toHaveFocus())
  })

  it("lists the matches by kind, each opening its page", async () => {
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    const palette = await openFromHeader(user)
    await typeQuery(user, palette, "garage")

    const item = await within(palette).findByTestId("palette-result-commodity-c1")
    // The item matched on a tag, shown by its label.
    expect(item).toHaveTextContent("Hedge trimmer")
    expect(item).toHaveTextContent("Tag: Garage")
    expect(within(palette).getByTestId("palette-result-location-l1")).toHaveTextContent(
      "Main street 1"
    )
    expect(within(palette).getByTestId("palette-result-area-a1")).toBeInTheDocument()
    expect(within(palette).getByTestId("palette-result-file-f1")).toHaveTextContent(
      "Garage door manual"
    )
    expect(within(palette).getByTestId("palette-result-tag-t1")).toHaveTextContent("Item tag")

    const headings = Array.from(palette.querySelectorAll("[cmdk-group-heading]")).map(
      (el) => el.textContent
    )
    expect(headings).toEqual([
      "Search",
      "Items",
      "Locations",
      "Areas",
      "Files",
      "Tags",
      "Navigation",
    ])

    await user.click(within(palette).getByTestId("palette-result-location-l1"))
    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent("/g/household/locations/l1")
    )
    expect(screen.queryByTestId("command-palette")).not.toBeInTheDocument()
  })

  it("offers the items of a location (#2897)", async () => {
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    const palette = await openFromHeader(user)
    await typeQuery(user, palette, "garage")

    const locationItems = await within(palette).findByTestId("palette-location-items-l1")
    expect(locationItems).toHaveAccessibleName("Show items in Garage")
    await user.click(locationItems)
    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent(
        "/g/household/commodities?location=l1"
      )
    )
    expect(getRecent(recentScope("u1", "household"))).toEqual([])
  })

  // jsdom has no layout, so this guards the classes that keep a result
  // title readable at 360 px (#2897): below sm the subtitle wraps under the
  // title and the second action shows its icon only, named by aria-label.
  it("keeps the title's width on a narrow screen", async () => {
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    const palette = await openFromHeader(user)
    await typeQuery(user, palette, "garage")

    const row = await within(palette).findByTestId("palette-result-area-a1")
    const title = within(row).getByText("Garage shelf")
    expect(title.parentElement).toHaveClass("flex-col", "sm:flex-row")
    expect(within(palette).getByTestId("palette-area-items-a1-label")).toHaveClass(
      "hidden",
      "sm:inline"
    )
    expect(within(palette).getByTestId("palette-area-items-a1")).toHaveAttribute(
      "title",
      "Show items in Garage shelf"
    )
  })

  it("offers the items of an area and of a tag", async () => {
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    let palette = await openFromHeader(user)
    await typeQuery(user, palette, "garage")

    const areaItems = await within(palette).findByTestId("palette-area-items-a1")
    expect(areaItems).toHaveAccessibleName("Show items in Garage shelf")
    await user.click(areaItems)
    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent("/g/household/commodities?area=a1")
    )

    palette = await openFromHeader(user)
    await typeQuery(user, palette, "garage")
    await user.click(await within(palette).findByTestId("palette-tag-list-t1"))
    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent(
        "/g/household/commodities?inactive=1&tags=garage"
      )
    )
    // Showing a list is not opening a result: nothing is remembered.
    expect(getRecent(recentScope("u1", "household"))).toEqual([])
  })

  it("hands the query to the search page and to the items list", async () => {
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    let palette = await openFromHeader(user)
    await typeQuery(user, palette, "garage")
    // The search page row is selected first, so Enter opens it.
    await within(palette).findByTestId("palette-result-commodity-c1")
    await user.keyboard("{Enter}")
    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent("/g/household/search?q=garage")
    )

    palette = await openFromHeader(user)
    await typeQuery(user, palette, "garage")
    const allItems = await within(palette).findByTestId("palette-all-items")
    expect(allItems).toHaveTextContent("All matching items (12)")
    await user.click(allItems)
    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent(
        "/g/household/commodities?q=garage&inactive=1"
      )
    )
  })

  it("remembers an opened result and lists it in the empty palette", async () => {
    pushRecent(recentScope("u1", "household"), {
      type: "area",
      id: "a9",
      title: "Attic",
      url: "/g/household/areas/a9",
    })
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    let palette = await openFromHeader(user)
    expect(within(palette).getByTestId("palette-recent-area-a9")).toHaveTextContent("Attic")

    await typeQuery(user, palette, "garage")
    await user.click(await within(palette).findByTestId("palette-result-commodity-c1"))
    await waitFor(() =>
      expect(screen.getByTestId("location")).toHaveTextContent("/g/household/commodities/c1")
    )

    palette = await openFromHeader(user)
    const recent = within(palette).getByTestId("palette-group-recent")
    const rows = within(recent).getAllByRole("option")
    expect(rows.map((row) => row.textContent)).toEqual(["Hedge trimmerItem", "AtticArea"])
  })

  // #3023: someone else who signed in on this browser before, in the same
  // group, keeps what they opened to themselves.
  it("keeps another user's recent items to that user", async () => {
    const other = recentScope("u2", "household")
    pushRecent(other, { type: "area", id: "a9", title: "Attic", url: "/g/household/areas/a9" })
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    const palette = await openFromHeader(user)
    expect(within(palette).queryByTestId("palette-recent-area-a9")).not.toBeInTheDocument()

    await typeQuery(user, palette, "garage")
    await user.click(await within(palette).findByTestId("palette-result-commodity-c1"))
    await waitFor(() =>
      expect(getRecent(recentScope("u1", "household")).map((e) => e.id)).toEqual(["c1"])
    )
    expect(getRecent(other).map((e) => e.id)).toEqual(["a9"])
  })

  it("says when nothing matches", async () => {
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    const palette = await openFromHeader(user)
    await typeQuery(user, palette, "zzzz")
    expect(await within(palette).findByTestId("palette-no-results")).toHaveTextContent(
      'No results for "zzzz".'
    )
  })

  it("has no axe violations with results on screen", async () => {
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    const palette = await openFromHeader(user)
    await typeQuery(user, palette, "garage")
    await within(palette).findByTestId("palette-result-tag-t1")
    expect(await axe(palette)).toHaveNoViolations()
  })
})

describe("<CommandPalette /> in a group that hides sections (#1607)", () => {
  function navigationOptions(palette: HTMLElement): string[] {
    return within(palette)
      .getAllByRole("option")
      .map((option) => option.textContent ?? "")
  }

  it("offers Warranties while the group shows them", async () => {
    useHandlers()
    const user = userEvent.setup()
    renderTopBar()
    const palette = await openFromHeader(user)
    expect(navigationOptions(palette)).toContain("Warranties")
  })

  it("leaves Warranties out while the group hides them", async () => {
    useHandlers()
    server.use(
      ...groupHandlers.list([
        { id: "g1", slug: "household", name: "Household", hidden_sections: ["warranties"] },
        { id: "g2", slug: "office", name: "Office" },
      ])
    )
    const user = userEvent.setup()
    renderTopBar()
    const palette = await openFromHeader(user)
    const options = navigationOptions(palette)
    expect(options).toContain("Tags")
    expect(options).not.toContain("Warranties")
  })
})
