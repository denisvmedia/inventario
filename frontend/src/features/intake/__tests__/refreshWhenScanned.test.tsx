import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { act, screen, waitFor } from "@testing-library/react"
import { QueryClient } from "@tanstack/react-query"

import { fileKeys } from "@/features/files/keys"
import { GroupProvider } from "@/features/group/GroupContext"
import { INTAKE_POLL_MS, useIntakeSession } from "@/features/intake/hooks"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, intakeHandlers } from "@/test/handlers"
import { receiptSession } from "@/test/handlers/intake"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

// A batch that stops scanning is read once more, with the file lists, one
// poll later (#1626): the server puts each scan's tag and title on the
// files after it stores the scan, so the read that sees the batch leave
// "processing" can still show the files without them.

const SLUG = "household"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]
const FILES_KEY = [...fileKeys.group(SLUG), "list", "probe"]

function Probe({ id }: { id: string }) {
  const session = useIntakeSession(id)
  return <p data-testid="state">{session.data?.state ?? "loading"}</p>
}

function render(id: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })
  queryClient.setQueryData(FILES_KEY, ["stale"])
  renderWithProviders({
    queryClient,
    initialPath: `/g/${SLUG}/intake/${id}`,
    routes: (
      <Route
        path="/g/:groupSlug/intake/:id"
        element={
          <GroupProvider>
            <Probe id={id} />
          </GroupProvider>
        }
      />
    ),
  })
  return queryClient
}

// detailReads counts the reads of session id.
function detailReads(id: string) {
  const count = { n: 0 }
  server.events.on("request:start", ({ request }) => {
    if (request.method === "GET" && new URL(request.url).pathname.endsWith(`/sessions/${id}`)) {
      count.n++
    }
  })
  return count
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
  vi.useFakeTimers({ shouldAdvanceTime: true })
})

afterEach(() => {
  vi.useRealTimers()
  server.events.removeAllListeners()
})

describe("useIntakeSession after the scans", () => {
  it("reads the batch and the files again one poll after it leaves processing", async () => {
    const session = receiptSession()
    session.sources[1] = { ...session.sources[1]!, status: "pending", error_code: undefined }
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const reads = detailReads(session.id)
    const queryClient = render(session.id)
    expect(await screen.findByText("processing")).toBeInTheDocument()

    // The worker finishes the last file.
    session.sources[1] = { ...session.sources[1]!, status: "scanned" }
    await act(async () => {
      vi.advanceTimersByTime(INTAKE_POLL_MS)
    })
    expect(await screen.findByText("review")).toBeInTheDocument()
    const afterTransition = reads.n
    expect(queryClient.getQueryState(FILES_KEY)?.isInvalidated).toBe(false)

    await act(async () => {
      vi.advanceTimersByTime(INTAKE_POLL_MS)
    })
    await waitFor(() => expect(reads.n).toBe(afterTransition + 1))
    expect(queryClient.getQueryState(FILES_KEY)?.isInvalidated).toBe(true)

    // Only once: nothing is read after that.
    await act(async () => {
      vi.advanceTimersByTime(INTAKE_POLL_MS * 3)
    })
    expect(reads.n).toBe(afterTransition + 1)
  })

  it("reads nothing again for a batch that was done scanning when it opened", async () => {
    const session = receiptSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const reads = detailReads(session.id)
    const queryClient = render(session.id)
    expect(await screen.findByText("review")).toBeInTheDocument()
    const opened = reads.n
    await act(async () => {
      vi.advanceTimersByTime(INTAKE_POLL_MS * 3)
    })
    expect(reads.n).toBe(opened)
    expect(queryClient.getQueryState(FILES_KEY)?.isInvalidated).toBe(false)
  })
})
