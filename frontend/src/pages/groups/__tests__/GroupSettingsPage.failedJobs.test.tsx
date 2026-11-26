import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { GroupSettingsPage } from "@/pages/groups/GroupSettingsPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import type { FailedJob } from "@/features/failed-jobs/api"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { toastSpies } from "@/test/global-mocks"
import { apiUrl, failedJobHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

function renderSettings() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/groups/g1/settings?section=data",
    routes: (
      <Route
        path="/groups/:groupId/settings"
        element={
          <AuthProvider>
            <GroupProvider>
              <ConfirmProvider>
                <GroupSettingsPage />
              </ConfirmProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

const group = {
  id: "g1",
  slug: "household",
  name: "Household",
  group_currency: "USD",
}

function baseHandlers(role: "admin" | "owner" | "user" | "viewer") {
  return [
    msw.get(apiUrl("/auth/me"), () =>
      HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
    ),
    msw.get(apiUrl("/groups"), () =>
      HttpResponse.json({ data: [{ id: "g1", type: "groups", attributes: group }] })
    ),
    msw.get(apiUrl("/groups/g1"), () =>
      HttpResponse.json({ data: { id: "g1", type: "groups", attributes: group } })
    ),
    msw.get(apiUrl("/groups/g1/members"), () =>
      HttpResponse.json({
        data: [
          {
            id: "m1",
            type: "memberships",
            attributes: { id: "m1", group_id: "g1", member_user_id: "u1", role },
          },
        ],
      })
    ),
    msw.get(apiUrl("/g/household/storage-usage"), () =>
      HttpResponse.json({
        used_bytes: 0,
        quota_bytes: null,
        breakdown: { images: 0, documents: 0, other: 0, exports: 0, trash: 0 },
      })
    ),
  ]
}

const exportJob: FailedJob = {
  id: "e1",
  job_type: "export",
  title: "Weekly backup",
  error: "disk full",
  attempts: 1,
  failed_at: "2026-11-20T10:00:00Z",
  retryable: true,
}

const restoreJob: FailedJob = {
  id: "r1",
  job_type: "restore",
  title: "Restore from Monday",
  error: "conflicting ids",
  attempts: 1,
  failed_at: "2026-11-19T10:00:00Z",
  retryable: false,
}

const intakeJob: FailedJob = {
  id: "s1",
  job_type: "intake",
  title: "receipt.pdf",
  error: "file_unreadable",
  attempts: 3,
  failed_at: "2026-11-18T10:00:00Z",
  retryable: true,
}

const currencyJob: FailedJob = {
  id: "c1",
  job_type: "currency-migration",
  title: "USD → EUR",
  error: "the worker stopped",
  attempts: 1,
  failed_at: "2026-11-17T10:00:00Z",
  retryable: false,
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<GroupSettingsPage /> failed jobs (#1307)", () => {
  it("lists the group's failed jobs for an admin, with Retry only where the type allows it", async () => {
    const backend = failedJobHandlers.group("household", [exportJob, restoreJob])
    server.use(...baseHandlers("admin"), ...backend.handlers)
    renderSettings()

    const card = await screen.findByTestId("failed-jobs-card")
    const rows = await within(card).findAllByTestId("failed-job-row")
    expect(rows).toHaveLength(2)

    expect(within(rows[0]).getByText("Weekly backup")).toBeInTheDocument()
    expect(within(rows[0]).getByText("Export")).toBeInTheDocument()
    expect(within(rows[0]).getByText("disk full")).toBeInTheDocument()
    expect(within(rows[0]).getByTestId("failed-job-retry")).toBeInTheDocument()

    expect(within(rows[1]).getByText("Restore")).toBeInTheDocument()
    expect(within(rows[1]).queryByTestId("failed-job-retry")).not.toBeInTheDocument()
    expect(within(rows[1]).getByTestId("failed-job-not-retryable")).toBeInTheDocument()
  })

  it("says what to do instead for a job that cannot be retried, and reads an intake error code", async () => {
    const backend = failedJobHandlers.group("household", [restoreJob, intakeJob, currencyJob])
    server.use(...baseHandlers("admin"), ...backend.handlers)
    renderSettings()

    const card = await screen.findByTestId("failed-jobs-card")
    const rows = await within(card).findAllByTestId("failed-job-row")
    expect(rows).toHaveLength(3)

    expect(within(rows[0]).getByTestId("failed-job-not-retryable")).toHaveTextContent(
      "Start a new restore from the backup instead. An operator can dismiss this one."
    )
    expect(within(rows[1]).getByTestId("failed-job-error")).toHaveTextContent(
      "The file could not be read."
    )
    expect(within(rows[1]).getByTestId("failed-job-retry")).toBeInTheDocument()
    expect(within(rows[2]).getByTestId("failed-job-not-retryable")).toHaveTextContent(
      "Start a new currency change instead. An operator can dismiss this one."
    )
    expect(within(rows[2]).getByTestId("failed-job-error")).toHaveTextContent("the worker stopped")
  })

  it("retries a job and drops it from the list", async () => {
    const user = userEvent.setup()
    const backend = failedJobHandlers.group("household", [exportJob, restoreJob])
    server.use(...baseHandlers("owner"), ...backend.handlers)
    renderSettings()

    const card = await screen.findByTestId("failed-jobs-card")
    await user.click(await within(card).findByRole("button", { name: "Retry Weekly backup" }))

    await waitFor(() => expect(within(card).getAllByTestId("failed-job-row")).toHaveLength(1))
    const retry = backend.state.requests.find((r) => r.method === "POST")
    expect(retry?.url.pathname).toBe("/api/v1/g/household/failed-jobs/export/e1/retry")
    expect(toastSpies.success).toHaveBeenCalledWith("The job is back in the queue.", undefined)
  })

  it("says so when the job was already retried elsewhere", async () => {
    const user = userEvent.setup()
    server.use(
      ...baseHandlers("admin"),
      msw.get(apiUrl("/g/household/failed-jobs"), () =>
        HttpResponse.json({
          data: [{ id: "e1", type: "failed_jobs", attributes: { ...exportJob, id: undefined } }],
          meta: { total: 1 },
        })
      ),
      msw.post(apiUrl("/g/household/failed-jobs/export/e1/retry"), () =>
        HttpResponse.json(
          { errors: [{ status: "404", code: "failed_job.not_found" }] },
          { status: 404 }
        )
      )
    )
    renderSettings()

    await user.click(await screen.findByTestId("failed-job-retry"))

    await waitFor(() =>
      expect(toastSpies.info).toHaveBeenCalledWith(
        "This job is no longer failed. The list is up to date now.",
        undefined
      )
    )
  })

  it("shows the empty state when nothing failed", async () => {
    const backend = failedJobHandlers.group("household", [])
    server.use(...baseHandlers("admin"), ...backend.handlers)
    renderSettings()

    expect(await screen.findByTestId("failed-jobs-empty")).toHaveTextContent(
      "No failed jobs in this group."
    )
  })

  it("leaves the card out for a member who is not an admin and never asks the server", async () => {
    const backend = failedJobHandlers.group("household", [exportJob])
    server.use(...baseHandlers("user"), ...backend.handlers)
    renderSettings()

    expect(await screen.findByTestId("group-section-data")).toBeInTheDocument()
    // The members query has resolved once the Data section knows the role.
    await waitFor(() => expect(screen.getByTestId("settings-open-exports")).toBeInTheDocument())
    expect(screen.queryByTestId("failed-jobs-card")).not.toBeInTheDocument()
    expect(backend.state.requests).toHaveLength(0)
  })
})
