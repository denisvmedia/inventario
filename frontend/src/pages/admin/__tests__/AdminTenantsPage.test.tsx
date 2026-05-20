import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
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

// Records the `q` query param of every GET /admin/tenants request and
// answers with a single-tenant page whose name echoes the search term —
// enough to assert the search box drives the server, not a client filter.
function seedTenants(seen: Array<string | null>) {
  server.use(
    http.get(api("/auth/me"), () => HttpResponse.json(adminUser)),
    http.get(api("/admin/tenants"), ({ request }) => {
      const q = new URL(request.url).searchParams.get("q")
      seen.push(q)
      return HttpResponse.json({
        data: [
          {
            id: "t1",
            name: q ? `Match ${q}` : "Acme Inc",
            slug: "acme",
            status: "active",
            user_count: 3,
            group_count: 1,
          },
        ],
        meta: { total: 1, page: 1, per_page: 20 },
      })
    })
  )
}

function renderPage() {
  return renderWithProviders({
    initialPath: "/admin/tenants",
    routes: (
      <Route
        path="/admin/tenants"
        element={
          <AuthProvider>
            <main>
              <AdminTenantsPage />
            </main>
          </AuthProvider>
        }
      />
    ),
  })
}

describe("AdminTenantsPage", () => {
  it("sends the typed search term to the server as ?q", async () => {
    const seen: Array<string | null> = []
    seedTenants(seen)
    renderPage()

    // Initial load fires with no `q`.
    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
    expect(seen[0]).toBeNull()

    await userEvent.type(screen.getByTestId("admin-tenants-search"), "globex")

    // The debounced value reaches the server and drives a fresh request.
    await waitFor(() => expect(seen).toContain("globex"))
    await waitFor(() => expect(screen.getByText("Match globex")).toBeInTheDocument())
  })

  it("clears the search and refetches the unfiltered list", async () => {
    const seen: Array<string | null> = []
    seedTenants(seen)
    renderPage()

    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
    await userEvent.type(screen.getByTestId("admin-tenants-search"), "globex")
    await waitFor(() => expect(screen.getByText("Match globex")).toBeInTheDocument())

    await userEvent.click(screen.getByLabelText("Clear search"))
    await waitFor(() => expect(screen.getByText("Acme Inc")).toBeInTheDocument())
  })
})
