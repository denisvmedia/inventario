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
import { AdminTenantsPage } from "@/pages/admin/AdminTenantsPage"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const adminUser = {
  id: "u1",
  email: "admin@example.com",
  name: "Admin",
  is_system_admin: true,
}

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

interface SeenRequest {
  q: string | null
  sort: string | null
  order: string | null
  page: string | null
}

// Records the query params of every GET /admin/tenants request and answers
// with a single-tenant page whose name echoes the search term — enough to
// assert search / sort / pagination drive the server, not a client filter.
function seedTenants(seen: SeenRequest[], opts: { total?: number } = {}) {
  server.use(
    http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
    http.get(api("/admin/tenants"), ({ request }) => {
      const url = new URL(request.url)
      const q = url.searchParams.get("q")
      seen.push({
        q,
        sort: url.searchParams.get("sort"),
        order: url.searchParams.get("order"),
        page: url.searchParams.get("page"),
      })
      return HttpResponse.json({
        data: [
          {
            id: "t1",
            name: q ? `Match ${q}` : "Acme Inc",
            slug: "acme",
            domain: "acme.example.com",
            status: "active",
            user_count: 3,
            group_count: 1,
          },
        ],
        meta: { total: opts.total ?? 1, page: 1, per_page: 20, total_pages: opts.total ? 3 : 1 },
      })
    })
  )
}

// Surfaces the live router search string + pathname into the DOM so
// tests can assert that sort / page / search state round-trips through
// the URL and that row activation navigates.
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

function renderPage(initialPath = "/admin/tenants") {
  return renderWithProviders({
    initialPath,
    routes: (
      <>
        <Route
          path="/admin/tenants"
          element={
            <AuthProvider>
              <main>
                <AdminTenantsPage />
                <LocationProbe />
              </main>
            </AuthProvider>
          }
        />
        {/* The tenant-detail route is a stub here — the click/keyboard
            navigation tests only assert the destination pathname. */}
        <Route
          path="/admin/tenants/:tenantId"
          element={
            <main>
              <div data-testid="tenant-detail-stub" />
              <LocationProbe />
            </main>
          }
        />
      </>
    ),
  })
}

describe("AdminTenantsPage", () => {
  it("renders the tenant table with a row per tenant", async () => {
    seedTenants([])
    renderPage()

    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
    expect(screen.getByTestId("admin-tenants-list")).toBeInTheDocument()
    expect(screen.getAllByTestId("admin-tenant-row")).toHaveLength(1)
  })

  it("sends the typed search term to the server as ?q and persists it in the URL", async () => {
    const seen: SeenRequest[] = []
    seedTenants(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
    expect(seen[0].q).toBeNull()

    await userEvent.type(screen.getByTestId("admin-tenants-search"), "globex")

    await waitFor(() => expect(seen.some((r) => r.q === "globex")).toBe(true))
    await waitFor(() => expect(screen.getByText("Match globex")).toBeInTheDocument())
    // Search round-trips through the URL so a copied link reproduces it.
    await waitFor(() => expect(locationSearch().get("q")).toBe("globex"))
  })

  it("clears the search and refetches the unfiltered list", async () => {
    const seen: SeenRequest[] = []
    seedTenants(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
    await userEvent.type(screen.getByTestId("admin-tenants-search"), "globex")
    await waitFor(() => expect(screen.getByText("Match globex")).toBeInTheDocument())

    await userEvent.click(screen.getByLabelText("Clear search"))
    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
  })

  it("seeds the search box from a deep-linked ?q param", async () => {
    const seen: SeenRequest[] = []
    seedTenants(seen)
    renderPage("/admin/tenants?q=northwind")

    await waitFor(() => expect(screen.getByText("Match northwind")).toBeInTheDocument())
    expect(screen.getByTestId("admin-tenants-search")).toHaveValue("northwind")
    expect(seen[0].q).toBe("northwind")
  })

  it("toggles sort order on a sortable header and writes it to the URL", async () => {
    const seen: SeenRequest[] = []
    seedTenants(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
    // Default sort is name/asc.
    expect(seen[0].sort).toBe("name")
    expect(seen[0].order).toBe("asc")

    // Clicking the active "Name" header flips to desc.
    await userEvent.click(screen.getByRole("button", { name: "Name" }))
    await waitFor(() => expect(seen.some((r) => r.order === "desc")).toBe(true))
    await waitFor(() => {
      const params = locationSearch()
      expect(params.get("sort")).toBe("name")
      expect(params.get("order")).toBe("desc")
    })
  })

  it("paginates and persists the page in the URL", async () => {
    const seen: SeenRequest[] = []
    seedTenants(seen, { total: 50 })
    renderPage()

    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())

    const pager = screen.getByTestId("admin-pagination")
    await userEvent.click(within(pager).getByLabelText("Next page"))

    await waitFor(() => expect(seen.some((r) => r.page === "2")).toBe(true))
    await waitFor(() => expect(locationSearch().get("page")).toBe("2"))
  })

  it("renders the error state when the request fails", async () => {
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/tenants"), () => HttpResponse.json({ errors: [] }, { status: 500 }))
    )
    renderPage()

    await waitFor(() =>
      expect(screen.getByText("Could not load tenants. Please try again.")).toBeInTheDocument()
    )
  })

  it("recovers from a deep link to an out-of-range page by snapping to the last page", async () => {
    const seen: SeenRequest[] = []
    // The server reports only 2 total pages regardless of the requested
    // page — a deep link to ?page=5 is out of range.
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/tenants"), ({ request }) => {
        const url = new URL(request.url)
        const reqPage = url.searchParams.get("page")
        seen.push({
          q: url.searchParams.get("q"),
          sort: url.searchParams.get("sort"),
          order: url.searchParams.get("order"),
          page: reqPage,
        })
        // page beyond total_pages → server returns an empty data array.
        const data =
          reqPage && Number(reqPage) > 2
            ? []
            : [
                {
                  id: "t1",
                  name: "Acme Inc",
                  slug: "acme",
                  domain: "acme.example.com",
                  status: "active",
                  user_count: 3,
                  group_count: 1,
                },
              ]
        return HttpResponse.json({
          data,
          meta: { total: 40, page: Number(reqPage ?? 1), per_page: 20, total_pages: 2 },
        })
      })
    )
    renderPage("/admin/tenants?page=5")

    // The page clamps back to the last real page (2) and the user lands
    // on data, not a stranded empty state.
    await waitFor(() => expect(locationSearch().get("page")).toBe("2"))
    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
    await waitFor(() => expect(seen.some((r) => r.page === "2")).toBe(true))
  })

  it("navigates to the tenant detail when a row is clicked", async () => {
    seedTenants([])
    renderPage()

    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
    await userEvent.click(screen.getByTestId("admin-tenant-row"))

    await waitFor(() =>
      expect(screen.getByTestId("location-pathname")).toHaveTextContent("/admin/tenants/t1")
    )
  })

  it("activates a tenant row from the keyboard via Enter and Space", async () => {
    seedTenants([])
    renderPage()

    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())

    // The row is focusable and exposes a button role for assistive tech.
    const row = screen.getByTestId("admin-tenant-row")
    expect(row).toHaveAttribute("role", "button")
    expect(row).toHaveAttribute("tabindex", "0")

    // Enter on the focused row drills into the detail route.
    row.focus()
    expect(row).toHaveFocus()
    await userEvent.keyboard("{Enter}")
    await waitFor(() =>
      expect(screen.getByTestId("location-pathname")).toHaveTextContent("/admin/tenants/t1")
    )
  })

  it("activates a tenant row with the Space key", async () => {
    seedTenants([])
    renderPage()

    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())

    const row = screen.getByTestId("admin-tenant-row")
    row.focus()
    await userEvent.keyboard("[Space]")
    await waitFor(() =>
      expect(screen.getByTestId("location-pathname")).toHaveTextContent("/admin/tenants/t1")
    )
  })

  it("renders the empty state when no tenants match", async () => {
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/tenants"), () =>
        HttpResponse.json({ data: [], meta: { total: 0, page: 1, per_page: 20, total_pages: 1 } })
      )
    )
    renderPage()

    await waitFor(() =>
      expect(screen.getByText("No tenants match your search.")).toBeInTheDocument()
    )
  })
})
