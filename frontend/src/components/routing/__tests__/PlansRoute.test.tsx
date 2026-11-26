import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { PlansRoute } from "@/components/routing/PlansRoute"
import { ProtectedRoute } from "@/components/routing/ProtectedRoute"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { featureFlagHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe({ testId = "loc" }: { testId?: string }) {
  const loc = useLocation()
  return <div data-testid={testId} data-pathname={loc.pathname} data-search={loc.search} />
}

// Mounted the way the router mounts /plans: behind ProtectedRoute, with
// GroupProvider inside it.
function renderAt(path: string) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: path,
    routes: (
      <>
        <Route
          path="/plans"
          element={
            <AuthProvider>
              <ProtectedRoute>
                <GroupProvider>
                  <PlansRoute />
                  <LocationProbe testId="plans-loc" />
                </GroupProvider>
              </ProtectedRoute>
            </AuthProvider>
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

const userAndGroups = [
  msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "alex@example.com" })),
  msw.get(api("/groups"), () =>
    HttpResponse.json({
      data: [
        {
          id: "g1",
          type: "groups",
          attributes: { id: "g1", slug: "household", name: "Household" },
        },
        {
          id: "g2",
          type: "groups",
          attributes: { id: "g2", slug: "office", name: "Office" },
        },
      ],
    })
  ),
]

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<PlansRoute /> (#1389)", () => {
  it("redirects to the home of the group named by ?g= when plans are off", async () => {
    server.use(...featureFlagHandlers.set({ plans: false }), ...userAndGroups)
    renderAt("/plans?g=office")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/office")
    )
  })

  it("redirects to / without a group hint when plans are off", async () => {
    server.use(...featureFlagHandlers.set({ plans: false }), ...userAndGroups)
    renderAt("/plans")
    await waitFor(() => expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/"))
    // GroupProvider does not pin ?g= on /plans while plans are off, so it
    // cannot pull the user back onto the page it is leaving.
    expect(screen.getByTestId("loc").getAttribute("data-search")).toBe("")
  })

  it("renders the coming-soon page and pins the group when plans are on", async () => {
    server.use(...featureFlagHandlers.set({ plans: true }), ...userAndGroups)
    renderAt("/plans")
    expect(await screen.findByTestId("coming-soon-page-plans")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId("plans-loc").getAttribute("data-search")).toBe("?g=household")
    )
    expect(screen.queryByTestId("loc")).not.toBeInTheDocument()
  })
})
