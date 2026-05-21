import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { AuthProvider } from "@/features/auth/AuthContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { AdminGroupsPage } from "@/pages/admin/AdminGroupsPage"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const adminUser = {
  id: "u1",
  email: "admin@example.com",
  name: "Admin",
  is_system_admin: true,
}

interface SeenRequest {
  q: string | null
  tenantID: string | null
  status: string | null
  sort: string | null
  order: string | null
  page: string | null
}

// Records the query params of every GET /admin/groups request and answers
// with a single-group page whose name echoes the search term — enough to
// assert search / sort / filters / pagination drive the server.
function seedGroups(seen: SeenRequest[], opts: { total?: number } = {}) {
  server.use(
    http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
    http.get(api("/admin/tenants"), () =>
      HttpResponse.json({
        data: [
          { id: "t1", name: "Acme Inc", slug: "acme", status: "active" },
          { id: "t2", name: "Globex", slug: "globex", status: "active" },
        ],
        meta: { total: 2, page: 1, per_page: 100, total_pages: 1 },
      })
    ),
    http.get(api("/admin/groups"), ({ request }) => {
      const url = new URL(request.url)
      const q = url.searchParams.get("q")
      seen.push({
        q,
        tenantID: url.searchParams.get("tenantID"),
        status: url.searchParams.get("status"),
        sort: url.searchParams.get("sort"),
        order: url.searchParams.get("order"),
        page: url.searchParams.get("page"),
      })
      return HttpResponse.json({
        data: [
          {
            id: "g1",
            name: q ? `Match ${q}` : "HQ Inventory",
            slug: "hq",
            currency: "USD",
            status: "active",
            member_count: 4,
            created_by: "owner@acme.example.com",
            created_at: "2024-01-10T00:00:00Z",
            tenant_id: "t1",
            tenant: { id: "t1", name: "Acme Inc", slug: "acme" },
          },
        ],
        meta: { total: opts.total ?? 1, page: 1, per_page: 20, total_pages: opts.total ? 3 : 1 },
      })
    })
  )
}

function LocationProbe() {
  const location = useLocation()
  return (
    <>
      <div data-testid="location-search">{location.search}</div>
      <div data-testid="location-pathname">{location.pathname}</div>
    </>
  )
}

function locationSearch(): URLSearchParams {
  return new URLSearchParams(screen.getByTestId("location-search").textContent ?? "")
}

function renderPage(initialPath = "/admin/groups") {
  return renderWithProviders({
    initialPath,
    routes: (
      <>
        <Route
          path="/admin/groups"
          element={
            <AuthProvider>
              <main>
                <AdminGroupsPage />
                <LocationProbe />
              </main>
            </AuthProvider>
          }
        />
        {/* The group-detail route is a stub here — the click navigation
            test only asserts the destination pathname. */}
        <Route
          path="/admin/groups/:groupId"
          element={
            <main>
              <div data-testid="group-detail-stub" />
              <LocationProbe />
            </main>
          }
        />
      </>
    ),
  })
}

describe("AdminGroupsPage", () => {
  beforeAll(async () => {
    await initI18n({ lng: "en" })
  })

  beforeEach(() => {
    clearAuth()
    __resetGroupContextForTests()
    __resetHttpForTests()
    setAccessToken("good-token")
  })

  it("renders the groups table with a row per group", async () => {
    seedGroups([])
    renderPage()

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())
    expect(screen.getByTestId("admin-groups-list")).toBeInTheDocument()
    expect(screen.getAllByTestId("admin-group-row")).toHaveLength(1)
  })

  it("sends the typed search term to the server as ?q and persists it in the URL", async () => {
    const seen: SeenRequest[] = []
    seedGroups(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())
    expect(seen[0].q).toBeNull()

    await userEvent.type(screen.getByTestId("admin-groups-search"), "warehouse")

    await waitFor(() => expect(seen.some((r) => r.q === "warehouse")).toBe(true))
    await waitFor(() => expect(screen.getByText("Match warehouse")).toBeInTheDocument())
    await waitFor(() => expect(locationSearch().get("q")).toBe("warehouse"))
  })

  it("clears the search via the clear button, dropping ?q and ?page from the URL", async () => {
    const seen: SeenRequest[] = []
    seedGroups(seen, { total: 50 })
    renderPage("/admin/groups?page=2")

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())

    await userEvent.type(screen.getByTestId("admin-groups-search"), "warehouse")
    await waitFor(() => expect(locationSearch().get("q")).toBe("warehouse"))
    // Setting ?q resets to page 1, so ?page is already gone here.
    await waitFor(() => expect(locationSearch().get("page")).toBeNull())

    await userEvent.click(screen.getByLabelText("Clear search"))

    await waitFor(() => expect(locationSearch().get("q")).toBeNull())
    expect(locationSearch().get("page")).toBeNull()
    expect(screen.getByTestId("admin-groups-search")).toHaveValue("")
  })

  it("seeds the search box from a deep-linked ?q param", async () => {
    const seen: SeenRequest[] = []
    seedGroups(seen)
    renderPage("/admin/groups?q=storage")

    await waitFor(() => expect(screen.getByText("Match storage")).toBeInTheDocument())
    expect(screen.getByTestId("admin-groups-search")).toHaveValue("storage")
    expect(seen[0].q).toBe("storage")
  })

  it("toggles sort order on a sortable header and writes it to the URL", async () => {
    const seen: SeenRequest[] = []
    seedGroups(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())
    expect(seen[0].sort).toBe("name")
    expect(seen[0].order).toBe("asc")

    await userEvent.click(screen.getByRole("button", { name: "Group" }))
    await waitFor(() => expect(seen.some((r) => r.order === "desc")).toBe(true))
    await waitFor(() => {
      const params = locationSearch()
      expect(params.get("sort")).toBe("name")
      expect(params.get("order")).toBe("desc")
    })
  })

  it("applies the status filter as ?status and persists it", async () => {
    const seen: SeenRequest[] = []
    seedGroups(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())
    expect(seen[0].status).toBeNull()

    await userEvent.click(screen.getByTestId("admin-groups-status-filter"))
    await userEvent.click(screen.getByRole("option", { name: "Pending deletion" }))

    await waitFor(() => expect(seen.some((r) => r.status === "pending_deletion")).toBe(true))
    await waitFor(() => expect(locationSearch().get("status")).toBe("pending_deletion"))
  })

  it("applies the tenant filter as ?tenantID and persists it", async () => {
    const seen: SeenRequest[] = []
    seedGroups(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())
    expect(seen[0].tenantID).toBeNull()

    await userEvent.click(screen.getByTestId("admin-groups-tenant-filter"))
    await userEvent.click(screen.getByRole("option", { name: "Globex" }))

    await waitFor(() => expect(seen.some((r) => r.tenantID === "t2")).toBe(true))
    await waitFor(() => expect(locationSearch().get("tenantID")).toBe("t2"))
  })

  it("paginates and persists the page in the URL", async () => {
    const seen: SeenRequest[] = []
    seedGroups(seen, { total: 50 })
    renderPage()

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())

    const pager = screen.getByTestId("admin-pagination")
    await userEvent.click(within(pager).getByLabelText("Next page"))

    await waitFor(() => expect(seen.some((r) => r.page === "2")).toBe(true))
    await waitFor(() => expect(locationSearch().get("page")).toBe("2"))
  })

  it("recovers from a deep link to an out-of-range page by snapping to the last page", async () => {
    const seen: SeenRequest[] = []
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/tenants"), () =>
        HttpResponse.json({ data: [], meta: { total: 0, page: 1, per_page: 100, total_pages: 1 } })
      ),
      http.get(api("/admin/groups"), ({ request }) => {
        const url = new URL(request.url)
        const reqPage = url.searchParams.get("page")
        seen.push({
          q: url.searchParams.get("q"),
          tenantID: url.searchParams.get("tenantID"),
          status: url.searchParams.get("status"),
          sort: url.searchParams.get("sort"),
          order: url.searchParams.get("order"),
          page: reqPage,
        })
        const data =
          reqPage && Number(reqPage) > 2
            ? []
            : [
                {
                  id: "g1",
                  name: "HQ Inventory",
                  slug: "hq",
                  currency: "USD",
                  status: "active",
                  member_count: 4,
                  tenant: { id: "t1", name: "Acme Inc", slug: "acme" },
                },
              ]
        return HttpResponse.json({
          data,
          meta: { total: 40, page: Number(reqPage ?? 1), per_page: 20, total_pages: 2 },
        })
      })
    )
    renderPage("/admin/groups?page=5")

    await waitFor(() => expect(locationSearch().get("page")).toBe("2"))
    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())
    await waitFor(() => expect(seen.some((r) => r.page === "2")).toBe(true))
  })

  it("navigates to the group detail when a row is clicked", async () => {
    seedGroups([])
    renderPage()

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-group-row"))

    await waitFor(() =>
      expect(screen.getByTestId("location-pathname")).toHaveTextContent("/admin/groups/g1")
    )
  })

  it("activates a group row from the keyboard via Enter", async () => {
    seedGroups([])
    renderPage()

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())

    const row = screen.getByTestId("admin-group-row")
    expect(row).toHaveAttribute("role", "button")
    expect(row).toHaveAttribute("tabindex", "0")
    row.focus()
    await userEvent.keyboard("{Enter}")
    await waitFor(() =>
      expect(screen.getByTestId("location-pathname")).toHaveTextContent("/admin/groups/g1")
    )
  })

  it("renders the empty state when no groups match", async () => {
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/tenants"), () =>
        HttpResponse.json({ data: [], meta: { total: 0, page: 1, per_page: 100, total_pages: 1 } })
      ),
      http.get(api("/admin/groups"), () =>
        HttpResponse.json({ data: [], meta: { total: 0, page: 1, per_page: 20, total_pages: 1 } })
      )
    )
    renderPage()

    await waitFor(() =>
      expect(screen.getByText("No groups match your filters.")).toBeInTheDocument()
    )
  })

  it("renders the error state when the request fails", async () => {
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/tenants"), () =>
        HttpResponse.json({ data: [], meta: { total: 0, page: 1, per_page: 100, total_pages: 1 } })
      ),
      http.get(api("/admin/groups"), () => HttpResponse.json({ errors: [] }, { status: 500 }))
    )
    renderPage()

    await waitFor(() =>
      expect(screen.getByText("Could not load groups. Please try again.")).toBeInTheDocument()
    )
  })
})
