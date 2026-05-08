import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { RootRedirect } from "@/pages/RootRedirect"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

const groupsPayload = {
  data: [
    { id: "g1", type: "groups", attributes: { id: "g1", slug: "household", name: "Household" } },
    { id: "g2", type: "groups", attributes: { id: "g2", slug: "office", name: "Office" } },
  ],
}

function LocationEcho() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

function renderRoot(initial = "/") {
  return renderWithProviders({
    initialPath: initial,
    routes: (
      <>
        <Route
          path="/"
          element={
            <AuthProvider>
              <GroupProvider>
                <RootRedirect />
              </GroupProvider>
            </AuthProvider>
          }
        />
        <Route
          path="/g/:groupSlug"
          element={
            <AuthProvider>
              <GroupProvider>
                <LocationEcho />
              </GroupProvider>
            </AuthProvider>
          }
        />
        <Route path="/no-group" element={<LocationEcho />} />
      </>
    ),
  })
}

describe("RootRedirect", () => {
  it("redirects to /no-group when the user has zero groups", async () => {
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "x@y.z", name: "X" })),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
  })

  it("redirects to /no-group when default_group_id is missing (invariant violation)", async () => {
    // Under the #1592 invariant the backend never returns a NULL
    // default_group_id when the user has memberships, so this state means
    // something is off — better to land on /no-group than to silently pick
    // an arbitrary group.
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "x@y.z", name: "X" })),
      msw.get(api("/groups"), () => HttpResponse.json(groupsPayload))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
  })

  it("honors user.default_group_id when the user is a member of that group", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "x@y.z", name: "X", default_group_id: "g2" })
      ),
      msw.get(api("/groups"), () => HttpResponse.json(groupsPayload))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/office")
    )
  })

  it("redirects to /no-group when default_group_id points at a group the user is not in", async () => {
    // No legacy "first group" fallback under #1592 — a stale default_group_id
    // is treated as a routing failure, not papered over with an arbitrary pick.
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "x@y.z", name: "X", default_group_id: "g999" })
      ),
      msw.get(api("/groups"), () => HttpResponse.json(groupsPayload))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
  })

  it("redirects to /no-group on a /groups error rather than a half-broken landing", async () => {
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "x@y.z", name: "X" })),
      msw.get(api("/groups"), () => HttpResponse.json({ error: "boom" }, { status: 500 }))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
  })

  it("redirects to /no-group when the default group has no usable slug (defensive)", async () => {
    // Slug is optional in the generated LocationGroup type; "/g/" with an
    // empty slug would drop into the 404. Under the #1592 invariant the
    // default group exists but if it's slug-less RootRedirect still bails
    // out cleanly rather than producing a broken URL.
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "x@y.z", name: "X", default_group_id: "g1" })
      ),
      msw.get(api("/groups"), () =>
        HttpResponse.json({
          data: [{ id: "g1", type: "groups", attributes: { id: "g1", name: "Slugless" } }],
        })
      )
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
  })
})
