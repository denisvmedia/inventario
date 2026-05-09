import { describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { http as msw, HttpResponse } from "msw"
import { screen } from "@testing-library/react"

import { GroupProvider } from "@/features/group/GroupContext"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"

function Probe() {
  const lock = useGroupMigrationLock()
  return (
    <div
      data-testid="lock-probe"
      data-locked={lock.locked ? "true" : "false"}
      data-id={lock.migrationId ?? ""}
    />
  )
}

function groupsEnvelope(attributes: Record<string, unknown>) {
  return { data: [{ id: "g1", type: "groups", attributes: { id: "g1", ...attributes } }] }
}

describe("useGroupMigrationLock", () => {
  it("returns locked: false when the active group has no currency_migration_id", async () => {
    server.use(
      msw.get(apiUrl("/groups"), () =>
        HttpResponse.json(groupsEnvelope({ slug: "household", name: "Household" }))
      )
    )
    renderWithProviders({
      initialPath: "/g/household",
      routes: (
        <Route
          path="/g/:groupSlug"
          element={
            <GroupProvider>
              <Probe />
            </GroupProvider>
          }
        />
      ),
    })
    const probe = await screen.findByTestId("lock-probe")
    expect(probe.getAttribute("data-locked")).toBe("false")
    expect(probe.getAttribute("data-id")).toBe("")
  })

  it("returns locked: true with the migration id when set on the active group", async () => {
    server.use(
      msw.get(apiUrl("/groups"), () =>
        HttpResponse.json(
          groupsEnvelope({
            slug: "household",
            name: "Household",
            currency_migration_id: "mig-42",
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
              <Probe />
            </GroupProvider>
          }
        />
      ),
    })
    const probe = await screen.findByTestId("lock-probe")
    // First render before the groups query resolves, lock is false.
    // After the query lands the GroupContext exposes the new group
    // and the probe re-renders with `locked: true`.
    await screen.findByText((_text, el) => el?.getAttribute("data-locked") === "true")
    expect(probe.getAttribute("data-id")).toBe("mig-42")
  })

  it("returns locked: false when used outside a GroupProvider", () => {
    renderWithProviders({ children: <Probe /> })
    const probe = screen.getByTestId("lock-probe")
    expect(probe.getAttribute("data-locked")).toBe("false")
  })
})
