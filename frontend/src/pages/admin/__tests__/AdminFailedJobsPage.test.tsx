import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { AuthProvider } from "@/features/auth/AuthContext"
import type { AdminFailedJob } from "@/features/admin/api"
import { BackofficeAuthProvider } from "@/features/backoffice/auth/context"
import { clearBackofficeAuth, setBackofficeAccessToken } from "@/features/backoffice/auth/storage"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { AdminFailedJobsPage } from "@/pages/admin/AdminFailedJobsPage"
import { toastSpies } from "@/test/global-mocks"
import { apiUrl, backofficeAuthHandlers, failedJobHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const adminUser = {
  id: "u1",
  email: "admin@example.com",
  name: "Admin",
  is_system_admin: true,
}

const exportJob: AdminFailedJob = {
  id: "e1",
  job_type: "export",
  title: "Weekly backup",
  error: "disk full",
  attempts: 2,
  failed_at: "2026-11-20T10:00:00Z",
  retryable: true,
  tenant_id: "t1",
  tenant_name: "Acme Inc",
  group_id: "g1",
  group_name: "Household",
  group_slug: "household",
}

const restoreJob: AdminFailedJob = {
  id: "r1",
  job_type: "restore",
  title: "Restore from Monday",
  error: "conflicting ids",
  attempts: 1,
  failed_at: "2026-11-19T10:00:00Z",
  retryable: false,
  tenant_id: "t2",
  tenant_name: "Globex",
  group_id: "g2",
  group_name: "Office",
  group_slug: "office",
}

// A personal data export belongs to a user: it has no group, and its title
// is the user's email address.
const personalJob: AdminFailedJob = {
  id: "p1",
  job_type: "personal-data-export",
  title: "alice@example.com",
  error: "the account is not active",
  attempts: 1,
  failed_at: "2026-11-18T10:00:00Z",
  retryable: true,
  tenant_id: "t1",
  tenant_name: "Acme Inc",
}

function seed(jobs: AdminFailedJob[] = [exportJob, restoreJob]) {
  const backend = failedJobHandlers.admin(jobs)
  server.use(
    http.get(apiUrl("/auth/me"), () => HttpResponse.json(adminUser)),
    http.get(apiUrl("/admin/tenants"), () =>
      HttpResponse.json({
        data: [
          { id: "t1", name: "Acme Inc", slug: "acme", status: "active" },
          { id: "t2", name: "Globex", slug: "globex", status: "active" },
        ],
        meta: { total: 2, page: 1, per_page: 100, total_pages: 1 },
      })
    ),
    ...backend.handlers
  )
  return backend
}

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location-search">{location.search}</div>
}

function locationSearch(): URLSearchParams {
  return new URLSearchParams(screen.getByTestId("location-search").textContent ?? "")
}

function renderPage(initialPath = "/admin/failed-jobs") {
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path="/admin/failed-jobs"
        element={
          <AuthProvider>
            <BackofficeAuthProvider>
              <main>
                <AdminFailedJobsPage />
                <LocationProbe />
              </main>
            </BackofficeAuthProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

function lastListRequest(backend: ReturnType<typeof seed>): URL | undefined {
  return backend.state.requests.filter((r) => r.method === "GET").at(-1)?.url
}

describe("AdminFailedJobsPage", () => {
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
    server.use(...backofficeAuthHandlers.signedIn())
  })

  it("lists the failed jobs of every tenant with their tenant, group and error", async () => {
    seed()
    renderPage()

    const rows = await screen.findAllByTestId("admin-failed-job-row")
    expect(rows).toHaveLength(2)
    expect(within(rows[0]).getByText("Weekly backup")).toBeInTheDocument()
    expect(within(rows[0]).getByText("Acme Inc")).toBeInTheDocument()
    expect(within(rows[0]).getByText("Household")).toBeInTheDocument()
    expect(within(rows[0]).getByText("disk full")).toBeInTheDocument()
    expect(within(rows[1]).getByText("Globex")).toBeInTheDocument()
  })

  it("lists a failed personal data export with its user and no group, and retries it", async () => {
    const backend = seed([personalJob])
    renderPage()

    const [row] = await screen.findAllByTestId("admin-failed-job-row")
    expect(within(row).getByText("Personal data export")).toBeInTheDocument()
    expect(within(row).getByText("alice@example.com")).toBeInTheDocument()
    expect(within(row).getByText("Acme Inc")).toBeInTheDocument()
    expect(within(row).getByText("—")).toBeInTheDocument()

    await userEvent.click(within(row).getByTestId("admin-failed-job-retry"))
    await waitFor(() =>
      expect(
        backend.state.requests.some(
          (r) =>
            r.method === "POST" &&
            r.url.pathname.endsWith("/admin/failed-jobs/personal-data-export/p1/retry")
        )
      ).toBe(true)
    )
  })

  it("offers Retry only where the type allows it and Discard on every job", async () => {
    seed()
    renderPage()

    const rows = await screen.findAllByTestId("admin-failed-job-row")
    expect(within(rows[0]).getByTestId("admin-failed-job-retry")).toBeInTheDocument()
    expect(within(rows[0]).getByTestId("admin-failed-job-discard")).toBeInTheDocument()
    expect(within(rows[1]).queryByTestId("admin-failed-job-retry")).not.toBeInTheDocument()
    expect(within(rows[1]).getByTestId("admin-failed-job-discard")).toBeInTheDocument()
  })

  it("sends the type and tenant filters to the server and keeps them in the URL", async () => {
    const backend = seed()
    renderPage("/admin/failed-jobs?type=restore&tenantID=t2")

    await waitFor(() => expect(screen.getAllByTestId("admin-failed-job-row")).toHaveLength(1))
    const url = lastListRequest(backend)
    expect(url?.searchParams.get("type")).toBe("restore")
    expect(url?.searchParams.get("tenantID")).toBe("t2")
    expect(screen.getByText("Restore from Monday")).toBeInTheDocument()
    expect(locationSearch().get("type")).toBe("restore")
  })

  it("ignores an unknown ?type instead of sending it", async () => {
    const backend = seed()
    renderPage("/admin/failed-jobs?type=weekly-digest")

    await waitFor(() => expect(screen.getAllByTestId("admin-failed-job-row")).toHaveLength(2))
    expect(lastListRequest(backend)?.searchParams.get("type")).toBeNull()
  })

  it("retries a job", async () => {
    const user = userEvent.setup()
    const backend = seed()
    renderPage()

    await user.click(await screen.findByRole("button", { name: "Retry Weekly backup" }))

    await waitFor(() => expect(screen.getAllByTestId("admin-failed-job-row")).toHaveLength(1))
    const post = backend.state.requests.find((r) => r.method === "POST")
    expect(post?.url.pathname).toBe("/api/v1/admin/failed-jobs/export/e1/retry")
    expect(toastSpies.success).toHaveBeenCalledWith("The job is back in the queue.", undefined)
  })

  it("discards a job after the confirmation", async () => {
    const user = userEvent.setup()
    const backend = seed()
    renderPage()

    await user.click(await screen.findByRole("button", { name: "Discard Restore from Monday" }))
    const dialog = await screen.findByTestId("admin-failed-job-discard-dialog")
    expect(backend.state.requests.some((r) => r.method === "POST")).toBe(false)
    const subject = within(dialog).getByTestId("admin-failed-job-discard-subject")
    expect(subject).toHaveTextContent("Restore from Monday")
    expect(subject).toHaveTextContent("Globex · Office")
    expect(subject).toHaveTextContent("r1")

    await user.click(within(dialog).getByTestId("admin-failed-job-discard-confirm"))

    await waitFor(() => expect(screen.getAllByTestId("admin-failed-job-row")).toHaveLength(1))
    const post = backend.state.requests.find((r) => r.method === "POST")
    expect(post?.url.pathname).toBe("/api/v1/admin/failed-jobs/restore/r1/discard")
    expect(screen.queryByTestId("admin-failed-job-discard-dialog")).not.toBeInTheDocument()
  })

  it("lets a support agent retry but not discard", async () => {
    server.use(...backofficeAuthHandlers.signedIn({ user: { role: "support_agent" } }))
    seed()
    renderPage()

    const rows = await screen.findAllByTestId("admin-failed-job-row")
    await waitFor(() =>
      expect(within(rows[0]).queryByTestId("admin-failed-job-discard")).not.toBeInTheDocument()
    )
    expect(within(rows[0]).getByTestId("admin-failed-job-retry")).toBeInTheDocument()
    expect(within(rows[1]).queryByTestId("admin-failed-job-discard")).not.toBeInTheDocument()
  })

  it("discards the jobs of the filters that failed before a day, after a dry-run count", async () => {
    const user = userEvent.setup()
    const backend = seed()
    renderPage("/admin/failed-jobs?tenantID=t1")

    await waitFor(() => expect(screen.getAllByTestId("admin-failed-job-row")).toHaveLength(1))
    await user.click(screen.getByTestId("admin-failed-jobs-bulk-open"))
    const dialog = await screen.findByTestId("admin-failed-jobs-bulk-dialog")
    expect(within(dialog).getByTestId("admin-failed-jobs-bulk-scope")).toHaveTextContent("Acme Inc")
    expect(within(dialog).getByTestId("admin-failed-jobs-bulk-confirm")).toBeDisabled()
    expect(backend.state.requests.some((r) => r.method === "POST")).toBe(false)

    await user.type(within(dialog).getByLabelText("Failed before"), "2026-11-21")

    await waitFor(() =>
      expect(within(dialog).getByTestId("admin-failed-jobs-bulk-count")).toHaveTextContent(
        "1 failed job will be discarded."
      )
    )
    const dryRun = backend.state.requests.find((r) => r.method === "POST")
    expect(dryRun?.url.pathname).toBe("/api/v1/admin/failed-jobs/bulk-discard")
    expect(dryRun?.url.searchParams.get("failed_before")).toBe("2026-11-21")
    expect(dryRun?.url.searchParams.get("tenantID")).toBe("t1")
    expect(dryRun?.url.searchParams.get("dry_run")).toBe("true")

    await user.click(within(dialog).getByRole("button", { name: "Discard 1 job" }))

    await waitFor(() => expect(screen.getByTestId("admin-failed-jobs-empty")).toBeInTheDocument())
    const discards = backend.state.requests.filter(
      (r) => r.method === "POST" && r.url.searchParams.get("dry_run") === null
    )
    expect(discards).toHaveLength(1)
    expect(discards[0].url.searchParams.get("tenantID")).toBe("t1")
    expect(backend.state.jobs.map((job) => job.id)).toEqual(["r1"])
    expect(toastSpies.success).toHaveBeenCalledWith("1 failed job is discarded.", undefined)
    expect(screen.queryByTestId("admin-failed-jobs-bulk-dialog")).not.toBeInTheDocument()
  })

  it("keeps the bulk discard closed to a day nothing failed before", async () => {
    const user = userEvent.setup()
    seed()
    renderPage()

    await screen.findAllByTestId("admin-failed-job-row")
    await user.click(screen.getByTestId("admin-failed-jobs-bulk-open"))
    const dialog = await screen.findByTestId("admin-failed-jobs-bulk-dialog")
    expect(within(dialog).getByTestId("admin-failed-jobs-bulk-scope")).toHaveTextContent(
      "All job types"
    )
    await user.type(within(dialog).getByLabelText("Failed before"), "2026-11-01")

    await waitFor(() =>
      expect(within(dialog).getByTestId("admin-failed-jobs-bulk-count")).toHaveTextContent(
        "No failed job of these filters failed before this day."
      )
    )
    expect(within(dialog).getByTestId("admin-failed-jobs-bulk-confirm")).toBeDisabled()
  })

  it("does not offer the bulk discard to a support agent", async () => {
    server.use(...backofficeAuthHandlers.signedIn({ user: { role: "support_agent" } }))
    seed()
    renderPage()

    await screen.findAllByTestId("admin-failed-job-row")
    await waitFor(() =>
      expect(screen.queryByTestId("admin-failed-jobs-bulk-open")).not.toBeInTheDocument()
    )
  })

  it("shows the empty state", async () => {
    seed([])
    renderPage()

    expect(await screen.findByTestId("admin-failed-jobs-empty")).toHaveTextContent(
      "No failed jobs."
    )
  })

  it("shows a different empty state when filters are on", async () => {
    seed([])
    renderPage("/admin/failed-jobs?tenantID=t1")

    expect(await screen.findByTestId("admin-failed-jobs-empty")).toHaveTextContent(
      "No failed jobs match these filters."
    )
  })
})
