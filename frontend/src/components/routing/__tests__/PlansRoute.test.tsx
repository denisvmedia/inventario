import { beforeEach, describe, expect, it } from "vitest"
import { delay, http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { PlansRoute } from "@/components/routing/PlansRoute"
import { ProtectedRoute } from "@/components/routing/ProtectedRoute"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { FEATURE_FLAGS_FALLBACK } from "@/features/feature-flags/api"
import { apiUrl, featureFlagHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

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

const me = msw.get(apiUrl("/auth/me"), () =>
  HttpResponse.json({ id: "u1", email: "alex@example.com" })
)

const groupsPayload = {
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
}

const userAndGroups = [me, msw.get(apiUrl("/groups"), () => HttpResponse.json(groupsPayload))]

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

  // The two halves of the loading guard. Each test delays one request so
  // the route renders while it is in flight; without that half of the
  // guard the route would redirect on what it has so far.
  it("stays on /plans while slow flags load when plans are on", async () => {
    server.use(
      ...userAndGroups,
      msw.get(apiUrl("/feature-flags"), async () => {
        await delay(300)
        return HttpResponse.json({ ...FEATURE_FLAGS_FALLBACK, plans: true })
      })
    )
    renderAt("/plans")
    expect(await screen.findByTestId("coming-soon-page-plans")).toBeInTheDocument()
    // A redirect would have unmounted /plans for good: the catch-all
    // route's probe would be on screen and the page would not be.
    expect(screen.queryByTestId("loc")).not.toBeInTheDocument()
  })

  it("redirects to the ?g= group when the groups load slowly and plans are off", async () => {
    server.use(
      ...featureFlagHandlers.set({ plans: false }),
      me,
      msw.get(apiUrl("/groups"), async () => {
        await delay(300)
        return HttpResponse.json(groupsPayload)
      })
    )
    renderAt("/plans?g=office")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/office")
    )
  })
})
