import { screen } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { GroupProvider } from "@/features/group/GroupContext"
import { StorageCard } from "@/features/storage/StorageCard"
import type { StorageUsage } from "@/features/storage/api"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function usage(trash: number): StorageUsage {
  return {
    used_bytes: 3072 + trash,
    quota_bytes: null,
    breakdown: { images: 1024, documents: 1024, other: 1024, exports: 0, trash },
  }
}

function renderCard(body: StorageUsage) {
  server.use(
    ...groupHandlers.list(groupFixture),
    http.get(apiUrl(`/g/${SLUG}/storage-usage`), () => HttpResponse.json(body))
  )
  return renderWithProviders({
    initialPath: `/g/${SLUG}/settings`,
    routes: (
      <Route
        path="/g/:groupSlug/settings"
        element={
          <GroupProvider>
            <StorageCard />
          </GroupProvider>
        }
      />
    ),
  })
}

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

describe("<StorageCard /> trash share", () => {
  it("shows the trash slice and a link to the trash page", async () => {
    renderCard(usage(2048))
    const note = await screen.findByTestId("storage-card-trash")
    expect(note).toHaveTextContent(/2.00 KiB of this is in the trash/)
    expect(screen.getByTestId("storage-card-open-trash").getAttribute("href")).toBe(
      `/g/${SLUG}/trash`
    )
    expect(screen.getByTestId("storage-pie-trash")).toBeInTheDocument()
  })

  it("leaves the note out when the trash is empty", async () => {
    renderCard(usage(0))
    await screen.findByTestId("storage-card-manage-files")
    expect(await screen.findByTestId("storage-pie")).toBeInTheDocument()
    expect(screen.queryByTestId("storage-card-trash")).toBeNull()
  })
})
