import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { AuthProvider } from "@/features/auth/AuthContext"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { AdminTenantDetailPage } from "@/pages/admin/AdminTenantDetailPage"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

const adminUser = {
  id: "u1",
  email: "admin@example.com",
  name: "Admin",
  is_system_admin: true,
}

const tenant = {
  id: "t1",
  name: "Northwind Estates",
  slug: "northwind",
  domain: "northwind.example.com",
  status: "active",
  plan_id: "business",
  user_count: 2,
  group_count: 1,
  created_at: "2023-02-11T00:00:00Z",
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

interface Seen {
  usersQ: string | null
  usersActive: string | null
  groupsTenantID: string | null
  groupsStatus: string | null
}

// Seeds the tenant detail + Users + Groups endpoints and records the
// query params each tab's request carried.
function seedDetail(seen: Seen, opts: { tenantStatus?: number } = {}) {
  server.use(
    http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
    http.get(api("/admin/tenants/t1"), () => {
      if (opts.tenantStatus && opts.tenantStatus !== 200) {
        return HttpResponse.json({ errors: [] }, { status: opts.tenantStatus })
      }
      return HttpResponse.json({ data: tenant })
    }),
    http.get(api("/admin/tenants/t1/users"), ({ request }) => {
      const url = new URL(request.url)
      seen.usersQ = url.searchParams.get("q")
      seen.usersActive = url.searchParams.get("is_active")
      return HttpResponse.json({
        data: [
          {
            id: "user-1",
            name: "Ada Lovelace",
            email: "ada@northwind.example.com",
            is_active: true,
            group_membership_count: 2,
            last_login_at: "2026-05-01T10:00:00Z",
          },
        ],
        meta: { total: 1, page: 1, per_page: 20, total_pages: 1 },
      })
    }),
    http.get(api("/admin/groups"), ({ request }) => {
      const url = new URL(request.url)
      seen.groupsTenantID = url.searchParams.get("tenantID")
      seen.groupsStatus = url.searchParams.get("status")
      return HttpResponse.json({
        data: [
          {
            id: "group-1",
            name: "HQ Inventory",
            slug: "hq",
            status: "active",
            currency: "USD",
            member_count: 4,
          },
        ],
        meta: { total: 1, page: 1, per_page: 20, total_pages: 1 },
      })
    })
  )
}

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location-search">{location.search}</div>
}

function renderPage(initialPath = "/admin/tenants/t1") {
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/admin/tenants/:tenantId"
        element={
          <AuthProvider>
            <main>
              <AdminTenantDetailPage />
              <LocationProbe />
            </main>
          </AuthProvider>
        }
      />
    ),
  })
}

describe("AdminTenantDetailPage", () => {
  it("renders the tenant header and the Users tab by default", async () => {
    const seen = {} as Seen
    seedDetail(seen)
    renderPage()

    await waitFor(() => expect(screen.getByTestId("admin-tenant-header")).toBeInTheDocument())
    expect(screen.getByRole("heading", { name: "Northwind Estates" })).toBeInTheDocument()
    expect(screen.getByText("northwind")).toBeInTheDocument()
    expect(screen.getByText("northwind.example.com")).toBeInTheDocument()
    expect(screen.getByText("business")).toBeInTheDocument()

    // Users tab is active by default — its row renders, scoped to t1.
    await waitFor(() => expect(screen.getByText("Ada Lovelace")).toBeInTheDocument())
    expect(screen.getByTestId("admin-tenant-users-table")).toBeInTheDocument()
  })

  it("switches to the Groups tab, persists ?tab, and scopes the request to the tenant", async () => {
    const seen = {} as Seen
    seedDetail(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("Ada Lovelace")).toBeInTheDocument())

    await userEvent.click(screen.getByTestId("admin-tenant-tab-groups"))

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())
    expect(seen.groupsTenantID).toBe("t1")
    await waitFor(() =>
      expect(
        new URLSearchParams(screen.getByTestId("location-search").textContent ?? "").get("tab")
      ).toBe("groups")
    )
  })

  it("lands on the Groups tab from a deep-linked ?tab=groups", async () => {
    const seen = {} as Seen
    seedDetail(seen)
    renderPage("/admin/tenants/t1?tab=groups")

    await waitFor(() => expect(screen.getByText("HQ Inventory")).toBeInTheDocument())
  })

  it("applies the Users isActive filter as ?is_active and persists it", async () => {
    const seen = {} as Seen
    seedDetail(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("Ada Lovelace")).toBeInTheDocument())
    expect(seen.usersActive).toBeNull()

    await userEvent.click(screen.getByTestId("admin-tenant-users-filter"))
    await userEvent.click(screen.getByRole("option", { name: "Blocked only" }))

    await waitFor(() => expect(seen.usersActive).toBe("false"))
    await waitFor(() =>
      expect(
        new URLSearchParams(screen.getByTestId("location-search").textContent ?? "").get("active")
      ).toBe("false")
    )
  })

  it("sends the Users search term to the server as ?q", async () => {
    const seen = {} as Seen
    seedDetail(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("Ada Lovelace")).toBeInTheDocument())
    await userEvent.type(screen.getByTestId("admin-tenant-users-search"), "ada")

    await waitFor(() => expect(seen.usersQ).toBe("ada"))
  })

  it("renders the error state when the tenant request fails", async () => {
    const seen = {} as Seen
    seedDetail(seen, { tenantStatus: 500 })
    renderPage()

    await waitFor(() =>
      expect(screen.getByText("Could not load this tenant. Please try again.")).toBeInTheDocument()
    )
  })

  it("renders the not-found state when the tenant request 404s", async () => {
    // The BE returns HTTP 404 for a missing tenant (registry.ErrNotFound
    // → NewNotFoundError) — a genuine not-found surfaces as a query
    // error, and the page must show the friendly not-found copy, not the
    // generic "Could not load" error card.
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/tenants/t1"), () => HttpResponse.json({ errors: [] }, { status: 404 }))
    )
    renderPage()

    await waitFor(() => expect(screen.getByText("Tenant not found.")).toBeInTheDocument())
    expect(
      screen.queryByText("Could not load this tenant. Please try again.")
    ).not.toBeInTheDocument()
  })

  it("renders the generic error state for a malformed 200-with-empty-body tenant response", async () => {
    // `getAdminTenant` fails fast when a 200 carries no `data` payload —
    // a malformed response is an error, not a not-found. The page shows
    // the generic load-error card, not the friendly not-found copy.
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/tenants/t1"), () => HttpResponse.json({}))
    )
    renderPage()

    await waitFor(() =>
      expect(screen.getByText("Could not load this tenant. Please try again.")).toBeInTheDocument()
    )
    expect(screen.queryByText("Tenant not found.")).not.toBeInTheDocument()
  })

  it("recovers from a deep link to an out-of-range Users-tab page", async () => {
    let lastUsersPage: string | null = null
    server.use(
      http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
      http.get(api("/admin/tenants/t1"), () => HttpResponse.json({ data: tenant })),
      http.get(api("/admin/tenants/t1/users"), ({ request }) => {
        const url = new URL(request.url)
        const reqPage = url.searchParams.get("page")
        lastUsersPage = reqPage
        const data =
          reqPage && Number(reqPage) > 2
            ? []
            : [
                {
                  id: "user-1",
                  name: "Ada Lovelace",
                  email: "ada@northwind.example.com",
                  is_active: true,
                  group_membership_count: 2,
                  last_login_at: "2026-05-01T10:00:00Z",
                },
              ]
        return HttpResponse.json({
          data,
          meta: { total: 40, page: Number(reqPage ?? 1), per_page: 20, total_pages: 2 },
        })
      })
    )
    renderPage("/admin/tenants/t1?page=5")

    // The Users tab clamps ?page back to the last real page (2).
    await waitFor(() =>
      expect(
        new URLSearchParams(screen.getByTestId("location-search").textContent ?? "").get("page")
      ).toBe("2")
    )
    await waitFor(() => expect(screen.getByText("Ada Lovelace")).toBeInTheDocument())
    await waitFor(() => expect(lastUsersPage).toBe("2"))
  })
})
