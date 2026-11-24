import { beforeEach, describe, expect, it } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"

import { FileEditPage } from "@/pages/files/FileEditPage"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl, groupHandlers, tagHandlers } from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

type Attrs = Record<string, unknown>

function fileAttrs(id: string, attrs: Attrs = {}): Attrs {
  return {
    id,
    title: `File ${id}`,
    description: "",
    path: `file-${id}`,
    ext: ".pdf",
    category: "documents",
    type: "document",
    mime_type: "application/pdf",
    tags: ["receipt"],
    created_at: "2026-04-30T10:00:00Z",
    ...attrs,
  }
}

// One mutable store behind list, detail and update, so a save is visible to
// the list query that follows it — the way the server behaves.
function serveFiles(rows: Attrs[], opts: { failUpdate?: boolean } = {}) {
  const store = rows.map((r) => ({ ...r }))
  const puts: Array<{ id: string; attributes: Attrs }> = []
  server.use(
    ...groupHandlers.list(groupFixture),
    ...tagHandlers.list(SLUG, []),
    http.get(apiUrl(`/g/${SLUG}/tags/autocomplete`), () => HttpResponse.json({ data: [] })),
    http.get(apiUrl(`/g/${SLUG}/files`), ({ request }) => {
      const params = new URL(request.url).searchParams
      const tags = (params.get("tags") ?? "").split(",").filter(Boolean)
      const category = params.get("category")
      const matched = store.filter(
        (f) =>
          (!category || f.category === category) &&
          tags.every((t) => (f.tags as string[]).includes(t))
      )
      const limit = Number(params.get("limit")) || 20
      const page = Math.max(Number(params.get("page")) || 1, 1)
      const paged = matched.slice((page - 1) * limit, page * limit)
      return HttpResponse.json({ data: paged, meta: { total: matched.length } })
    }),
    http.get(apiUrl(`/g/${SLUG}/files/:id`), ({ params }) => {
      const row = store.find((f) => f.id === params.id)
      if (!row) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      return HttpResponse.json({
        id: row.id,
        type: "files",
        attributes: row,
        meta: { signed_urls: { [row.id as string]: { url: `/signed/${row.id}` } } },
      })
    }),
    http.put(apiUrl(`/g/${SLUG}/files/:id`), async ({ params, request }) => {
      if (opts.failUpdate) {
        return HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
      }
      const body = (await request.json()) as { data: { attributes: Attrs } }
      const id = params.id as string
      puts.push({ id, attributes: body.data.attributes })
      const row = store.find((f) => f.id === id)!
      Object.assign(row, body.data.attributes)
      return HttpResponse.json({ id, type: "files", attributes: row })
    })
  )
  return { store, puts }
}

function Where() {
  const location = useLocation()
  return (
    <p data-testid="where">
      {location.pathname}
      {location.search}
    </p>
  )
}

function renderReview(path: string) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: path,
    routes: (
      <>
        <Route
          path="/g/:groupSlug/files/:id/edit"
          element={
            <GroupProvider>
              <ConfirmProvider>
                <FileEditPage />
                <Where />
              </ConfirmProvider>
            </GroupProvider>
          }
        />
        <Route path="/g/:groupSlug/files/:id" element={<Where />} />
        <Route path="/g/:groupSlug/commodities/:id" element={<Where />} />
      </>
    ),
  })
}

const LIST = `/g/${SLUG}/files?tags=receipt&view=grid`
const FROM = `from=${encodeURIComponent(LIST)}`

function reviewPath(id: string, from = FROM) {
  return `/g/${SLUG}/files/${id}/edit${from ? `?${from}` : ""}`
}

function whereParts() {
  const raw = screen.getByTestId("where").textContent ?? ""
  return new URL(raw, "http://app.test")
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<FileEditPage />", () => {
  it("loads metadata and lets the user save edited values", async () => {
    const user = userEvent.setup()
    const { puts } = serveFiles([fileAttrs("f1", { title: "Old title" })])
    renderReview(reviewPath("f1", ""))

    const titleInput = await screen.findByTestId("file-edit-title")
    expect(titleInput).toHaveValue("Old title")
    await user.clear(titleInput)
    await user.type(titleInput, "New title")

    const save = screen.getByTestId("file-edit-save")
    await waitFor(() => expect(save).not.toBeDisabled())
    await user.click(save)

    // Without a list, Save returns to the file's detail panel.
    await waitFor(() => expect(whereParts().pathname).toBe(`/g/${SLUG}/files/f1`))
    expect(puts).toHaveLength(1)
    expect(puts[0].attributes.title).toBe("New title")
  })

  it("blocks save until a required field validation passes", async () => {
    const user = userEvent.setup()
    serveFiles([fileAttrs("f1", { path: "kept" })])
    renderReview(reviewPath("f1", ""))

    const path = await screen.findByTestId("file-edit-path")
    expect(path).toHaveValue("kept")
    await user.clear(path)
    await user.click(screen.getByTestId("file-edit-save"))
    expect(await screen.findByText(/filename is required|path is required/i)).toBeInTheDocument()
    expect(whereParts().pathname).toBe(`/g/${SLUG}/files/f1/edit`)
  })

  it("shows the file's place in the list it came from and links to its neighbors", async () => {
    serveFiles([
      fileAttrs("f1"),
      fileAttrs("f2"),
      fileAttrs("f3"),
      fileAttrs("f4"),
      fileAttrs("x1", { tags: [] }),
    ])
    renderReview(reviewPath("f2"))

    await screen.findByTestId("file-edit-title")
    await waitFor(() =>
      expect(screen.getByTestId("file-review-position")).toHaveTextContent("2 of 4")
    )
    expect(screen.getByTestId("file-review-previous")).toHaveAttribute("href", reviewPath("f1"))
    expect(screen.getByTestId("file-review-next")).toHaveAttribute("href", reviewPath("f3"))
  })

  it("Save and next saves, then opens the next file of the list", async () => {
    const user = userEvent.setup()
    const { puts } = serveFiles([fileAttrs("f1"), fileAttrs("f2"), fileAttrs("f3")])
    renderReview(reviewPath("f2"))

    const title = await screen.findByTestId("file-edit-title")
    await waitFor(() => expect(screen.getByTestId("file-review-save-next")).toBeEnabled())
    await user.clear(title)
    await user.type(title, "Renamed")
    await user.click(screen.getByTestId("file-review-save-next"))

    await waitFor(() => expect(whereParts().pathname).toBe(`/g/${SLUG}/files/f3/edit`))
    expect(whereParts().searchParams.get("from")).toBe(LIST)
    expect(puts).toEqual([{ id: "f2", attributes: expect.objectContaining({ title: "Renamed" }) }])
    await waitFor(() => expect(screen.getByTestId("file-edit-title")).toHaveValue("File f3"))
  })

  it("Save and next stays on the file with the entered values when the save fails", async () => {
    const user = userEvent.setup()
    serveFiles([fileAttrs("f1"), fileAttrs("f2"), fileAttrs("f3")], { failUpdate: true })
    renderReview(reviewPath("f2"))

    const title = await screen.findByTestId("file-edit-title")
    await waitFor(() => expect(screen.getByTestId("file-review-save-next")).toBeEnabled())
    await user.clear(title)
    await user.type(title, "Not saved")
    await user.click(screen.getByTestId("file-review-save-next"))

    await waitFor(() => expect(screen.getByTestId("file-review-save-next")).toBeEnabled())
    expect(whereParts().pathname).toBe(`/g/${SLUG}/files/f2/edit`)
    expect(screen.getByTestId("file-edit-title")).toHaveValue("Not saved")
  })

  it("continues with the file that followed when an edit takes this one out of the filter", async () => {
    const user = userEvent.setup()
    const { store } = serveFiles([
      fileAttrs("f1"),
      fileAttrs("f2"),
      fileAttrs("f3"),
      fileAttrs("f4"),
    ])
    renderReview(reviewPath("f2"))

    await screen.findByTestId("file-edit-title")
    await waitFor(() => expect(screen.getByTestId("file-review-save-next")).toBeEnabled())
    // Dropping the "receipt" tag takes f2 out of the ?tags=receipt list.
    await user.click(screen.getByRole("button", { name: /remove receipt/i }))
    await user.click(screen.getByTestId("file-review-save-next"))

    await waitFor(() => expect(whereParts().pathname).toBe(`/g/${SLUG}/files/f3/edit`))
    expect(store.find((f) => f.id === "f2")?.tags).toEqual([])
    // f2 is gone from the list, so f3 is now second of three.
    await waitFor(() =>
      expect(screen.getByTestId("file-review-position")).toHaveTextContent("2 of 3")
    )
    expect(screen.getByTestId("file-review-previous")).toHaveAttribute("href", reviewPath("f1"))
    expect(screen.getByTestId("file-review-next")).toHaveAttribute("href", reviewPath("f4"))
  })

  it("Cancel returns to the list as the user left it, with this file's panel open", async () => {
    const user = userEvent.setup()
    serveFiles([fileAttrs("f1"), fileAttrs("f2")])
    renderReview(reviewPath("f2"))

    await screen.findByTestId("file-edit-title")
    await user.click(screen.getByTestId("file-review-cancel"))

    await waitFor(() => expect(whereParts().pathname).toBe(`/g/${SLUG}/files/f2`))
    expect(whereParts().search).toBe("?tags=receipt&view=grid")
  })

  it("crosses into the next page and returns to the page the user ended on", async () => {
    const user = userEvent.setup()
    const rows = Array.from({ length: 26 }, (_, i) => fileAttrs(`f${i + 1}`))
    serveFiles(rows)
    const list = `/g/${SLUG}/files?tags=receipt`
    renderReview(reviewPath("f24", `from=${encodeURIComponent(list)}`))

    await screen.findByTestId("file-edit-title")
    await waitFor(() =>
      expect(screen.getByTestId("file-review-position")).toHaveTextContent("24 of 26")
    )
    await waitFor(() =>
      expect(screen.getByTestId("file-review-next")).toHaveAttribute(
        "href",
        reviewPath("f25", `from=${encodeURIComponent(`${list}&page=2`)}`)
      )
    )

    await user.click(screen.getByTestId("file-review-next"))
    await waitFor(() => expect(screen.getByTestId("file-edit-title")).toHaveValue("File f25"))
    await waitFor(() =>
      expect(screen.getByTestId("file-review-position")).toHaveTextContent("25 of 26")
    )
    expect(screen.getByTestId("file-review-previous")).toHaveAttribute(
      "href",
      reviewPath("f24", `from=${encodeURIComponent(list)}`)
    )

    await user.click(screen.getByTestId("file-review-back"))
    await waitFor(() => expect(whereParts().pathname).toBe(`/g/${SLUG}/files/f25`))
    expect(whereParts().search).toBe("?tags=receipt&page=2")
  })

  it("asks before leaving unsaved edits for another file", async () => {
    const user = userEvent.setup()
    serveFiles([fileAttrs("f1"), fileAttrs("f2"), fileAttrs("f3")])
    renderReview(reviewPath("f2"))

    const title = await screen.findByTestId("file-edit-title")
    await waitFor(() => expect(screen.getByTestId("file-review-next")).toHaveAttribute("href"))
    await user.type(title, " edited")

    await user.click(screen.getByTestId("file-review-next"))
    const dialog = await screen.findByTestId("unsaved-changes-dialog")
    await user.click(within(dialog).getByTestId("unsaved-changes-keep-editing"))
    expect(whereParts().pathname).toBe(`/g/${SLUG}/files/f2/edit`)
    expect(screen.getByTestId("file-edit-title")).toHaveValue("File f2 edited")

    await user.click(screen.getByTestId("file-review-next"))
    await user.click(await screen.findByTestId("unsaved-changes-discard"))
    await waitFor(() => expect(whereParts().pathname).toBe(`/g/${SLUG}/files/f3/edit`))
  })

  it("works from a direct link: no list controls, back goes to the file's panel", async () => {
    serveFiles([fileAttrs("f1")])
    renderReview(reviewPath("f1", ""))

    await screen.findByTestId("file-edit-title")
    expect(screen.queryByTestId("file-review-pager")).not.toBeInTheDocument()
    expect(screen.queryByTestId("file-review-save-next")).not.toBeInTheDocument()
    expect(screen.getByTestId("file-review-back")).toHaveAttribute("href", `/g/${SLUG}/files/f1`)
  })

  it("returns to the item page it was opened from", async () => {
    serveFiles([fileAttrs("f1")])
    const itemPage = `/g/${SLUG}/commodities/c1?tab=files`
    renderReview(reviewPath("f1", `from=${encodeURIComponent(itemPage)}`))

    await screen.findByTestId("file-edit-title")
    expect(screen.queryByTestId("file-review-pager")).not.toBeInTheDocument()
    expect(screen.getByTestId("file-review-back")).toHaveAttribute("href", itemPage)
  })

  it("ignores a context that points outside the group", async () => {
    serveFiles([fileAttrs("f1")])
    renderReview(reviewPath("f1", `from=${encodeURIComponent("https://evil.example/")}`))

    await screen.findByTestId("file-edit-title")
    expect(screen.getByTestId("file-review-back")).toHaveAttribute("href", `/g/${SLUG}/files/f1`)
  })

  it("keeps an unsupported file editable, with a download fallback", async () => {
    serveFiles([fileAttrs("z1", { mime_type: "application/zip", ext: ".zip", type: "other" })])
    renderReview(reviewPath("z1", ""))

    expect(await screen.findByTestId("file-detail-no-preview")).toBeInTheDocument()
    expect(screen.getByTestId("file-review-download")).toHaveAttribute("href", "/signed/z1")
    expect(screen.getByTestId("file-edit-title")).toBeEnabled()
  })

  it("switches between Preview and Details without losing entered values", async () => {
    const user = userEvent.setup()
    serveFiles([fileAttrs("p1", { mime_type: "image/png", ext: ".png", category: "images" })])
    renderReview(reviewPath("p1", ""))

    const title = await screen.findByTestId("file-edit-title")
    await user.type(title, " draft")
    await user.click(screen.getByTestId("file-review-tab-preview"))
    expect(screen.getByTestId("file-review-preview")).toHaveAttribute("data-state", "active")
    expect(screen.getByTestId("file-review-details")).toHaveAttribute("data-state", "inactive")
    // Both panels stay mounted; the inactive one is only hidden.
    expect(screen.getByTestId("file-edit-title")).toBe(title)

    await user.click(screen.getByTestId("file-review-tab-details"))
    expect(screen.getByTestId("file-edit-title")).toHaveValue("File p1 draft")
  })

  it("has no axe violations", async () => {
    serveFiles([fileAttrs("f1"), fileAttrs("f2")])
    const { container } = renderReview(reviewPath("f1"))
    await screen.findByTestId("file-edit-title")
    await waitFor(() =>
      expect(screen.getByTestId("file-review-position")).toHaveTextContent("1 of 2")
    )
    expect(await axe(container)).toHaveNoViolations()
  })
})
