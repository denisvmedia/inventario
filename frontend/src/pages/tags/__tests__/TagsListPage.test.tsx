import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { TagsListPage } from "@/pages/tags/TagsListPage"
import { groupHandlers, tagHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
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

function seed() {
  server.use(
    ...groupHandlers.list(groupFixture),
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

  it("opens the create dialog from the CTA", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("tag-row-kitchen")
    await user.click(screen.getByTestId("tags-create-button"))
    expect(await screen.findByTestId("tag-form-dialog")).toBeVisible()
  })

  it("is axe-clean once data has loaded", async () => {
    seed()
    const { baseElement } = renderPage()
    await screen.findByTestId("tag-row-kitchen")
    const results = await axe(baseElement)
    expect(results).toHaveNoViolations()
  })
})
