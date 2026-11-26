import type { ReactNode } from "react"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { AuthProvider } from "@/features/auth/AuthContext"
import { BackofficeAuthProvider } from "@/features/backoffice/auth/context"
import { clearBackofficeAuth, setBackofficeAccessToken } from "@/features/backoffice/auth/storage"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { AdminTenantDetailPage } from "@/pages/admin/AdminTenantDetailPage"
import { AdminTenantsPage } from "@/pages/admin/AdminTenantsPage"
import { backofficeAuthHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

// The tenant actions of #1037: create on the Tenants page; edit, suspend,
// reactivate and delete on the tenant detail page. All of them are for a
// platform admin only.

const api = (path: string) => `${window.location.origin}/api/v1${path}`

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

type TenantRow = typeof tenant

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  clearBackofficeAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setBackofficeAccessToken("good-bo-token")
})

function signInAs(role: "platform_admin" | "support_agent") {
  server.use(...backofficeAuthHandlers.signedIn({ user: { role } }))
}

// A 4xx in the BE's error envelope, with a typed code.
function codedError(status: number, code: string) {
  return HttpResponse.json(
    { errors: [{ status: "Error", code, error: { error: { message: code } } }] },
    { status }
  )
}

// Seeds the read endpoints both pages need. The tenant row is held in a
// box so a test can change what the detail endpoint returns.
function seedReads(box: { tenant: TenantRow }) {
  server.use(
    http.get(api("/auth/me"), () =>
      HttpResponse.json({ id: "u1", email: "a@example.com", name: "A" })
    ),
    http.get(api("/admin/tenants"), () =>
      HttpResponse.json({
        data: [box.tenant],
        meta: { total: 1, page: 1, per_page: 20, total_pages: 1 },
      })
    ),
    http.get(api("/admin/tenants/:id"), ({ params }) =>
      params.id === box.tenant.id
        ? HttpResponse.json({ data: box.tenant })
        : HttpResponse.json({ errors: [] }, { status: 404 })
    ),
    http.get(api("/admin/tenants/:id/users"), () =>
      HttpResponse.json({ data: [], meta: { total: 0, page: 1, per_page: 20, total_pages: 1 } })
    ),
    http.get(api("/admin/groups"), () =>
      HttpResponse.json({ data: [], meta: { total: 0, page: 1, per_page: 20, total_pages: 1 } })
    )
  )
}

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location-pathname">{location.pathname}</div>
}

function renderAt(initialPath: string) {
  const shell = (page: ReactNode) => (
    <AuthProvider>
      <BackofficeAuthProvider>
        <main>
          {page}
          <LocationProbe />
        </main>
      </BackofficeAuthProvider>
    </AuthProvider>
  )
  return renderWithProviders({
    initialPath,
    routes: (
      <>
        <Route path="/admin/tenants" element={shell(<AdminTenantsPage />)} />
        <Route path="/admin/tenants/:tenantId" element={shell(<AdminTenantDetailPage />)} />
      </>
    ),
  })
}

function pathname() {
  return screen.getByTestId("location-pathname").textContent
}

describe("tenant actions: who sees them", () => {
  it("shows no tenant actions to a support agent", async () => {
    signInAs("support_agent")
    seedReads({ tenant })
    renderAt("/admin/tenants/t1")

    await waitFor(() => expect(screen.getByTestId("admin-tenant-header")).toBeInTheDocument())
    expect(screen.queryByTestId("admin-tenant-edit")).not.toBeInTheDocument()
    expect(screen.queryByTestId("admin-tenant-suspend")).not.toBeInTheDocument()
    expect(screen.queryByTestId("admin-tenant-danger-zone")).not.toBeInTheDocument()
  })

  it("hides New tenant from a support agent", async () => {
    signInAs("support_agent")
    seedReads({ tenant })
    renderAt("/admin/tenants")

    await waitFor(() => expect(screen.getByText("Northwind Estates")).toBeInTheDocument())
    expect(screen.queryByTestId("admin-tenant-create")).not.toBeInTheDocument()
  })

  it("shows every action to a platform admin", async () => {
    signInAs("platform_admin")
    seedReads({ tenant })
    renderAt("/admin/tenants/t1")

    await waitFor(() => expect(screen.getByTestId("admin-tenant-edit")).toBeInTheDocument())
    expect(screen.getByTestId("admin-tenant-suspend")).toBeInTheDocument()
    expect(screen.queryByTestId("admin-tenant-reactivate")).not.toBeInTheDocument()
    expect(screen.getByTestId("admin-tenant-delete")).toBeInTheDocument()
  })
})

describe("create tenant", () => {
  beforeEach(() => signInAs("platform_admin"))

  it("creates a tenant from the name alone and opens it", async () => {
    const box = { tenant }
    seedReads(box)
    let body: unknown
    server.use(
      http.post(api("/admin/tenants"), async ({ request }) => {
        body = await request.json()
        box.tenant = { ...tenant, id: "t-new", name: "Acme Corp", slug: "acme-corp", domain: "" }
        return HttpResponse.json({ data: box.tenant }, { status: 201 })
      })
    )
    renderAt("/admin/tenants")

    await userEvent.click(await screen.findByTestId("admin-tenant-create"))
    await userEvent.type(screen.getByTestId("admin-tenant-name"), "  Acme Corp ")
    await userEvent.click(screen.getByTestId("admin-tenant-form-submit"))

    // The name is trimmed; an empty slug and domain are left to the BE.
    await waitFor(() => expect(body).toEqual({ name: "Acme Corp" }))
    await waitFor(() => expect(pathname()).toBe("/admin/tenants/t-new"))
    expect(await screen.findByRole("heading", { name: "Acme Corp" })).toBeInTheDocument()
  })

  it("checks the slug and domain before sending anything", async () => {
    seedReads({ tenant })
    let posted = false
    server.use(
      http.post(api("/admin/tenants"), () => {
        posted = true
        return HttpResponse.json({ data: tenant }, { status: 201 })
      })
    )
    renderAt("/admin/tenants")

    await userEvent.click(await screen.findByTestId("admin-tenant-create"))
    await userEvent.type(screen.getByTestId("admin-tenant-name"), "Acme")
    await userEvent.type(screen.getByTestId("admin-tenant-slug"), "Acme Corp")
    await userEvent.type(screen.getByTestId("admin-tenant-domain"), "https://acme.test")
    await userEvent.click(screen.getByTestId("admin-tenant-form-submit"))

    expect(await screen.findByTestId("admin-tenant-slug-error")).toHaveTextContent(
      "Use lowercase letters, digits and hyphens only."
    )
    expect(screen.getByTestId("admin-tenant-domain-error")).toHaveTextContent(
      "Enter a bare lowercase hostname"
    )
    expect(posted).toBe(false)
  })

  it("puts a taken slug next to the slug field", async () => {
    seedReads({ tenant })
    server.use(http.post(api("/admin/tenants"), () => codedError(409, "tenant.slug_taken")))
    renderAt("/admin/tenants")

    await userEvent.click(await screen.findByTestId("admin-tenant-create"))
    await userEvent.type(screen.getByTestId("admin-tenant-name"), "Northwind")
    await userEvent.type(screen.getByTestId("admin-tenant-slug"), "northwind")
    await userEvent.click(screen.getByTestId("admin-tenant-form-submit"))

    expect(await screen.findByTestId("admin-tenant-slug-error")).toHaveTextContent(
      "Another tenant already uses this slug."
    )
    expect(screen.getByTestId("admin-tenant-form-dialog")).toBeInTheDocument()
  })
})

describe("edit tenant", () => {
  beforeEach(() => signInAs("platform_admin"))

  it("sends only the fields that changed", async () => {
    const box = { tenant }
    seedReads(box)
    let body: unknown
    server.use(
      http.patch(api("/admin/tenants/t1"), async ({ request }) => {
        body = await request.json()
        box.tenant = { ...tenant, name: "Northwind Holdings" }
        return HttpResponse.json({ data: box.tenant })
      })
    )
    renderAt("/admin/tenants/t1")

    await userEvent.click(await screen.findByTestId("admin-tenant-edit"))
    const name = screen.getByTestId("admin-tenant-name")
    expect(name).toHaveValue("Northwind Estates")
    await userEvent.clear(name)
    await userEvent.type(name, "Northwind Holdings")
    await userEvent.click(screen.getByTestId("admin-tenant-form-submit"))

    await waitFor(() => expect(body).toEqual({ name: "Northwind Holdings" }))
    expect(await screen.findByRole("heading", { name: "Northwind Holdings" })).toBeInTheDocument()
  })

  it("does not let the slug be cleared", async () => {
    seedReads({ tenant })
    renderAt("/admin/tenants/t1")

    await userEvent.click(await screen.findByTestId("admin-tenant-edit"))
    await userEvent.clear(screen.getByTestId("admin-tenant-slug"))
    await userEvent.click(screen.getByTestId("admin-tenant-form-submit"))

    expect(await screen.findByTestId("admin-tenant-slug-error")).toHaveTextContent(
      "A slug is required."
    )
  })
})

describe("suspend and reactivate", () => {
  beforeEach(() => signInAs("platform_admin"))

  it("suspends with a reason and shows the new status", async () => {
    const box = { tenant }
    seedReads(box)
    let body: unknown
    server.use(
      http.post(api("/admin/tenants/t1/suspend"), async ({ request }) => {
        body = await request.json()
        box.tenant = { ...tenant, status: "suspended" }
        return HttpResponse.json({ data: box.tenant })
      })
    )
    renderAt("/admin/tenants/t1")

    await userEvent.click(await screen.findByTestId("admin-tenant-suspend"))
    const confirm = screen.getByTestId("admin-tenant-status-confirm")
    // A blank reason cannot be sent.
    expect(confirm).toBeDisabled()
    await userEvent.type(screen.getByTestId("admin-tenant-status-reason"), "   ")
    expect(confirm).toBeDisabled()
    await userEvent.type(screen.getByTestId("admin-tenant-status-reason"), " unpaid invoice ")
    await userEvent.click(confirm)

    await waitFor(() => expect(body).toEqual({ reason: "unpaid invoice" }))
    await waitFor(() =>
      expect(screen.queryByTestId("admin-tenant-status-dialog")).not.toBeInTheDocument()
    )
    const header = screen.getByTestId("admin-tenant-header")
    expect(within(header).getByText("Suspended")).toBeInTheDocument()
    expect(screen.getByTestId("admin-tenant-reactivate")).toBeInTheDocument()
  })

  it("explains why the tenant behind the back office cannot be suspended", async () => {
    seedReads({ tenant })
    server.use(
      http.post(api("/admin/tenants/t1/suspend"), () =>
        codedError(422, "admin.tenant.serves_this_host")
      )
    )
    renderAt("/admin/tenants/t1")

    await userEvent.click(await screen.findByTestId("admin-tenant-suspend"))
    await userEvent.type(screen.getByTestId("admin-tenant-status-reason"), "test")
    await userEvent.click(screen.getByTestId("admin-tenant-status-confirm"))

    expect(await screen.findByTestId("admin-tenant-status-error")).toHaveTextContent(
      "would lock the back office out"
    )
  })

  it("reactivates a suspended tenant", async () => {
    const box = { tenant: { ...tenant, status: "suspended" } }
    seedReads(box)
    let called = false
    server.use(
      http.post(api("/admin/tenants/t1/reactivate"), () => {
        called = true
        box.tenant = { ...tenant, status: "active" }
        return HttpResponse.json({ data: box.tenant })
      })
    )
    renderAt("/admin/tenants/t1")

    await userEvent.click(await screen.findByTestId("admin-tenant-reactivate"))
    await userEvent.type(screen.getByTestId("admin-tenant-status-reason"), "paid")
    await userEvent.click(screen.getByTestId("admin-tenant-status-confirm"))

    await waitFor(() => expect(called).toBe(true))
    expect(await screen.findByTestId("admin-tenant-suspend")).toBeInTheDocument()
  })
})

describe("delete tenant", () => {
  beforeEach(() => signInAs("platform_admin"))

  it("deletes only once the slug is typed exactly, then returns to the list", async () => {
    seedReads({ tenant })
    let confirmSlug: string | null = null
    server.use(
      http.delete(api("/admin/tenants/t1"), ({ request }) => {
        confirmSlug = new URL(request.url).searchParams.get("confirm_slug")
        return new HttpResponse(null, { status: 204 })
      })
    )
    renderAt("/admin/tenants/t1")

    await userEvent.click(await screen.findByTestId("admin-tenant-delete"))
    const input = screen.getByTestId("admin-tenant-delete-slug")
    const confirm = screen.getByTestId("admin-tenant-delete-confirm")
    expect(confirm).toBeDisabled()

    // The name, another case and a prefix are not the slug.
    await userEvent.type(input, "Northwind Estates")
    expect(confirm).toBeDisabled()
    await userEvent.clear(input)
    await userEvent.type(input, "NORTHWIND")
    expect(confirm).toBeDisabled()
    await userEvent.clear(input)
    await userEvent.type(input, "north")
    expect(confirm).toBeDisabled()

    await userEvent.type(input, "wind")
    expect(confirm).toBeEnabled()
    await userEvent.click(confirm)

    await waitFor(() => expect(confirmSlug).toBe("northwind"))
    await waitFor(() => expect(pathname()).toBe("/admin/tenants"))
  })

  it("keeps the dialog open with the reason when the server refuses", async () => {
    seedReads({ tenant })
    server.use(
      http.delete(api("/admin/tenants/t1"), () =>
        codedError(422, "admin.tenant.confirmation_mismatch")
      )
    )
    renderAt("/admin/tenants/t1")

    await userEvent.click(await screen.findByTestId("admin-tenant-delete"))
    await userEvent.type(screen.getByTestId("admin-tenant-delete-slug"), "northwind")
    await userEvent.click(screen.getByTestId("admin-tenant-delete-confirm"))

    expect(await screen.findByTestId("admin-tenant-delete-error")).toHaveTextContent(
      "The slug you typed does not match"
    )
    expect(pathname()).toBe("/admin/tenants/t1")
  })
})
