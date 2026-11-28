import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { act, screen, waitFor } from "@testing-library/react"
import { QueryClient } from "@tanstack/react-query"

import type { ReactNode } from "react"

import { fileKeys } from "@/features/files/keys"
import { GroupProvider } from "@/features/group/GroupContext"
import { useInbox } from "@/features/inbox/hooks"
import { INTAKE_POLL_MS, useIntakeSession, useIntakeSessions } from "@/features/intake/hooks"
import { intakeKeys } from "@/features/intake/keys"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, inboxHandlers, intakeHandlers } from "@/test/handlers"
import { receiptSession } from "@/test/handlers/intake"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

// A batch that stops scanning marks the file lists stale (#1626). The server
// writes each scan's tag and title on the files with the stored scan
// (#2940), so this happens on the read that sees the batch leave
// "processing", and the batch is not read again for it.

const SLUG = "household"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]
const FILES_KEY = [...fileKeys.group(SLUG), "list", "probe"]

function Probe({ id }: { id: string }) {
  const session = useIntakeSession(id)
  return <p data-testid="state">{session.data?.state ?? "loading"}</p>
}

function SessionsProbe() {
  const page = useIntakeSessions()
  return <p data-testid="state">{page.data?.sessions.map((s) => s.state).join() ?? "loading"}</p>
}

function InboxProbe() {
  const page = useInbox()
  return (
    <p data-testid="state">
      {page.data?.entries.map((e) => e.intake?.state ?? "").join() ?? "loading"}
    </p>
  )
}

// renderProbe renders probe in a group, with a file list in the cache to
// see it read again.
function renderProbe(probe: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })
  queryClient.setQueryData(FILES_KEY, ["stale"])
  const utils = renderWithProviders({
    queryClient,
    initialPath: `/g/${SLUG}/probe`,
    routes: <Route path="/g/:groupSlug/probe" element={<GroupProvider>{probe}</GroupProvider>} />,
  })
  return { queryClient, unmount: utils.unmount }
}

function render(id: string) {
  return renderProbe(<Probe id={id} />).queryClient
}

// processingSession is a batch whose second file is still being scanned.
function processingSession() {
  const session = receiptSession()
  session.sources[1] = { ...session.sources[1]!, status: "pending", error_code: undefined }
  return session
}

async function poll(times = 1) {
  await act(async () => {
    vi.advanceTimersByTime(INTAKE_POLL_MS * times)
  })
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
  it("marks the files stale as it leaves processing and reads the batch no more", async () => {
    const session = processingSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const reads = detailReads(session.id)
    const queryClient = render(session.id)
    expect(await screen.findByText("processing")).toBeInTheDocument()

    // The worker finishes the last file.
    session.sources[1] = { ...session.sources[1]!, status: "scanned" }
    await poll()
    expect(await screen.findByText("review")).toBeInTheDocument()
    await waitFor(() => expect(queryClient.getQueryState(FILES_KEY)?.isInvalidated).toBe(true))
    const afterTransition = reads.n

    await poll(3)
    expect(reads.n).toBe(afterTransition)
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

describe("useIntakeSessions after the scans", () => {
  it("marks the batch and the files stale as the list shows it leave processing", async () => {
    const session = processingSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const { queryClient } = renderProbe(<SessionsProbe />)
    // A page that shows the batch keeps it in the cache.
    queryClient.setQueryData(intakeKeys.detail(SLUG, session.id), { id: session.id })
    expect(await screen.findByText("processing")).toBeInTheDocument()

    session.sources[1] = { ...session.sources[1]!, status: "scanned" }
    await poll()
    expect(await screen.findByText("review")).toBeInTheDocument()
    await waitFor(() => expect(queryClient.getQueryState(FILES_KEY)?.isInvalidated).toBe(true))
    expect(queryClient.getQueryState(intakeKeys.detail(SLUG, session.id))?.isInvalidated).toBe(true)
  })
})

describe("useInbox after the scans", () => {
  it("marks the files stale as a batch in the inbox leaves processing", async () => {
    const fake = inboxHandlers.backend(SLUG, [inboxHandlers.intakeSession("s1", "processing")])
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    const { queryClient } = renderProbe(<InboxProbe />)
    expect(await screen.findByText("processing")).toBeInTheDocument()

    fake.entries[0] = inboxHandlers.intakeSession("s1", "review")
    await poll()
    expect(await screen.findByText("review")).toBeInTheDocument()
    await waitFor(() => expect(queryClient.getQueryState(FILES_KEY)?.isInvalidated).toBe(true))
  })
})
