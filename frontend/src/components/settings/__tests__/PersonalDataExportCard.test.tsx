import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import { toast } from "sonner"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { PersonalDataExportCard } from "@/components/settings/PersonalDataExportCard"
import { dataExportHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

function renderCard() {
  return renderWithProviders({
    initialPath: "/settings",
    routes: <Route path="/settings" element={<PersonalDataExportCard />} />,
  })
}

const completed = {
  id: "e1",
  status: "completed" as const,
  created_at: "2026-11-20T10:00:00Z",
  completed_at: "2026-11-20T10:02:00Z",
  expires_at: "2026-11-27T10:02:00Z",
  size_bytes: 2048,
  group_count: 1,
  item_count: 3,
  file_count: 2,
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  // sonner is mocked for the whole suite in src/test/setup.ts; the card's
  // toasts land on those spies.
  vi.mocked(toast.success).mockClear()
  vi.mocked(toast.error).mockClear()
})

afterEach(() => {
  vi.useRealTimers()
})

describe("<PersonalDataExportCard />", () => {
  it("explains the daily limit and the retention, and requests a copy", async () => {
    const requested = vi.fn()
    server.use(
      ...dataExportHandlers.requestCreated(
        {
          id: "e2",
          status: "pending",
          created_at: "2026-11-26T10:00:00Z",
          expires_at: "2026-12-03T10:00:00Z",
        },
        requested
      )
    )
    const user = userEvent.setup()
    renderCard()

    const card = await screen.findByTestId("personal-data-export")
    expect(within(card).getByText("Download my data")).toBeInTheDocument()
    expect(card).toHaveTextContent("one copy a day")
    expect(card).toHaveTextContent("kept for 7 days")
    expect(card).toHaveTextContent("cannot be restored")

    const button = screen.getByTestId("personal-data-export-request")
    await waitFor(() => expect(button).toBeEnabled())
    await user.click(button)

    await waitFor(() => expect(requested).toHaveBeenCalledTimes(1))
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith(expect.stringContaining("email"), undefined)
    )
  })

  it("lists a finished copy with its size and deletion date, and downloads it", async () => {
    server.use(
      ...dataExportHandlers.list({ exports: [completed] }),
      ...dataExportHandlers.downloadURL("e1", "/api/v1/data-exports/e1?uid=u1&exp=1&sig=x")
    )
    const clicked: string[] = []
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (
      this: HTMLAnchorElement
    ) {
      clicked.push(this.getAttribute("href") ?? "")
    })
    const user = userEvent.setup()
    renderCard()

    const row = await screen.findByTestId("personal-data-export-row")
    expect(row).toHaveAttribute("data-status", "completed")
    expect(row).toHaveTextContent("Ready")
    expect(row).toHaveTextContent("2.00 KiB")
    expect(row).toHaveTextContent("deleted on")

    await user.click(within(row).getByTestId("personal-data-export-download"))
    await waitFor(() => expect(clicked).toEqual(["/api/v1/data-exports/e1?uid=u1&exp=1&sig=x"]))
    click.mockRestore()
  })

  it("shows a copy being prepared and one that failed, without download links", async () => {
    server.use(
      ...dataExportHandlers.list({
        exports: [
          {
            id: "e3",
            status: "in_progress",
            created_at: "2026-11-26T09:00:00Z",
            expires_at: "2026-12-03T09:00:00Z",
          },
          {
            id: "e4",
            status: "failed",
            created_at: "2026-11-20T09:00:00Z",
            expires_at: "2026-11-27T09:00:00Z",
          },
        ],
      })
    )
    renderCard()

    const rows = await screen.findAllByTestId("personal-data-export-row")
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent("Preparing")
    expect(rows[0]).toHaveTextContent("updates when it is ready")
    expect(rows[1]).toHaveTextContent("Failed")
    expect(screen.queryByTestId("personal-data-export-download")).not.toBeInTheDocument()
  })

  it("disables the request button until the next request is allowed", async () => {
    server.use(
      ...dataExportHandlers.list({
        exports: [completed],
        next_request_at: new Date(Date.now() + 3_600_000).toISOString(),
      })
    )
    renderCard()

    expect(await screen.findByTestId("personal-data-export-next")).toHaveTextContent(
      "You can request the next copy on"
    )
    expect(screen.getByTestId("personal-data-export-request")).toBeDisabled()
  })

  it("says when the next copy is possible after a 429", async () => {
    server.use(...dataExportHandlers.requestRateLimited("2026-11-27T10:00:00Z"))
    const user = userEvent.setup()
    renderCard()

    const button = await screen.findByTestId("personal-data-export-request")
    await waitFor(() => expect(button).toBeEnabled())
    await user.click(button)

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(expect.stringContaining("one copy a day"), undefined)
    )
  })

  it("reloads the list and says so when a copy expired before its download", async () => {
    const listed = vi.fn()
    server.use(
      ...dataExportHandlers.listSequence([{ exports: [completed] }, { exports: [] }], listed),
      ...dataExportHandlers.downloadRefused("e1", 404, "data_export.not_found")
    )
    const user = userEvent.setup()
    renderCard()

    const row = await screen.findByTestId("personal-data-export-row")
    await user.click(within(row).getByTestId("personal-data-export-download"))

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        expect.stringContaining("no longer available"),
        undefined
      )
    )
    await waitFor(() =>
      expect(screen.queryByTestId("personal-data-export-row")).not.toBeInTheDocument()
    )
    expect(listed).toHaveBeenCalledTimes(2)
  })

  it("reloads the list and says so when a copy is not ready for its download", async () => {
    server.use(
      ...dataExportHandlers.listSequence([
        { exports: [completed] },
        { exports: [{ ...completed, status: "pending", created_at: new Date().toISOString() }] },
      ]),
      ...dataExportHandlers.downloadRefused("e1", 409, "data_export.not_ready")
    )
    const user = userEvent.setup()
    renderCard()

    const row = await screen.findByTestId("personal-data-export-row")
    await user.click(within(row).getByTestId("personal-data-export-download"))

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(expect.stringContaining("not ready yet"), undefined)
    )
    await waitFor(() =>
      expect(screen.getByTestId("personal-data-export-row")).toHaveAttribute(
        "data-status",
        "pending"
      )
    )
  })

  it("enables the request button when the next request is allowed", async () => {
    const listed = vi.fn()
    server.use(
      ...dataExportHandlers.listSequence(
        [
          { exports: [completed], next_request_at: new Date(Date.now() + 700).toISOString() },
          { exports: [completed] },
        ],
        listed
      )
    )
    renderCard()

    const button = await screen.findByTestId("personal-data-export-request")
    await waitFor(() => expect(screen.getByTestId("personal-data-export-next")).toBeInTheDocument())
    expect(button).toBeDisabled()

    await waitFor(() => expect(button).toBeEnabled(), { timeout: 3_000 })
    expect(screen.queryByTestId("personal-data-export-next")).not.toBeInTheDocument()
    await waitFor(() => expect(listed).toHaveBeenCalledTimes(2))
  })

  it("announces a copy that becomes ready", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const preparing = {
      id: "e5",
      status: "in_progress" as const,
      created_at: new Date().toISOString(),
      expires_at: "2026-12-03T09:00:00Z",
    }
    server.use(
      ...dataExportHandlers.listSequence([
        { exports: [preparing] },
        { exports: [{ ...preparing, status: "completed", size_bytes: 10 }] },
      ])
    )
    renderCard()

    const live = await screen.findByTestId("personal-data-export-announcement")
    await screen.findByTestId("personal-data-export-row")
    expect(live).toHaveAttribute("aria-live", "polite")
    expect(live).toBeEmptyDOMElement()

    await vi.advanceTimersByTimeAsync(5_000)
    await waitFor(() => expect(live).toHaveTextContent("is ready to download"))
  })

  it("stops polling for a copy that has been unfinished for hours", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const listed = vi.fn()
    server.use(
      ...dataExportHandlers.listSequence(
        [
          {
            exports: [
              {
                id: "e6",
                status: "pending",
                created_at: new Date(Date.now() - 3 * 3_600_000).toISOString(),
                expires_at: "2026-12-03T09:00:00Z",
              },
            ],
          },
        ],
        listed
      )
    )
    renderCard()

    await screen.findByTestId("personal-data-export-row")
    await vi.advanceTimersByTimeAsync(15_000)
    expect(listed).toHaveBeenCalledTimes(1)
  })

  it("has no axe violations", async () => {
    server.use(...dataExportHandlers.list({ exports: [completed] }))
    const { container } = renderCard()
    await screen.findByTestId("personal-data-export-row")
    expect(await axe(container)).toHaveNoViolations()
  })
})
