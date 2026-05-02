import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { SearchPage } from "@/pages/SearchPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearRecent, pushRecent } from "@/features/search/recent"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const groupsHandler = msw.get(api("/groups"), () =>
  HttpResponse.json({
    data: [
      {
        id: "g1",
        type: "groups",
        attributes: { id: "g1", slug: "household", name: "Household" },
      },
    ],
  })
)

const userHandler = msw.get(api("/auth/me"), () =>
  HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
)

function renderSearch(initial = "/g/household/search") {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: initial,
    routes: (
      <Route
        path="/g/:groupSlug/search"
        element={
          <AuthProvider>
            <GroupProvider>
              <SearchPage />
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  clearRecent("household")
  clearRecent("default")
})

describe("<SearchPage />", () => {
  it("shows the empty state with hints when no query is set", async () => {
    server.use(groupsHandler, userHandler)
    renderSearch()
    expect(await screen.findByTestId("search-empty")).toBeInTheDocument()
    expect(screen.getByTestId("recent-empty")).toBeInTheDocument()
  })

  it("renders recent items when present", async () => {
    pushRecent("household", {
      type: "commodity",
      id: "c1",
      title: "Cordless drill",
      url: "/g/household/commodities/c1",
    })
    pushRecent("household", {
      type: "location",
      id: "l1",
      title: "Garage",
      url: "/g/household/locations/l1",
    })
    server.use(groupsHandler, userHandler)
    renderSearch()
    await waitFor(() => expect(screen.getByTestId("recent-list")).toBeInTheDocument())
    expect(screen.getByTestId("recent-commodity-c1")).toHaveTextContent(/cordless drill/i)
    expect(screen.getByTestId("recent-location-l1")).toHaveTextContent(/garage/i)
  })

  it("submits a query into the URL and renders grouped results", async () => {
    server.use(
      groupsHandler,
      userHandler,
      msw.get(api("/g/household/search"), ({ request }) => {
        const url = new URL(request.url)
        const type = url.searchParams.get("type")
        if (type === "commodities") {
          return HttpResponse.json({
            data: [{ id: "c1", type: "commodities", attributes: { id: "c1", name: "Drill" } }],
            meta: { total: 1, entity_type: "commodities", query: "drill" },
          })
        }
        // Other types return 0 to keep the assertion simple.
        return HttpResponse.json({ data: [], meta: { total: 0, entity_type: type } })
      })
    )
    const user = userEvent.setup()
    renderSearch()
    const input = await screen.findByTestId("search-input")
    await user.type(input, "drill")
    await user.click(screen.getByTestId("search-submit"))
    expect(await screen.findByTestId("result-commodity-c1")).toHaveTextContent(/drill/i)
    // Empty groups render their per-group empty body.
    await waitFor(() => expect(screen.getByTestId("group-locations-empty")).toBeInTheDocument())
  })

  it("renders the no-results card when every section is empty", async () => {
    server.use(
      groupsHandler,
      userHandler,
      msw.get(api("/g/household/search"), () => HttpResponse.json({ data: [], meta: { total: 0 } }))
    )
    const user = userEvent.setup()
    renderSearch()
    await user.type(await screen.findByTestId("search-input"), "qwerty")
    await user.click(screen.getByTestId("search-submit"))
    expect(await screen.findByTestId("search-no-results")).toHaveTextContent(/qwerty/i)
  })

  it("renders the unavailable stub for tags regardless of query", async () => {
    server.use(
      groupsHandler,
      userHandler,
      msw.get(api("/g/household/search"), () => HttpResponse.json({ data: [], meta: { total: 0 } }))
    )
    const user = userEvent.setup()
    renderSearch()
    await user.type(await screen.findByTestId("search-input"), "anything")
    await user.click(screen.getByTestId("search-submit"))
    // Tags is BE-blocked on #1400 — always rendered as a stub.
    expect(await screen.findByTestId("group-tags")).toHaveTextContent(/coming|tracked under/i)
  })

  it("renders the files unavailable stub when the BE returns 501 for that type", async () => {
    server.use(
      groupsHandler,
      userHandler,
      msw.get(api("/g/household/search"), ({ request }) => {
        const url = new URL(request.url)
        const type = url.searchParams.get("type")
        if (type === "files") {
          // Files search ships behind #1398; the BE responds 501 today.
          // The api wrapper folds that into `unavailable: true` so the
          // page renders the stub instead of leaking the transport error.
          return HttpResponse.json({ message: "not implemented" }, { status: 501 })
        }
        return HttpResponse.json({
          data: [{ id: "c1", type: "commodities", attributes: { id: "c1", name: "Drill" } }],
          meta: { total: 1, entity_type: type, query: "drill" },
        })
      })
    )
    const user = userEvent.setup()
    renderSearch()
    await user.type(await screen.findByTestId("search-input"), "drill")
    await user.click(screen.getByTestId("search-submit"))
    // Wait for the commodities fetch to land so we know the page has
    // moved past initial load — the files section's 501 has resolved by
    // then too (both queries fire in parallel from the same effect).
    await screen.findByTestId("result-commodity-c1")
    const filesGroup = screen.getByTestId("group-files")
    expect(filesGroup).toHaveTextContent(/coming|tracked under|#1398/i)
    // No misleading "0 matches" body for an unavailable section.
    expect(filesGroup.querySelector('[data-testid="group-files-empty"]')).toBeNull()
    // Still shows the "no results" page-level card? It must NOT — files
    // is unavailable (excluded from the count), commodities has 1, the
    // others returned [] (genuinely 0). With usable sections present,
    // `allEmpty` is false because commodities total > 0.
    expect(screen.queryByTestId("search-no-results")).toBeNull()
  })

  it("clears the query via the X button", async () => {
    server.use(groupsHandler, userHandler)
    const user = userEvent.setup()
    renderSearch("/g/household/search?q=drill")
    const input = (await screen.findByTestId("search-input")) as HTMLInputElement
    await waitFor(() => expect(input).toHaveValue("drill"))
    await user.click(screen.getByTestId("search-clear"))
    await waitFor(() => expect(input).toHaveValue(""))
    expect(screen.getByTestId("search-empty")).toBeInTheDocument()
  })

  it("has no axe violations on the empty state", async () => {
    server.use(groupsHandler, userHandler)
    const { container } = renderSearch()
    await waitFor(() => expect(screen.getByTestId("search-empty")).toBeInTheDocument())
    expect(await axe(container)).toHaveNoViolations()
  })
})
