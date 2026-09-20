import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { FirstItemResolver } from "@/pages/FirstItemResolver"
import { buildDefaults, readDraft, writeDraft } from "@/features/commodities/draft"
import { peekPendingFirstItem, savePendingFirstItem } from "@/features/auth/firstItemHandoff"
import type { LocationGroup } from "@/features/group/api"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl, groupHandlers } from "@/test/handlers"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

// The staged files live in IndexedDB, which jsdom doesn't implement. Mocking
// the store gives the tests control over what comes back and, more to the
// point, lets them assert exactly when it is cleared — that timing is the
// no-data-loss contract this suite exists to protect.
const filesStore = vi.hoisted(() => ({
  loadPendingFiles: vi.fn(),
  clearPendingFiles: vi.fn(),
  savePendingFiles: vi.fn(),
}))
vi.mock("@/lib/pending-files-store", () => filesStore)

const DRAFT_KEY = "commodity-draft:anon:create"

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

function renderResolver() {
  return renderWithProviders({
    initialPath: "/first-item",
    routes: (
      <>
        <Route path="/first-item" element={<FirstItemResolver />} />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

function stashMarker(currency = "CZK") {
  savePendingFirstItem({ draftKey: DRAFT_KEY, currency, savedAt: Date.now() })
}

function stashDraft(name = "Espresso machine") {
  writeDraft(DRAFT_KEY, {
    ...buildDefaults(undefined, "CZK", true),
    name,
    short_name: "espresso",
    type: "electronics",
  })
}

function group(id: string, slug: string | undefined, name: string): LocationGroup {
  return { id, slug, name, group_currency: "CZK" } as LocationGroup
}

const household = group("g1", "household", "Household")
const office = group("g2", "office", "Office")

// createdCommodity registers the POST the replay makes into `slug` and
// returns a box holding the request body so a test can assert what was sent.
function createdCommodity(slug: string, id: string | null = "c1") {
  const seen: { body?: { data?: { attributes?: Record<string, unknown> } } } = {}
  server.use(
    msw.post(apiUrl(`/g/${slug}/commodities`), async ({ request }) => {
      seen.body = (await request.json()) as typeof seen.body
      return HttpResponse.json({
        data: {
          id: id ?? undefined,
          type: "commodities",
          attributes: { name: "Espresso machine" },
        },
      })
    })
  )
  return seen
}

async function expectNavigatedTo(pathname: string) {
  await waitFor(() => expect(screen.getByTestId("loc")).toHaveAttribute("data-pathname", pathname))
}

// expectStashIntact asserts the whole no-data-loss invariant in one place:
// the marker is back in storage, the draft was never removed, and the staged
// files were never dropped.
function expectStashIntact() {
  expect(peekPendingFirstItem()).not.toBeNull()
  expect(readDraft(DRAFT_KEY)).toBeDefined()
  expect(filesStore.clearPendingFiles).not.toHaveBeenCalled()
}

beforeEach(() => {
  window.localStorage.clear()
  __resetGroupContextForTests()
  __resetHttpForTests()
  filesStore.loadPendingFiles.mockReset().mockResolvedValue([])
  filesStore.clearPendingFiles.mockReset().mockResolvedValue(undefined)
})

describe("<FirstItemResolver />", () => {
  it("bounces to / when there is no marker", async () => {
    server.use(...groupHandlers.list([household]))
    renderResolver()
    await expectNavigatedTo("/")
  })

  it("replays into the only group and clears the stash afterwards", async () => {
    stashMarker()
    stashDraft()
    server.use(...groupHandlers.list([household]))
    const post = createdCommodity("household")

    renderResolver()

    await expectNavigatedTo("/g/household/commodities/c1")
    expect(post.body?.data?.attributes?.name).toBe("Espresso machine")
    expect(readDraft(DRAFT_KEY)).toBeUndefined()
    expect(peekPendingFirstItem()).toBeNull()
    expect(filesStore.clearPendingFiles).toHaveBeenCalledWith(DRAFT_KEY)
  })

  it("offers a picker when the user belongs to more than one group", async () => {
    stashMarker()
    stashDraft()
    server.use(...groupHandlers.list([household, office]))
    createdCommodity("office")

    renderResolver()

    await screen.findByTestId("first-item-resolver-picker")
    const choices = screen.getAllByTestId("first-item-resolver-group")
    expect(choices).toHaveLength(2)

    await userEvent.click(choices[1])

    await expectNavigatedTo("/g/office/commodities/c1")
  })

  it("creates a Main group seeded with the stashed currency when there are none", async () => {
    stashMarker("SEK")
    stashDraft()
    server.use(...groupHandlers.empty())
    const seen: { attributes?: Record<string, unknown> } = {}
    server.use(
      msw.post(apiUrl("/groups"), async ({ request }) => {
        const body = (await request.json()) as { data?: { attributes?: Record<string, unknown> } }
        seen.attributes = body.data?.attributes
        return HttpResponse.json({
          data: { id: "g9", type: "groups", attributes: { ...seen.attributes, slug: "main" } },
        })
      })
    )
    createdCommodity("main")

    renderResolver()

    await expectNavigatedTo("/g/main/commodities/c1")
    expect(seen.attributes?.name).toBe("Main")
    expect(seen.attributes?.group_currency).toBe("SEK")
  })

  it("uploads the staged files and links them to the created item", async () => {
    stashMarker()
    stashDraft()
    filesStore.loadPendingFiles.mockResolvedValue([
      { id: "p1", file: new File(["x"], "receipt.png", { type: "image/png" }), tags: ["receipt"] },
    ])
    server.use(...groupHandlers.list([household]))
    createdCommodity("household")
    const linked: { attributes?: Record<string, unknown> } = {}
    server.use(
      msw.post(apiUrl("/g/household/uploads/file"), () =>
        HttpResponse.json({ id: "f1", type: "files", attributes: { path: "receipt" } })
      ),
      msw.put(apiUrl("/g/household/files/f1"), async ({ request }) => {
        const body = (await request.json()) as { data?: { attributes?: Record<string, unknown> } }
        linked.attributes = body.data?.attributes
        return HttpResponse.json({ id: "f1", type: "files", attributes: {} })
      })
    )

    renderResolver()

    await expectNavigatedTo("/g/household/commodities/c1")
    expect(linked.attributes?.linked_entity_type).toBe("commodity")
    expect(linked.attributes?.linked_entity_id).toBe("c1")
    expect(linked.attributes?.tags).toEqual(["receipt"])
  })

  it("keeps the item when a file upload fails", async () => {
    stashMarker()
    stashDraft()
    filesStore.loadPendingFiles.mockResolvedValue([
      { id: "p1", file: new File(["x"], "receipt.png", { type: "image/png" }), tags: [] },
    ])
    server.use(...groupHandlers.list([household]))
    createdCommodity("household")
    server.use(
      msw.post(apiUrl("/g/household/uploads/file"), () =>
        HttpResponse.json({ error: "boom" }, { status: 500 })
      )
    )

    renderResolver()

    // The commodity is already persisted, so a failed attach must not send
    // the user back to the retry screen — it would POST the item twice.
    await expectNavigatedTo("/g/household/commodities/c1")
    expect(readDraft(DRAFT_KEY)).toBeUndefined()
  })

  it("preserves the stash when the groups query fails", async () => {
    stashMarker()
    stashDraft()
    server.use(...groupHandlers.error())

    renderResolver()

    await screen.findByTestId("first-item-resolver-error")
    expectStashIntact()
  })

  it("preserves the stash when the create call fails", async () => {
    stashMarker()
    stashDraft()
    server.use(...groupHandlers.list([household]))
    server.use(
      msw.post(apiUrl("/g/household/commodities"), () =>
        HttpResponse.json({ error: "boom" }, { status: 500 })
      )
    )

    renderResolver()

    await screen.findByTestId("first-item-resolver-error")
    expectStashIntact()
  })

  it("preserves the stash when the create response carries no id", async () => {
    stashMarker()
    stashDraft()
    server.use(...groupHandlers.list([household]))
    createdCommodity("household", null)

    renderResolver()

    await screen.findByTestId("first-item-resolver-error")
    expectStashIntact()
  })

  it("preserves the stash when the target group has no slug", async () => {
    stashMarker()
    stashDraft()
    // No commodities handler is registered: MSW fails an unhandled request,
    // so this also proves no POST was attempted against "/g//commodities".
    server.use(...groupHandlers.list([group("g1", undefined, "Slugless")]))

    renderResolver()

    await screen.findByTestId("first-item-resolver-error")
    expectStashIntact()
  })

  it("retries a failed replay without losing the draft", async () => {
    stashMarker()
    stashDraft()
    server.use(...groupHandlers.list([household]))
    let attempts = 0
    server.use(
      msw.post(apiUrl("/g/household/commodities"), () => {
        attempts += 1
        if (attempts === 1) return HttpResponse.json({ error: "boom" }, { status: 500 })
        return HttpResponse.json({
          data: { id: "c1", type: "commodities", attributes: { name: "Espresso machine" } },
        })
      })
    )

    renderResolver()

    await screen.findByTestId("first-item-resolver-error")
    await userEvent.click(screen.getByTestId("first-item-resolver-retry"))

    await expectNavigatedTo("/g/household/commodities/c1")
    expect(attempts).toBe(2)
    expect(peekPendingFirstItem()).toBeNull()
  })

  it("leaves the stash in place when the user skips", async () => {
    stashMarker()
    stashDraft()
    server.use(...groupHandlers.error())

    renderResolver()

    await screen.findByTestId("first-item-resolver-error")
    await userEvent.click(screen.getByTestId("first-item-resolver-skip"))

    await expectNavigatedTo("/")
    // Skipping is "not now", not "throw it away" — the next visit replays it.
    expectStashIntact()
  })

  it("clears the leftovers and falls through when the draft is gone", async () => {
    stashMarker()
    server.use(...groupHandlers.list([household]))

    renderResolver()

    await expectNavigatedTo("/")
    expect(peekPendingFirstItem()).toBeNull()
    expect(filesStore.clearPendingFiles).toHaveBeenCalledWith(DRAFT_KEY)
    expect(screen.queryByTestId("first-item-resolver-error")).not.toBeInTheDocument()
  })
})
