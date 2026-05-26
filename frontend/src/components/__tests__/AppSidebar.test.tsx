import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { AppSidebar } from "@/components/AppSidebar"
import { SidebarProvider } from "@/components/ui/sidebar"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { apiUrl, authHandlers, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

// Mount AppSidebar inside the same provider chain Shell uses at runtime
// (AuthProvider -> GroupProvider -> SidebarProvider) so the test sees the
// same currentGroup / groups wiring the production sidebar does.
function renderSidebar(initialPath = "/no-group") {
  setAccessToken("good-token")
  // The current cases only need the root /g/:slug shape — if a future
  // test passes /g/<slug>/<sub>, this mount will still resolve only
  // /g/:slug, won't match, and AppSidebar won't render. Widen the
  // pattern then (e.g. /g/:groupSlug/*).
  const routePath = initialPath.startsWith("/g/") ? "/g/:groupSlug" : initialPath
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path={routePath}
        element={
          <AuthProvider>
            <GroupProvider>
              <SidebarProvider>
                <AppSidebar />
              </SidebarProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<AppSidebar /> — group-section gating (#1886)", () => {
  it("hides Inventory + Group sections when the user belongs to no groups", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.empty())
    renderSidebar("/no-group")
    // Personal always renders unconditionally. The Inventory + Group
    // sections render during the in-flight window (groups === undefined)
    // and only retract once /groups resolves []. Wait for the retract
    // explicitly so the test isn't reading the loading-state snapshot.
    await waitFor(() =>
      expect(screen.queryByTestId("sidebar-inventory-group")).not.toBeInTheDocument()
    )
    expect(screen.queryByTestId("sidebar-manage-group")).not.toBeInTheDocument()
    expect(screen.getByText("Personal")).toBeInTheDocument()
  })

  it("renders all three sections when the user has at least one group", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    renderSidebar("/g/household")
    await waitFor(() => expect(screen.getByTestId("sidebar-inventory-group")).toBeInTheDocument())
    expect(screen.getByTestId("sidebar-manage-group")).toBeInTheDocument()
    expect(screen.getByText("Personal")).toBeInTheDocument()
  })

  it("keeps the group sections rendered while /groups is still loading", async () => {
    // Hang the /groups response so we observe the in-flight state and
    // verify we don't flash the sections off and back on. A never-
    // resolving handler keeps `groups === undefined` for the entirety
    // of the test, which is exactly the loading branch the gate covers.
    server.use(
      ...authHandlers.signedIn(),
      http.get(apiUrl("/groups"), () => new Promise<HttpResponse>(() => {}))
    )
    renderSidebar("/no-group")
    // Personal renders unconditionally — using it as the readiness probe
    // so the assertion isn't racing the first render.
    await waitFor(() => expect(screen.getByText("Personal")).toBeInTheDocument())
    // groups hasn't resolved, so the section headers must still be in
    // the DOM. If they weren't, the user would see "no sections" briefly
    // and then "sections appear" once /groups lands.
    expect(screen.getByTestId("sidebar-inventory-group")).toBeInTheDocument()
    expect(screen.getByTestId("sidebar-manage-group")).toBeInTheDocument()
  })
})
