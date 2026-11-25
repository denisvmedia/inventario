import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
// test/setup.ts mocks sonner with one spy per variant.
import { toast } from "sonner"
import { axe } from "jest-axe"

import { CommoditiesListPage } from "@/pages/commodities/CommoditiesListPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  apiUrl,
  areaHandlers,
  commodityHandlers,
  groupHandlers,
  tagHandlers,
} from "@/test/handlers"
import { bulkResultResponse } from "@/test/handlers/commodities"
import { pickRadixSelect } from "@/test/radix"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]
const areaFixture = [
  { id: "a1", type: "areas", attributes: { id: "a1", name: "Garage", location_id: "l1" } },
]

function commodityRes(id: string, attrs: Record<string, unknown>) {
  return {
    id,
    type: "commodities",
    attributes: { id, count: 1, status: "in_use", type: "other", area_id: "a1", ...attrs },
  }
}

const ROWS = [
  commodityRes("c1", { name: "First", tags: ["old"] }),
  commodityRes("c2", { name: "Second" }),
]

// listWithTotal serves one page of rows for a query that matches `total`
// items in all.
function listWithTotal(rows: unknown[], total: number) {
  return msw.get(apiUrl(`/g/${SLUG}/commodities`), () =>
    HttpResponse.json({ data: rows, meta: { commodities: total } })
  )
}

function renderList() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <CommoditiesListPage />
            </ConfirmProvider>
          </GroupProvider>
        }
      />
    ),
  })
}

type User = ReturnType<typeof userEvent.setup>

async function selectBoth(user: User) {
  const cards = await screen.findAllByTestId("commodity-card")
  await user.click(within(cards[0]).getByTestId("commodity-select"))
  await user.click(within(cards[1]).getByTestId("commodity-select"))
}

async function openEditMenuItem(user: User, testId: string) {
  await user.click(await screen.findByTestId("commodities-bulk-edit"))
  await user.click(await screen.findByTestId(testId))
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  localStorage.removeItem("commodities:viewMode")
  vi.mocked(toast.success).mockClear()
  vi.mocked(toast.error).mockClear()
  server.use(
    msw.get(apiUrl(`/g/${SLUG}/settings`), () => HttpResponse.json({})),
    ...groupHandlers.list(groupFixture),
    ...areaHandlers.list(SLUG, areaFixture),
    ...tagHandlers.list(SLUG, [
      { id: "t1", slug: "garage", label: "Garage", color: "blue" },
      { id: "t2", slug: "old", label: "Old", color: "muted" },
    ])
  )
})

describe("<CommoditiesListPage /> bulk actions", () => {
  it("adds and removes tags without replacing the others", async () => {
    const user = userEvent.setup()
    const requests: unknown[] = []
    server.use(
      ...commodityHandlers.list(SLUG, ROWS),
      ...commodityHandlers.bulkUpdate(SLUG, { onRequest: (a) => requests.push(a) })
    )
    renderList()
    await selectBoth(user)
    await openEditMenuItem(user, "commodities-bulk-edit-tags")

    const dialog = await screen.findByTestId("bulk-tags-dialog")
    expect(within(dialog).getByText("Edit the tags of 2 items")).toBeInTheDocument()
    await user.click(await within(dialog).findByTestId("bulk-tags-add-option-garage"))
    await user.type(within(dialog).getByTestId("bulk-tags-add-input"), "Spare parts{Enter}")
    await user.click(within(dialog).getByTestId("bulk-tags-remove-option-old"))
    // Removing a tag takes it off the add side, and the other way round.
    await user.click(within(dialog).getByTestId("bulk-tags-remove-option-garage"))
    await user.click(within(dialog).getByTestId("bulk-tags-add-option-garage"))
    // The remove side picks existing tags only.
    expect(within(dialog).queryByTestId("bulk-tags-remove-input")).toBeNull()
    expect(await axe(dialog)).toHaveNoViolations()

    await user.click(within(dialog).getByTestId("bulk-tags-confirm"))
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toEqual({
      ids: ["c1", "c2"],
      patch: { tags: { add: ["Spare parts", "garage"], remove: ["old"] } },
    })
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith("2 items updated.", expect.anything())
    )
    await waitFor(() =>
      expect(screen.queryByTestId("commodities-bulk-bar")).not.toBeInTheDocument()
    )
  })

  it("selects every matching item and sends the query, not ids", async () => {
    const user = userEvent.setup()
    const requests: Record<string, unknown>[] = []
    server.use(
      listWithTotal(ROWS, 120),
      ...commodityHandlers.bulkUpdate(SLUG, {
        onRequest: (a) => requests.push(a),
        respond: () => bulkResultResponse({ succeeded: ["c1", "c2"] }),
      })
    )
    renderList()
    const cards = await screen.findAllByTestId("commodity-card")
    await user.click(within(cards[0]).getByTestId("commodity-select"))
    // The link appears once the whole page is selected.
    expect(screen.queryByTestId("commodities-select-all-matching")).toBeNull()
    await user.click(screen.getByTestId("commodities-select-all"))
    await user.click(await screen.findByTestId("commodities-select-all-matching"))

    expect(screen.getByTestId("commodities-bulk-count")).toHaveTextContent(
      "All 120 matching items selected"
    )
    // Label sheets take ids, so they are not offered for a query.
    expect(screen.queryByTestId("commodities-bulk-print-labels")).toBeNull()

    await openEditMenuItem(user, "commodities-bulk-edit-type")
    const dialog = await screen.findByTestId("bulk-type-dialog")
    expect(within(dialog).getByText("Set the type of 120 items")).toBeInTheDocument()
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Electronics$/i })
    await user.click(within(dialog).getByTestId("bulk-type-confirm"))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].ids).toBeUndefined()
    expect(requests[0].expected_count).toBe(120)
    expect(requests[0].patch).toEqual({ type: "electronics" })
    const query = new URLSearchParams(requests[0].query as string)
    expect(query.get("include_inactive")).toBe("false")
    expect(query.has("page")).toBe(false)
  })

  it("unchecking a row of all matching keeps the rest of the page", async () => {
    const user = userEvent.setup()
    server.use(listWithTotal(ROWS, 120))
    renderList()
    const cards = await screen.findAllByTestId("commodity-card")
    await user.click(within(cards[0]).getByTestId("commodity-select"))
    await user.click(screen.getByTestId("commodities-select-all"))
    await user.click(await screen.findByTestId("commodities-select-all-matching"))
    await user.click(within(cards[0]).getByTestId("commodity-select"))
    expect(screen.getByTestId("commodities-bulk-count")).toHaveTextContent("1 item selected")
  })

  it("says why all matching items cannot be selected above the limit", async () => {
    const user = userEvent.setup()
    server.use(listWithTotal(ROWS, 501))
    renderList()
    const cards = await screen.findAllByTestId("commodity-card")
    await user.click(within(cards[0]).getByTestId("commodity-select"))
    await user.click(screen.getByTestId("commodities-select-all"))
    expect(await screen.findByTestId("commodities-select-all-too-many")).toBeInTheDocument()
    expect(screen.queryByTestId("commodities-select-all-matching")).toBeNull()
  })

  it("changes the status with the date and note of a single change", async () => {
    const user = userEvent.setup()
    const requests: Record<string, unknown>[] = []
    server.use(
      ...commodityHandlers.list(SLUG, ROWS),
      ...commodityHandlers.bulkUpdate(SLUG, { onRequest: (a) => requests.push(a) })
    )
    renderList()
    await selectBoth(user)
    await openEditMenuItem(user, "commodities-bulk-status-lost")

    const dialog = await screen.findByTestId("status-transition-dialog")
    expect(within(dialog).getByText("Mark 2 items as Lost")).toBeInTheDocument()
    const date = within(dialog).getByTestId("status-transition-date")
    await user.clear(date)
    await user.type(date, "2026-11-20")
    await user.type(within(dialog).getByTestId("status-transition-note"), "Moving day")
    await user.click(within(dialog).getByTestId("status-transition-confirm"))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].patch).toEqual({
      status: { value: "lost", date: "2026-11-20", note: "Moving day" },
    })
  })

  it("confirms a change back to in use", async () => {
    const user = userEvent.setup()
    const requests: Record<string, unknown>[] = []
    server.use(
      ...commodityHandlers.list(SLUG, ROWS),
      ...commodityHandlers.bulkUpdate(SLUG, { onRequest: (a) => requests.push(a) })
    )
    renderList()
    await selectBoth(user)
    await openEditMenuItem(user, "commodities-bulk-status-in_use")
    await user.click(await screen.findByRole("button", { name: "Mark as in use" }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].patch).toEqual({ status: { value: "in_use" } })
  })

  it("lists the failed items and retries only those", async () => {
    const user = userEvent.setup()
    const requests: Record<string, unknown>[] = []
    server.use(
      ...commodityHandlers.list(SLUG, ROWS),
      ...commodityHandlers.bulkUpdate(SLUG, {
        onRequest: (a) => requests.push(a),
        respond: () =>
          requests.length === 1
            ? bulkResultResponse({
                succeeded: ["c1"],
                failed: [
                  {
                    id: "c2",
                    name: "Second",
                    code: "invalid",
                    error: "short_name: cannot be blank.",
                  },
                ],
              })
            : bulkResultResponse({ succeeded: ["c2"] }),
      })
    )
    renderList()
    await selectBoth(user)
    await openEditMenuItem(user, "commodities-bulk-edit-type")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    await user.click(screen.getByTestId("bulk-type-confirm"))

    const result = await screen.findByTestId("bulk-result-dialog")
    expect(within(result).getByText("1 item changed · 1 item failed")).toBeInTheDocument()
    const failed = within(result).getByTestId("bulk-result-failed")
    expect(within(failed).getByRole("link", { name: "Second" })).toHaveAttribute(
      "href",
      expect.stringContaining(`/g/${SLUG}/commodities/c2`)
    )
    expect(within(failed).getByText("short_name: cannot be blank.")).toBeInTheDocument()
    expect(await axe(result)).toHaveNoViolations()
    // The failed item stays selected.
    expect(screen.getByTestId("commodities-bulk-count")).toHaveTextContent("1 item selected")

    await user.click(within(result).getByTestId("bulk-result-retry"))
    await waitFor(() => expect(requests).toHaveLength(2))
    expect(requests[1]).toEqual({ ids: ["c2"], patch: { type: "furniture" } })
    await waitFor(() => expect(screen.queryByTestId("bulk-result-dialog")).not.toBeInTheDocument())
    expect(toast.success).toHaveBeenCalledWith("1 item updated.", expect.anything())
  })

  it("refuses to act when the query now matches a different number of items", async () => {
    const user = userEvent.setup()
    server.use(
      listWithTotal(ROWS, 120),
      ...commodityHandlers.bulkDelete(SLUG, {
        respond: () =>
          HttpResponse.json(
            {
              errors: [
                {
                  status: "Conflict",
                  code: "bulk.selection_changed",
                  meta: { expected: "120", matched: "121" },
                },
              ],
            },
            { status: 409 }
          ),
      })
    )
    renderList()
    const cards = await screen.findAllByTestId("commodity-card")
    await user.click(within(cards[0]).getByTestId("commodity-select"))
    await user.click(screen.getByTestId("commodities-select-all"))
    await user.click(await screen.findByTestId("commodities-select-all-matching"))
    await user.click(screen.getByTestId("commodities-bulk-delete"))
    // The confirm dialog's Delete is the last button of that name.
    await waitFor(() => expect(document.body).toHaveAttribute("data-scroll-locked"))
    const buttons = screen.getAllByRole("button", { name: /^Delete$/i })
    await user.click(buttons[buttons.length - 1])

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "The filter now matches 121 items, not 120. Nothing was changed. Check the list and try again.",
        undefined
      )
    )
    // Nothing happened, so the selection is still there.
    expect(screen.getByTestId("commodities-bulk-count")).toHaveTextContent(
      "All 120 matching items selected"
    )
  })
})
