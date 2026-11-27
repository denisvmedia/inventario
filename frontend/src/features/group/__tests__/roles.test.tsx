import type { ReactNode } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"

import { GroupProvider } from "@/features/group/GroupContext"
import { canEditContent, canManageLocations, useGroupPermissions } from "@/features/group/roles"
import type { GroupRole } from "@/features/group/api"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl, groupHandlers, memberHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

const SLUG = "household"

function Permissions() {
  const { role, canEditContent, canManageLocations } = useGroupPermissions()
  return (
    <output data-testid="permissions">
      {`${role ?? "unknown"} content:${canEditContent} locations:${canManageLocations}`}
    </output>
  )
}

function renderPermissions(children: ReactNode = <Permissions />) {
  setAccessToken("good-token")
  return renderWithProviders({
    withAuth: true,
    initialPath: `/g/${SLUG}/items`,
    routes: (
      <Route path="/g/:groupSlug/items" element={<GroupProvider>{children}</GroupProvider>} />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }])
  )
})

describe("group role permissions (#2886)", () => {
  // The server's write gates: content takes users and up, locations and
  // areas take admins and owners (go/apiserver/apiserver.go).
  it.each<[GroupRole | null, boolean, boolean]>([
    ["owner", true, true],
    ["admin", true, true],
    ["user", true, false],
    ["viewer", false, false],
    [null, false, false],
  ])("%s may edit content: %s, manage locations: %s", (role, content, locations) => {
    expect(canEditContent(role)).toBe(content)
    expect(canManageLocations(role)).toBe(locations)
  })

  it("reads the caller's role from the group's member list", async () => {
    server.use(...memberHandlers.asRole("g1", "user"))
    renderPermissions()
    expect(await screen.findByText("user content:true locations:false")).toBeInTheDocument()
  })

  it("allows nothing when the member list cannot be read", async () => {
    let answered = false
    server.use(...memberHandlers.asRole("g1", "owner"))
    // Registered after asRole, so it answers the member list instead.
    server.use(
      http.get(apiUrl("/groups/g1/members"), () => {
        answered = true
        return HttpResponse.json({}, { status: 500 })
      })
    )
    renderPermissions()
    await waitFor(() => expect(answered).toBe(true))
    expect(await screen.findByTestId("permissions")).toHaveTextContent(
      "unknown content:false locations:false"
    )
  })
})
