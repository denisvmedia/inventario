import { describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { http as msw, HttpResponse } from "msw"
import { screen } from "@testing-library/react"

import { CurrencyMigrationBanner } from "@/components/CurrencyMigrationBanner"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"

function envelope(g: { id: string; slug: string; name: string; currency_migration_id?: string }) {
  return {
    data: [
      {
        id: g.id,
        type: "groups",
        attributes: g,
      },
    ],
  }
}

describe("<CurrencyMigrationBanner />", () => {
  it("renders nothing outside a GroupProvider", () => {
    renderWithProviders({ children: <CurrencyMigrationBanner /> })
    expect(screen.queryByTestId("currency-migration-banner")).toBeNull()
  })

  it("renders nothing when the active group has no currency_migration_id", async () => {
    server.use(
      msw.get(apiUrl("/groups"), () =>
        HttpResponse.json(envelope({ id: "g1", slug: "household", name: "Household" }))
      )
    )
    renderWithProviders({
      initialPath: "/g/household",
      routes: (
        <Route
          path="/g/:groupSlug"
          element={
            <GroupProvider>
              <CurrencyMigrationBanner />
            </GroupProvider>
          }
        />
      ),
    })
    // Banner stays hidden even after the groups query resolves.
    expect(screen.queryByTestId("currency-migration-banner")).toBeNull()
  })

  it("shows the banner with the group name when currency_migration_id is set", async () => {
    server.use(
      msw.get(apiUrl("/groups"), () =>
        HttpResponse.json(
          envelope({
            id: "g1",
            slug: "household",
            name: "Household",
            currency_migration_id: "mig-1",
          })
        )
      )
    )
    renderWithProviders({
      initialPath: "/g/household",
      routes: (
        <Route
          path="/g/:groupSlug"
          element={
            <GroupProvider>
              <CurrencyMigrationBanner />
            </GroupProvider>
          }
        />
      ),
    })
    const banner = await screen.findByTestId("currency-migration-banner")
    expect(banner).toHaveTextContent("Household")
  })
})
