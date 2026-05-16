import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { TagsListPage } from "@/pages/tags/TagsListPage"
import { commodityHandlers, groupHandlers, tagHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

function renderPage() {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/tags`,
    routes: (
      <Route
        path="/g/:groupSlug/tags"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <TagsListPage />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

// One commodity tagged with "kitchen" so the row's preview-chip path
// renders. The Tags page client-side aggregates commodities into a
// `slug → [items]` map, so we need at least one row that references the
// tag fixture below.
const commodityFixture = [
  {
    id: "c1",
    type: "commodities",
    attributes: {
      name: "Espresso machine",
      short_name: "Espresso",
      tags: ["kitchen"],
    },
  },
]

function seed() {
  server.use(
    ...groupHandlers.list(groupFixture),
    ...commodityHandlers.list(SLUG, commodityFixture),
    ...tagHandlers.stats(SLUG, {
      tags_total: 2,
      items_tagged: 5,
      items_untagged: 1,
      files_tagged: 3,
      files_untagged: 2,
    }),
    ...tagHandlers.list(SLUG, [
      {
        id: "t1",
        slug: "kitchen",
        label: "Kitchen",
        color: "amber",
        meta: { usage: { commodities: 2, files: 0 } },
      },
      {
        id: "t2",
        slug: "garden",
        label: "Garden",
        color: "green",
        meta: { usage: { commodities: 0, files: 0 } },
      },
    ])
  )
}

describe("<TagsListPage />", () => {
  it("renders the stats bar with values from /tags/stats", async () => {
    seed()
    renderPage()
    await waitFor(() => expect(screen.getByTestId("tags-stats-tags-total")).toHaveTextContent("2"))
    expect(screen.getByTestId("tags-stats-items-tagged")).toHaveTextContent("5")
    expect(screen.getByTestId("tags-stats-files-untagged")).toHaveTextContent("2")
  })

  it("renders one row per tag with the inline usage block", async () => {
    seed()
    renderPage()
    expect(await screen.findByTestId("tag-row-kitchen")).toBeVisible()
    expect(screen.getByTestId("tag-row-kitchen-usage")).toHaveTextContent("2 items")
    expect(screen.getByTestId("tag-row-garden-usage")).toHaveTextContent(/Not used yet/i)
  })

  it("renders a tag preview footer card with all tag pills", async () => {
    seed()
    renderPage()
    expect(await screen.findByTestId("tags-preview")).toBeVisible()
    expect(screen.getByTestId("tags-preview-kitchen")).toBeInTheDocument()
    expect(screen.getByTestId("tags-preview-garden")).toBeInTheDocument()
  })

  it("renders item-preview chips for tags with usage", async () => {
    seed()
    renderPage()
    // The Tags page pulls /commodities once, builds a slug→items map and
    // surfaces up to 2 names + an overflow count on each row.
    const preview = await screen.findByTestId("tag-row-kitchen-preview")
    expect(preview).toHaveTextContent("Espresso")
  })

  it("opens the create dialog from the CTA", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("tag-row-kitchen")
    await user.click(screen.getByTestId("tags-create-button"))
    expect(await screen.findByTestId("tag-form-dialog")).toBeVisible()
  })

  it("expands the inline create row when the toggle is clicked", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("tag-row-kitchen")
    await user.click(screen.getByTestId("tags-inline-create-toggle"))
    expect(await screen.findByTestId("tags-inline-create")).toBeVisible()
    expect(screen.getByTestId("tags-inline-create-label")).toBeVisible()
    expect(screen.getByTestId("tags-inline-create-save")).toBeVisible()
  })

  it("clears the search input when the clear button is clicked", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("tag-row-kitchen")
    await user.type(screen.getByTestId("tags-search-input") as HTMLInputElement, "kitchen")
    // The clear button only mounts when the input has a value — its
    // appearance proves the conditional render path; clicking it
    // exercises the setPendingSearch("") branch.
    const clear = await screen.findByTestId("tags-search-clear")
    await user.click(clear)
    // Re-query the input after the click — the URL-debounce effect can
    // cause the Tabs subtree to re-render, which can detach the
    // pre-click element reference even though the visible DOM still
    // shows the new state.
    await waitFor(() => {
      const fresh = screen.getByTestId("tags-search-input") as HTMLInputElement
      expect(fresh.value).toBe("")
    })
  })

  it("rewrites the sort field/order when the sort dropdown changes", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("tag-row-kitchen")
    const select = screen.getByTestId("tags-sort") as HTMLSelectElement
    expect(select.value).toBe("label.asc")
    await user.selectOptions(select, "usage.desc")
    // The select's value tracks `${urlSort}.${urlOrder}`; after
    // `patchSort` writes to the URL via `useSearchParams`, the
    // component re-renders with the new value.
    await waitFor(() => expect(select.value).toBe("usage.desc"))
  })

  it("submits a new tag through the inline create row", async () => {
    seed()
    server.use(
      ...tagHandlers.create(SLUG, {
        id: "t3",
        slug: "office",
        label: "Office",
        color: "blue",
      })
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("tag-row-kitchen")
    await user.click(screen.getByTestId("tags-inline-create-toggle"))
    const labelInput = await screen.findByTestId("tags-inline-create-label")
    await user.type(labelInput, "Office")
    await user.click(screen.getByTestId("tags-inline-create-save"))
    // POST /tags landed when the input is cleared and the picker resets
    // to the default colour — both happen inside the success branch of
    // onInlineCreate.
    await waitFor(() => {
      expect((labelInput as HTMLInputElement).value).toBe("")
    })
  })

  it("renders the error block when /tags returns 500", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...commodityHandlers.list(SLUG, []),
      ...tagHandlers.stats(SLUG, {
        tags_total: 0,
        items_tagged: 0,
        items_untagged: 0,
        files_tagged: 0,
        files_untagged: 0,
      }),
      // Override the list factory with a 500 so `tagsQuery.isError`
      // flips and the destructive alert renders in place of the list.
      http.get(`${window.location.origin}/api/v1/g/${SLUG}/tags`, () =>
        HttpResponse.json({ error: "boom" }, { status: 500 })
      )
    )
    renderPage()
    expect(await screen.findByTestId("tags-list-error")).toBeVisible()
  })

  it("is axe-clean once data has loaded", async () => {
    seed()
    const { baseElement } = renderPage()
    await screen.findByTestId("tag-row-kitchen")
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
